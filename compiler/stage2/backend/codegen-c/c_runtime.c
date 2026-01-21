// Dast C Runtime Implementation
#include "c_runtime.h"

// Global state
static int g_argc = 0;
static char **g_argv = NULL;

// Runtime initialization
void dast_runtime_init(int argc, char **argv) {
  g_argc = argc;
  g_argv = argv;
}

void dast_runtime_cleanup(void) {
  // Nothing to clean up for now
}

// String operations
dast_string_t dast_string_from_literal(const char *s) {
  dast_string_t str;
  str.len = strlen(s);
  str.cap = str.len + 1;
  str.data = (char *)malloc(str.cap);
  memcpy(str.data, s, str.len);
  str.data[str.len] = '\0';
  return str;
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
    result.data = (char *)malloc(1);
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
  result.data = (char *)malloc(result.cap);
  memcpy(result.data, s.data + start, length);
  result.data[length] = '\0';
  return result;
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
    arr->data = realloc(arr->data, new_cap * arr->elem_size);
    arr->cap = new_cap;
  }
  memcpy((char *)arr->data + arr->len * arr->elem_size, elem, arr->elem_size);
  arr->len++;
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

// I/O operations
void dast_print(dast_string_t s) { fwrite(s.data, 1, s.len, stdout); }

void dast_println(dast_string_t s) {
  fwrite(s.data, 1, s.len, stdout);
  putchar('\n');
}

void dast_print_int(dast_int n) { printf("%lld", (long long)n); }

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
  result.data = (char *)malloc(result.cap);
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
