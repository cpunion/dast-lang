# Dast vs Rust: 简化设计对比

## 核心差异：简化哲学

**Rust**: 编译期 100% 内存安全，零运行时开销
**Dast**: 编译期 90% 安全 + 简化规则 + 可选运行时检查

---

## 关键简化点

### 1. 生命周期：无需显式标注

```rust
// ❌ Rust: 繁琐的生命周期标注
fn longest<'a>(x: &'a str, y: &'a str) -> &'a str {
    if x.len() > y.len() { x } else { y }
}

struct Parser<'a> {
    source: &'a str,
    pos: usize,
}

impl<'a> Parser<'a> {
    fn parse(&mut self) -> Result<&'a str, Error> { ... }
}
```

```dast
// ✅ Dast: 完全自动推断
fn longest(x: &str, y: &str) -> &str {
    if x.len() > y.len() { x } else { y }
}

struct Parser {
    source: &str,
    pos: usize,
}

impl Parser {
    fn parse(self: &mut Self) -> Result<&str, Error> { ... }
}
```

**简化原则**:
- 编译器自动推断生命周期
- 无法推断时 → 编译错误，提示使用值返回或 unsafe
- 不暴露 `'a` `'b` 等符号给用户

---

### 2. 借用检查：简化规则

```rust
// ❌ Rust: 严格借用检查导致合法代码无法编译
fn process(data: &mut [i32]) {
    let first = &data[0];        // 不可变借用
    data[1] = 10;                // 编译错误: 已有不可变借用
    println!("{}", first);
}

// Rust 需要这样写:
fn process(data: &mut [i32]) {
    let first_val = data[0];     // 复制值
    data[1] = 10;                // OK
    println!("{}", first_val);
}
```

```dast
// ✅ Dast: 允许非重叠访问
fn process(data: &mut [i32]) {
    let first = &data[0]         // 借用索引 0
    data[1] = 10                 // OK: 访问索引 1，不重叠
    println("{}", first)
}

// 规则: 只检查明显的别名冲突
// - 同一变量的可变/不可变借用 → 错误
// - 不同索引/字段的访问 → 允许
```

**简化原则**:
- 只检查明显的别名冲突
- 允许程序员用 unsafe 绕过复杂情况
- 运行时可选边界检查 (debug 模式)

---

### 3. 自引用结构：直接支持

```rust
// ❌ Rust: 自引用结构极其困难
struct SelfRef {
    data: String,
    ptr: *const String,  // 必须用裸指针
}

// 需要 Pin + unsafe + PhantomPinned
use std::pin::Pin;
use std::marker::PhantomPinned;

struct SelfRef {
    data: String,
    ptr: *const String,
    _pin: PhantomPinned,
}
```

```dast
// ✅ Dast: 直接支持 (编译器特殊处理)
@pinned  // 标记为不可移动
struct SelfRef {
    data: String,
    ptr: &String,  // 可以直接用引用
}

impl SelfRef {
    fn new(s: String) -> Box<Self> {
        let mut obj = Box.new(SelfRef {
            data: s,
            ptr: null,
        })
        obj.ptr = &obj.data  // 编译器保证 Box 不会移动
        return obj
    }
}
```

**简化原则**:
- `@pinned` 标记 → 编译器禁止移动
- 只能在 Box/堆上创建
- 无需 PhantomPinned 等复杂机制

---

### 4. 智能指针：统一接口

```rust
// ❌ Rust: 多种智能指针，API 不统一
use std::rc::Rc;
use std::sync::Arc;
use std::cell::RefCell;

let rc: Rc<i32> = Rc::new(42);
let arc: Arc<i32> = Arc::new(42);
let cell: RefCell<i32> = RefCell::new(42);

// 需要记住不同的方法
let val = *rc;
let val = *arc;
let val = *cell.borrow();  // 不同的解引用方式
```

```dast
// ✅ Dast: 统一的智能指针
let owned: Box<i32> = Box.new(42)
let shared: Shared<i32> = Shared.new(42)  // 线程安全共享
let cell: Cell<i32> = Cell.new(42)        // 内部可变

// 统一的解引用
let val = *owned
let val = *shared
let val = *cell
```

