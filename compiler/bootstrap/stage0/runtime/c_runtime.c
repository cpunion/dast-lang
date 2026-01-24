#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <dirent.h>
#include <errno.h>
#include <sys/stat.h>
#include <unistd.h>
#include <sys/wait.h>

typedef int64_t dast_int;
typedef int32_t dast_i32;
typedef int32_t dast_bool;

typedef struct DastArray {
	dast_int len;
	dast_int cap;
	dast_int *data;
} DastArray;

typedef struct DastStruct {
	const char *name;
	dast_int field_expected;
	dast_int field_len;
	dast_int field_cap;
	const char **field_names;
	dast_int *field_values;
} DastStruct;

static int g_argc = 0;
static char **g_argv = NULL;

static void dast_rt_panic(const char *msg) {
	fprintf(stderr, "stage0: <runtime>:0:0: error %s\n", msg);
	exit(1);
}

static void *dast_xmalloc(size_t n) {
	void *p = malloc(n);
	if (!p) {
		dast_rt_panic("out of memory");
	}
	return p;
}

static void *dast_xrealloc(void *p, size_t n) {
	void *q = realloc(p, n);
	if (!q) {
		dast_rt_panic("out of memory");
	}
	return q;
}

void dast_set_args(int argc, char **argv) {
	g_argc = argc;
	g_argv = argv;
}

DastArray *dast_array_new(dast_int cap) {
	if (cap < 0) {
		cap = 0;
	}
	DastArray *arr = (DastArray *)dast_xmalloc(sizeof(DastArray));
	arr->len = 0;
	arr->cap = cap > 0 ? cap : 4;
	arr->data = (dast_int *)dast_xmalloc(sizeof(dast_int) * (size_t)arr->cap);
	return arr;
}

static void dast_array_grow(DastArray *arr, dast_int need) {
	if (arr->cap >= need) {
		return;
	}
	dast_int cap = arr->cap > 0 ? arr->cap : 4;
	while (cap < need) {
		cap *= 2;
	}
	arr->cap = cap;
	arr->data = (dast_int *)dast_xrealloc(arr->data, sizeof(dast_int) * (size_t)arr->cap);
}

void dast_array_push(DastArray *arr, dast_int value) {
	if (!arr) {
		dast_rt_panic("push expects array");
	}
	dast_array_grow(arr, arr->len + 1);
	arr->data[arr->len++] = value;
}

dast_int dast_array_get(DastArray *arr, dast_int index) {
	if (!arr) {
		dast_rt_panic("indexing requires array");
	}
	if (index < 0 || index >= arr->len) {
		dast_rt_panic("index out of bounds");
	}
	return arr->data[index];
}

dast_int dast_array_get_unchecked(DastArray *arr, dast_int index) {
	if (!arr) {
		dast_rt_panic("indexing requires array");
	}
	return arr->data[index];
}

void dast_array_set(DastArray *arr, dast_int index, dast_int value) {
	if (!arr) {
		dast_rt_panic("index assignment requires array");
	}
	if (index < 0 || index >= arr->len) {
		dast_rt_panic("index out of bounds");
	}
	arr->data[index] = value;
}

void dast_array_set_unchecked(DastArray *arr, dast_int index, dast_int value) {
	if (!arr) {
		dast_rt_panic("index assignment requires array");
	}
	arr->data[index] = value;
}

dast_int *dast_array_index_addr(DastArray *arr, dast_int index) {
	if (!arr) {
		dast_rt_panic("indexing requires array");
	}
	if (index < 0 || index >= arr->len) {
		dast_rt_panic("index out of bounds");
	}
	return &arr->data[index];
}

dast_int dast_array_len(DastArray *arr) {
	if (!arr) {
		return 0;
	}
	return arr->len;
}

static dast_int dast_struct_index(DastStruct *st, const char *field) {
	for (dast_int i = 0; i < st->field_len; i++) {
		if (strcmp(st->field_names[i], field) == 0) {
			return i;
		}
	}
	return -1;
}

