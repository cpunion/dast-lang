// QBE runtime wrappers (pointer-based) for stage2
#include "../codegen-c/c_runtime.h"
#include <stdlib.h>

static dast_string_t **g_string_ptrs = NULL;
static size_t g_string_ptrs_len = 0;
static size_t g_string_ptrs_cap = 0;
static dast_array_t **g_array_ptrs = NULL;
static size_t g_array_ptrs_len = 0;
static size_t g_array_ptrs_cap = 0;

static void *qbe_rt_xrealloc(void *p, size_t n) {
  return dast_rt_realloc(p, n);
}

static void qbe_track_string_ptr(dast_string_t *s) {
  if (!s) return;
  for (size_t i = 0; i < g_string_ptrs_len; i++) {
    if (g_string_ptrs[i] == s) return;
  }
  if (g_string_ptrs_len >= g_string_ptrs_cap) {
    size_t next = g_string_ptrs_cap == 0 ? 64 : g_string_ptrs_cap * 2;
    g_string_ptrs = (dast_string_t **)qbe_rt_xrealloc(g_string_ptrs, sizeof(dast_string_t *) * next);
    g_string_ptrs_cap = next;
  }
  g_string_ptrs[g_string_ptrs_len++] = s;
}

static int qbe_untrack_string_ptr(dast_string_t *s) {
  if (!s) return 0;
  for (size_t i = 0; i < g_string_ptrs_len; i++) {
    if (g_string_ptrs[i] == s) {
      g_string_ptrs[i] = g_string_ptrs[g_string_ptrs_len - 1];
      g_string_ptrs_len--;
      return 1;
    }
  }
  return 0;
}

static void qbe_track_array_ptr(dast_array_t *arr) {
  if (!arr) return;
  for (size_t i = 0; i < g_array_ptrs_len; i++) {
    if (g_array_ptrs[i] == arr) return;
  }
  if (g_array_ptrs_len >= g_array_ptrs_cap) {
    size_t next = g_array_ptrs_cap == 0 ? 64 : g_array_ptrs_cap * 2;
    g_array_ptrs = (dast_array_t **)qbe_rt_xrealloc(g_array_ptrs, sizeof(dast_array_t *) * next);
    g_array_ptrs_cap = next;
  }
  g_array_ptrs[g_array_ptrs_len++] = arr;
}

static int qbe_untrack_array_ptr(dast_array_t *arr) {
  if (!arr) return 0;
  for (size_t i = 0; i < g_array_ptrs_len; i++) {
    if (g_array_ptrs[i] == arr) {
      g_array_ptrs[i] = g_array_ptrs[g_array_ptrs_len - 1];
      g_array_ptrs_len--;
      return 1;
    }
  }
  return 0;
}

static dast_array_t *qbe_string_array_to_ptr(dast_array_t arr) {
  dast_array_t *out = (dast_array_t *)dast_rt_malloc(sizeof(dast_array_t));
  if (!out) return NULL;
  *out = dast_array_new(sizeof(dast_string_t *));
  qbe_track_array_ptr(out);
  if (arr.len > 0 && arr.data) {
    dast_string_t *vals = (dast_string_t *)arr.data;
    for (dast_int i = 0; i < arr.len; i++) {
      dast_string_t *p = (dast_string_t *)dast_rt_malloc(sizeof(dast_string_t));
      if (!p) continue;
      *p = vals[i];
      qbe_track_string_ptr(p);
      dast_array_push(out, &p);
    }
  }
  dast_array_free(arr);
  return out;
}

static dast_array_t qbe_string_ptr_array_to_value(dast_array_t *arr) {
  dast_array_t out = dast_array_new(sizeof(dast_string_t));
  if (!arr || arr->len == 0 || !arr->data) {
    return out;
  }
  dast_string_t **vals = (dast_string_t **)arr->data;
  for (dast_int i = 0; i < arr->len; i++) {
    dast_string_t v = vals[i] ? *vals[i] : (dast_string_t){0};
    dast_array_push(&out, &v);
  }
  return out;
}

// String wrappers (pointer-based)
dast_string_t *dast_string_from_literal_ptr(const char *s) {
  dast_string_t *out = (dast_string_t *)dast_rt_malloc(sizeof(dast_string_t));
  if (!out) return NULL;
  *out = dast_string_from_literal(s);
  qbe_track_string_ptr(out);
  return out;
}

dast_string_t *dast_string_from_cstr_ptr(const char *s) {
  dast_string_t *out = (dast_string_t *)dast_rt_malloc(sizeof(dast_string_t));
  if (!out) return NULL;
  *out = dast_string_from_cstr(s);
  qbe_track_string_ptr(out);
  return out;
}

dast_int dast_string_len_ptr(dast_string_t *s) {
  if (!s) return 0;
  return dast_string_len(*s);
}

dast_string_t *dast_string_concat_ptr(dast_string_t *a, dast_string_t *b) {
  dast_string_t *out = (dast_string_t *)dast_rt_malloc(sizeof(dast_string_t));
  if (!out) return NULL;
  *out = dast_string_concat(a ? *a : (dast_string_t){0}, b ? *b : (dast_string_t){0});
  qbe_track_string_ptr(out);
  return out;
}

dast_string_t *dast_string_clone_ptr(dast_string_t *s) {
  dast_string_t *out = (dast_string_t *)dast_rt_malloc(sizeof(dast_string_t));
  if (!out) return NULL;
  *out = dast_string_clone(s ? *s : (dast_string_t){0});
  qbe_track_string_ptr(out);
  return out;
}

