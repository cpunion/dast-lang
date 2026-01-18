# 不安全场景代码示例

本文档提供具体的代码示例，展示 Dast 简化设计可能导致的不安全场景。

---

## 场景 1: 复杂生命周期推断失败

### 简化来源
**去除显式生命周期标注** - 编译器自动推断所有生命周期

### Rust 代码 (安全)

```rust
// 复杂的生命周期关系
struct Parser<'input, 'context> {
    input: &'input str,
    context: &'context Context,
}

impl<'input, 'context> Parser<'input, 'context> {
    // 返回值生命周期绑定到 'input
    fn parse_token(&mut self) -> &'input str {
        // 使用 context 但不返回其引用
        self.context.log("parsing");

        // 返回 input 的切片
        &self.input[0..5]
    }
}

// 使用
fn use_parser() {
    let input = String::from("hello world");
    let context = Context::new();

    let mut parser = Parser {
        input: &input,
        context: &context,
    };

    let token = parser.parse_token();
    drop(context);  // OK: context 可以先释放
    println!("{}", token);  // token 仍然有效
}
```

### Dast 代码 (编译失败)

```dast
// ❌ 编译器无法推断 input 和 context 的生命周期关系
struct Parser {
    input: &str,
    context: &Context,
}

impl Parser {
    fn parse_token(self: &mut Self) -> &str {
        self.context.log("parsing")
        return self.input[0..5]
    }
}

// 编译错误:
// Cannot infer lifetime of return value.
// It could be tied to 'self.input' or 'self.context'.
//
// Suggestions:
// 1. Return owned String instead of &str
// 2. Simplify struct to only hold one reference
// 3. Use unsafe with explicit lifetime cast
```

### Dast 解决方案

```dast
// 方案 1: 返回拥有的值
impl Parser {
    fn parse_token(self: &mut Self) -> String {
        self.context.log("parsing")
        return self.input[0..5].to_string()  // 复制
    }
}

// 方案 2: 简化结构
struct Parser {
    input: &str,
    // context 通过参数传递，不存储
}

impl Parser {
    fn parse_token(self: &mut Self, context: &Context) -> &str {
        context.log("parsing")
        return self.input[0..5]  // OK: 只有一个引用字段
    }
}

// 方案 3: unsafe (高级用户)
impl Parser {
    fn parse_token(self: &mut Self) -> &str {
        self.context.log("parsing")
        unsafe {
            // SAFETY: 我们知道返回值绑定到 self.input，
            // 而 self.input 的生命周期比 self.context 长
            std.mem.transmute(self.input[0..5])
        }
    }
}
```

### 影响评估
- **频率**: 约 3% 的 Rust 代码
- **场景**: 多个引用字段的结构体
- **缓解**: 重构设计或使用 unsafe

---

## 场景 2: 非重叠借用的别名问题

### 简化来源
**放宽借用检查** - 允许对不同索引/字段的同时可变访问

### Rust 代码 (编译失败)

```rust
fn swap_in_place(arr: &mut [i32], i: usize, j: usize) {
    let a = &mut arr[i];  // 第一个可变借用
    let b = &mut arr[j];  // 编译错误: 第二个可变借用
    std::mem::swap(a, b);
}

// Rust 强制使用安全的 API
fn swap_in_place(arr: &mut [i32], i: usize, j: usize) {
    // 必须用 split_at_mut 证明不重叠
    if i == j { return; }
    let (left, right) = if i < j {
        arr.split_at_mut(j)
    } else {
        arr.split_at_mut(i)
    };
    std::mem::swap(&mut left[i], &mut right[0]);
}
```

### Dast 代码 (编译通过，运行时风险)

```dast
// ✅ 编译通过 (Dast 假设 i != j)
fn swap_in_place(arr: &mut [i32], i: usize, j: usize) {
    let a = &mut arr[i]
    let b = &mut arr[j]
    std.mem.swap(a, b)
}

// 使用
fn main() {
    let arr = [1, 2, 3, 4, 5]

    // ✅ 正常情况: i != j
    swap_in_place(&mut arr, 0, 2)  // OK

    // ❌ 危险情况: i == j
    swap_in_place(&mut arr, 2, 2)  // UB: 两个可变引用指向同一位置！
}
```

### 运行时行为

