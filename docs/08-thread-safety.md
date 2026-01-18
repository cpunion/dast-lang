# 线程安全

## 核心理念

**编译期保证线程安全** - 通过 Sendable/Shareable trait 防止数据竞争

---

## Sendable/Shareable Trait

### 定义

```dast
// 可以安全地发送到其他线程
trait Sendable { }

// 可以安全地在线程间共享引用
trait Shareable { }
```

### 自动推导

```dast
// 基础类型自动实现
i32, f64, bool, String  // Sendable + Shareable

// 复合类型自动推导
struct Data {
    x: i32,
    y: String,
}
// Data 自动实现 Sendable + Shareable

// 包含非 Shareable 字段
struct Container {
    data: Cell[i32],  // Cell 不是 Shareable
}
// Container 是 Sendable，但不是 Shareable
```

---

## 线程安全类型

### Atomic - 原子类型

```dast
struct Atomic[T] {
    // 原子操作
}

// 自动实现 Sendable + Shareable
let counter = Atomic.new(0)
counter.fetch_add(1)
```

### Mutex - 互斥锁

```dast
struct Mutex[T] {
    // 互斥访问
}

impl[T: Sendable] Mutex[T] {
    fn lock(self: &Self) -> MutexGuard[T]
}

// Mutex[T] 是 Sendable + Shareable (T: Sendable)
let data = Mutex.new(vec![1, 2, 3])
{
    let mut guard = data.lock()
    guard.push(4)
}
```

### Channel - 消息传递

```dast
fn channel[T: Sendable]() -> (Sender[T], Receiver[T])

let (tx, rx) = channel()
tx.send(42)?
let value = rx.recv()?
```

---

## 编译器强制规则

### 规则 1: spawn 要求 Sendable

```dast
fn spawn[F: FnOnce() + Sendable](f: F)

// ✅ OK
let data = vec![1, 2, 3]
spawn(move || {
    println("{:?}", data)  // data 是 Sendable
})

// ❌ 错误
let rc = Rc.new(42)  // Rc 不是 Sendable
spawn(move || {
    println("{}", *rc)  // 编译错误
})
```

### 规则 2: 共享引用要求 Shareable

```dast
struct Shared[T: Sendable + Shareable] {
    // 引用计数，线程安全
}

// ✅ OK
let s = Shared.new(42)  // i32 是 Shareable

// ❌ 错误
let s = Shared.new(Cell.new(42))  // Cell 不是 Shareable
```

### 规则 3: Channel 要求 Sendable

```dast
let (tx, rx) = channel[i32]()  // ✅ OK

let (tx, rx) = channel[&i32]()  // ❌ 引用不是 Sendable
```

---

## 常见类型的线程安全性

| 类型 | Sendable | Shareable |
|------|----------|-----------|
| i32, String | ✅ | ✅ |
| Vec[T] | ✅ (T: Sendable) | ✅ (T: Shareable) |
| Box[T] | ✅ (T: Sendable) | ✅ (T: Shareable) |
| &T | ❌ | - |
| &mut T | ❌ | - |
| Atomic[T] | ✅ | ✅ |
| Mutex[T] | ✅ (T: Sendable) | ✅ |
| Cell[T] | ✅ (T: Sendable) | ❌ |
| Rc[T] | ❌ | ❌ |
| Shared[T] | ✅ (T: Sendable + Shareable) | ✅ |

---

## 实际示例

### 共享计数器

```dast
let counter = Shared.new(Atomic.new(0))

for i in 0..10 {
    let counter = counter.clone()
    spawn(move || {
        counter.fetch_add(1)
    })
}
```

### 互斥锁保护

```dast
let data = Shared.new(Mutex.new(vec![]))

for i in 0..10 {
    let data = data.clone()
    spawn(move || {
        let mut guard = data.lock()
        guard.push(i)
    })
}
```

### Channel 通信

```dast
let (tx, rx) = channel()

spawn(move || {
    tx.send(42).unwrap()
})

let value = rx.recv().unwrap()
```

---

## 与 Rust 对比

| 特性 | Rust | Dast |
|------|------|------|
| 线程安全 trait | Send/Sync | Sendable/Shareable |
| 自动推导 | ✅ | ✅ |
| 编译期检查 | ✅ | ✅ |
| 命名 | 技术性 | 描述性 |

**改进**: 更直观的命名（Sendable/Shareable）

---

## 错误示例

### 数据竞争

```dast
// ❌ 编译错误
let mut counter = 0
spawn(move || {
    counter += 1  // 错误: i32 不是 Shareable
})

// ✅ 正确
let counter = Atomic.new(0)
spawn(move || {
    counter.fetch_add(1)
})
```

