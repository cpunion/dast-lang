// Dast C Runtime Header
#ifndef DAST_C_RUNTIME_H
#define DAST_C_RUNTIME_H

#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef int64_t dast_int;
typedef int32_t dast_i32;
typedef uint8_t dast_u8;
typedef uint32_t dast_u32;
typedef uint64_t dast_u64;

typedef struct {
  char *data;
  size_t len;
  size_t cap;
} dast_string_t;

typedef struct {
  void *data;
  size_t len;
  size_t cap;
  size_t elem_size;
} dast_array_t;

// Union type for holding any Dast value
typedef union {
  int64_t i;
  bool b;
  dast_string_t s;
  dast_array_t a;
  void *p;
} dast_value_t;

// Runtime init/cleanup
void dast_runtime_init(int argc, char **argv);
void dast_runtime_cleanup(void);

// String operations
dast_string_t dast_string_from_literal(const char *s);
dast_int dast_string_char_at(dast_string_t s, dast_int i);
dast_string_t dast_string_substr(dast_string_t s, dast_int start,
                                 dast_int length);

// Array operations
dast_array_t dast_array_new(size_t elem_size);
dast_int dast_array_len(dast_array_t *arr);
void dast_array_push(dast_array_t *arr, void *elem);
void *dast_array_get(dast_array_t *arr, dast_int i);
void dast_array_set(dast_array_t *arr, dast_int i, void *elem);

// I/O operations
void dast_print(dast_string_t s);
void dast_println(dast_string_t s);
dast_string_t dast_int_to_string(dast_int n);
void dast_panic(dast_string_t msg);

#endif
