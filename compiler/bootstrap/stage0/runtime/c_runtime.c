#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stddef.h>
#include <dirent.h>
#include <errno.h>
#include <sys/stat.h>
#include <unistd.h>
#include <sys/wait.h>
#include <execinfo.h>
#include <pthread.h>
#include <stdatomic.h>

typedef int64_t dast_int;
typedef int32_t dast_i32;
typedef int32_t dast_bool;

typedef struct DastString {
	char *data;
	dast_int len;
	dast_int cap;
} DastString;

typedef struct DastArray {
	dast_int len;
	dast_int cap;
	dast_int *data;
	dast_int debug_id;
} DastArray;

typedef struct DastStruct {
	const char *name;
	dast_int field_expected;
	dast_int field_len;
	dast_int field_cap;
	const char **field_names;
	dast_int *field_values;
} DastStruct;

// Per-allocation header so we can account malloc/realloc/free totals safely.
typedef union DastAllocHeader {
	size_t size;
	long double _align;
	void *_ptr_align;
} DastAllocHeader;

typedef struct DastFreedRange {
	uintptr_t start;
	size_t size;
} DastFreedRange;

#define DAST_MIN_VALID_PTR ((uintptr_t)0x100000u)

static int g_argc = 0;
static char **g_argv = NULL;
static DastString **g_string_allocs = NULL;
static size_t g_string_allocs_len = 0;
static size_t g_string_allocs_cap = 0;
static DastFreedRange *g_freed_ranges = NULL;
static size_t g_freed_ranges_len = 0;
static size_t g_freed_ranges_cap = 0;
static void **g_array_allocs = NULL;
static size_t g_array_allocs_len = 0;
static size_t g_array_allocs_cap = 0;
static void **g_struct_allocs = NULL;
static size_t g_struct_allocs_len = 0;
static size_t g_struct_allocs_cap = 0;
static dast_int g_next_array_id = 1;
static int g_drop_debug = 0;
static int g_drop_debug_reported = 0;
static int g_str_debug = 0;
static int g_str_debug_checked = 0;
static int g_alloc_debug_checked = 0;
static size_t g_alloc_warn_bytes = 0;
static size_t g_alloc_max_bytes = 0;
static size_t g_alloc_total_max_bytes = 0;
static atomic_size_t g_alloc_total_bytes = 0;
static atomic_size_t g_alloc_total_peak_bytes = 0;
static int g_array_debug_checked = 0;
static int g_array_debug = 0;
static int g_array_bt = 0;
static dast_int g_array_debug_id = 0;
static pthread_once_t g_drop_debug_once = PTHREAD_ONCE_INIT;
static pthread_once_t g_str_debug_once = PTHREAD_ONCE_INIT;
static pthread_once_t g_alloc_debug_once = PTHREAD_ONCE_INIT;
static pthread_once_t g_array_debug_once = PTHREAD_ONCE_INIT;

static void dast_drop_debug_report(void);

static void dast_drop_debug_init_once(void) {
	const char *env = getenv("DAST_DROP_DEBUG");
	if (env && *env && strcmp(env, "0") != 0) {
		g_drop_debug = 1;
		// Drop debugging benefits from string/freed-pointer tracking.
		g_str_debug = 1;
		atexit(dast_drop_debug_report);
	}
}

static void dast_drop_debug_init(void) {
	pthread_once(&g_drop_debug_once, dast_drop_debug_init_once);
}

static void dast_str_debug_init_once(void) {
	g_str_debug_checked = 1;
	const char *env = getenv("DAST_STR_DEBUG");
	if (env && *env && strcmp(env, "0") != 0) {
		g_str_debug = 1;
	}
}

static void dast_str_debug_init(void) {
	pthread_once(&g_str_debug_once, dast_str_debug_init_once);
}

static void dast_alloc_debug_init_once(void) {
	g_alloc_debug_checked = 1;
	const char *warn_mb = getenv("DAST_ALLOC_WARN_MB");
	if (warn_mb && *warn_mb) {
		long long mb = strtoll(warn_mb, NULL, 10);
		if (mb > 0) {
			g_alloc_warn_bytes = (size_t)mb * 1024u * 1024u;
		}
	}
	const char *max_mb = getenv("DAST_ALLOC_MAX_MB");
	if (max_mb && *max_mb) {
		long long mb = strtoll(max_mb, NULL, 10);
		if (mb > 0) {
			g_alloc_max_bytes = (size_t)mb * 1024u * 1024u;
		}
	}
	const char *total_max_mb = getenv("DAST_ALLOC_TOTAL_MAX_MB");
	if (total_max_mb && *total_max_mb) {
		long long mb = strtoll(total_max_mb, NULL, 10);
		if (mb > 0) {
			g_alloc_total_max_bytes = (size_t)mb * 1024u * 1024u;
		}
	}
}

static void dast_alloc_debug_init(void) {
	pthread_once(&g_alloc_debug_once, dast_alloc_debug_init_once);
}

