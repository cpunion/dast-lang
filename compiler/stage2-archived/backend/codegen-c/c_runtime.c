// Dast C Runtime Implementation
#include "c_runtime.h"
#include <dirent.h>
#include <errno.h>
#include <spawn.h>
#include <sys/stat.h>
#include <sys/wait.h>

// Global state
static int g_argc = 0;
static char **g_argv = NULL;
static const char **g_string_allocs = NULL;
static size_t g_string_allocs_len = 0;
static size_t g_string_allocs_cap = 0;
static void **g_array_allocs = NULL;
static size_t g_array_allocs_len = 0;
static size_t g_array_allocs_cap = 0;
static void **g_struct_allocs = NULL;
static size_t g_struct_allocs_len = 0;
static size_t g_struct_allocs_cap = 0;
static size_t g_mem_used = 0;
static size_t g_mem_limit = 0;

static void dast_rt_oom(void) {
  fprintf(stderr, "out of memory\n");
  exit(1);
}

void dast_set_mem_limit(size_t bytes) { g_mem_limit = bytes; }

static void *dast_rt_alloc_raw(size_t n) {
  size_t total = n + sizeof(size_t);
  if (g_mem_limit && g_mem_used + total > g_mem_limit) {
    dast_rt_oom();
  }
  void *raw = malloc(total);
  if (!raw && total != 0) {
    dast_rt_oom();
  }
  if (raw) {
    *(size_t *)raw = total;
    g_mem_used += total;
  }
  return raw;
}

static void *dast_rt_realloc_raw(void *raw, size_t n) {
  size_t old = raw ? *(size_t *)raw : 0;
  size_t total = n + sizeof(size_t);
  if (g_mem_limit && g_mem_used - old + total > g_mem_limit) {
    dast_rt_oom();
  }
  void *next = realloc(raw, total);
  if (!next && total != 0) {
    dast_rt_oom();
  }
  if (next) {
    *(size_t *)next = total;
    g_mem_used = g_mem_used - old + total;
  }
  return next;
}

void *dast_rt_malloc(size_t n) {
  void *raw = dast_rt_alloc_raw(n == 0 ? 1 : n);
  return raw ? (char *)raw + sizeof(size_t) : NULL;
}

void *dast_rt_realloc(void *p, size_t n) {
  if (!p) {
    return dast_rt_malloc(n);
  }
  void *raw = (char *)p - sizeof(size_t);
  void *next = dast_rt_realloc_raw(raw, n == 0 ? 1 : n);
  return next ? (char *)next + sizeof(size_t) : NULL;
}

void dast_rt_free(void *p) {
  if (!p) return;
  void *raw = (char *)p - sizeof(size_t);
  size_t total = *(size_t *)raw;
  if (g_mem_used >= total) {
    g_mem_used -= total;
  } else {
    g_mem_used = 0;
  }
  free(raw);
}

char *dast_rt_strdup(const char *s) {
  if (!s) return NULL;
  size_t n = strlen(s);
  char *out = (char *)dast_rt_malloc(n + 1);
  if (!out) return NULL;
  memcpy(out, s, n);
  out[n] = '\0';
  return out;
}

static void dast_track_string(const char *s) {
  if (!s) return;
  for (size_t i = 0; i < g_string_allocs_len; i++) {
    if (g_string_allocs[i] == s) return;
  }
  if (g_string_allocs_len >= g_string_allocs_cap) {
    size_t next = g_string_allocs_cap == 0 ? 64 : g_string_allocs_cap * 2;
    g_string_allocs = (const char **)dast_rt_realloc(g_string_allocs, sizeof(char *) * next);
    g_string_allocs_cap = next;
  }
  g_string_allocs[g_string_allocs_len++] = s;
}

static int dast_untrack_string(const char *s) {
  if (!s) return 0;
  for (size_t i = 0; i < g_string_allocs_len; i++) {
    if (g_string_allocs[i] == s) {
      g_string_allocs[i] = g_string_allocs[g_string_allocs_len - 1];
      g_string_allocs_len--;
      return 1;
    }
  }
  return 0;
}