DastStruct *dast_struct_new(const char *name, dast_int field_count) {
	DastStruct *st = (DastStruct *)dast_xmalloc(sizeof(DastStruct));
	st->name = name ? name : "";
	st->field_expected = field_count;
	st->field_len = 0;
	st->field_cap = field_count > 0 ? field_count : 4;
	st->field_names = (const char **)dast_xmalloc(sizeof(char *) * (size_t)st->field_cap);
	st->field_values = (dast_int *)dast_xmalloc(sizeof(dast_int) * (size_t)st->field_cap);
	return st;
}

static void dast_struct_set_value(DastStruct *st, const char *field, dast_int value) {
	if (!st) {
		dast_rt_panic("field assignment requires struct");
	}
	dast_int idx = dast_struct_index(st, field);
	if (idx >= 0) {
		st->field_values[idx] = value;
		return;
	}
	if (st->field_expected > 0 && st->field_len >= st->field_expected) {
		dast_rt_panic("unknown field");
	}
	if (st->field_len >= st->field_cap) {
		st->field_cap *= 2;
		st->field_names = (const char **)dast_xrealloc(st->field_names, sizeof(char *) * (size_t)st->field_cap);
		st->field_values = (dast_int *)dast_xrealloc(st->field_values, sizeof(dast_int) * (size_t)st->field_cap);
	}
	st->field_names[st->field_len] = field;
	st->field_values[st->field_len] = value;
	st->field_len++;
}

static dast_int dast_struct_get_value(DastStruct *st, const char *field) {
	if (!st) {
		dast_rt_panic("field access requires struct");
	}
	dast_int idx = dast_struct_index(st, field);
	if (idx < 0) {
		dast_rt_panic("unknown field");
	}
	return st->field_values[idx];
}

void dast_struct_set_i64(DastStruct *st, const char *field, dast_int value) {
	dast_struct_set_value(st, field, value);
}

void dast_struct_set_i32(DastStruct *st, const char *field, dast_i32 value) {
	dast_struct_set_value(st, field, (dast_int)value);
}

void dast_struct_set_bool(DastStruct *st, const char *field, dast_bool value) {
	dast_struct_set_value(st, field, (dast_int)(value != 0));
}

void dast_struct_set_string(DastStruct *st, const char *field, const char *value) {
	dast_struct_set_value(st, field, (dast_int)(intptr_t)value);
}

void dast_struct_set_ptr(DastStruct *st, const char *field, dast_int value) {
	dast_struct_set_value(st, field, value);
}

dast_int dast_struct_get_i64(DastStruct *st, const char *field) {
	return dast_struct_get_value(st, field);
}

dast_i32 dast_struct_get_i32(DastStruct *st, const char *field) {
	return (dast_i32)dast_struct_get_value(st, field);
}

dast_bool dast_struct_get_bool(DastStruct *st, const char *field) {
	return (dast_bool)(dast_struct_get_value(st, field) != 0);
}

const char *dast_struct_get_string(DastStruct *st, const char *field) {
	return (const char *)(intptr_t)dast_struct_get_value(st, field);
}

dast_int dast_struct_get_ptr(DastStruct *st, const char *field) {
	return dast_struct_get_value(st, field);
}

dast_int *dast_struct_field_addr(DastStruct *st, const char *field) {
	if (!st) {
		dast_rt_panic("addr_field requires struct");
	}
	dast_int idx = dast_struct_index(st, field);
	if (idx < 0) {
		dast_rt_panic("unknown field");
	}
	return &st->field_values[idx];
}

dast_int dast_string_len(const char *s) {
	if (!s) {
		return 0;
	}
	return (dast_int)strlen(s);
}

dast_bool dast_string_eq(const char *a, const char *b) {
	if (!a || !b) {
		return a == b;
	}
	return strcmp(a, b) == 0;
}

