# 内存管理

## 核心原则

1. **RAII 自动释放** - 作用域结束自动调用 Drop
2. **所有权系统** - 每个值有唯一所有者
3. **借用检查** - 编译期防止悬垂引用
4. **零成本抽象** - 无运行时开销

---

## RAII 自动释放

### Drop Trait

```dast
trait Drop {
    fn drop(self: &mut Self)
}

// 自动实现
struct File {
    fd: i32,
}

impl Drop for File {
    fn drop(self: &mut Self) {
        close(self.fd)  // 自动调用
    }
}
```

### 调用时机

```dast
fn example() {
    let file = File.open("data.txt")
    // 使用 file
}  // 自动调用 file.drop()

// 提前返回
fn early_return() -> Result[(), Error] {
    let file = File.open("data.txt")?
    return .Err(error)  // 自动调用 file.drop()
}
```

---

## 所有权与移动语义

### 基本规则

```dast
// 规则 1: 每个值有唯一所有者
let s1 = String.from("hello")
let s2 = s1  // s1 移动到 s2，s1 不再有效

// 规则 2: 所有者离开作用域，值被释放
{
    let s = String.from("hello")
}  // s 被 drop

// 规则 3: 函数调用转移所有权
fn take_ownership(s: String) {
    // s 的所有权转移到这里
}  // s 被 drop

let s = String.from("hello")
take_ownership(s)  // s 移动，不再有效
```

### Copy vs Move

```dast
// Copy 类型（按位复制）
let x = 42
let y = x  // x 仍然有效（Copy）

// Move 类型（转移所有权）
let s1 = String.from("hello")
let s2 = s1  // s1 不再有效（Move）
```

**自动判定**:
- Copy: 基础类型（i32, f64, bool, char）
- Move: 资源类型（String, Vec, File, Box）

---

## 借用检查

### 不可变借用

```dast
fn calculate_length(s: &String) -> usize {
    s.len()  // 只读访问
}

let s = String.from("hello")
let len = calculate_length(&s)  // 借用，s 仍然有效
println("{}", s)  // OK
```

### 可变借用

```dast
fn append(s: &mut String) {
    s.push_str(" world")
}

let mut s = String.from("hello")
append(&mut s)
println("{}", s)  // "hello world"
```

### 借用规则

```dast
// ✅ 多个不可变借用
let s = String.from("hello")
let r1 = &s
let r2 = &s
println("{} {}", r1, r2)  // OK

// ❌ 不可变借用 + 可变借用
let s = String.from("hello")
let r1 = &s
let r2 = &mut s  // 错误：已有不可变借用

// ✅ 可变借用（唯一）
let mut s = String.from("hello")
let r = &mut s
r.push_str(" world")  // OK
```

---

## 生命周期自动推断

### 无需显式标注

```dast
// Dast: 自动推断
fn longest(x: &str, y: &str) -> &str {
    if x.len() > y.len() { x } else { y }
}

// 编译器自动推断: 返回值生命周期 = min(x, y)
```

### 结构体中的引用

```dast
struct Parser {
    input: &str,  // 自动推断生命周期
}

impl Parser {
    fn parse(self: &mut Self) -> Result[&str, Error] {
        // 返回值生命周期绑定到 self.input
    }
}
```

---

## 智能指针

### Box - 堆分配

```dast
struct Box[T] {
    // 唯一所有权，RAII 释放
}

let b = Box.new(42)
// 离开作用域自动释放
```

### Unique - 可空堆指针

```dast
struct Unique[T] {
    // 类似 Box，但可以为 null
}

let mut u: Unique[Node] = Unique.null()
u = Unique.new(Node{})
if u.is_some() {
    // 使用
}
```

### Shared - 共享所有权

```dast
struct Shared[T] {
    // 引用计数，线程安全
}

let s1 = Shared.new(42)
let s2 = s1.clone()  // 引用计数 +1
// s1, s2 都有效
```

---

## Unsafe 机制

### 设计原则

- 默认安全
- Unsafe 块显式标记
- 最小化 unsafe 范围

### Unsafe 操作

