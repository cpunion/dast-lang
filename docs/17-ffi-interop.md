# FFI 与互操作

## 设计原则

1. **C FFI 为核心** - 与 C 的完美互操作
2. **零成本抽象** - 无运行时开销
3. **双向调用** - Dast ↔ C 互相调用
4. **类型安全** - 编译期检查

---

## C FFI

### 调用 C 函数

```dast
// 声明外部 C 函数
extern "C" {
    fn printf(format: *const c_char, ...) -> c_int
    fn malloc(size: c_size) -> *mut c_void
    fn free(ptr: *mut c_void)
}

// 使用
fn main() {
    unsafe {
        let msg = c"Hello, %s!\n"
        printf(msg, c"World")
    }
}
```

### 导出给 C 调用

```dast
// 导出为 C ABI
@export("dast_calculate")
@no_mangle
fn calculate(a: c_int, b: c_int) -> c_int {
    return a + b
}

// 生成的 C 头文件:
// int dast_calculate(int a, int b);
```

### 结构体互操作

```dast
// C 兼容布局
@repr(C)
struct Point {
    x: f32,
    y: f32,
}

// 与 C 结构体二进制兼容
// struct Point { float x; float y; };

// 使用
extern "C" {
    fn process_point(p: *const Point)
}

let p = Point { x: 1.0, y: 2.0 }
unsafe {
    process_point(&p)
}
```

---

## 类型映射

### 基础类型

| Dast 类型 | C 类型 | 说明 |
|-----------|--------|------|
| `i8` | `int8_t` | 8 位有符号 |
| `u8` | `uint8_t` | 8 位无符号 |
| `i16` | `int16_t` | 16 位有符号 |
| `u16` | `uint16_t` | 16 位无符号 |
| `i32` | `int32_t` | 32 位有符号 |
| `u32` | `uint32_t` | 32 位无符号 |
| `i64` | `int64_t` | 64 位有符号 |
| `u64` | `uint64_t` | 64 位无符号 |
| `isize` | `ssize_t` | 指针大小有符号 |
| `usize` | `size_t` | 指针大小无符号 |
| `f32` | `float` | 32 位浮点 |
| `f64` | `double` | 64 位浮点 |

### 指针类型

| Dast 类型 | C 类型 |
|-----------|--------|
| `*const T` | `const T*` |
| `*mut T` | `T*` |
| `&T` | `const T*` (临时) |
| `&mut T` | `T*` (临时) |

### 特殊类型

```dast
// C 兼容类型别名
type c_char = i8
type c_int = i32
type c_uint = u32
type c_long = i64  // 平台相关
type c_ulong = u64
type c_float = f32
type c_double = f64
type c_void = ()
type c_size = usize
```

---

## 字符串处理

### C 字符串字面量

```dast
// C 字符串（以 \0 结尾）
let msg = c"Hello, World!"
// 类型: *const c_char

// 使用
extern "C" {
    fn puts(s: *const c_char) -> c_int
}

unsafe {
    puts(c"Hello from Dast!")
}
```

### 字符串转换

```dast
// Dast String -> C 字符串
let s = String.from("Hello")
let c_str = s.as_c_str()  // CString
unsafe {
    printf(c"%s\n", c_str.as_ptr())
}

// C 字符串 -> Dast String
extern "C" {
    fn get_name() -> *const c_char
}

unsafe {
    let c_str = get_name()
    let s = CStr.from_ptr(c_str).to_string()
}
```

---

## 回调函数

### 函数指针

```dast
// C 回调类型
type Callback = extern "C" fn(i32) -> i32

extern "C" {
    fn register_callback(cb: Callback)
}

// Dast 函数作为回调
extern "C" fn my_callback(x: i32) -> i32 {
    return x * 2
}

unsafe {
    register_callback(my_callback)
}
```

### 闭包封装

```dast
// 闭包 -> C 回调（需要 trampoline）
fn with_callback[F: FnMut(i32) -> i32](f: F) {
    // 将闭包包装为 C 回调
    let ctx = Box.new(f)
    let ctx_ptr = Box.into_raw(ctx) as *mut c_void

    extern "C" fn trampoline[F](ctx: *mut c_void, x: i32) -> i32
    where F: FnMut(i32) -> i32 {
        unsafe {
            let f = &mut *(ctx as *mut F)
            f(x)
        }
    }

    unsafe {
        register_callback_with_context(trampoline::[F], ctx_ptr)
    }
}
```

---

## WASM 互操作

### 导入 JS 函数

```dast
// 从 JS 导入
@wasm_import("env", "console_log")
extern fn js_log(ptr: *const u8, len: usize)

@wasm_import("env", "fetch")
extern fn js_fetch(url: *const u8, url_len: usize) -> i32

// 使用
fn log_message(msg: &str) {
    unsafe {
        js_log(msg.as_ptr(), msg.len())
    }
}
```

### 导出给 JS

```dast
// 导出给 JS
@wasm_export
fn add(a: i32, b: i32) -> i32 {
    return a + b
}

@wasm_export
fn process_data(ptr: *mut u8, len: usize) {
    unsafe {
        let data = slice.from_raw_parts_mut(ptr, len)
        // 处理数据
    }
}
```

### 内存管理

```dast
// WASM 内存分配器
@wasm_export
fn alloc(size: usize) -> *mut u8 {
    let layout = Layout.from_size_align(size, 1).unwrap()
    unsafe {
        std.alloc.alloc(layout)
    }
}

@wasm_export
fn dealloc(ptr: *mut u8, size: usize) {
    let layout = Layout.from_size_align(size, 1).unwrap()
    unsafe {
        std.alloc.dealloc(ptr, layout)
    }
}
```

---

## 构建配置

### 生成 C 头文件

```bash
# 自动生成 C 头文件
$ dast build --emit=c-header

# 输出: target/dast.h
```

### 链接 C 库

```toml
# dast.toml
[dependencies]
libz-sys = { version = "1.0", features = ["static"] }

[build]
links = ["z", "m"]  # 链接 libz 和 libm
```

---

## 平台特定

### Windows DLL

```dast
@export("MyFunction")
@dllexport  // Windows 特定
fn my_function() {
    // ...
}
```

### macOS Framework

```bash
$ dast build --target=aarch64-apple-darwin --crate-type=dylib
$ install_name_tool -id @rpath/MyLib.dylib target/libmylib.dylib
```

---

## 安全性

### Unsafe 边界

```dast
// ❌ 不安全: 直接暴露 unsafe
pub fn process(ptr: *mut u8, len: usize) {
    unsafe {
        // 直接操作裸指针
    }
}

// ✅ 安全: 封装 unsafe
pub fn process(data: &mut [u8]) {
    unsafe {
        // 内部使用 unsafe，但接口安全
        ffi_process(data.as_mut_ptr(), data.len())
    }
}
```

---

## 总结

| 特性 | 支持 |
|------|------|
| C FFI | ✅ 完整 |
| 导出/导入 | ✅ 双向 |
| 结构体布局 | ✅ @repr(C) |
| 回调函数 | ✅ 函数指针 |
| WASM 互操作 | ✅ 导入/导出 |
| 自动头文件生成 | ✅ --emit=c-header |
| 类型安全 | ✅ 编译期检查 |
