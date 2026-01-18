# 线程安全设计方案

## 线程安全问题分类

### 1. 数据竞争 (Data Race)

多线程同时访问同一内存，至少一个是写操作。

```dast
// ❌ 问题代码
let counter = 0
spawn(|| counter += 1)
spawn(|| counter += 1)
// 结果: 不确定 (可能是 1 或 2)
```

### 2. 共享可变状态

```dast
// ❌ 问题代码
struct State { value: i32 }
let state = Shared.new(State { value: 0 })

spawn(|| state.value += 1)   // 线程 1 修改
spawn(|| println(state.value)) // 线程 2 读取 (可能读到旧值/新值/撕裂值)
```

### 3. 发送非线程安全类型

```dast
// ❌ 问题代码
let rc = Rc.new(42)  // Rc 不是线程安全的
spawn(move || {
    println(*rc)  // 跨线程使用非线程安全类型
})
```

### 4. 闭包捕获竞争

```dast
// ❌ 问题代码
let data = Data.new()
spawn(|| data.modify())  // 闭包捕获 data
spawn(|| data.read())    // 同时捕获，竞争
```

### 5. 内部可变性泄漏

```dast
// ❌ 问题代码
struct Wrapper {
    cell: Cell<i32>,  // Cell 允许内部可变
}

let w = Shared.new(Wrapper { cell: Cell.new(0) })
spawn(|| w.cell.set(1))  // 多线程修改 Cell
spawn(|| w.cell.set(2))  // Cell 不是线程安全的
```

---

## Rust 的解决方案

### Send/Sync Trait

```rust
// Send: 可以跨线程移动所有权
unsafe trait Send {}

// Sync: 可以跨线程共享引用 (&T 是 Send)
unsafe trait Sync {}

// 规则:
// - T: Send → 可以 move 到其他线程
// - T: Sync → &T 可以在多线程间共享
// - Arc<T> 要求 T: Send + Sync
// - Mutex<T> 使 T 变成 Sync (通过锁保护)
```

### 自动推导

```rust
// 自动推导规则:
// - 基础类型: Send + Sync
// - 结构体: 所有字段都是 Send → 结构体是 Send
// - Rc<T>: !Send (不能跨线程)
// - RefCell<T>: !Sync (不能共享引用)
// - Mutex<T>: Send + Sync
```

---

## Dast 方案评估

### 方案 A: 完整复制 Rust 的 Send/Sync

```dast
trait Send {}
trait Sync {}

// 自动推导，与 Rust 相同
```

**优点**: 100% 线程安全
**缺点**: 增加复杂度，需要理解 Send/Sync

### 方案 B: @thread_safe 显式标记

```dast
// 需要跨线程使用的类型必须标记
@thread_safe
struct Counter {
    value: Atomic<i32>,
}

// 编译器验证所有字段是线程安全的
```

**优点**: 简单直观
**缺点**: 可能漏标记

### 方案 C: 类型分层 (推荐)

```dast
// 层次 1: 默认单线程
struct Data { ... }  // 普通类型，只能单线程

// 层次 2: 显式跨线程
@sendable  // 可以移动到其他线程
struct Message { ... }

// 层次 3: 可共享
@shareable  // 可以在多线程间共享引用
struct Config { ... }

// 规则:
// - spawn 要求闭包捕获的类型是 @sendable
// - Shared<T> 要求 T 是 @shareable
```

---

## 推荐方案: 简化的 Send/Sync

### 设计

```dast
// 两个核心标记 (编译器自动推导 + 可手动标记)
trait Sendable {}   // 可跨线程移动
trait Shareable {}  // 可跨线程共享

// 自动推导规则:
// 1. 基础类型 (i32, f64, bool 等): Sendable + Shareable
// 2. 不可变引用 &T: 如果 T: Shareable 则 Sendable
// 3. Box<T>: 如果 T: Sendable 则 Sendable
// 4. 结构体: 所有字段满足则自动推导
// 5. Atomic<T>: Sendable + Shareable
// 6. Mutex<T>: Sendable + Shareable (包装任意 T)
```

### 示例

```dast
// ✅ 自动推导为 Sendable + Shareable
struct Point { x: f32, y: f32 }

// ✅ 自动推导为 Sendable (所有字段是 Sendable)
struct Message {
    id: u64,
    data: String,
}

// ❌ 自动推导失败 (Cell 不是 Shareable)
struct NotShareable {
    cell: Cell<i32>,
}

// ⚠️ 需要检查
let msg = Message { ... }
spawn(move || {
    process(msg)  // OK: Message 是 Sendable
})

// ❌ 编译错误
let ns = NotShareable { ... }
let shared = Shared.new(ns)  // 错误: NotShareable 不是 Shareable
```

### 强制规则

```dast
// spawn 签名
fn spawn<F>(f: F) where F: FnOnce() + Sendable

// Shared 签名
struct Shared<T: Shareable> { ... }

// 这样编译器自动检查类型是否满足约束
```

---

## 边缘情况处理

### 1. 不安全代码 (unsafe)

```dast
// unsafe 块内可以绕过检查
struct RawPointer {
    ptr: *mut i32,
}

// 手动实现 Sendable
unsafe impl Sendable for RawPointer {}
// 程序员保证线程安全
```

### 2. 复杂闭包捕获

```dast
let data = Data.new()
let mutex = Mutex.new(data)

spawn(move || {
    let guard = mutex.lock()
    guard.modify()  // OK: 通过 Mutex 保护
})
```

### 3. 动态检查 (ThreadSanitizer)

```bash
# 开发阶段使用
$ dast test --sanitize=thread
```

---

## 与 Rust 的对比

| 特性 | Rust | Dast (推荐方案) |
|------|------|----------------|
| Trait 名称 | Send/Sync | Sendable/Shareable |
| 自动推导 | ✅ | ✅ |
| 编译期检查 | ✅ 100% | ✅ ~99% |
| unsafe 绕过 | ✅ | ✅ |
| 学习成本 | 需理解 Send/Sync | 类似但命名更直观 |

---

## 安全性提升评估

| 方案 | 安全性 | 复杂度 | 推荐 |
|------|--------|--------|------|
| 无检查 | 80% | 低 | ❌ |
| @thread_safe 标记 | 95% | 低 | ⚠️ |
| 简化 Send/Sync | 99% | 中 | ✅ |
| 完整 Send/Sync | 100% | 高 | ⚠️ |

**推荐**: 采用简化的 Sendable/Shareable，达到 ~99% 安全性。

---

## 实现路线

### 第一阶段: 基础

1. 实现 Sendable/Shareable trait
2. 自动推导规则
3. spawn/Shared 类型约束

### 第二阶段: 增强

4. unsafe impl 支持
5. ThreadSanitizer 集成
6. 更好的错误信息

### 第三阶段: 优化

7. 编译期检查优化
8. 边缘情况覆盖

---

## 总结

**推荐方案**: 简化的 Sendable/Shareable 系统

- **安全性**: 98% → 99%+
- **复杂度**: 中等 (比 Rust 简单)
- **学习成本**: 低 (命名直观)
- **覆盖率**: ~99% 场景

**剩余 ~1%**: unsafe 代码 + ThreadSanitizer 补充