static void dast_track_array(void *p) {
  if (!p) return;
  for (size_t i = 0; i < g_array_allocs_len; i++) {
    if (g_array_allocs[i] == p) return;
  }
  if (g_array_allocs_len >= g_array_allocs_cap) {
    size_t next = g_array_allocs_cap == 0 ? 64 : g_array_allocs_cap * 2;
    g_array_allocs = (void **)dast_rt_realloc(g_array_allocs, sizeof(void *) * next);
    g_array_allocs_cap = next;
  }
  g_array_allocs[g_array_allocs_len++] = p;
}

static int dast_untrack_array(void *p) {
  if (!p) return 0;
  for (size_t i = 0; i < g_array_allocs_len; i++) {
    if (g_array_allocs[i] == p) {
      g_array_allocs[i] = g_array_allocs[g_array_allocs_len - 1];
      g_array_allocs_len--;
      return 1;
    }
  }
  return 0;
}

static void dast_track_struct(void *p) {
  if (!p) return;
  for (size_t i = 0; i < g_struct_allocs_len; i++) {
    if (g_struct_allocs[i] == p) return;
  }
  if (g_struct_allocs_len >= g_struct_allocs_cap) {
    size_t next = g_struct_allocs_cap == 0 ? 64 : g_struct_allocs_cap * 2;
    g_struct_allocs = (void **)dast_rt_realloc(g_struct_allocs, sizeof(void *) * next);
    g_struct_allocs_cap = next;
  }
  g_struct_allocs[g_struct_allocs_len++] = p;
}

static int dast_untrack_struct(void *p) {
  if (!p) return 0;
  for (size_t i = 0; i < g_struct_allocs_len; i++) {
    if (g_struct_allocs[i] == p) {
      g_struct_allocs[i] = g_struct_allocs[g_struct_allocs_len - 1];
      g_struct_allocs_len--;
      return 1;
    }
  }
  return 0;
}

// Runtime initialization
void dast_runtime_init(int argc, char **argv) {
  g_argc = argc;
  g_argv = argv;
  const char *limit_bytes = getenv("DAST_MEM_LIMIT_BYTES");
  const char *limit_mb = getenv("DAST_MEM_LIMIT_MB");
  if (limit_bytes && *limit_bytes) {
    size_t n = (size_t)strtoull(limit_bytes, NULL, 10);
    if (n > 0) {
      g_mem_limit = n;
      return;
    }
  }
  if (limit_mb && *limit_mb) {
    size_t n = (size_t)strtoull(limit_mb, NULL, 10);
    if (n > 0) {
      g_mem_limit = n * 1024 * 1024;
      return;
    }
  }
}

void dast_runtime_cleanup(void) {
  // Nothing to clean up for now
}

// String operations
dast_string_t dast_string_from_literal(const char *s) {
  return dast_string_from_cstr(s);
}

dast_string_t dast_string_from_cstr(const char *s) {
  dast_string_t str;
  str.len = strlen(s);
  str.cap = str.len + 1;
  str.data = (char *)dast_rt_malloc(str.cap);
  dast_track_string(str.data);
  memcpy(str.data, s, str.len);
  str.data[str.len] = '\0';
  return str;
}

dast_int dast_string_len(dast_string_t s) { return (dast_int)s.len; }

dast_string_t dast_string_concat(dast_string_t a, dast_string_t b) {
  dast_string_t out;
  out.len = a.len + b.len;
  out.cap = out.len + 1;
  out.data = (char *)dast_rt_malloc(out.cap);
  dast_track_string(out.data);
  memcpy(out.data, a.data, a.len);
  memcpy(out.data + a.len, b.data, b.len);
  out.data[out.len] = '\0';
  return out;
}

dast_string_t dast_string_clone(dast_string_t s) {
  dast_string_t out;
  if (!s.data || s.len == 0) {
    out.len = 0;
    out.cap = 1;
    out.data = (char *)dast_rt_malloc(out.cap);
    dast_track_string(out.data);
    out.data[0] = '\0';
    return out;
  }
  out.len = s.len;
  out.cap = s.len + 1;
  out.data = (char *)dast_rt_malloc(out.cap);
  dast_track_string(out.data);
  memcpy(out.data, s.data, s.len);
  out.data[s.len] = '\0';
  return out;
}

