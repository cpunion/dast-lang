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

typedef struct {
  uint8_t _;
} dast_unit_t;

typedef struct dast_struct_t dast_struct_t;

// Union type for holding any Dast value (runtime/internal)
typedef union {
  int64_t i;
  bool b;
  dast_string_t s;
  dast_array_t a;
  dast_struct_t *st;
  void *p;
} dast_value_t;

struct dast_struct_t {
  const char *name;
  size_t len;
  const char **field_names;
  dast_value_t *field_values;
};

// Runtime init/cleanup
void dast_runtime_init(int argc, char **argv);
void dast_runtime_cleanup(void);

// String operations
dast_string_t dast_string_from_cstr(const char *s);
dast_string_t dast_string_from_literal(const char *s);
dast_int dast_string_len(dast_string_t s);
dast_string_t dast_string_concat(dast_string_t a, dast_string_t b);
bool dast_string_eq(dast_string_t a, dast_string_t b);
bool dast_string_eq_cstr(dast_string_t a, const char *b);
dast_int dast_string_char_at(dast_string_t s, dast_int i);
dast_string_t dast_string_substr(dast_string_t s, dast_int start,
                                 dast_int length);

// Array operations
dast_array_t dast_array_new(size_t elem_size);
dast_int dast_array_len(dast_array_t *arr);
void dast_array_push(dast_array_t *arr, void *elem);
void dast_array_pop(dast_array_t *arr, void *out);
void *dast_array_get(dast_array_t *arr, dast_int i);
void dast_array_set(dast_array_t *arr, dast_int i, void *elem);

// I/O operations
void dast_print(dast_string_t s);
void dast_println(dast_string_t s);
void dast_print_cstr(const char *s);
void dast_print_int(dast_int n);
void dast_print_bool(bool b);
void dast_print_newline(void);
dast_string_t dast_int_to_string(dast_int n);
void dast_eprint(dast_string_t s);
void dast_eprintln(dast_string_t s);
void dast_eprint_cstr(const char *s);
void dast_eprint_int(dast_int n);
void dast_eprint_bool(bool b);
void dast_eprint_newline(void);
void dast_eprint_array(dast_array_t *arr);
void dast_print_array(dast_array_t *arr);
void dast_print_struct(dast_struct_t *st);
void dast_panic(dast_string_t msg);
int64_t dast_exec(dast_string_t cmd, dast_array_t args);

// Struct operations
dast_struct_t *dast_struct_new(const char *name, size_t field_count);
void dast_struct_set_int(dast_struct_t *st, const char *field, dast_int v);
void dast_struct_set_bool(dast_struct_t *st, const char *field, bool v);
void dast_struct_set_string(dast_struct_t *st, const char *field, dast_string_t v);
void dast_struct_set_array(dast_struct_t *st, const char *field, dast_array_t v);
void dast_struct_set_struct(dast_struct_t *st, const char *field, dast_struct_t *v);
void dast_struct_set_ref(dast_struct_t *st, const char *field, void *v);
dast_int dast_struct_get_int(dast_struct_t *st, const char *field);
bool dast_struct_get_bool(dast_struct_t *st, const char *field);
dast_string_t dast_struct_get_string(dast_struct_t *st, const char *field);
dast_array_t dast_struct_get_array(dast_struct_t *st, const char *field);
dast_struct_t *dast_struct_get_struct(dast_struct_t *st, const char *field);
void *dast_struct_get_ref(dast_struct_t *st, const char *field);

// Misc builtins
dast_array_t dast_args(void);
dast_string_t dast_read_file(dast_string_t path);
dast_array_t dast_read_dir(dast_string_t path);
void dast_write_file(dast_string_t path, dast_string_t data);
void dast_mkdir(dast_string_t path);
dast_string_t dast_read_line(void);
dast_string_t dast_read_bytes(dast_int n);
void dast_exit(dast_int code);

#endif
