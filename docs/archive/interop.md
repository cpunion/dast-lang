# 互操作性设计

## 设计原则

1. **C FFI 为核心**: 与 C 的完美互操作是基础
2. **渐进式扩展**: C++ 等复杂语言待元编程成熟后支持
3. **双向调用**: Dast 调用 C，C 调用 Dast

---

## C FFI

### 调用 C 函数

```
// 声明外部 C 函数
extern "C" {
    fn printf(format: *const c_char, ...) -> c_int
    fn malloc(size: c_size) -> *mut c_void
    fn free(ptr: *mut c_void)
}

// 使用
fn main() {
    let msg = c"Hello, %s!\n"
    printf(msg, c"World")
}
```

### 导出给 C 调用

```
// 导出为 C ABI
@export("dast_calculate")
fn calculate(a: c_int, b: c_int) -> c_int {
    return a + b
}

// 生成的 C 头文件:
// int dast_calculate(int a, int b);
```

### 结构体互操作

```
// C 兼容布局
@repr(C)
struct Point {
    x: f32,
    y: f32,
}

// 与 C 结构体二进制兼容
// struct Point { float x; float y; };
```

---

## 类型映射

| Dast 类型 | C 类型 |
|-----------|--------|
| `i8/u8` | `int8_t/uint8_t` |
| `i32/u32` | `int32_t/uint32_t` |
| `isize/usize` | `ssize_t/size_t` |
| `*T / *mut T` | `const T* / T*` |
| `c_char` | `char` |
| `c_void` | `void` |

---

## 平台相关

### WASM/JS 互操作

```
// 导入 JS 函数
@wasm_import("env", "console_log")
extern fn js_log(ptr: *const u8, len: usize)

// 导出给 JS
@wasm_export
fn add(a: i32, b: i32) -> i32 {
    return a + b
}
```

---

## 未来扩展 (待元编程支持后)

- [ ] C++ 类/虚函数绑定
- [ ] Objective-C 消息传递
- [ ] Swift 互操作

---

## 待讨论

- [ ] 回调函数的封装模式
- [ ] 字符串转换便利 API
- [ ] 自动绑定生成工具
