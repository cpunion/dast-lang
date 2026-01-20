// Dast C Runtime Implementation
// Provides runtime support for compiled Dast programs

#include "c_runtime.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// =============================================================================
// Global State
// =============================================================================

dast_array_t dast_args;

// =============================================================================
// String Implementation
// =============================================================================

dast_string_t dast_string_new(const char *s) {
  if (s == NULL) {
    return (dast_string_t){.data = "", .len = 0, .cap = 0, .owned = false};
  }
  size_t len = strlen(s);
  char *data = (char *)malloc(len + 1);
  memcpy(data, s, len + 1);
  return (dast_string_t){
      .data = data, .len = len, .cap = len + 1, .owned = true};
}

dast_string_t dast_string_from_literal(const char *s) {
  return (dast_string_t){
      .data = s, .len = s ? strlen(s) : 0, .cap = 0, .owned = false};
}

dast_string_t dast_string_concat(dast_string_t a, dast_string_t b) {
  size_t new_len = a.len + b.len;
  char *data = (char *)malloc(new_len + 1);
  memcpy(data, a.data, a.len);
  memcpy(data + a.len, b.data, b.len);
  data[new_len] = '\0';
  return (dast_string_t){
      .data = data, .len = new_len, .cap = new_len + 1, .owned = true};
}

dast_string_t dast_string_substr(dast_string_t s, dast_int start,
                                 dast_int length) {
  if (start < 0 || start >= (dast_int)s.len || length <= 0) {
    return dast_string_from_literal("");
  }
  if (start + length > (dast_int)s.len) {
    length = s.len - start;
  }
  char *data = (char *)malloc(length + 1);
  memcpy(data, s.data + start, length);
  data[length] = '\0';
  return (dast_string_t){.data = data,
                         .len = (size_t)length,
                         .cap = (size_t)length + 1,
                         .owned = true};
}

dast_int dast_string_len(dast_string_t s) { return (dast_int)s.len; }

dast_int dast_string_char_at(dast_string_t s, dast_int idx) {
  if (idx < 0 || idx >= (dast_int)s.len) {
    return -1;
  }
  return (unsigned char)s.data[idx];
}

dast_int dast_string_find_char(dast_string_t s, dast_int c, dast_int start) {
  if (start < 0)
    start = 0;
  for (dast_int i = start; i < (dast_int)s.len; i++) {
    if ((unsigned char)s.data[i] == c) {
      return i;
    }
  }
  return -1;
}

bool dast_string_eq(dast_string_t a, dast_string_t b) {
  if (a.len != b.len)
    return false;
  return memcmp(a.data, b.data, a.len) == 0;
}

void dast_string_free(dast_string_t *s) {
  if (s->owned && s->data) {
    free((void *)s->data);
  }
  s->data = "";
  s->len = 0;
  s->cap = 0;
  s->owned = false;
}

// =============================================================================
// Array Implementation
// =============================================================================

dast_array_t dast_array_new(size_t elem_size) {
  return (dast_array_t){
      .data = NULL, .len = 0, .cap = 0, .elem_size = elem_size};
}

void dast_array_push(dast_array_t *arr, const void *elem) {
  if (arr->len >= arr->cap) {
    size_t new_cap = arr->cap == 0 ? 8 : arr->cap * 2;
    void *new_data = realloc(arr->data, new_cap * arr->elem_size);
    if (!new_data) {
      dast_panic(dast_string_from_literal("out of memory"));
      return;
    }
    arr->data = new_data;
    arr->cap = new_cap;
  }
  memcpy((char *)arr->data + arr->len * arr->elem_size, elem, arr->elem_size);
  arr->len++;
}

void *dast_array_get(dast_array_t *arr, dast_int idx) {
  if (idx < 0 || idx >= (dast_int)arr->len) {
    dast_panic(dast_string_from_literal("array index out of bounds"));
    return NULL;
  }
  return (char *)arr->data + idx * arr->elem_size;
}

void dast_array_set(dast_array_t *arr, dast_int idx, const void *elem) {
  if (idx < 0 || idx >= (dast_int)arr->len) {
    dast_panic(dast_string_from_literal("array index out of bounds"));
    return;
  }
  memcpy((char *)arr->data + idx * arr->elem_size, elem, arr->elem_size);
}