bool dast_string_eq(dast_string_t a, dast_string_t b) {
  if (a.len != b.len) return false;
  return memcmp(a.data, b.data, a.len) == 0;
}

bool dast_string_eq_cstr(dast_string_t a, const char *b) {
  size_t blen = strlen(b);
  if (a.len != blen) return false;
  return memcmp(a.data, b, a.len) == 0;
}

dast_int dast_string_char_at(dast_string_t s, dast_int i) {
  if (i < 0 || (size_t)i >= s.len) {
    fprintf(stderr, "string index out of bounds\n");
    exit(1);
  }
  return (dast_int)(unsigned char)s.data[i];
}

dast_string_t dast_string_substr(dast_string_t s, dast_int start,
                                 dast_int length) {
  dast_string_t result;
  if (start < 0)
    start = 0;
  if ((size_t)start >= s.len) {
    result.data = (char *)dast_rt_malloc(1);
    dast_track_string(result.data);
    result.data[0] = '\0';
    result.len = 0;
    result.cap = 1;
    return result;
  }
  if ((size_t)(start + length) > s.len) {
    length = s.len - start;
  }
  result.len = length;
  result.cap = length + 1;
  result.data = (char *)dast_rt_malloc(result.cap);
  dast_track_string(result.data);
  memcpy(result.data, s.data + start, length);
  result.data[length] = '\0';
  return result;
}

void dast_string_free(dast_string_t s) {
  if (!s.data) return;
  if (!dast_untrack_string(s.data)) return;
  dast_rt_free(s.data);
}

// Array operations
dast_array_t dast_array_new(size_t elem_size) {
  dast_array_t arr;
  arr.data = NULL;
  arr.len = 0;
  arr.cap = 0;
  arr.elem_size = elem_size;
  return arr;
}

dast_int dast_array_len(dast_array_t *arr) { return (dast_int)arr->len; }

void dast_array_push(dast_array_t *arr, void *elem) {
  if (arr->len >= arr->cap) {
    size_t new_cap = arr->cap == 0 ? 4 : arr->cap * 2;
    void *old = arr->data;
    arr->data = dast_rt_realloc(arr->data, new_cap * arr->elem_size);
    arr->cap = new_cap;
    if (arr->data != old) {
      if (old) {
        (void)dast_untrack_array(old);
      }
      if (arr->data) {
        dast_track_array(arr->data);
      }
    } else if (!old && arr->data) {
      dast_track_array(arr->data);
    }
  }
  memcpy((char *)arr->data + arr->len * arr->elem_size, elem, arr->elem_size);
  arr->len++;
}

void dast_array_pop(dast_array_t *arr, void *out) {
  if (arr->len == 0) {
    fprintf(stderr, "pop from empty array\n");
    exit(1);
  }
  size_t idx = arr->len - 1;
  memcpy(out, (char *)arr->data + idx * arr->elem_size, arr->elem_size);
  arr->len--;
}

void *dast_array_get(dast_array_t *arr, dast_int i) {
  if (i < 0 || (size_t)i >= arr->len) {
    fprintf(stderr, "array index out of bounds\n");
    exit(1);
  }
  return (char *)arr->data + i * arr->elem_size;
}

void dast_array_set(dast_array_t *arr, dast_int i, void *elem) {
  if (i < 0 || (size_t)i >= arr->len) {
    fprintf(stderr, "array index out of bounds\n");
    exit(1);
  }
  memcpy((char *)arr->data + i * arr->elem_size, elem, arr->elem_size);
}

void dast_array_free(dast_array_t arr) {
  if (!arr.data) return;
  if (!dast_untrack_array(arr.data)) return;
  dast_rt_free(arr.data);
}

// I/O operations
void dast_print(dast_string_t s) { fwrite(s.data, 1, s.len, stdout); }

void dast_print_cstr(const char *s) { fputs(s, stdout); }

void dast_println(dast_string_t s) {
  fwrite(s.data, 1, s.len, stdout);
  putchar('\n');
}

void dast_print_int(dast_int n) { printf("%lld", (long long)n); }