const char *dast_string_concat(const char *a, const char *b) {
	if (!a) {
		a = "";
	}
	if (!b) {
		b = "";
	}
	size_t la = strlen(a);
	size_t lb = strlen(b);
	char *out = (char *)dast_xmalloc(la + lb + 1);
	memcpy(out, a, la);
	memcpy(out + la, b, lb);
	out[la + lb] = '\0';
	return out;
}

static void dast_print_i64_to(FILE *out, dast_int v) {
	fprintf(out, "%lld", (long long)v);
}

static void dast_print_bool_to(FILE *out, dast_bool v) {
	fputs(v ? "true" : "false", out);
}

static void dast_print_string_to(FILE *out, const char *s) {
	if (!s) {
		s = "";
	}
	fputs(s, out);
}

static void dast_print_struct_to(FILE *out, DastStruct *st) {
	if (!st) {
		fputs("<struct>", out);
		return;
	}
	fputs(st->name, out);
	fputs("{...}", out);
}

static void dast_print_array_to(FILE *out, DastArray *arr) {
	if (!arr) {
		fputs("[]", out);
		return;
	}
	fprintf(out, "[len=%lld]", (long long)arr->len);
}

static void dast_print_ptr_to(FILE *out, dast_int v) {
	fprintf(out, "0x%llx", (unsigned long long)v);
}

void dast_print_space(void) {
	fputs(" ", stdout);
}

void dast_print_newline(void) {
	fputs("\n", stdout);
}

void dast_eprint_space(void) {
	fputs(" ", stderr);
}

void dast_eprint_newline(void) {
	fputs("\n", stderr);
}

void dast_print_i64(dast_int v) { dast_print_i64_to(stdout, v); }
void dast_print_bool(dast_bool v) { dast_print_bool_to(stdout, v); }
void dast_print_string(const char *s) { dast_print_string_to(stdout, s); }
void dast_print_struct(dast_int v) { dast_print_struct_to(stdout, (DastStruct *)(intptr_t)v); }
void dast_print_array(dast_int v) { dast_print_array_to(stdout, (DastArray *)(intptr_t)v); }
void dast_print_ptr(dast_int v) { dast_print_ptr_to(stdout, v); }

void dast_eprint_i64(dast_int v) { dast_print_i64_to(stderr, v); }
void dast_eprint_bool(dast_bool v) { dast_print_bool_to(stderr, v); }
void dast_eprint_string(const char *s) { dast_print_string_to(stderr, s); }
void dast_eprint_struct(dast_int v) { dast_print_struct_to(stderr, (DastStruct *)(intptr_t)v); }
void dast_eprint_array(dast_int v) { dast_print_array_to(stderr, (DastArray *)(intptr_t)v); }
void dast_eprint_ptr(dast_int v) { dast_print_ptr_to(stderr, v); }

void dast_push(dast_int *arr_ref, dast_int value) {
	if (!arr_ref) {
		dast_rt_panic("push expects &mut array");
	}
	DastArray *arr = (DastArray *)(intptr_t)(*arr_ref);
	dast_array_push(arr, value);
}

dast_int dast_pop(dast_int *arr_ref) {
	if (!arr_ref) {
		dast_rt_panic("pop expects &mut array");
	}
	DastArray *arr = (DastArray *)(intptr_t)(*arr_ref);
	if (!arr || arr->len == 0) {
		dast_rt_panic("pop from empty array");
	}
	dast_int value = arr->data[arr->len - 1];
	arr->len--;
	return value;
}

dast_int dast_char_at(const char *s, dast_int idx) {
	if (!s) {
		dast_rt_panic("char_at expects string");
	}
	dast_int len = (dast_int)strlen(s);
	if (idx < 0 || idx >= len) {
		dast_rt_panic("char_at index out of bounds");
	}
	return (unsigned char)s[idx];
}

const char *dast_substr(const char *s, dast_int start, dast_int len) {
	if (!s) {
		dast_rt_panic("substr expects string");
	}
	dast_int slen = (dast_int)strlen(s);
	if (start < 0 || len < 0 || start > slen || start + len > slen) {
		dast_rt_panic("substr out of bounds");
	}
	char *out = (char *)dast_xmalloc((size_t)len + 1);
	memcpy(out, s + start, (size_t)len);
	out[len] = '\0';
	return out;
}