static void dast_array_debug_init_once(void) {
	g_array_debug_checked = 1;
	const char *env = getenv("DAST_ARRAY_DEBUG");
	if (env && *env && strcmp(env, "0") != 0) {
		g_array_debug = 1;
	}
	const char *id = getenv("DAST_ARRAY_DEBUG_ID");
	if (id && *id) {
		long long v = strtoll(id, NULL, 10);
		if (v > 0) {
			g_array_debug_id = (dast_int)v;
			g_array_debug = 1;
		}
	}
	const char *bt = getenv("DAST_ARRAY_BT");
	if (bt && *bt && strcmp(bt, "0") != 0) {
		g_array_bt = 1;
	}
}

static void dast_array_debug_init(void) {
	pthread_once(&g_array_debug_once, dast_array_debug_init_once);
}

static int dast_array_debug_match(const DastArray *arr) {
	if (!g_array_debug) {
		return 0;
	}
	if (!arr) {
		return g_array_debug_id == 0;
	}
	return g_array_debug_id == 0 || arr->debug_id == g_array_debug_id;
}

static void dast_debug_backtrace(void) {
	void *frames[32];
	int n = backtrace(frames, 32);
	char **syms = backtrace_symbols(frames, n);
	if (!syms) {
		return;
	}
	fprintf(stderr, "stage0: <runtime>:0:0: backtrace (%d frames)\n", n);
	for (int i = 0; i < n; i++) {
		fprintf(stderr, "  %s\n", syms[i]);
	}
	free(syms);
}

static void dast_rt_panic(const char *msg) {
	fprintf(stderr, "stage0: <runtime>:0:0: error %s\n", msg);
	exit(1);
}

static void dast_alloc_total_fail(const char *kind, size_t request, size_t current, size_t limit) {
	fprintf(stderr,
	        "stage0: <runtime>:0:0: error %s %zu exceeds total limit %zu (current %zu)\n",
	        kind, request, limit, current);
	exit(1);
}

static void dast_alloc_total_note(size_t current) {
	size_t peak = atomic_load_explicit(&g_alloc_total_peak_bytes, memory_order_relaxed);
	while (current > peak) {
		if (atomic_compare_exchange_weak_explicit(&g_alloc_total_peak_bytes, &peak, current,
		                                          memory_order_relaxed, memory_order_relaxed)) {
			break;
		}
	}
}

// Reserve bytes in the global total counter, honoring the total limit if set.
static int dast_alloc_total_reserve(size_t delta, size_t *prev_out, size_t *next_out) {
	if (delta == 0) {
		size_t cur = atomic_load_explicit(&g_alloc_total_bytes, memory_order_relaxed);
		if (prev_out) {
			*prev_out = cur;
		}
		if (next_out) {
			*next_out = cur;
		}
		return 1;
	}
	if (g_alloc_total_max_bytes == 0) {
		size_t prev = atomic_fetch_add_explicit(&g_alloc_total_bytes, delta, memory_order_relaxed);
		size_t next = prev + delta;
		dast_alloc_total_note(next);
		if (prev_out) {
			*prev_out = prev;
		}
		if (next_out) {
			*next_out = next;
		}
		return 1;
	}
	size_t cur = atomic_load_explicit(&g_alloc_total_bytes, memory_order_relaxed);
	for (;;) {
		size_t next = cur + delta;
		if (next > g_alloc_total_max_bytes) {
			if (prev_out) {
				*prev_out = cur;
			}
			if (next_out) {
				*next_out = next;
			}
			return 0;
		}
		if (atomic_compare_exchange_weak_explicit(&g_alloc_total_bytes, &cur, next,
		                                          memory_order_relaxed, memory_order_relaxed)) {
			dast_alloc_total_note(next);
			if (prev_out) {
				*prev_out = cur;
			}
			if (next_out) {
				*next_out = next;
			}
			return 1;
		}
	}
}

static void dast_alloc_total_release(size_t delta) {
	if (delta == 0) {
		return;
	}
	atomic_fetch_sub_explicit(&g_alloc_total_bytes, delta, memory_order_relaxed);
}

static void dast_freed_remove_exact(void *p) {
	if (!g_str_debug || !p) {
		return;
	}
	uintptr_t addr = (uintptr_t)p;
	for (size_t i = 0; i < g_freed_ranges_len; i++) {
		if (g_freed_ranges[i].start == addr) {
			g_freed_ranges[i] = g_freed_ranges[g_freed_ranges_len - 1];
			g_freed_ranges_len--;
			return;
		}
	}
}

static void dast_freed_add(void *p, size_t n) {
	if (!g_str_debug || !p || n == 0) {
		return;
	}
	dast_freed_remove_exact(p);
	if (g_freed_ranges_len >= g_freed_ranges_cap) {
		size_t next = g_freed_ranges_cap == 0 ? 128 : g_freed_ranges_cap * 2;
		DastFreedRange *nr = (DastFreedRange *)realloc(g_freed_ranges, sizeof(DastFreedRange) * next);
		if (!nr) {
			return;
		}
		g_freed_ranges = nr;
		g_freed_ranges_cap = next;
	}
	g_freed_ranges[g_freed_ranges_len++] = (DastFreedRange){(uintptr_t)p, n};
}