```dast
// 1. 解引用裸指针
unsafe {
    let ptr: *const i32 = &42
    let value = *ptr
}

// 2. 调用 unsafe 函数
unsafe fn dangerous() {
    // 不安全操作
}

unsafe {
    dangerous()
}

// 3. 实现 unsafe trait
unsafe trait UnsafeTrait { }

unsafe impl UnsafeTrait for MyType { }
```

### 安全抽象

```dast
// 内部使用 unsafe，外部安全
struct Vec[T] {
    ptr: *mut T,
    len: usize,
    cap: usize,
}

impl[T] Vec[T] {
    fn push(self: &mut Self, value: T) {
        unsafe {
            // 内部 unsafe 操作
            // 但保证外部调用安全
        }
    }
}
```

---

## 与 Rust 对比

| 特性 | Rust | Dast |
|------|------|------|
| RAII | ✅ | ✅ |
| 所有权 | ✅ | ✅ |
| 借用检查 | ✅ 完整 | ✅ 简化 |
| 生命周期 | 显式标注 | 自动推断 |
| Drop | ✅ | ✅ |
| Unsafe | ✅ | ✅ |

**简化点**: 无需显式生命周期标注

---

## 总结

| 特性 | 决策 |
|------|------|
| 内存管理 | RAII 自动释放 |
| 所有权 | 唯一所有者 + 移动语义 |
| 借用检查 | 编译期检查 |
| 生命周期 | 自动推断 |
| 智能指针 | Box, Unique, Shared |
| Unsafe | 显式标记 |

## Arena 分配器

### 设计

批量分配，整体释放，适合大量小对象。

```dast
struct Arena {
    chunks: Vec[Chunk],
    current: *mut u8,
    end: *mut u8,
}

impl Arena {
    fn new(size: usize) -> Self {
        // 初始化 Arena
    }
    
    fn alloc[T](self: &mut Self) -> *mut T {
        // 分配对象，返回裸指针
        // 无单独释放
    }
}

impl Drop for Arena {
    fn drop(self: &mut Self) {
        // 一次性释放所有 chunks
    }
}
```

### 使用场景

```dast
// 编译器 AST
let arena = Arena.new(64 * 1024)
for item in items {
    let node = arena.alloc[AstNode]()
    // 不单独释放
}
// arena 离开作用域，一次性回收全部

// 游戏帧数据
fn process_frame() {
    let arena = Arena.new(1024 * 1024)
    // 分配大量临时对象
}  // 帧结束，全部释放
```

---

## Drop 调用时机详解

### Panic 处理

```dast
fn with_panic() {
    let a = Resource.new("a")
    let b = Resource.new("b")
    
    panic("error!")  // 栈展开: b.drop(), a.drop()
}
```

### Unwinding vs Abort

| 模式 | 优点 | 缺点 | 适用场景 |
|------|------|------|---------|
| **Unwinding** | 资源正确释放 | 代码体积大、性能开销 | 主机应用 |
| **Abort** | 体积小、无开销 | 资源可能泄漏 | 嵌入式、性能关键 |

```bash
# 默认 unwinding
$ dast build

# 使用 abort
$ dast build --panic=abort
```

### Drop 中不能 Panic

```dast
impl Drop for Resource {
    fn drop(self: &mut Self) {
        // ❌ 危险: Drop 中 panic 导致双重 panic
        if !self.cleanup() {
            panic("cleanup failed")
        }
        
        // ✅ 正确: 记录错误但不 panic
        if !self.cleanup() {
            eprintln("Warning: cleanup failed")
        }
    }
}
```

---

## Unsafe 最佳实践

### 1. 最小化 Unsafe 块

```dast
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

### 2. 文档化安全性

```dast
unsafe fn copy_nonoverlapping[T](src: *const T, dst: *mut T, count: usize) {
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

### 3. 安全抽象模式

```dast
struct SortedVec[T] {
    inner: Vec[T],
}

impl[T: Ord] SortedVec[T] {
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
    fn get_min(self: &Self) -> Option[&T] {
        self.inner.first()
    }
    
    // unsafe: 绕过检查，假设调用者维护不变量
    unsafe fn from_sorted_unchecked(vec: Vec[T]) -> Self {
        // SAFETY: 调用者保证 vec 已排序
        SortedVec { inner: vec }
    }
}
```