```dast
// Debug 模式 (--mode=debug)
fn swap_in_place(arr: &mut [i32], i: usize, j: usize) {
    // 编译器自动插入检查
    if i == j {
        panic("Aliased mutable references: arr[{}] and arr[{}]", i, j)
    }

    let a = &mut arr[i]
    let b = &mut arr[j]
    std.mem.swap(a, b)
}

// Release 模式 (--mode=release)
// 无检查，假设程序员正确
// 如果 i == j，行为未定义
```

### 安全的 Dast 写法

```dast
fn swap_in_place(arr: &mut [i32], i: usize, j: usize) {
    // 显式检查
    assert(i != j, "indices must be different")

    let a = &mut arr[i]
    let b = &mut arr[j]
    std.mem.swap(a, b)
}

// 或使用标准库的安全 API
fn swap_in_place(arr: &mut [i32], i: usize, j: usize) {
    arr.swap(i, j)  // 内部已处理 i == j 的情况
}
```

### 影响评估
- **频率**: 约 2% 的代码
- **场景**: 数组/切片的多个可变索引访问
- **缓解**: Debug 模式运行时检查，或手动 assert

---

## 场景 3: 迭代器失效

### 简化来源
**放宽借用检查** - 允许在迭代时修改容器（Rust 禁止）

### Rust 代码 (编译失败)

```rust
fn remove_negatives(vec: &mut Vec<i32>) {
    // ❌ 编译错误
    for (i, &val) in vec.iter().enumerate() {
        if val < 0 {
            vec.remove(i);  // 错误: 迭代时不能修改
        }
    }
}

// Rust 强制使用安全模式
fn remove_negatives(vec: &mut Vec<i32>) {
    vec.retain(|&x| x >= 0);  // 使用 retain API
}
```

### Dast 代码 (编译通过，运行时危险)

```dast
// ✅ 编译通过
fn remove_negatives(vec: &mut Vec<i32>) {
    for i, val in vec.iter().enumerate() {
        if val < 0 {
            vec.remove(i)  // 编译通过！
        }
    }
}

// 运行时问题:
let vec = Vec.from([-1, 2, -3, 4])
remove_negatives(&mut vec)

// 问题 1: 迭代器失效
// - iter() 持有 vec 的不可变引用
// - remove() 需要可变引用
// - 内部指针可能失效

// 问题 2: 索引错位
// 移除索引 0 后，原索引 1 变成新索引 0
// 但 enumerate() 仍然递增，跳过元素
```

### 运行时行为

```dast
// Debug 模式
fn remove_negatives(vec: &mut Vec<i32>) {
    for i, val in vec.iter().enumerate() {
        if val < 0 {
            vec.remove(i)  // Panic: Cannot modify Vec while iterating
        }
    }
}

// 输出:
// thread 'main' panicked at 'Cannot modify Vec while iterating'
// note: Vec has active iterator at src/main.dast:3

// Release 模式
// 未定义行为: 可能崩溃、数据损坏、或看似正常运行
```

### 安全的 Dast 写法

```dast
// 方案 1: 使用 retain
fn remove_negatives(vec: &mut Vec<i32>) {
    vec.retain(|x| x >= 0)
}

// 方案 2: 收集索引后删除
fn remove_negatives(vec: &mut Vec<i32>) {
    let to_remove = vec.iter()
        .enumerate()
        .filter(|(_, &x)| x < 0)
        .map(|(i, _)| i)
        .collect::<Vec<_>>()

    // 从后往前删除，避免索引错位
    for i in to_remove.iter().rev() {
        vec.remove(*i)
    }
}

// 方案 3: 创建新 Vec
fn remove_negatives(vec: &mut Vec<i32>) {
    *vec = vec.iter()
        .filter(|&&x| x >= 0)
        .copied()
        .collect()
}
```

### 影响评估
- **频率**: 约 2% 的代码
- **场景**: 迭代时修改容器
- **缓解**: Debug 模式 panic，强制使用正确模式

---

## 场景 4: 并发数据竞争

### 简化来源
**自动推导 Send/Sync** - 不强制显式标记线程安全性

### Rust 代码 (编译失败)

```rust
use std::sync::Arc;

struct Counter {
    value: i32,  // 非原子
}

fn race_condition() {
    let counter = Arc::new(Counter { value: 0 });

    let mut handles = vec![];
    for _ in 0..10 {
        let c = counter.clone();
        handles.push(std::thread::spawn(move || {
            c.value += 1;  // ❌ 编译错误: Counter 没有实现 Sync
        }));
    }

    for h in handles {
        h.join().unwrap();
    }
}

// 错误信息:
// error[E0277]: `Counter` cannot be shared between threads safely
// help: consider using `std::sync::Mutex` or `std::sync::atomic::AtomicI32`
```