static int dast_ptr_in_freed(const void *p) {
	if (!g_str_debug || !p) {
		return 0;
	}
	uintptr_t addr = (uintptr_t)p;
	for (size_t i = 0; i < g_freed_ranges_len; i++) {
		uintptr_t start = g_freed_ranges[i].start;
		uintptr_t end = start + g_freed_ranges[i].size;
		if (addr >= start && addr < end) {
			return 1;
		}
	}
	return 0;
}

static const char *dast_safe_cstr(const char *s) {
	if (!s) {
		return "<null>";
	}
	uintptr_t sp = (uintptr_t)s;
	if (sp < DAST_MIN_VALID_PTR) {
		return "<bad>";
	}
	dast_str_debug_init();
	if (dast_ptr_in_freed(s)) {
		return "<freed>";
	}
	return s;
}

static void *dast_xmalloc(size_t n) {
	dast_alloc_debug_init();
	dast_str_debug_init();
	if (n == 0) {
		n = 1;
	}
	if (g_alloc_warn_bytes > 0 && n >= g_alloc_warn_bytes) {
		fprintf(stderr, "stage0: <runtime>:0:0: warn large alloc %zu bytes\n", n);
	}
	if (g_alloc_max_bytes > 0 && n >= g_alloc_max_bytes) {
		fprintf(stderr, "stage0: <runtime>:0:0: error alloc %zu exceeds max %zu bytes\n", n, g_alloc_max_bytes);
		exit(1);
	}
	size_t cur = 0;
	if (!dast_alloc_total_reserve(n, &cur, NULL)) {
		dast_alloc_total_fail("alloc", n, cur, g_alloc_total_max_bytes);
	}
	DastAllocHeader *h = (DastAllocHeader *)malloc(sizeof(DastAllocHeader) + n);
	if (!h) {
		dast_alloc_total_release(n);
		dast_rt_panic("out of memory");
	}
	h->size = n;
	void *out = (void *)(h + 1);
	dast_freed_remove_exact(out);
	return out;
}

static void *dast_xrealloc(void *p, size_t n) {
	dast_alloc_debug_init();
	dast_str_debug_init();
	if (!p) {
		return dast_xmalloc(n);
	}
	if (n == 0) {
		n = 1;
	}
	if (g_alloc_warn_bytes > 0 && n >= g_alloc_warn_bytes) {
		fprintf(stderr, "stage0: <runtime>:0:0: warn large realloc %zu bytes\n", n);
	}
	if (g_alloc_max_bytes > 0 && n >= g_alloc_max_bytes) {
		fprintf(stderr, "stage0: <runtime>:0:0: error realloc %zu exceeds max %zu bytes\n", n, g_alloc_max_bytes);
		exit(1);
	}
	DastAllocHeader *h = ((DastAllocHeader *)p) - 1;
	size_t old = h->size;
	size_t inc = n > old ? (n - old) : 0;
	size_t dec = old > n ? (old - n) : 0;
	size_t cur = 0;
	if (inc > 0 && !dast_alloc_total_reserve(inc, &cur, NULL)) {
		dast_alloc_total_fail("realloc", inc, cur, g_alloc_total_max_bytes);
	}
	void *oldp = p;
	DastAllocHeader *nh = (DastAllocHeader *)realloc(h, sizeof(DastAllocHeader) + n);
	if (!nh) {
		if (inc > 0) {
			dast_alloc_total_release(inc);
		}
		dast_rt_panic("out of memory");
	}
	nh->size = n;
	if (dec > 0) {
		dast_alloc_total_release(dec);
	}
	void *out = (void *)(nh + 1);
	if (out != oldp) {
		dast_freed_add(oldp, old);
	}
	dast_freed_remove_exact(out);
	return out;
}

static void dast_xfree(void *p) {
	if (!p) {
		return;
	}
	dast_str_debug_init();
	DastAllocHeader *h = ((DastAllocHeader *)p) - 1;
	size_t old = h->size;
	dast_freed_add(p, old);
	dast_alloc_total_release(old);
	free(h);
}

static void dast_track_string(DastString *s) {
	if (!s) {
		return;
	}
	for (size_t i = 0; i < g_string_allocs_len; i++) {
		if (g_string_allocs[i] == s) {
			return;
		}
	}
	if (g_string_allocs_len >= g_string_allocs_cap) {
		size_t next = g_string_allocs_cap == 0 ? 64 : g_string_allocs_cap * 2;
		g_string_allocs = (DastString **)dast_xrealloc(g_string_allocs, sizeof(DastString *) * next);
		g_string_allocs_cap = next;
	}
	g_string_allocs[g_string_allocs_len++] = s;
}

static int dast_untrack_string(DastString *s) {
	if (!s) {
		return 0;
	}
	for (size_t i = 0; i < g_string_allocs_len; i++) {
		if (g_string_allocs[i] == s) {
			g_string_allocs[i] = g_string_allocs[g_string_allocs_len - 1];
			g_string_allocs_len--;
			return 1;
		}
	}
	return 0;
}