**简化原则**:
- 只提供必要的智能指针类型
- 统一的操作符重载
- 自动选择 Rc/Arc (单线程/多线程)

---

### 5. Trait 系统：简化约束

```rust
// ❌ Rust: 复杂的 trait bound
fn process<T>(items: Vec<T>)
where
    T: Clone + Debug + Send + Sync + 'static,
    T::Item: Ord,
{
    ...
}

// 关联类型
trait Iterator {
    type Item;
    fn next(&mut self) -> Option<Self::Item>;
}
```

```dast
// ✅ Dast: 简化的 trait
fn process<T: Clone + Debug>(items: Vec<T>) {
    // Send/Sync 自动推导
    // 'static 不需要显式标注
}

// 关联类型简化
trait Iterator {
    fn next(self: &mut Self) -> Option<T>  // 直接用泛型
}
```

**简化原则**:
- Send/Sync 自动推导
- 生命周期约束自动推断
- 减少 where 子句的使用

---

### 6. 错误处理：更灵活

```rust
// ❌ Rust: 严格的 Result 类型
fn may_fail() -> Result<i32, Error> {
    let x = other_call()?;  // ? 只能用于 Result
    Ok(x + 1)
}
```

```dast
// ✅ Dast: 统一的错误处理
fn may_fail() -> i32 throws Error {
    let x = other_call()?   // ? 传播错误
    return x + 1
}

// 或使用 Result (兼容 Rust 风格)
fn may_fail() -> Result<i32, Error> {
    let x = other_call()?
    return .ok(x + 1)
}
```

**简化原则**:
- `throws` 关键字更直观
- 与 Result 类型兼容
- 可选的异常机制 (编译期检查)

---

### 7. 宏系统：简化元编程

```rust
// ❌ Rust: 复杂的宏语法
macro_rules! vec {
    ( $( $x:expr ),* ) => {
        {
            let mut temp_vec = Vec::new();
            $(
                temp_vec.push($x);
            )*
            temp_vec
        }
    };
}
```

```dast
// ✅ Dast: 编译期函数 (comptime)
comptime fn vec(items: []T) -> Vec<T> {
    let mut v = Vec.new()
    for item in items {
        v.push(item)
    }
    return v
}

// 使用
let v = vec([1, 2, 3])  // 编译期展开
```

**简化原则**:
- 用 `comptime` 标记编译期执行
- 使用普通函数语法，无需特殊宏语法
- 类似 Zig 的 comptime

---

## 安全性权衡

| 特性 | Rust | Dast | 权衡 |
|------|------|------|------|
| 生命周期 | 显式标注 | 自动推断 | 简单但某些边缘情况需 unsafe |
| 借用检查 | 完整 | 简化 | 允许非重叠访问，减少误报 |
| 自引用 | Pin + unsafe | @pinned | 直接支持常见模式 |
| 编译期保证 | 100% | ~90% | 剩余 10% 用 unsafe 或运行时检查 |
| 学习曲线 | 陡峭 | 平缓 | 更易上手 |

---

## 运行时可选检查

```bash
# Debug 模式: 启用运行时检查
$ dast build --mode=debug
# - 数组边界检查
# - 整数溢出检查
# - 空指针检查

# Release 模式: 禁用检查
$ dast build --mode=release
# - 零开销
# - 假设代码正确
```

---

## 总结：Dast 的简化哲学

1. **编译器更智能** - 自动推断生命周期、Send/Sync
2. **规则更宽松** - 允许非重叠访问、自引用结构
3. **语法更简洁** - 无 `'a`、少 where、comptime 替代宏
4. **实用主义** - 90% 安全 + unsafe 逃生舱 + 可选运行时检查
5. **渐进式** - 初学者无需理解生命周期，高级用户可用 unsafe

**核心目标**: 在保持内存安全的前提下，降低心智负担，提高开发效率。

---

## 待讨论

- [ ] 借用检查的具体简化规则
- [ ] @pinned 的实现细节
- [ ] comptime 的能力边界
- [ ] 运行时检查的性能影响