### Dast 代码 (编译通过，数据竞争)

```dast
struct Counter {
    value: i32,
}

fn race_condition() {
    let counter = Shared.new(Counter { value: 0 })

    let handles = []
    for _ in 0..10 {
        let c = counter.clone()
        handles.push(spawn(move || {
            c.value += 1  // ✅ 编译通过，但有数据竞争！
        }))
    }

    for h in handles {
        h.join()
    }

    println("Final value: {}", counter.value)
    // 期望: 10
    // 实际: 不确定 (1-10 之间的任意值)
}
```

### 为什么 Dast 允许？

```dast
// Dast 的自动推导逻辑:
// 1. Counter 只包含 i32
// 2. i32 是 Copy + Send
// 3. 因此 Counter 是 Send (可以跨线程传递)
// 4. Shared<T> 要求 T: Send，满足
// 5. 编译通过

// 但是:
// - Counter 没有内部同步机制
// - 多线程同时修改 value 导致数据竞争
```

### 检测方法

```bash
# 使用 ThreadSanitizer 检测
$ dast build --sanitize=thread
$ ./program

# 输出:
WARNING: ThreadSanitizer: data race (pid=12345)
  Write of size 4 at 0x7b0400000000 by thread T2:
    #0 race_condition::{{closure}} src/main.dast:8

  Previous write of size 4 at 0x7b0400000000 by thread T1:
    #0 race_condition::{{closure}} src/main.dast:8
```

### 安全的 Dast 写法

```dast
// 方案 1: 使用原子类型
struct Counter {
    value: Atomic<i32>,
}

fn safe_increment() {
    let counter = Shared.new(Counter {
        value: Atomic.new(0)
    })

    for _ in 0..10 {
        let c = counter.clone()
        spawn(move || {
            c.value.fetch_add(1)  // 原子操作
        })
    }
}

// 方案 2: 使用 Mutex
struct Counter {
    value: Mutex<i32>,
}

fn safe_increment() {
    let counter = Shared.new(Counter {
        value: Mutex.new(0)
    })

    for _ in 0..10 {
        let c = counter.clone()
        spawn(move || {
            let mut val = c.value.lock()
            *val += 1
        })
    }
}

// 方案 3: 显式标记 (未来可能支持)
@not_thread_safe
struct Counter {
    value: i32,
}
// 编译器拒绝在 Shared 中使用
```

### 影响评估
- **频率**: 约 1% 的并发代码
- **场景**: 共享可变状态
- **缓解**: ThreadSanitizer + 测试

---

## 场景 5: 自引用结构的意外移动

### 简化来源
**@pinned 简化 Pin 机制** - 比 Rust Pin 更简单但限制更多

### Rust 代码 (复杂但安全)

```rust
use std::pin::Pin;

struct SelfRef {
    data: String,
    ptr: *const String,
}

impl SelfRef {
    fn new(s: String) -> Pin<Box<Self>> {
        let mut boxed = Box::pin(SelfRef {
            data: s,
            ptr: std::ptr::null(),
        });

        let ptr = &boxed.data as *const String;
        unsafe {
            let mut_ref = Pin::as_mut(&mut boxed);
            Pin::get_unchecked_mut(mut_ref).ptr = ptr;
        }

        boxed
    }

    fn get_data(&self) -> &str {
        unsafe { &*self.ptr }
    }
}

// 使用
fn use_self_ref() {
    let obj = SelfRef::new("hello".to_string());
    println!("{}", obj.get_data());  // OK

    // let obj2 = obj;  // 编译错误: Pin<Box<T>> 不能移动
}
```

### Dast 代码 (简单但限制多)

```dast
@pinned
struct SelfRef {
    data: String,
    ptr: &String,
}

impl SelfRef {
    fn new(s: String) -> Box<Self> {
        let mut obj = Box.new(SelfRef {
            data: s,
            ptr: null,
        })
        obj.ptr = &obj.data  // 编译器允许，因为 Box 不会移动
        return obj
    }

    fn get_data(self: &Self) -> &str {
        return self.ptr
    }
}

// 使用
fn use_self_ref() {
    let obj = SelfRef.new("hello")
    println("{}", obj.get_data())  // OK

    // ❌ 编译错误
    let obj2 = obj  // Error: Cannot move @pinned type

    // ❌ 编译错误
    let obj3 = *obj  // Error: Cannot move out of Box<@pinned>

    // ✅ 只能通过引用使用
    process(&obj)
}
```

