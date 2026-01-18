# 异步编程详细设计

## 核心原则

**语法与运行时分离** - 编译器只负责生成状态机，运行时由第三方实现

| 层面 | 提供者 | 内容 |
|------|--------|------|
| **语言/编译器** | Dast 核心 | Future trait, async/await 语法转换 |
| **标准库** | 可选 | 基础运行时（参考实现） |
| **第三方** | 生态 | 各种运行时实现 (tokio, async-std 等) |

---

## 核心决策

基于现有设计，采用 **Pull 模式 async/await**（类似 Rust）

---

## 1. Future Trait 设计

### 基础 Trait

```dast
trait Future {
    type Output

    fn poll(self: &mut Self, cx: &Context) -> Poll[Self.Output]
}

enum Poll[T] {
    Ready(T),
    Pending,
}
```

### Context 设计

```dast
struct Context {
    waker: &Waker,
}

trait Waker {
    fn wake(self: &Self)
    fn wake_by_ref(self: &Self)
}
```

---

## 2. async/await 语法

### 基础语法

```dast
async fn fetch_data(url: &str) -> Result[Data, Error] {
    let response = http.get(url).await?
    let body = response.read_body().await?
    return parse(body)
}

// 调用
async fn main() {
    let data = fetch_data("https://api.example.com").await?
}
```

### 编译器转换

```dast
// 源代码
async fn example() -> i32 {
    let x = async_op1().await
    let y = async_op2().await
    return x + y
}

// 编译为状态机
enum ExampleFuture {
    State0,
    State1 { x: i32 },
    State2 { x: i32, y: i32 },
    Done,
}

impl Future for ExampleFuture {
    type Output = i32

    fn poll(self: &mut Self, cx: &Context) -> Poll[i32] {
        loop {
            match self {
                .State0 => {
                    let x = match async_op1().poll(cx) {
                        .Ready(v) => v,
                        .Pending => return .Pending,
                    }
                    *self = .State1 { x }
                }
                .State1 { x } => {
                    let y = match async_op2().poll(cx) {
                        .Ready(v) => v,
                        .Pending => return .Pending,
                    }
                    *self = .State2 { x: *x, y }
                }
                .State2 { x, y } => {
                    return .Ready(x + y)
                }
                .Done => panic("polled after completion"),
            }
        }
    }
}
```

---

## 3. 并发组合

### join (并行等待)

```dast
async fn fetch_all() -> (Data1, Data2) {
    let (a, b) = join(
        fetch_data("url1"),
        fetch_data("url2")
    ).await
    return (a, b)
}

// 实现
async fn join[F1: Future, F2: Future](
    f1: F1,
    f2: F2
) -> (F1.Output, F2.Output) {
    // 同时 poll 两个 future
}
```

### select (竞争等待)

```dast
async fn race() -> Data {
    select! {
        data = fetch_fast() => data,
        data = fetch_slow() => data,
        timeout = sleep(Duration.seconds(5)) => {
            return default_data()
        }
    }
}
```

---

## 4. 取消语义

### 方案 A: Drop = 取消 (Rust 风格)

```dast
async fn operation() {
    let fut = long_running_task()
    // drop(fut) 自动取消
}
```

**优点**: 简单、自动
**缺点**: 无法区分正常完成和取消

### 方案 B: 显式取消

```dast
async fn operation() {
    let handle = spawn(long_running_task())
    handle.cancel()  // 显式取消
}
```

**优点**: 明确
**缺点**: 需要手动管理

### 建议: 混合方案

```dast
// Drop 自动取消（默认）
{
    let fut = task()
}  // 自动取消

// 显式取消（需要时）
let handle = spawn(task())
handle.cancel()
```

---

## 5. 结构化并发

### Scope (作用域并发)

```dast
async fn parallel_work() {
    scope(|s| {
        s.spawn(async { task1().await })
        s.spawn(async { task2().await })
        s.spawn(async { task3().await })
    }).await
    // 所有任务完成后才继续
}
```

### Nursery (托儿所模式)

```dast
async fn supervised() {
    let nursery = Nursery.new()

    nursery.spawn(async { worker1().await })
    nursery.spawn(async { worker2().await })

    // 等待所有任务
    nursery.wait_all().await

    // 或取消所有
    nursery.cancel_all()
}
```

---

## 6. 运行时设计

### 单线程运行时

```dast
fn main() {
    let rt = Runtime.new_single_thread()
    rt.block_on(async {
        main_async().await
    })
}
```

### 多线程运行时

```dast
fn main() {
    let rt = Runtime.new_multi_thread()
        .worker_threads(4)
        .build()

    rt.block_on(async {
        main_async().await
    })
}
```

### 无运行时 (嵌入式)

```dast
fn main() {
    let mut fut = main_async()
    let waker = noop_waker()
    let mut cx = Context { waker: &waker }

    loop {
        match fut.poll(&mut cx) {
            .Ready(v) => break v,
            .Pending => {
                // 等待事件
                wait_for_events()
            }
        }
    }
}
```

---

## 7. 与其他语言对比

| 特性 | JavaScript | Go | Rust | Dast |
|------|-----------|-----|------|------|
| 模型 | Push (Promise) | CSP (goroutine) | Pull (Future) | Pull (Future) |
| 语法 | async/await | go/chan | async/await | async/await |
| 零成本 | ❌ | ❌ | ✅ | ✅ |
| 取消 | AbortController | context | Drop | Drop + 显式 |
| 结构化并发 | ❌ | ❌ | ⚠️ 第三方 | ✅ |

---

## 8. 待确认问题

1. **Stream trait** - 异步迭代器？
2. **AsyncRead/AsyncWrite** - 异步 I/O trait？
3. **Pin** - 是否需要？如何简化？
4. **Send 约束** - 跨线程 Future 的要求？

---

## 建议

| 特性 | 决策 |
|------|------|
| 模型 | Pull (Rust 风格) |
| 取消 | Drop + 显式 |
| 结构化并发 | ✅ 内置 scope |
| 运行时 | 可选（单线程/多线程/无） |
| Pin | 简化版（结合 @pinned） |