void dast_print_bool(bool b) { fputs(b ? "true" : "false", stdout); }

void dast_print_newline(void) { putchar('\n'); }

void dast_eprint(dast_string_t s) { fwrite(s.data, 1, s.len, stderr); }

void dast_eprint_cstr(const char *s) { fputs(s, stderr); }

void dast_eprintln(dast_string_t s) {
  fwrite(s.data, 1, s.len, stderr);
  fputc('\n', stderr);
}

void dast_eprint_int(dast_int n) { fprintf(stderr, "%lld", (long long)n); }

void dast_eprint_bool(bool b) { fputs(b ? "true" : "false", stderr); }

void dast_eprint_newline(void) { fputc('\n', stderr); }

void dast_eprint_array(dast_array_t *arr) {
  fprintf(stderr, "[len=%zu]", arr->len);
}

void dast_print_array(dast_array_t *arr) {
  printf("[len=%zu]", arr->len);
}

void dast_print_struct(dast_struct_t *st) {
  if (st && st->name) {
    printf("%s{...}", st->name);
  } else {
    printf("struct{...}");
  }
}

void dast_println_int(dast_string_t label, dast_int n) {
  fwrite(label.data, 1, label.len, stdout);
  printf(" %lld\n", (long long)n);
}

dast_string_t dast_int_to_string(dast_int n) {
  char buf[32];
  int len = snprintf(buf, sizeof(buf), "%lld", (long long)n);
  dast_string_t result;
  result.len = len;
  result.cap = len + 1;
  result.data = (char *)dast_rt_malloc(result.cap);
  dast_track_string(result.data);
  memcpy(result.data, buf, len);
  result.data[len] = '\0';
  return result;
}

void dast_panic(dast_string_t msg) {
  fprintf(stderr, "panic: ");
  fwrite(msg.data, 1, msg.len, stderr);
  fprintf(stderr, "\n");
  exit(1);
}

// =============================================================================
// Process execution
// =============================================================================

extern char **environ;

static char *dast_string_to_cstr(dast_string_t s) {
  char *buf = (char *)dast_rt_malloc(s.len + 1);
  memcpy(buf, s.data, s.len);
  buf[s.len] = '\0';
  return buf;
}

int64_t dast_exec(dast_string_t cmd, dast_array_t args) {
  size_t argc = args.len;
  char **argv = (char **)dast_rt_malloc(sizeof(char *) * (argc + 2));
  argv[0] = dast_string_to_cstr(cmd);
  for (size_t i = 0; i < argc; i++) {
    dast_string_t *s = (dast_string_t *)dast_array_get(&args, (dast_int)i);
    argv[i + 1] = dast_string_to_cstr(*s);
  }
  argv[argc + 1] = NULL;

  pid_t pid = 0;
  int status = 0;
  int rc = posix_spawnp(&pid, argv[0], NULL, NULL, argv, environ);
  if (rc != 0) {
    fprintf(stderr, "exec failed: %s\n", strerror(rc));
    for (size_t i = 0; i < argc + 1; i++) {
      dast_rt_free(argv[i]);
    }
    dast_rt_free(argv);
    return 127;
  }
  if (waitpid(pid, &status, 0) < 0) {
    fprintf(stderr, "exec wait failed\n");
    for (size_t i = 0; i < argc + 1; i++) {
      dast_rt_free(argv[i]);
    }
    dast_rt_free(argv);
    return 127;
  }
  for (size_t i = 0; i < argc + 1; i++) {
    dast_rt_free(argv[i]);
  }
  dast_rt_free(argv);

  if (WIFEXITED(status)) {
    return (int64_t)WEXITSTATUS(status);
  }
  if (WIFSIGNALED(status)) {
    return (int64_t)(128 + WTERMSIG(status));
  }
  return 127;
}

// =============================================================================
// Struct operations
// =============================================================================

static int dast_struct_find(dast_struct_t *st, const char *field) {
  for (size_t i = 0; i < st->len; i++) {
    if (st->field_names[i] && strcmp(st->field_names[i], field) == 0) {
      return (int)i;
    }
  }
  return -1;
}