### 问题场景

```dast
// 问题: 不能将 @pinned 类型放入容器
fn collect_refs() {
    let vec: Vec<Box<SelfRef>> = Vec.new()

    for i in 0..10 {
        let obj = SelfRef.new(format("item {}", i))
        vec.push(obj)  // ❌ 编译错误: push 需要移动
    }
}

// Rust 可以用 Pin<Box<T>>
// Dast 必须用其他方式
```

### Dast 解决方案

```dast
// 方案 1: 使用索引而非指针
struct SelfRef {
    data: String,
    offset: usize,  // 偏移量而非指针
}

impl SelfRef {
    fn get_data(self: &Self) -> &str {
        return &self.data[self.offset..]
    }
}

// 方案 2: 分离数据和引用
struct Data {
    content: String,
}

struct Ref {
    data: &Data,
    offset: usize,
}

// 方案 3: 使用 unsafe 绕过限制
fn collect_refs() {
    let vec: Vec<*mut SelfRef> = Vec.new()  // 裸指针

    for i in 0..10 {
        let obj = SelfRef.new(format("item {}", i))
        unsafe {
            vec.push(Box.into_raw(obj))  // 转为裸指针
        }
    }

    // 使用时需要 unsafe
    unsafe {
        for ptr in &vec {
            println("{}", (**ptr).get_data())
        }
    }

    // 清理
    unsafe {
        for ptr in vec {
            Box.from_raw(ptr).drop()
        }
    }
}
```

### 影响评估
- **频率**: 约 1% 的代码
- **场景**: 自引用数据结构（少见）
- **缓解**: 重新设计数据结构

---

## 场景 6: 整数溢出

### 简化来源
**性能优先** - Release 模式允许整数回绕

### Rust 代码 (相同行为)

```rust
fn calculate(x: i32) -> i32 {
    x + 1  // Debug: panic on overflow
           // Release: wrapping
}

fn main() {
    let result = calculate(i32::MAX);
    println!("{}", result);
    // Debug: panic
    // Release: -2147483648 (回绕)
}
```

### Dast 代码 (相同)

```dast
fn calculate(x: i32) -> i32 {
    return x + 1
}

fn main() {
    let result = calculate(i32.MAX)
    println("{}", result)
    // Debug: panic
    // Release: -2147483648
}
```

### 安全写法

```dast
fn calculate(x: i32) -> Result<i32, Error> {
    return x.checked_add(1).ok_or("overflow")
}

// 或使用饱和运算
fn calculate_saturating(x: i32) -> i32 {
    return x.saturating_add(1)  // MAX + 1 = MAX
}

// 或显式回绕
fn calculate_wrapping(x: i32) -> i32 {
    return x.wrapping_add(1)  // 明确意图
}
```

### 影响评估
- **频率**: 约 1% 的代码
- **场景**: 数值计算
- **缓解**: 使用 checked_*/saturating_*/wrapping_*

---

## 总结表

| 场景 | 简化来源 | 频率 | Debug 行为 | Release 行为 | 缓解方法 |
|------|---------|------|-----------|-------------|---------|
| 复杂生命周期 | 无显式标注 | ~3% | 编译错误 | 编译错误 | 重构/unsafe |
| 非重叠借用 | 放宽检查 | ~2% | Panic | UB | 手动 assert |
| 迭代器失效 | 放宽检查 | ~2% | Panic | UB | 使用正确 API |
| 数据竞争 | 自动推导 | ~1% | 正常运行 | UB | Sanitizer |
| @pinned 限制 | 简化 Pin | ~1% | 编译错误 | 编译错误 | 重新设计 |
| 整数溢出 | 性能优先 | ~1% | Panic | 回绕 | checked_* |

---

## 建议

1. **开发阶段**: 始终使用 Debug 模式
2. **测试阶段**: 使用 Sanitizers
3. **生产部署**: 关键代码使用 `--strict` 模式
4. **代码审查**: 重点关注数组索引、并发、数值计算
