# ValueType-Only 线程安全方案

## 核心理念

**不共享内存，只传递值** (Don't share memory, share by communicating)

```dast
// ❌ 禁止: 跨线程访问共享变量
spawn(|| shared_data.modify())

// ✅ 只允许: 移动所有权 + Channel 通信
spawn(move || owned_data.process())
channel.send(data)
```

---

## 设计规则

### 规则 1: spawn 只能 move 捕获

```dast
let data = Data.new()

// ✅ 移动所有权
spawn(move || {
    data.process()  // data 已移动，线程独占
})
// data 不可再用

// ❌ 禁止借用捕获
spawn(|| {
    data.read()  // 编译错误: 不能捕获外部引用
})
```

### 规则 2: Channel 只传值类型

```dast
// 值类型 trait (自动推导)
trait ValueType {}
// 自动实现: 基础类型, String, Vec<V>, Box<V>, 结构体(所有字段是 ValueType)
// 不实现: *mut T, *const T, &T, &mut T

// Channel 约束
let (tx, rx) = channel<T>()  // 要求 T: ValueType

// ✅ OK
channel<i32>()
channel<String>()
channel<Vec<Data>>()

// ❌ 编译错误
channel<*mut i32>()   // 裸指针
channel<&Data>()      // 引用
```

### 规则 3: 共享只读

```dast
// Shared<T> 允许共享，但只能读
let config = Shared.new(Config.load())

spawn(move || config.read())   // ✅ 读
spawn(move || config.read())   // ✅ 多线程读
spawn(move || config.modify()) // ❌ 编译错误: 不可修改
```

### 规则 4: Scoped Threads 临时借用

```dast
fn process(data: &Data) {
    // scoped 保证所有线程在作用域内完成
    scoped(|s| {
        s.spawn(|| data.read())   // ✅ 可以借用
        s.spawn(|| data.read())   // ✅ 多个读
    })  // 等待所有线程完成
    // data 仍有效
}
```

---

## 优势

### 1. 零学习成本
- 无需理解 Send/Sync/Sendable/Shareable
- 规则简单: "移动或 Channel"

### 2. 100% 编译期安全
- 不可能存在数据竞争 (在 safe 代码中)
- 无运行时检查开销

### 3. 无额外语法
- 不需要 trait bound
- 不需要特殊标记

### 4. 错误信息清晰
```dast
// 错误信息
spawn(|| data.read())
// Error: spawn 闭包不能捕获外部引用 'data'
// Help: 使用 move 移动所有权，或使用 channel 通信
```

### 5. 强制良好架构
- 鼓励消息传递
- 减少共享状态
- 代码更易理解

---

## 劣势

### 1. 共享可变状态需要 Channel 模式

```dast
// Rust 可以直接共享
let counter = Arc::new(Mutex::new(0));
spawn(|| *counter.lock() += 1);

// ValueType 需要 Actor 模式
fn counter_actor(rx: Receiver<Msg>) {
    let mut count = 0;
    for msg in rx {
        match msg {
            .Inc(resp) => { count += 1; resp.send(()) }
            .Get(resp) => resp.send(count)
        }
    }
}
```

**影响**: 代码更冗长

### 2. 高性能共享数据受限

```dast
// Rust: 零复制共享读
let data: Arc<LargeData> = Arc::new(load_data());
spawn(|| data.read());  // 零复制

// ValueType: 需要 Shared (只读)
let data = Shared.new(load_data());
spawn(move || data.read());  // OK 但创建 Shared 有开销
```

**影响**: 某些场景有额外开销

### 3. 无法共享非只读资源

```dast
// Rust: 共享连接池
let pool = Arc::new(Pool::new());
spawn(|| pool.acquire());  // 内部有 Mutex

// ValueType: 需要 Channel 包装
// 更复杂的实现
```

**影响**: 某些模式无法直接表达

### 4. FFI 受限

```dast
// 需要 unsafe 绕过检查
unsafe fn send_raw_pointer() {
    let ptr: *mut Data = ...;
    channel.send_unsafe(ptr);  // 特殊 unsafe API
}
```

**影响**: FFI 代码需要更多 unsafe

---

## 与 Rust 对比

### 语法对比

```rust
// Rust: 需要理解 Send/Sync
fn spawn<F>(f: F) where F: FnOnce() + Send + 'static
Arc<T> where T: Send + Sync
```

```dast
// Dast (ValueType): 无特殊 trait
fn spawn<F>(f: F) where F: FnOnce()  // 编译器自动检查
Shared<T>  // 只允许不可变
```

### 共享状态对比

| 场景 | Rust | Dast (ValueType) |
|------|------|-----------------|
| 只读共享 | `Arc<T>` | `Shared<T>` |
| 可变共享 | `Arc<Mutex<T>>` | Channel/Actor |
| 原子操作 | `AtomicI32` | `Atomic<i32>` |
| 临时借用 | 复杂 | `scoped` |

### 代码量对比

```rust
// Rust: 共享计数器
use std::sync::{Arc, Mutex};
let counter = Arc::new(Mutex::new(0));
let c = counter.clone();
thread::spawn(move || {
    *c.lock().unwrap() += 1;
});
```

```dast
// Dast: Channel 模式
let (tx, rx) = channel()
spawn(move || {
    for _ in rx { count += 1 }
})
tx.send(())
```

**Rust 更简洁于共享可变，Dast 更简洁于消息传递**

### 安全性对比

| 方面 | Rust | Dast (ValueType) |
|------|------|-----------------|
| 编译期检查 | 100% | 100% |
| Send/Sync 理解 | 需要 | 不需要 |
| 错误可能性 | 低 | 极低 |
| unsafe 绕过 | 有 | 有 (更少场景) |

### 表达力对比

| 模式 | Rust | Dast (ValueType) |
|------|------|-----------------|
| 消息传递 | ✅ | ✅ |
| 共享只读 | ✅ | ✅ |
| 共享可变 | ✅ 直接 | ⚠️ Channel |
| 细粒度锁 | ✅ | ❌ |
| 无锁数据结构 | ✅ | ❌ |

---

## 适用场景

### ✅ 非常适合

- Web 服务 (请求独立)
- CLI 工具
- 数据处理管道
- 游戏逻辑 (Actor 模式)
- 业务应用

### ⚠️ 可行但受限

- 高性能数据库
- 游戏引擎核心
- 操作系统内核

### ❌ 不太适合

- 无锁数据结构库
- 高频共享状态系统

---

## 总结

| 指标 | ValueType-Only |
|------|---------------|
| 安全性 | 100% |
| 简单性 | ⭐⭐⭐⭐⭐ |
| 灵活性 | ⭐⭐⭐ |
| 性能 | ⭐⭐⭐⭐ |
| 学习成本 | 极低 |

**核心权衡**:
```
用少量灵活性换取极大的简单性
```

**适合 Dast 的原因**:
1. Dast 目标是"简单但安全"
2. 大多数应用不需要复杂共享
3. 与借用检查的简化哲学一致
4. 高级需求可用 unsafe 绕过