static void dast_track_array(DastArray *arr) {
	if (!g_drop_debug || !arr) {
		return;
	}
	for (size_t i = 0; i < g_array_allocs_len; i++) {
		if (g_array_allocs[i] == arr) {
			return;
		}
	}
	if (g_array_allocs_len >= g_array_allocs_cap) {
		size_t next = g_array_allocs_cap == 0 ? 64 : g_array_allocs_cap * 2;
		g_array_allocs = (void **)dast_xrealloc(g_array_allocs, sizeof(void *) * next);
		g_array_allocs_cap = next;
	}
	g_array_allocs[g_array_allocs_len++] = arr;
}

static int dast_untrack_array(DastArray *arr) {
	if (!g_drop_debug || !arr) {
		return 1;
	}
	for (size_t i = 0; i < g_array_allocs_len; i++) {
		if (g_array_allocs[i] == arr) {
			g_array_allocs[i] = g_array_allocs[g_array_allocs_len - 1];
			g_array_allocs_len--;
			return 1;
		}
	}
	return 0;
}

static void dast_track_struct(DastStruct *st) {
	if (!g_drop_debug || !st) {
		return;
	}
	for (size_t i = 0; i < g_struct_allocs_len; i++) {
		if (g_struct_allocs[i] == st) {
			return;
		}
	}
	if (g_struct_allocs_len >= g_struct_allocs_cap) {
		size_t next = g_struct_allocs_cap == 0 ? 64 : g_struct_allocs_cap * 2;
		g_struct_allocs = (void **)dast_xrealloc(g_struct_allocs, sizeof(void *) * next);
		g_struct_allocs_cap = next;
	}
	g_struct_allocs[g_struct_allocs_len++] = st;
}

static int dast_untrack_struct(DastStruct *st) {
	if (!g_drop_debug || !st) {
		return 1;
	}
	for (size_t i = 0; i < g_struct_allocs_len; i++) {
		if (g_struct_allocs[i] == st) {
			g_struct_allocs[i] = g_struct_allocs[g_struct_allocs_len - 1];
			g_struct_allocs_len--;
			return 1;
		}
	}
	return 0;
}

static int dast_struct_is_live(DastStruct *st) {
	if (!g_drop_debug || !st) {
		return 1;
	}
	for (size_t i = 0; i < g_struct_allocs_len; i++) {
		if (g_struct_allocs[i] == st) {
			return 1;
		}
	}
	return 0;
}

static void dast_drop_debug_report(void) {
	if (!g_drop_debug || g_drop_debug_reported) {
		return;
	}
	g_drop_debug_reported = 1;
	if (g_string_allocs_len == 0 && g_array_allocs_len == 0 && g_struct_allocs_len == 0) {
		return;
	}
	fprintf(stderr,
		"stage0: <runtime>:0:0: error drop check failed: strings=%zu arrays=%zu structs=%zu\n",
		g_string_allocs_len, g_array_allocs_len, g_struct_allocs_len);
	_Exit(1);
}

void dast_string_free(DastString *s) {
	if (!s) {
		return;
	}
	if (!dast_untrack_string(s)) {
		dast_str_debug_init();
		if (dast_ptr_in_freed(s)) {
			if (g_drop_debug) {
				fprintf(stderr, "stage0: <runtime>:0:0: double free string %p\n", (void *)s);
				dast_debug_backtrace();
				dast_rt_panic("string double free");
			}
		}
		// Likely a static string literal; do not attempt to free it.
		return;
	}
	if (s->data) {
		dast_xfree(s->data);
	}
	dast_xfree(s);
}

void dast_set_args(int argc, char **argv) {
	dast_drop_debug_init();
	g_argc = argc;
	g_argv = argv;
}

DastArray *dast_array_new(dast_int cap) {
	if (cap < 0) {
		cap = 0;
	}
	dast_array_debug_init();
	DastArray *arr = (DastArray *)dast_xmalloc(sizeof(DastArray));
	dast_track_array(arr);
	arr->len = 0;
	arr->cap = cap > 0 ? cap : 4;
	arr->data = (dast_int *)dast_xmalloc(sizeof(dast_int) * (size_t)arr->cap);
	arr->debug_id = g_next_array_id++;
	if (dast_array_debug_match(arr)) {
		fprintf(stderr, "stage0: <runtime>:0:0: array new id=%lld arr=%p cap=%lld\n",
		        (long long)arr->debug_id, (void *)arr, (long long)arr->cap);
	}
	return arr;
}

static void dast_array_grow(DastArray *arr, dast_int need) {
	if (arr->cap >= need) {
		return;
	}
	dast_array_debug_init();
	dast_int cap = arr->cap > 0 ? arr->cap : 4;
	while (cap < need) {
		cap *= 2;
	}
	dast_alloc_debug_init();
	if (g_alloc_warn_bytes > 0) {
		size_t bytes = sizeof(dast_int) * (size_t)cap;
		if (bytes >= g_alloc_warn_bytes) {
			fprintf(stderr,
				"stage0: <runtime>:0:0: warn array grow id=%lld arr=%p need=%lld cap=%lld bytes=%zu\n",
				(long long)arr->debug_id, (void *)arr, (long long)need, (long long)cap, bytes);
			if (g_array_bt) {
				dast_debug_backtrace();
			}
		}
	}
	arr->cap = cap;
	arr->data = (dast_int *)dast_xrealloc(arr->data, sizeof(dast_int) * (size_t)arr->cap);
	if (dast_array_debug_match(arr)) {
		fprintf(stderr,
		        "stage0: <runtime>:0:0: array grow id=%lld arr=%p cap=%lld need=%lld\n",
		        (long long)arr->debug_id, (void *)arr, (long long)arr->cap, (long long)need);
	}
}