static int dast_struct_insert(dast_struct_t *st, const char *field) {
  for (size_t i = 0; i < st->len; i++) {
    if (st->field_names[i] == NULL) {
      st->field_names[i] = field;
      return (int)i;
    }
  }
  return -1;
}

dast_struct_t *dast_struct_new(const char *name, size_t field_count) {
  dast_struct_t *st = (dast_struct_t *)dast_rt_malloc(sizeof(dast_struct_t));
  if (!st) return NULL;
  memset(st, 0, sizeof(dast_struct_t));
  st->name = name;
  st->len = field_count;
  st->field_names = (const char **)dast_rt_malloc(field_count * sizeof(char *));
  st->field_values = (dast_value_t *)dast_rt_malloc(field_count * sizeof(dast_value_t));
  if (st->field_names) {
    memset((void *)st->field_names, 0, field_count * sizeof(char *));
  }
  if (st->field_values) {
    memset(st->field_values, 0, field_count * sizeof(dast_value_t));
  }
  return st;
}

static int dast_struct_slot(dast_struct_t *st, const char *field) {
  int idx = dast_struct_find(st, field);
  if (idx >= 0) return idx;
  idx = dast_struct_insert(st, field);
  if (idx < 0) {
    fprintf(stderr, "struct field '%s' not found\n", field);
    exit(1);
  }
  return idx;
}

void dast_struct_set_int(dast_struct_t *st, const char *field, dast_int v) {
  int idx = dast_struct_slot(st, field);
  st->field_values[idx].i = v;
}

void dast_struct_set_bool(dast_struct_t *st, const char *field, bool v) {
  int idx = dast_struct_slot(st, field);
  st->field_values[idx].b = v;
}

void dast_struct_set_string(dast_struct_t *st, const char *field, dast_string_t v) {
  int idx = dast_struct_slot(st, field);
  st->field_values[idx].s = v;
}

void dast_struct_set_array(dast_struct_t *st, const char *field, dast_array_t v) {
  int idx = dast_struct_slot(st, field);
  st->field_values[idx].a = v;
}

void dast_struct_set_struct(dast_struct_t *st, const char *field, dast_struct_t *v) {
  int idx = dast_struct_slot(st, field);
  st->field_values[idx].st = v;
}

void dast_struct_set_ref(dast_struct_t *st, const char *field, void *v) {
  int idx = dast_struct_slot(st, field);
  st->field_values[idx].p = v;
}

dast_int dast_struct_get_int(dast_struct_t *st, const char *field) {
  int idx = dast_struct_find(st, field);
  if (idx < 0) {
    fprintf(stderr, "struct field '%s' not found\n", field);
    exit(1);
  }
  return st->field_values[idx].i;
}

bool dast_struct_get_bool(dast_struct_t *st, const char *field) {
  int idx = dast_struct_find(st, field);
  if (idx < 0) {
    fprintf(stderr, "struct field '%s' not found\n", field);
    exit(1);
  }
  return st->field_values[idx].b;
}

dast_string_t dast_struct_get_string(dast_struct_t *st, const char *field) {
  int idx = dast_struct_find(st, field);
  if (idx < 0) {
    fprintf(stderr, "struct field '%s' not found\n", field);
    exit(1);
  }
  return st->field_values[idx].s;
}

dast_array_t dast_struct_get_array(dast_struct_t *st, const char *field) {
  int idx = dast_struct_find(st, field);
  if (idx < 0) {
    fprintf(stderr, "struct field '%s' not found\n", field);
    exit(1);
  }
  return st->field_values[idx].a;
}

dast_struct_t *dast_struct_get_struct(dast_struct_t *st, const char *field) {
  int idx = dast_struct_find(st, field);
  if (idx < 0) {
    fprintf(stderr, "struct field '%s' not found\n", field);
    exit(1);
  }
  return st->field_values[idx].st;
}

void *dast_struct_get_ref(dast_struct_t *st, const char *field) {
  int idx = dast_struct_find(st, field);
  if (idx < 0) {
    fprintf(stderr, "struct field '%s' not found\n", field);
    exit(1);
  }
  return st->field_values[idx].p;
}

