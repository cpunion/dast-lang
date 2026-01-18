# Drop 与 Unsafe 机制

## Drop 析构机制

### 核心原则

1. **自动调用** - 值离开作用域时自动执行
2. **逆序析构** - 后创建先释放 (LIFO)
3. **确定性** - 编译期确定调用时机
4. **不可手动调用** - 防止双重释放

---

## Drop Trait 设计

### 定义

```
trait Drop {
    fn drop(self: &mut Self)
}
```

### 实现示例

```
struct File {
    fd: i32,
    path: String,
}

impl Drop for File {
    fn drop(self: &mut Self) {
        println("Closing file: {}", self.path)
        close(self.fd)
    }
}

// 使用
fn example() {
    let f1 = File.open("a.txt")  // 1. 创建
    let f2 = File.open("b.txt")  // 2. 创建

    process(f1, f2)

}  // ← 作用域结束
   //   3. f2.drop() 先调用
   //   4. f1.drop() 后调用 (逆序)
```

---

## Drop 调用时机

### 1. 正常作用域结束

```
fn normal_scope() {
    let a = Resource.new("a")
    {
        let b = Resource.new("b")
        let c = Resource.new("c")
    }  // c.drop(), b.drop()

    let d = Resource.new("d")
}  // d.drop(), a.drop()
```

### 2. 提前返回 (return)

```
fn early_return(cond: bool) {
    let a = Resource.new("a")

    if cond {
        let b = Resource.new("b")
        return  // b.drop(), a.drop() 在此调用
    }

    let c = Resource.new("c")
}  // c.drop(), a.drop()
```

### 3. 错误传播 (?)

```
fn with_error() -> Result<(), Error> {
    let a = Resource.new("a")
    let b = Resource.new("b")

    may_fail()?  // 如果失败: b.drop(), a.drop() 然后返回错误

    let c = Resource.new("c")
    return .ok(())
}  // c.drop(), b.drop(), a.drop()
```

### 4. Panic (运行时错误)

```
fn with_panic() {
    let a = Resource.new("a")
    let b = Resource.new("b")

    panic("error!")  // 栈展开: b.drop(), a.drop()
                     // 然后终止程序或被 catch
}
```

**设计决策**: 支持栈展开 (unwinding) 还是直接终止 (abort)?

| 模式 | 优点 | 缺点 | 适用场景 |
|------|------|------|---------|
| **Unwinding** | 资源正确释放 | 代码体积大、性能开销 | 主机应用 |
| **Abort** | 体积小、无开销 | 资源可能泄漏 | 嵌入式、性能关键 |

**建议**:
- 默认 unwinding (主机平台)
- `--panic=abort` 编译选项 (嵌入式/优化)

---

## Drop 的限制

### 1. 不可手动调用

```
let file = File.open("x.txt")
file.drop()  // 编译错误: drop 不能手动调用

// 原因: 作用域结束会再次调用 → 双重释放
```

### 2. 提前释放的正确方式

```
// 方式 1: 使用内部作用域
{
    let file = File.open("x.txt")
    process(file)
}  // file 在此释放

// 方式 2: 显式 drop 函数 (标准库提供)
let file = File.open("x.txt")
process(&file)
std.mem.drop(file)  // 移动所有权并立即析构
// file 不可再用
```

### 3. Drop 中不能 panic

```
impl Drop for Resource {
    fn drop(self: &mut Self) {
        // ❌ 危险: Drop 中 panic 导致双重 panic → 程序终止
        if !self.cleanup() {
            panic("cleanup failed")  // 编译警告或错误
        }

        // ✅ 正确: 记录错误但不 panic
        if !self.cleanup() {
            eprintln("Warning: cleanup failed for {}", self.name)
        }
    }
}
```

---

## Unsafe 机制

### 设计原则

1. **最小化** - 大部分代码应该是安全的
2. **显式标记** - unsafe 操作必须在 unsafe 块中
3. **审计友好** - 搜索 `unsafe` 关键字即可定位所有风险点
4. **文档化** - unsafe 块应注释说明为何安全

---

## Unsafe 操作类别

### 1. 解引用裸指针

```
fn raw_pointer_deref() {
    let x: i32 = 42
    let ptr: *const i32 = &x

    // let val = *ptr  // 编译错误: 需要 unsafe

    unsafe {
        let val = *ptr  // OK: 在 unsafe 块中
        println("{}", val)
    }
}
```

### 2. 调用 unsafe 函数

```
// 声明 unsafe 函数
unsafe fn dangerous_operation(ptr: *mut u8, len: usize) {
    // 假设 ptr 有效且长度正确
    for i in 0..len {
        *ptr.offset(i) = 0
    }
}

fn caller() {
    let mut buf = [0u8; 10]
    let ptr = buf.as_mut_ptr()

    // dangerous_operation(ptr, 10)  // 编译错误

    unsafe {
        dangerous_operation(ptr, 10)  // OK
    }
}
```

### 3. 访问可变静态变量

```
static mut COUNTER: i32 = 0

fn increment() {
    // COUNTER += 1  // 编译错误: 可变静态需要 unsafe

    unsafe {
        COUNTER += 1  // OK
    }
}
```

### 4. 实现 unsafe trait

```
// 标记为 unsafe 的 trait
unsafe trait Send {}

struct MyType {
    ptr: *mut i32,
}

// 实现时需要 unsafe 关键字
unsafe impl Send for MyType {}
// 程序员保证: MyType 可以安全地在线程间传递
```