dast_int dast_array_len(dast_array_t *arr) { return (dast_int)arr->len; }

void dast_array_free(dast_array_t *arr) {
  if (arr->data) {
    free(arr->data);
  }
  arr->data = NULL;
  arr->len = 0;
  arr->cap = 0;
}

// =============================================================================
// I/O Implementation
// =============================================================================

void dast_print(dast_string_t s) { fwrite(s.data, 1, s.len, stdout); }

void dast_println(dast_string_t s) {
  fwrite(s.data, 1, s.len, stdout);
  putchar('\n');
}

dast_string_t dast_read_line(void) {
  char *line = NULL;
  size_t len = 0;
  ssize_t read = getline(&line, &len, stdin);
  if (read == -1) {
    free(line);
    return dast_string_from_literal("");
  }
  // Remove trailing newline
  if (read > 0 && line[read - 1] == '\n') {
    line[read - 1] = '\0';
    read--;
  }
  dast_string_t result = {
      .data = line, .len = (size_t)read, .cap = len, .owned = true};
  return result;
}

dast_string_t dast_read_file(dast_string_t path) {
  // Create null-terminated path
  char *cpath = (char *)malloc(path.len + 1);
  memcpy(cpath, path.data, path.len);
  cpath[path.len] = '\0';

  FILE *f = fopen(cpath, "rb");
  free(cpath);

  if (!f) {
    return dast_string_from_literal("");
  }

  fseek(f, 0, SEEK_END);
  long size = ftell(f);
  fseek(f, 0, SEEK_SET);

  char *data = (char *)malloc(size + 1);
  fread(data, 1, size, f);
  data[size] = '\0';
  fclose(f);

  return (dast_string_t){.data = data,
                         .len = (size_t)size,
                         .cap = (size_t)size + 1,
                         .owned = true};
}

void dast_write_file(dast_string_t path, dast_string_t content) {
  char *cpath = (char *)malloc(path.len + 1);
  memcpy(cpath, path.data, path.len);
  cpath[path.len] = '\0';

  FILE *f = fopen(cpath, "wb");
  free(cpath);

  if (!f) {
    return;
  }

  fwrite(content.data, 1, content.len, f);
  fclose(f);
}

dast_array_t dast_read_dir(dast_string_t path) {
  // Simplified - returns empty array for now
  return dast_array_new(sizeof(dast_string_t));
}

void dast_mkdir(dast_string_t path) {
  char *cpath = (char *)malloc(path.len + 1);
  memcpy(cpath, path.data, path.len);
  cpath[path.len] = '\0';

#ifdef _WIN32
  _mkdir(cpath);
#else
  mkdir(cpath, 0755);
#endif

  free(cpath);
}

// =============================================================================
// Conversion Implementation
// =============================================================================

dast_string_t dast_int_to_string(dast_int n) {
  char buf[32];
  int len = snprintf(buf, sizeof(buf), "%lld", (long long)n);
  return dast_string_new(buf);
}

dast_int dast_string_to_int(dast_string_t s) {
  char *cstr = (char *)malloc(s.len + 1);
  memcpy(cstr, s.data, s.len);
  cstr[s.len] = '\0';
  dast_int result = strtoll(cstr, NULL, 10);
  free(cstr);
  return result;
}

dast_string_t dast_bool_to_string(bool b) {
  return dast_string_from_literal(b ? "true" : "false");
}

// =============================================================================
// Runtime Control
// =============================================================================

void dast_panic(dast_string_t msg) {
  fprintf(stderr, "panic: ");
  fwrite(msg.data, 1, msg.len, stderr);
  fprintf(stderr, "\n");
  exit(1);
}

void dast_runtime_init(int argc, char **argv) {
  dast_args = dast_array_new(sizeof(dast_string_t));
  for (int i = 0; i < argc; i++) {
    dast_string_t arg = dast_string_from_literal(argv[i]);
    dast_array_push(&dast_args, &arg);
  }
}

void dast_runtime_cleanup(void) { dast_array_free(&dast_args); }