// =============================================================================
// Misc builtins
// =============================================================================

dast_array_t dast_args(void) {
  dast_array_t arr = dast_array_new(sizeof(dast_string_t));
  for (int i = 1; i < g_argc; i++) {
    dast_string_t s = dast_string_from_cstr(g_argv[i]);
    dast_array_push(&arr, &s);
  }
  return arr;
}

dast_string_t dast_read_file(dast_string_t path) {
  FILE *f = fopen(path.data, "rb");
  if (!f) {
    fprintf(stderr, "read_file failed: %s\n", path.data);
    exit(1);
  }
  fseek(f, 0, SEEK_END);
  long size = ftell(f);
  fseek(f, 0, SEEK_SET);
  if (size < 0) size = 0;
  dast_string_t out;
  out.len = (size_t)size;
  out.cap = out.len + 1;
  out.data = (char *)dast_rt_malloc(out.cap);
  dast_track_string(out.data);
  size_t n = fread(out.data, 1, out.len, f);
  fclose(f);
  out.len = n;
  out.data[out.len] = '\0';
  return out;
}

dast_array_t dast_read_dir(dast_string_t path) {
  dast_array_t arr = dast_array_new(sizeof(dast_string_t));
  DIR *dir = opendir(path.data);
  if (!dir) {
    if (errno == ENOENT || errno == ENOTDIR) {
      return arr;
    }
    fprintf(stderr, "read_dir failed: %s\n", path.data);
    exit(1);
  }
  struct dirent *ent;
  while ((ent = readdir(dir)) != NULL) {
    const char *name = ent->d_name;
    if (strcmp(name, ".") == 0 || strcmp(name, "..") == 0) continue;
    dast_string_t s = dast_string_from_cstr(name);
    dast_array_push(&arr, &s);
  }
  closedir(dir);
  return arr;
}

void dast_write_file(dast_string_t path, dast_string_t data) {
  FILE *f = fopen(path.data, "wb");
  if (!f) {
    fprintf(stderr, "write_file failed: %s\n", path.data);
    exit(1);
  }
  fwrite(data.data, 1, data.len, f);
  fclose(f);
}

void dast_mkdir(dast_string_t path) {
#ifdef _WIN32
  int rc = _mkdir(path.data);
#else
  int rc = mkdir(path.data, 0755);
#endif
  if (rc != 0 && errno != EEXIST) {
    fprintf(stderr, "mkdir failed: %s\n", path.data);
    exit(1);
  }
}

dast_string_t dast_read_line(void) {
  size_t cap = 256;
  char *buf = (char *)dast_rt_malloc(cap);
  if (!fgets(buf, (int)cap, stdin)) {
    buf[0] = '\0';
  }
  size_t len = strlen(buf);
  if (len > 0 && buf[len - 1] == '\n') {
    buf[len - 1] = '\0';
    len--;
  }
  dast_string_t out;
  out.len = len;
  out.cap = len + 1;
  out.data = (char *)dast_rt_malloc(out.cap);
  dast_track_string(out.data);
  memcpy(out.data, buf, len);
  out.data[len] = '\0';
  dast_rt_free(buf);
  return out;
}

dast_string_t dast_read_bytes(dast_int n) {
  if (n < 0) n = 0;
  dast_string_t out;
  out.len = (size_t)n;
  out.cap = out.len + 1;
  out.data = (char *)dast_rt_malloc(out.cap);
  dast_track_string(out.data);
  size_t readn = fread(out.data, 1, out.len, stdin);
  out.len = readn;
  out.data[out.len] = '\0';
  return out;
}

void dast_exit(dast_int code) { exit((int)code); }

void *dast_struct_alloc(size_t size) {
  void *ptr = dast_rt_malloc(size == 0 ? 1 : size);
  if (!ptr && size != 0) {
    fprintf(stderr, "out of memory\n");
    exit(1);
  }
  memset(ptr, 0, size);
  dast_track_struct(ptr);
  return ptr;
}

void dast_struct_free(void *ptr) {
  if (!ptr) return;
  if (!dast_untrack_struct(ptr)) return;
  dast_rt_free(ptr);
}
