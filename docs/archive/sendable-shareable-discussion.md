# Sendable/Shareable 线程安全方案

## 核心理念

**简化的 Send/Sync** - 保持 Rust 的安全性，简化命名和规则

```dast
trait Sendable {}   // 可跨线程移动 (类似 Rust Send)
trait Shareable {}  // 可跨线程共享引用 (类似 Rust Sync)
```

---

## 设计规则

### 自动推导

```dast
// 基础类型: Sendable + Shareable
i32, f64, bool, char  // ✅ 自动

// 结构体: 所有字段满足则自动推导
struct Point { x: f32, y: f32 }  // ✅ 自动 Sendable + Shareable
struct Data { items: Vec<i32> }  // ✅ 自动 Sendable + Shareable

// 特殊类型
Atomic<T>     // ✅ Sendable + Shareable
Mutex<T>      // ✅ Sendable + Shareable (T: Sendable)
Cell<T>       // ✅ Sendable, ❌ 非 Shareable
Rc<T>         // ❌ 非 Sendable
Arc<T>        // ✅ Sendable + Shareable (T: Sendable + Shareable)
```

### 编译器强制

```dast
// spawn 要求 Sendable
fn spawn<F>(f: F) where F: FnOnce() + Sendable

// Arc 要求 Sendable + Shareable
struct Arc<T: Sendable + Shareable> { ... }

// 编译错误示例
let rc = Rc.new(42)
spawn(move || {
    println(*rc)  // 错误: Rc<i32> 不是 Sendable
})
```

### 使用示例

```dast
// 共享只读
let config = Arc.new(Config.load())
spawn(move || config.read())  // ✅

// 共享可变
let counter = Arc.new(Mutex.new(0))
spawn(move || {
    *counter.lock() += 1  // ✅ Mutex 保护
})

// Channel 通信
let (tx, rx) = channel<Message>()  // Message: Sendable
spawn(move || tx.send(msg))
```

---

## 优势

### 1. 高灵活性
- 支持 Arc/Mutex 直接共享
- 支持无锁数据结构
- 覆盖所有并发模式

### 2. 高性能
- 零复制共享读 (Arc)
- 细粒度锁
- 无锁算法可行

### 3. 接近 Rust
- 概念对应清晰
- 生态兼容性好
- 文档可复用

### 4. 编译期 100% 安全
- 类型系统保证
- 无运行时开销

---

## 劣势

### 1. 需要学习两个概念

```dast
// 需要理解:
// - Sendable: 什么可以移动到其他线程
// - Shareable: 什么可以共享引用
```

### 2. 错误信息可能复杂

```
Error: Arc<Cell<i32>> 不满足 Shareable
Because: Cell<i32> 不是 Shareable
Note: Cell 允许内部可变但不是线程安全的
```

### 3. 需要 unsafe impl

```dast
struct RawHandle { ptr: *mut c_void }

// 手动保证线程安全
unsafe impl Sendable for RawHandle {}
unsafe impl Shareable for RawHandle {}
```

---

## 与 Rust 对比

| 方面 | Rust | Dast |
|------|------|------|
| Trait 名称 | Send, Sync | Sendable, Shareable |
| 自动推导 | ✅ | ✅ |
| unsafe impl | ✅ | ✅ |
| 错误信息 | 复杂 | 简化 |

### 命名改进

| Rust | Dast | 原因 |
|------|------|------|
| Send | Sendable | 形容词，更直观 |
| Sync | Shareable | "可共享" 比 "同步" 更清晰 |

---

## 与 ValueType 对比

| 方面 | ValueType | Sendable/Shareable |
|------|-----------|-------------------|
| 学习成本 | 极低 | 低-中 |
| 灵活性 | 中 | 高 |
| 共享可变 | Channel 模式 | Arc<Mutex> 直接 |
| 无锁结构 | ❌ | ✅ |
| 代码量 | 中 | 少 |
| 安全性 | 100% | 100% |

### 代码对比: 共享计数器

```dast
// ValueType: Channel 模式
let (tx, rx) = channel()
spawn(move || {
    let mut count = 0
    for msg in rx {
        match msg {
            .Inc => count += 1,
            .Get(resp) => resp.send(count),
        }
    }
})
tx.send(.Inc)

// Sendable/Shareable: 直接共享
let counter = Arc.new(Mutex.new(0))
spawn(move || *counter.lock() += 1)
```

---

## 适用场景

| 场景 | 适合度 |
|------|--------|
| 高性能服务 | ⭐⭐⭐⭐⭐ |
| 游戏引擎 | ⭐⭐⭐⭐⭐ |
| 数据库 | ⭐⭐⭐⭐⭐ |
| 操作系统 | ⭐⭐⭐⭐⭐ |
| 简单应用 | ⭐⭐⭐⭐ |

---

## 总结

| 指标 | Sendable/Shareable |
|------|-------------------|
| 安全性 | 100% |
| 简单性 | ⭐⭐⭐⭐ |
| 灵活性 | ⭐⭐⭐⭐⭐ |
| 性能 | ⭐⭐⭐⭐⭐ |
| 学习成本 | 低-中 |

**核心权衡**:
```
两个简单概念换取完整灵活性
```