### 5. 访问 union 字段

```
union Value {
    int: i32,
    float: f32,
}

fn read_union() {
    let v = Value { int: 42 }

    // let f = v.float  // 编译错误

    unsafe {
        let f = v.float  // OK: 程序员保证当前是 float
    }
}
```

---

## Unsafe 块的语法

### 基本形式

```
unsafe {
    // 不安全操作
}
```

### 表达式形式

```
let val = unsafe { *ptr }
```

### Unsafe 函数

```
unsafe fn low_level_api() {
    // 整个函数体都是 unsafe 上下文
    // 内部可以直接进行不安全操作
}
```

---

## 安全抽象

**核心思想**: 使用 unsafe 实现底层，暴露安全的高层 API

```
// 底层: unsafe 实现
struct Vec<T> {
    ptr: *mut T,
    len: usize,
    cap: usize,
}

impl<T> Vec<T> {
    // 内部使用 unsafe，但接口是安全的
    fn push(self: &mut Self, value: T) {
        if self.len == self.cap {
            self.grow()
        }

        unsafe {
            // 我们保证: len < cap，ptr 有效
            *self.ptr.offset(self.len) = value
        }
        self.len += 1
    }

    fn get(self: &Self, index: usize) -> Option<&T> {
        if index >= self.len {
            return .none
        }

        unsafe {
            // 我们保证: index < len，ptr 有效
            return .some(&*self.ptr.offset(index))
        }
    }
}

// 用户代码: 完全安全
fn user_code() {
    let vec = Vec.new()
    vec.push(42)
    vec.push(100)

    if let .some(val) = vec.get(0) {
        println("{}", val)
    }
}
```

---

## Unsafe 最佳实践

### 1. 最小化 unsafe 块

```
// ❌ 不好: 整个函数都是 unsafe
unsafe fn process(data: &[u8]) {
    let len = data.len()
    let ptr = data.as_ptr()

    for i in 0..len {
        let byte = *ptr.offset(i)
        process_byte(byte)
    }
}

// ✅ 好: 只在必要处使用 unsafe
fn process(data: &[u8]) {
    let len = data.len()
    let ptr = data.as_ptr()

    for i in 0..len {
        let byte = unsafe { *ptr.offset(i) }
        process_byte(byte)
    }
}

// ✅ 更好: 使用安全抽象
fn process(data: &[u8]) {
    for byte in data {
        process_byte(byte)
    }
}
```

### 2. 文档化 unsafe 的安全性

```
unsafe fn copy_nonoverlapping<T>(src: *const T, dst: *mut T, count: usize) {
    // SAFETY: 调用者必须保证:
    // 1. src 和 dst 都是有效指针
    // 2. src 和 dst 不重叠
    // 3. src 可读取 count 个 T
    // 4. dst 可写入 count 个 T

    unsafe {
        intrinsics.copy_nonoverlapping(src, dst, count)
    }
}
```

### 3. 使用 unsafe 函数封装不变量

```
struct SortedVec<T> {
    inner: Vec<T>,
}

impl<T: Ord> SortedVec<T> {
    // 不变量: inner 始终有序

    fn new() -> Self {
        SortedVec { inner: Vec.new() }
    }

    fn insert(self: &mut Self, value: T) {
        let pos = self.inner.binary_search(&value).unwrap_or_else(|e| e)
        self.inner.insert(pos, value)
        // 维护不变量
    }

    // 安全: 依赖不变量
    fn get_min(self: &Self) -> Option<&T> {
        self.inner.first()
    }

    // unsafe: 绕过检查，假设调用者维护不变量
    unsafe fn from_sorted_unchecked(vec: Vec<T>) -> Self {
        // SAFETY: 调用者保证 vec 已排序
        SortedVec { inner: vec }
    }
}
```

---

## 与 FFI 的结合

```
// C 函数总是 unsafe
extern "C" {
    fn malloc(size: usize) -> *mut c_void
    fn free(ptr: *mut c_void)
}

// 安全包装
fn allocate<T>() -> Box<T> {
    unsafe {
        let ptr = malloc(size_of<T>()) as *mut T
        if ptr.is_null() {
            panic("allocation failed")
        }
        Box.from_raw(ptr)
    }
}
```

---

## 编译器检查

### 1. Unsafe 块外禁止不安全操作

```
fn safe_code() {
    let ptr: *const i32 = ...
    let val = *ptr  // 编译错误: 解引用裸指针需要 unsafe
}
```

### 2. Unsafe 函数必须在 unsafe 上下文调用

```
unsafe fn dangerous() { ... }

fn caller() {
    dangerous()  // 编译错误

    unsafe {
        dangerous()  // OK
    }
}
```

### 3. 可选: Unsafe 块审计

```bash
# 列出所有 unsafe 块
$ dast audit unsafe src/

src/vec.rs:42: unsafe block (reason: pointer offset)
src/string.rs:128: unsafe block (reason: uninitialized memory)
src/ffi.rs:15: unsafe function call (malloc)

Total: 3 unsafe blocks
```

---

## 待讨论

- [ ] Panic 策略: unwinding vs abort 的默认选择
- [ ] Drop 中是否允许异步操作 (async drop)
- [ ] Unsafe 块是否需要强制注释 (SAFETY: ...)
- [ ] 是否引入 `trusted` 关键字 (标记已审计的 unsafe 代码)