### 非线程安全类型

```dast
// ❌ 编译错误
let rc = Rc.new(42)
spawn(move || {
    println("{}", *rc)  // 错误: Rc 不是 Sendable
})

// ✅ 正确
let shared = Shared.new(42)
spawn(move || {
    println("{}", *shared)
})
```

---

## 总结

| 特性 | 决策 |
|------|------|
| 线程安全 | Sendable/Shareable trait |
| 自动推导 | 编译器自动推导 |
| 原子类型 | Atomic[T] |
| 互斥锁 | Mutex[T] |
| 消息传递 | Channel[T] |
| 编译期检查 | 100% 安全 |

## Scoped Threads

### 设计

允许临时借用跨线程，作用域保证安全。

```dast
fn scoped_example() {
    let mut data = vec![1, 2, 3]
    
    std.thread.scope(|scope| {
        scope.spawn(|| {
            // 可以借用 data
            println("{:?}", data)
        })
        
        scope.spawn(|| {
            // 多个线程可以不可变借用
            println("{:?}", data)
        })
    })  // scope 结束，保证所有线程已结束
    
    // data 仍然有效
    data.push(4)
}
```

### 与普通 spawn 对比

```dast
// ❌ 普通 spawn: 不能借用
fn normal_spawn() {
    let data = vec![1, 2, 3]
    spawn(|| {
        println("{:?}", data)  // 错误: data 不是 Sendable
    })
}

// ✅ Scoped spawn: 可以借用
fn scoped_spawn() {
    let data = vec![1, 2, 3]
    std.thread.scope(|scope| {
        scope.spawn(|| {
            println("{:?}", data)  // OK: 作用域保证
        })
    })
}
```

---

## 闭包捕获

### move 闭包的 Sendable 推导

```dast
// 自动推导闭包的 Sendable
let data = vec![1, 2, 3]  // Vec 是 Sendable

spawn(move || {
    // move 闭包捕获 data
    // 编译器推导: 闭包是 Sendable (因为 Vec 是 Sendable)
    println("{:?}", data)
})
```

### 捕获变量的线程安全检查

```dast
// ✅ 捕获 Sendable 类型
let counter = Atomic.new(0)
spawn(move || {
    counter.fetch_add(1)  // OK
})

// ❌ 捕获非 Sendable 类型
let rc = Rc.new(42)
spawn(move || {
    println("{}", *rc)  // 错误: Rc 不是 Sendable
})

// ✅ 使用 Shared 替代
let shared = Shared.new(42)
spawn(move || {
    println("{}", *shared)  // OK
})
```

### 借用捕获 vs 移动捕获

```dast
// 借用捕获: 需要 Scoped Threads
let data = vec![1, 2, 3]
std.thread.scope(|scope| {
    scope.spawn(|| {
        println("{:?}", data)  // 借用
    })
})

// 移动捕获: 普通 spawn
let data = vec![1, 2, 3]
spawn(move || {
    println("{:?}", data)  // 移动
})
// data 不再有效
```

---

## 内部可变性

### Cell vs Mutex

```dast
// Cell: 单线程内部可变
struct Counter {
    value: Cell[i32],  // Cell 不是 Shareable
}

// ✅ 单线程使用
let counter = Counter { value: Cell.new(0) }
counter.value.set(counter.value.get() + 1)

// ❌ 多线程共享
let counter = Shared.new(Counter { value: Cell.new(0) })
// 错误: Counter 不是 Shareable (因为 Cell 不是)

// ✅ 使用 Mutex
struct ThreadSafeCounter {
    value: Mutex[i32],  // Mutex 是 Shareable
}

let counter = Shared.new(ThreadSafeCounter { value: Mutex.new(0) })
spawn(move || {
    let mut guard = counter.value.lock()
    *guard += 1
})
```

---

## 完整示例

### 多线程计数器

```dast
// 使用 Atomic
let counter = Shared.new(Atomic.new(0))

let handles = Vec.new()
for i in 0..10 {
    let counter = counter.clone()
    let handle = spawn(move || {
        for _ in 0..1000 {
            counter.fetch_add(1)
        }
    })
    handles.push(handle)
}

for handle in handles {
    handle.join()
}

println("Counter: {}", counter.load())  // 10000
```

### 生产者-消费者

```dast
let (tx, rx) = channel[i32]()

// 生产者
spawn(move || {
    for i in 0..10 {
        tx.send(i).unwrap()
    }
})

// 消费者
spawn(move || {
    while let .Ok(value) = rx.recv() {
        println("Received: {}", value)
    }
})
```