bool dast_string_eq_ptr(dast_string_t *a, dast_string_t *b) {
  if (!a || !b) return false;
  return dast_string_eq(*a, *b);
}

bool dast_string_eq_cstr_ptr(dast_string_t *a, const char *b) {
  if (!a) return false;
  return dast_string_eq_cstr(*a, b);
}

dast_int dast_string_char_at_ptr(dast_string_t *s, dast_int i) {
  if (!s) return 0;
  return dast_string_char_at(*s, i);
}

dast_string_t *dast_string_substr_ptr(dast_string_t *s, dast_int start, dast_int length) {
  dast_string_t *out = (dast_string_t *)dast_rt_malloc(sizeof(dast_string_t));
  if (!out) return NULL;
  *out = dast_string_substr(s ? *s : (dast_string_t){0}, start, length);
  qbe_track_string_ptr(out);
  return out;
}

void dast_string_free_ptr(dast_string_t *s) {
  if (!s) return;
  if (!qbe_untrack_string_ptr(s)) return;
  dast_string_free(*s);
  dast_rt_free(s);
}

// Array wrappers (pointer-based)
dast_array_t *dast_array_new_ptr(size_t elem_size) {
  dast_array_t *arr = (dast_array_t *)dast_rt_malloc(sizeof(dast_array_t));
  if (!arr) return NULL;
  *arr = dast_array_new(elem_size);
  qbe_track_array_ptr(arr);
  return arr;
}

dast_int dast_array_len_ptr(dast_array_t *arr) {
  if (!arr) return 0;
  return dast_array_len(arr);
}

void dast_array_push_ptr(dast_array_t *arr, void *elem) {
  if (!arr) return;
  dast_array_push(arr, elem);
}

void dast_array_pop_ptr(dast_array_t *arr, void *out) {
  if (!arr) return;
  dast_array_pop(arr, out);
}

void *dast_array_get_ptr(dast_array_t *arr, dast_int i) {
  if (!arr) return NULL;
  return dast_array_get(arr, i);
}

void dast_array_set_ptr(dast_array_t *arr, dast_int i, void *elem) {
  if (!arr) return;
  dast_array_set(arr, i, elem);
}

void dast_array_free_ptr(dast_array_t *arr) {
  if (!arr) return;
  if (!qbe_untrack_array_ptr(arr)) return;
  dast_array_free(*arr);
  dast_rt_free(arr);
}

// I/O wrappers
void dast_print_ptr(dast_string_t *s) {
  if (!s) return;
  dast_print(*s);
}

void dast_println_ptr(dast_string_t *s) {
  if (!s) {
    dast_print_newline();
    return;
  }
  dast_println(*s);
}

void dast_eprint_ptr(dast_string_t *s) {
  if (!s) return;
  dast_eprint(*s);
}

void dast_eprintln_ptr(dast_string_t *s) {
  if (!s) {
    dast_eprint_newline();
    return;
  }
  dast_eprintln(*s);
}

dast_string_t *dast_read_file_ptr(dast_string_t *path) {
  dast_string_t *out = (dast_string_t *)dast_rt_malloc(sizeof(dast_string_t));
  if (!out) return NULL;
  *out = dast_read_file(path ? *path : (dast_string_t){0});
  qbe_track_string_ptr(out);
  return out;
}

dast_array_t *dast_read_dir_ptr(dast_string_t *path) {
  dast_array_t raw = dast_read_dir(path ? *path : (dast_string_t){0});
  return qbe_string_array_to_ptr(raw);
}

void dast_write_file_ptr(dast_string_t *path, dast_string_t *data) {
  dast_write_file(path ? *path : (dast_string_t){0}, data ? *data : (dast_string_t){0});
}

void dast_mkdir_ptr(dast_string_t *path) {
  dast_mkdir(path ? *path : (dast_string_t){0});
}

dast_string_t *dast_read_line_ptr(void) {
  dast_string_t *out = (dast_string_t *)dast_rt_malloc(sizeof(dast_string_t));
  if (!out) return NULL;
  *out = dast_read_line();
  qbe_track_string_ptr(out);
  return out;
}

dast_string_t *dast_read_bytes_ptr(dast_int n) {
  dast_string_t *out = (dast_string_t *)dast_rt_malloc(sizeof(dast_string_t));
  if (!out) return NULL;
  *out = dast_read_bytes(n);
  qbe_track_string_ptr(out);
  return out;
}

dast_string_t *dast_int_to_string_ptr(dast_int n) {
  dast_string_t *out = (dast_string_t *)dast_rt_malloc(sizeof(dast_string_t));
  if (!out) return NULL;
  *out = dast_int_to_string(n);
  qbe_track_string_ptr(out);
  return out;
}

dast_array_t *dast_args_ptr(void) {
  dast_array_t raw = dast_args();
  return qbe_string_array_to_ptr(raw);
}

int64_t dast_exec_ptr(dast_string_t *cmd, dast_array_t *args) {
  dast_array_t vals = qbe_string_ptr_array_to_value(args);
  int64_t out = dast_exec(cmd ? *cmd : (dast_string_t){0}, vals);
  dast_array_free(vals);
  return out;
}

void dast_panic_ptr(dast_string_t *msg) {
  dast_panic(msg ? *msg : (dast_string_t){0});
}