const char *dast_read_file(const char *path) {
	if (!path) {
		dast_rt_panic("read_file expects string path");
	}
	FILE *f = fopen(path, "rb");
	if (!f) {
		dast_rt_panic("read_file failed");
	}
	fseek(f, 0, SEEK_END);
	long size = ftell(f);
	if (size < 0) {
		fclose(f);
		dast_rt_panic("read_file failed");
	}
	fseek(f, 0, SEEK_SET);
	char *buf = (char *)dast_xmalloc((size_t)size + 1);
	size_t read = fread(buf, 1, (size_t)size, f);
	fclose(f);
	buf[read] = '\0';
	return buf;
}

void dast_write_file(const char *path, const char *data) {
	if (!path || !data) {
		dast_rt_panic("write_file expects string path and data");
	}
	FILE *f = fopen(path, "wb");
	if (!f) {
		dast_rt_panic("write_file failed");
	}
	fwrite(data, 1, strlen(data), f);
	fclose(f);
}

static int dast_mkdir_p(const char *path) {
	if (!path || !*path) {
		return 0;
	}
	char *buf = strdup(path);
	if (!buf) {
		return -1;
	}
	for (char *p = buf + 1; *p; p++) {
		if (*p == '/') {
			*p = '\0';
			if (mkdir(buf, 0755) != 0 && errno != EEXIST) {
				free(buf);
				return -1;
			}
			*p = '/';
		}
	}
	int res = mkdir(buf, 0755);
	if (res != 0 && errno == EEXIST) {
		res = 0;
	}
	free(buf);
	return res;
}

void dast_mkdir(const char *path) {
	if (!path) {
		dast_rt_panic("mkdir expects string path");
	}
	if (dast_mkdir_p(path) != 0) {
		dast_rt_panic("mkdir failed");
	}
}

DastArray *dast_read_dir(const char *path) {
	if (!path) {
		dast_rt_panic("read_dir expects string path");
	}
	DIR *dir = opendir(path);
	if (!dir) {
		if (errno == ENOENT || errno == ENOTDIR) {
			return dast_array_new(0);
		}
		dast_rt_panic("read_dir failed");
	}
	DastArray *arr = dast_array_new(8);
	struct dirent *ent;
	while ((ent = readdir(dir)) != NULL) {
		if (strcmp(ent->d_name, ".") == 0 || strcmp(ent->d_name, "..") == 0) {
			continue;
		}
		dast_array_push(arr, (dast_int)(intptr_t)strdup(ent->d_name));
	}
	closedir(dir);
	return arr;
}

DastArray *dast_args(void) {
	DastArray *arr = dast_array_new(g_argc);
	for (int i = 0; i < g_argc; i++) {
		dast_array_push(arr, (dast_int)(intptr_t)g_argv[i]);
	}
	return arr;
}

const char *dast_read_line(void) {
	size_t cap = 64;
	size_t len = 0;
	char *buf = (char *)dast_xmalloc(cap);
	int c;
	while ((c = fgetc(stdin)) != EOF) {
		if (c == '\n') {
			break;
		}
		if (c == '\r') {
			int next = fgetc(stdin);
			if (next != '\n' && next != EOF) {
				ungetc(next, stdin);
			}
			break;
		}
		if (len + 1 >= cap) {
			cap *= 2;
			buf = (char *)dast_xrealloc(buf, cap);
		}
		buf[len++] = (char)c;
	}
	buf[len] = '\0';
	return buf;
}

const char *dast_read_bytes(dast_int count) {
	if (count < 0) {
		dast_rt_panic("read_bytes count must be non-negative");
	}
	char *buf = (char *)dast_xmalloc((size_t)count + 1);
	size_t total = 0;
	while (total < (size_t)count) {
		size_t n = fread(buf + total, 1, (size_t)count - total, stdin);
		if (n == 0) {
			break;
		}
		total += n;
	}
	buf[total] = '\0';
	return buf;
}

