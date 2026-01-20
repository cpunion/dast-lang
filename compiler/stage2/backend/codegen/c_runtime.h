// Dast C Runtime Header
// Provides runtime support for compiled Dast programs

#ifndef DAST_RUNTIME_H
#define DAST_RUNTIME_H

#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>

// =============================================================================
// Basic Types
// =============================================================================

typedef int64_t dast_int;
typedef int32_t dast_i32;
typedef int64_t dast_i64;
typedef uint8_t dast_u8;
typedef uint32_t dast_u32;
typedef uint64_t dast_u64;

// =============================================================================
// String Type
// =============================================================================

typedef struct {
    const char* data;
    size_t len;
    size_t cap;
    bool owned;  // true if we own the memory
} dast_string_t;

dast_string_t dast_string_new(const char* s);
dast_string_t dast_string_from_literal(const char* s);
dast_string_t dast_string_concat(dast_string_t a, dast_string_t b);
dast_string_t dast_string_substr(dast_string_t s, dast_int start, dast_int length);
dast_int dast_string_len(dast_string_t s);
dast_int dast_string_char_at(dast_string_t s, dast_int idx);
dast_int dast_string_find_char(dast_string_t s, dast_int c, dast_int start);
bool dast_string_eq(dast_string_t a, dast_string_t b);
void dast_string_free(dast_string_t* s);

// =============================================================================
// Array Type
// =============================================================================

typedef struct {
    void* data;
    size_t len;
    size_t cap;
    size_t elem_size;
} dast_array_t;

dast_array_t dast_array_new(size_t elem_size);
void dast_array_push(dast_array_t* arr, const void* elem);
void* dast_array_get(dast_array_t* arr, dast_int idx);
void dast_array_set(dast_array_t* arr, dast_int idx, const void* elem);
dast_int dast_array_len(dast_array_t* arr);
void dast_array_free(dast_array_t* arr);

// =============================================================================
// I/O Functions
// =============================================================================

void dast_print(dast_string_t s);
void dast_println(dast_string_t s);
dast_string_t dast_read_line(void);
dast_string_t dast_read_file(dast_string_t path);
void dast_write_file(dast_string_t path, dast_string_t content);
dast_array_t dast_read_dir(dast_string_t path);
void dast_mkdir(dast_string_t path);

// =============================================================================
// Conversion Functions
// =============================================================================

dast_string_t dast_int_to_string(dast_int n);
dast_int dast_string_to_int(dast_string_t s);
dast_string_t dast_bool_to_string(bool b);

// =============================================================================
// Runtime Control
// =============================================================================

void dast_panic(dast_string_t msg);
void dast_runtime_init(int argc, char** argv);
void dast_runtime_cleanup(void);

// Command line arguments
extern dast_array_t dast_args;

#endif // DAST_RUNTIME_H