void dast_array_push(DastArray *arr, dast_int value) {
	if (!arr) {
		dast_rt_panic("push expects array");
	}
	dast_array_grow(arr, arr->len + 1);
	arr->data[arr->len++] = value;
	if (dast_array_debug_match(arr)) {
		fprintf(stderr,
		        "stage0: <runtime>:0:0: array push id=%lld len=%lld cap=%lld\n",
		        (long long)arr->debug_id, (long long)arr->len, (long long)arr->cap);
		if (g_array_bt) {
			dast_debug_backtrace();
		}
	}
}

dast_int dast_array_get(DastArray *arr, dast_int index) {
	if (!arr) {
		dast_rt_panic("indexing requires array");
	}
	if (dast_array_debug_match(arr)) {
		fprintf(stderr,
		        "stage0: <runtime>:0:0: array get id=%lld index=%lld len=%lld\n",
		        (long long)arr->debug_id, (long long)index, (long long)arr->len);
		if (g_array_bt) {
			dast_debug_backtrace();
		}
	}
	if (index < 0 || index >= arr->len) {
		fprintf(stderr,
		        "stage0: <runtime>:0:0: error index %lld out of bounds len=%lld id=%lld\n",
		        (long long)index, (long long)arr->len, (long long)arr->debug_id);
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
	if (dast_array_debug_match(arr)) {
		fprintf(stderr,
		        "stage0: <runtime>:0:0: array set id=%lld index=%lld len=%lld\n",
		        (long long)arr->debug_id, (long long)index, (long long)arr->len);
		if (g_array_bt) {
			dast_debug_backtrace();
		}
	}
	if (index < 0 || index >= arr->len) {
		fprintf(stderr,
		        "stage0: <runtime>:0:0: error index %lld out of bounds len=%lld id=%lld\n",
		        (long long)index, (long long)arr->len, (long long)arr->debug_id);
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
		fprintf(stderr,
		        "stage0: <runtime>:0:0: error index %lld out of bounds len=%lld id=%lld\n",
		        (long long)index, (long long)arr->len, (long long)arr->debug_id);
		dast_rt_panic("index out of bounds");
	}
	return &arr->data[index];
}

dast_int dast_array_len(DastArray *arr) {
	if (!arr) {
		return 0;
	}
	if (dast_array_debug_match(arr)) {
		fprintf(stderr, "stage0: <runtime>:0:0: array len id=%lld len=%lld\n",
		        (long long)arr->debug_id, (long long)arr->len);
		if (g_array_bt) {
			dast_debug_backtrace();
		}
	}
	return arr->len;
}

static dast_int dast_struct_index(DastStruct *st, const char *field) {
	dast_str_debug_init();
	if (!dast_struct_is_live(st)) {
		dast_debug_backtrace();
		if (g_drop_debug) {
			const char *sname = st && st->name ? dast_safe_cstr(st->name) : "<struct>";
			fprintf(stderr, "stage0: <runtime>:0:0: error use-after-free struct %p (%s)\n", (void *)st, sname);
		}
		dast_rt_panic("use-after-free struct");
	}
	if (!field) {
		const char *sname = st && st->name ? dast_safe_cstr(st->name) : "<struct>";
		fprintf(stderr, "stage0: <runtime>:0:0: error null field name in %s\n", sname);
		dast_rt_panic("null field name");
	}
	uintptr_t fptr = (uintptr_t)field;
	if (fptr < DAST_MIN_VALID_PTR) {
		const char *sname = st && st->name ? dast_safe_cstr(st->name) : "<struct>";
		fprintf(stderr,
		        "stage0: <runtime>:0:0: error invalid field pointer %p in %s\n",
		        (void *)field, sname);
		dast_debug_backtrace();
		dast_rt_panic("invalid field pointer");
	}
	if (g_str_debug && fptr >= DAST_MIN_VALID_PTR && field[0] == '_') {
		const char *sname = st && st->name ? dast_safe_cstr(st->name) : "<struct>";
		fprintf(stderr, "stage0: <runtime>:0:0: debug struct %p (%s) field=%s len=%lld\n", (void *)st, sname, field,
		        (long long)st->field_len);
	}
	for (dast_int i = 0; i < st->field_len; i++) {
		if (!st->field_names[i]) {
			continue;
		}
		if (strcmp(st->field_names[i], field) == 0) {
			return i;
		}
	}
	return -1;
}

DastStruct *dast_struct_new(const char *name, dast_int field_count) {
	DastStruct *st = (DastStruct *)dast_xmalloc(sizeof(DastStruct));
	dast_track_struct(st);
	st->name = name ? name : "";
	if (name && (uintptr_t)name < DAST_MIN_VALID_PTR) {
		fprintf(stderr, "stage0: <runtime>:0:0: error invalid struct name pointer %p\n", (void *)name);
		dast_debug_backtrace();
		dast_rt_panic("invalid struct name");
	}
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
	uintptr_t fptr = (uintptr_t)field;
	if (g_str_debug && field && fptr >= DAST_MIN_VALID_PTR && field[0] == '_') {
		const char *sname = st->name ? dast_safe_cstr(st->name) : "<struct>";
		const char *fname = dast_safe_cstr(field);
		fprintf(stderr, "stage0: <runtime>:0:0: debug set %p (%s) field=%s val=%lld len=%lld\n", (void *)st, sname,
		        fname, (long long)value, (long long)st->field_len);
	}
	dast_int idx = dast_struct_index(st, field);
	if (idx >= 0) {
		st->field_values[idx] = value;
		return;
	}
	if (st->field_expected > 0 && st->field_len >= st->field_expected) {
		/* Allow growth for unnamed structs (legacy dynamic usage). */
		const char *sname_safe = st->name ? dast_safe_cstr(st->name) : "";
		if (st->name && sname_safe[0] != '<' && sname_safe[0] != '\0') {
			const char *fname = field ? field : "<null>";
			const char *sname = st->name ? sname_safe : "<struct>";
			fprintf(stderr, "stage0: <runtime>:0:0: error unknown field '%s' in %s\n", fname, sname);
			dast_debug_backtrace();
			exit(1);
		}
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
		const char *fname = dast_safe_cstr(field);
		const char *sname = st && st->name ? dast_safe_cstr(st->name) : "<struct>";
		fprintf(stderr, "stage0: <runtime>:0:0: error unknown field '%s' in %s\n", fname, sname);
		dast_debug_backtrace();
		if (st->field_len > 0 && st->field_names) {
			fprintf(stderr, "stage0: <runtime>:0:0: note fields:");
			for (dast_int i = 0; i < st->field_len; i++) {
				const char *nm = st->field_names[i] ? dast_safe_cstr(st->field_names[i]) : "<null>";
				fprintf(stderr, " %s", nm);
			}
			fprintf(stderr, "\n");
		}
		exit(1);
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

void dast_struct_set_string(DastStruct *st, const char *field, DastString *value) {
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

const DastString *dast_struct_get_string(DastStruct *st, const char *field) {
	return (const DastString *)(intptr_t)dast_struct_get_value(st, field);
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
		const char *fname = dast_safe_cstr(field);
		const char *sname = st && st->name ? dast_safe_cstr(st->name) : "<struct>";
		fprintf(stderr, "stage0: <runtime>:0:0: error unknown field '%s' in %s\n", fname, sname);
		exit(1);
	}
	return &st->field_values[idx];
}

static DastString *dast_string_alloc(size_t len) {
	DastString *s = (DastString *)dast_xmalloc(sizeof(DastString));
	s->len = (dast_int)len;
	s->cap = (dast_int)len;
	s->data = (char *)dast_xmalloc(len + 1);
	s->data[len] = '\0';
	dast_track_string(s);
	return s;
}

static DastString *dast_string_from_cstr(const char *src) {
	dast_str_debug_init();
	if (g_str_debug) {
		fprintf(stderr, "stage0: <runtime>:0:0: debug string_from_cstr %p\n", (void *)src);
	}
	if (!src) {
		return dast_string_alloc(0);
	}
	if (g_str_debug) {
		uintptr_t sp = (uintptr_t)src;
		if (sp < DAST_MIN_VALID_PTR) {
			fprintf(stderr, "stage0: <runtime>:0:0: invalid cstr pointer %p\n", (void *)src);
			dast_debug_backtrace();
			dast_rt_panic("invalid cstr pointer");
		}
		if (dast_ptr_in_freed(src)) {
			fprintf(stderr, "stage0: <runtime>:0:0: cstr points to freed memory %p\n", (void *)src);
			dast_debug_backtrace();
			dast_rt_panic("invalid cstr pointer");
		}
		for (size_t i = 0; i < g_string_allocs_len; i++) {
			if ((const char *)g_string_allocs[i] == src) {
				fprintf(stderr,
					"stage0: <runtime>:0:0: cstr is DastString* %p (len=%lld cap=%lld)\n",
					(void *)src,
					(long long)g_string_allocs[i]->len,
					(long long)g_string_allocs[i]->cap);
				dast_debug_backtrace();
				dast_rt_panic("invalid cstr pointer");
			}
		}
	}
	size_t len = strlen(src);
	DastString *s = dast_string_alloc(len);
	if (len > 0) {
		memcpy(s->data, src, len);
	}
	return s;
}

static DastString *dast_string_wrap(char *buf, size_t len, size_t cap) {
	DastString *s = (DastString *)dast_xmalloc(sizeof(DastString));
	s->data = buf;
	s->len = (dast_int)len;
	s->cap = (dast_int)cap;
	dast_track_string(s);
	return s;
}

dast_int dast_string_len(const DastString *s) {
	if (!s) {
		return 0;
	}
	return s->len;
}

dast_bool dast_string_eq(const DastString *a, const DastString *b) {
	if (!a || !b) {
		return a == b;
	}
	if (a->len != b->len) {
		return 0;
	}
	if (a->len == 0) {
		return 1;
	}
	return memcmp(a->data, b->data, (size_t)a->len) == 0;
}

DastString *dast_string_concat(const DastString *a, const DastString *b) {
	size_t la = a ? (size_t)a->len : 0;
	size_t lb = b ? (size_t)b->len : 0;
	DastString *out = dast_string_alloc(la + lb);
	if (la > 0) {
		memcpy(out->data, a->data, la);
	}
	if (lb > 0) {
		memcpy(out->data + la, b->data, lb);
	}
	return out;
}

DastString *dast_string_clone(const DastString *s) {
	if (!s) {
		return dast_string_alloc(0);
	}
	DastString *out = dast_string_alloc((size_t)s->len);
	if (s->len > 0) {
		memcpy(out->data, s->data, (size_t)s->len);
	}
	return out;
}

static void dast_print_i64_to(FILE *out, dast_int v) {
	fprintf(out, "%lld", (long long)v);
}

static void dast_print_bool_to(FILE *out, dast_bool v) {
	fputs(v ? "true" : "false", out);
}

static void dast_print_string_to(FILE *out, const DastString *s) {
	if (!s || !s->data) {
		fputs("", out);
		return;
	}
	fputs(s->data, out);
}

static void dast_print_struct_to(FILE *out, DastStruct *st) {
	if (!st) {
		fputs("<struct>", out);
		return;
	}
	fputs(dast_safe_cstr(st->name), out);
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
void dast_print_string(const DastString *s) { dast_print_string_to(stdout, s); }
void dast_print_struct(dast_int v) { dast_print_struct_to(stdout, (DastStruct *)(intptr_t)v); }
void dast_print_array(dast_int v) { dast_print_array_to(stdout, (DastArray *)(intptr_t)v); }
void dast_print_ptr(dast_int v) { dast_print_ptr_to(stdout, v); }

void dast_eprint_i64(dast_int v) { dast_print_i64_to(stderr, v); }
void dast_eprint_bool(dast_bool v) { dast_print_bool_to(stderr, v); }
void dast_eprint_string(const DastString *s) { dast_print_string_to(stderr, s); }
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

DastString *dast_substr(const DastString *s, dast_int start, dast_int len);

dast_int dast_char_at(const DastString *s, dast_int idx) {
	if (!s || !s->data) {
		dast_rt_panic("char_at expects string");
	}
	dast_int slen = s->len;
	if (idx < 0 || idx >= slen) {
		dast_rt_panic("char_at index out of bounds");
	}
	return (unsigned char)s->data[idx];
}

DastString *dast_substr(const DastString *s, dast_int start, dast_int len) {
	if (!s || !s->data) {
		dast_rt_panic("substr expects string");
	}
	dast_int slen = s->len;
	if (start < 0 || len < 0 || start > slen || start + len > slen) {
		dast_rt_panic("substr out of bounds");
	}
	DastString *out = dast_string_alloc((size_t)len);
	if (len > 0) {
		memcpy(out->data, s->data + start, (size_t)len);
	}
	return out;
}

DastString *dast_read_file(const DastString *path) {
	if (!path || !path->data) {
		dast_rt_panic("read_file expects string path");
	}
	FILE *f = fopen(path->data, "rb");
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
	return dast_string_wrap(buf, read, read);
}

void dast_write_file(const DastString *path, const DastString *data) {
	if (!path || !path->data || !data || !data->data) {
		dast_rt_panic("write_file expects string path and data");
	}
	FILE *f = fopen(path->data, "wb");
	if (!f) {
		dast_rt_panic("write_file failed");
	}
	fwrite(data->data, 1, (size_t)data->len, f);
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

void dast_mkdir(const DastString *path) {
	if (!path || !path->data) {
		dast_rt_panic("mkdir expects string path");
	}
	if (dast_mkdir_p(path->data) != 0) {
		dast_rt_panic("mkdir failed");
	}
}

DastArray *dast_read_dir(const DastString *path) {
	if (!path || !path->data) {
		dast_rt_panic("read_dir expects string path");
	}
	DIR *dir = opendir(path->data);
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
		DastString *dup = dast_string_from_cstr(ent->d_name);
		dast_array_push(arr, (dast_int)(intptr_t)dup);
	}
	closedir(dir);
	return arr;
}

DastArray *dast_args(void) {
	DastArray *arr = dast_array_new(g_argc);
	for (int i = 0; i < g_argc; i++) {
		DastString *arg = dast_string_from_cstr(g_argv[i]);
		dast_array_push(arr, (dast_int)(intptr_t)arg);
	}
	return arr;
}

DastString *dast_read_line(void) {
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
	return dast_string_wrap(buf, len, cap);
}

DastString *dast_read_bytes(dast_int count) {
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
	return dast_string_wrap(buf, total, total);
}

dast_int dast_exec(const DastString *cmd, DastArray *args) {
	if (!cmd || !cmd->data) {
		dast_rt_panic("exec expects string command");
	}
	size_t argc = args ? (size_t)args->len : 0;
	char **argv = (char **)dast_xmalloc(sizeof(char *) * (argc + 2));
	argv[0] = cmd->data;
	for (size_t i = 0; i < argc; i++) {
		DastString *arg = (DastString *)(intptr_t)args->data[i];
		argv[i + 1] = arg ? arg->data : "";
	}
	argv[argc + 1] = NULL;
	pid_t pid = fork();
	if (pid == 0) {
		execvp(cmd->data, argv);
		_exit(127);
	}
	int status = 0;
	if (pid > 0) {
		waitpid(pid, &status, 0);
	}
	dast_xfree(argv);
	if (pid <= 0) {
		dast_rt_panic("exec failed");
	}
	if (WIFEXITED(status)) {
		return (dast_int)WEXITSTATUS(status);
	}
	return 1;
}

DastString *dast_int_to_string(dast_int v) {
	char buf[64];
	snprintf(buf, sizeof(buf), "%lld", (long long)v);
	return dast_string_from_cstr(buf);
}

dast_i32 dast_parse_int(const DastString *s) {
	if (!s || !s->data) {
		return 0;
	}
	char *end = NULL;
	long v = strtol(s->data, &end, 10);
	if (end == s->data) {
		return 0;
	}
	return (dast_i32)v;
}

dast_int dast_string_to_int(const DastString *s) {
	if (!s || !s->data) {
		return 0;
	}
	char *end = NULL;
	long long v = strtoll(s->data, &end, 10);
	if (end == s->data) {
		return 0;
	}
	return (dast_int)v;
}

dast_bool dast_has_prefix(const DastString *s, const DastString *prefix) {
	if (!s || !prefix || !s->data || !prefix->data) {
		return 0;
	}
	size_t plen = (size_t)prefix->len;
	if (plen == 0) {
		return 1;
	}
	if ((size_t)s->len < plen) {
		return 0;
	}
	return strncmp(s->data, prefix->data, plen) == 0 ? 1 : 0;
}

void dast_exit(dast_int code) {
	exit((int)code);
}

static DastString *dast_sanitize_ident(const DastString *s) {
	if (!s || !s->data || s->len == 0) {
		return dast_string_from_cstr("tmp");
	}
	size_t n = (size_t)s->len;
	DastString *out = dast_string_alloc(n);
	for (size_t i = 0; i < n; i++) {
		char c = s->data[i];
		if ((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			out->data[i] = c;
		} else {
			out->data[i] = '_';
		}
	}
	return out;
}

void dast_array_free(DastArray *arr) {
	if (!arr) {
		return;
	}
	if (!dast_untrack_array(arr)) {
		dast_rt_panic("array double free");
	}
	dast_xfree(arr->data);
	dast_xfree(arr);
}

void dast_struct_free(DastStruct *st) {
	if (!st) {
		return;
	}
	if (g_drop_debug) {
		const char *sname = st->name ? dast_safe_cstr(st->name) : "<struct>";
		fprintf(stderr, "stage0: <runtime>:0:0: drop struct %p (%s)\n", (void *)st, sname);
	}
	if (!dast_untrack_struct(st)) {
		dast_rt_panic("struct double free");
	}
	dast_xfree(st->field_names);
	dast_xfree(st->field_values);
	dast_xfree(st);
}

DastString *dast_ast_expr1(const DastString *src) {
	if (!src || !src->data) {
		return dast_string_alloc(0);
	}
	return dast_string_from_cstr(src->data);
}

DastString *dast_ast_expr2(const DastString *src, DastArray *splices) {
	(void)splices;
	if (!src || !src->data) {
		return dast_string_alloc(0);
	}
	return dast_string_from_cstr(src->data);
}

DastString *dast_ast_stmt1(const DastString *src) {
	return dast_ast_expr1(src);
}

DastString *dast_ast_stmt2(const DastString *src, DastArray *splices) {
	return dast_ast_expr2(src, splices);
}

DastString *dast_ast_item1(const DastString *src) {
	return dast_ast_expr1(src);
}

DastString *dast_ast_item2(const DastString *src, DastArray *splices) {
	return dast_ast_expr2(src, splices);
}

DastString *dast_ast_block1(const DastString *src) {
	return dast_ast_expr1(src);
}

DastString *dast_ast_block2(const DastString *src, DastArray *splices) {
	return dast_ast_expr2(src, splices);
}

DastString *dast_ast_to_string(const DastString *ast) {
	if (!ast || !ast->data) {
		return dast_string_alloc(0);
	}
	return dast_string_from_cstr(ast->data);
}

DastString *dast_gensym(const DastString *prefix) {
	static dast_int counter = 0;
	DastString *clean = dast_sanitize_ident(prefix);
	char buf[128];
	snprintf(buf, sizeof(buf), "__dast_%s_%lld", clean->data ? clean->data : "", (long long)counter++);
	dast_string_free(clean);
	return dast_string_from_cstr(buf);
}

DastString *dast_bind(const DastString *name) {
	if (!name || !name->data) {
		return dast_string_alloc(0);
	}
	return dast_sanitize_ident(name);
}