dast_int dast_exec(const char *cmd, DastArray *args) {
	if (!cmd) {
		dast_rt_panic("exec expects string command");
	}
	size_t argc = args ? (size_t)args->len : 0;
	char **argv = (char **)dast_xmalloc(sizeof(char *) * (argc + 2));
	argv[0] = (char *)cmd;
	for (size_t i = 0; i < argc; i++) {
		argv[i + 1] = (char *)(intptr_t)args->data[i];
	}
	argv[argc + 1] = NULL;
	pid_t pid = fork();
	if (pid == 0) {
		execvp(cmd, argv);
		_exit(127);
	}
	int status = 0;
	if (pid > 0) {
		waitpid(pid, &status, 0);
	}
	free(argv);
	if (pid <= 0) {
		dast_rt_panic("exec failed");
	}
	if (WIFEXITED(status)) {
		return (dast_int)WEXITSTATUS(status);
	}
	return 1;
}

const char *dast_int_to_string(dast_int v) {
	char buf[64];
	snprintf(buf, sizeof(buf), "%lld", (long long)v);
	return strdup(buf);
}

dast_i32 dast_parse_int(const char *s) {
	if (!s) {
		return 0;
	}
	char *end = NULL;
	long v = strtol(s, &end, 10);
	if (end == s) {
		return 0;
	}
	return (dast_i32)v;
}

dast_int dast_string_to_int(const char *s) {
	if (!s) {
		return 0;
	}
	char *end = NULL;
	long long v = strtoll(s, &end, 10);
	if (end == s) {
		return 0;
	}
	return (dast_int)v;
}

dast_bool dast_has_prefix(const char *s, const char *prefix) {
	if (!s || !prefix) {
		return 0;
	}
	size_t plen = strlen(prefix);
	return strncmp(s, prefix, plen) == 0 ? 1 : 0;
}

void dast_exit(dast_int code) {
	exit((int)code);
}

static char *dast_sanitize_ident(const char *s) {
	if (!s || !*s) {
		return strdup("tmp");
	}
	size_t n = strlen(s);
	char *out = (char *)dast_xmalloc(n + 1);
	for (size_t i = 0; i < n; i++) {
		char c = s[i];
		if ((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			out[i] = c;
		} else {
			out[i] = '_';
		}
	}
	out[n] = '\0';
	return out;
}

const char *dast_ast_expr1(const char *src) {
	if (!src) {
		return "";
	}
	return strdup(src);
}

const char *dast_ast_expr2(const char *src, DastArray *splices) {
	(void)splices;
	if (!src) {
		return "";
	}
	return strdup(src);
}

const char *dast_ast_stmt1(const char *src) {
	return dast_ast_expr1(src);
}

const char *dast_ast_stmt2(const char *src, DastArray *splices) {
	return dast_ast_expr2(src, splices);
}

const char *dast_ast_item1(const char *src) {
	return dast_ast_expr1(src);
}

const char *dast_ast_item2(const char *src, DastArray *splices) {
	return dast_ast_expr2(src, splices);
}

const char *dast_ast_block1(const char *src) {
	return dast_ast_expr1(src);
}

const char *dast_ast_block2(const char *src, DastArray *splices) {
	return dast_ast_expr2(src, splices);
}

const char *dast_ast_to_string(const char *ast) {
	if (!ast) {
		return "";
	}
	return strdup(ast);
}

const char *dast_gensym(const char *prefix) {
	static dast_int counter = 0;
	char *clean = dast_sanitize_ident(prefix);
	char buf[128];
	snprintf(buf, sizeof(buf), "__dast_%s_%lld", clean, (long long)counter++);
	free(clean);
	return strdup(buf);
}

const char *dast_bind(const char *name) {
	if (!name) {
		return "";
	}
	return dast_sanitize_ident(name);
}
