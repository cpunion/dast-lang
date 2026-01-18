# 那 10% 不安全的场景

## 概述

Dast 的 "90% 安全" 意味着：
- **90%** 的代码由编译器静态保证内存安全
- **10%** 需要程序员额外注意或使用 unsafe

这 10% 主要来自**简化规则带来的边缘情况**。

---

## 具体的 10% 场景

### 1. 复杂生命周期推断失败 (~3%)

#### Rust 能编译，Dast 需要 unsafe

```rust
// ✅ Rust: 复杂但合法的生命周期
struct Context<'a, 'b> {
    x: &'a str,
    y: &'b str,
}

fn complex<'a, 'b>(ctx: &Context<'a, 'b>) -> &'a str
where 'b: 'a  // 'b 比 'a 活得更久
{
    ctx.x
}
```

```dast
// ❌ Dast: 编译器无法推断复杂关系
struct Context {
    x: &str,
    y: &str,
}

fn complex(ctx: &Context) -> &str {
    return ctx.x  // 编译错误: 无法确定返回值生命周期
}

// ✅ 解决方案 1: 简化设计
fn complex(x: &str, y: &str) -> &str {
    return x  // OK: 直接参数推断
}

// ✅ 解决方案 2: 返回值而非引用
fn complex(ctx: &Context) -> String {
    return ctx.x.to_string()  // 返回拥有的值
}

// ✅ 解决方案 3: unsafe (高级用户)
fn complex(ctx: &Context) -> &str {
    unsafe {
        // SAFETY: 程序员保证 ctx.x 的生命周期足够长
        return std.mem.transmute(ctx.x)
    }
}
```

**影响**: 约 3% 的 Rust 代码需要重构或 unsafe

---

### 2. 非重叠借用的误判 (~2%)

#### Dast 允许但可能不安全

```dast
// ✅ Dast: 允许（看起来安全）
fn swap_elements(arr: &mut [i32], i: usize, j: usize) {
    let a = &mut arr[i]
    let b = &mut arr[j]
    std.mem.swap(a, b)  // 如果 i == j 呢？
}

// 调用
swap_elements(&mut data, 5, 5)  // 运行时: i == j，两个可变引用指向同一位置
```

**Rust 的处理**: 编译错误，强制使用 `split_at_mut`
**Dast 的处理**:
- Debug 模式: 运行时检查 `i != j`，panic
- Release 模式: 未定义行为 (UB)

```dast
// ✅ 安全的写法
fn swap_elements(arr: &mut [i32], i: usize, j: usize) {
    assert(i != j, "indices must be different")
    let a = &mut arr[i]
    let b = &mut arr[j]
    std.mem.swap(a, b)
}
```

**影响**: 约 2% 的代码需要手动添加运行时检查

---

### 3. 自引用结构的移动 (~1%)

#### @pinned 的限制

```dast
@pinned
struct SelfRef {
    data: String,
    ptr: &String,
}

fn create() -> Box<SelfRef> {
    let mut obj = Box.new(SelfRef {
        data: "hello".to_string(),
        ptr: null,
    })
    obj.ptr = &obj.data
    return obj
}

fn use_it() {
    let obj1 = create()
    let obj2 = obj1  // ❌ 编译错误: @pinned 类型不能移动

    // ✅ 只能通过引用使用
    let obj = create()
    process(&obj)
}
```

**Rust 的处理**: Pin<Box<T>>，复杂但完全安全
**Dast 的处理**: @pinned 禁止移动，简单但限制更多

**影响**: 约 1% 的代码需要重新设计数据结构

---

### 4. 迭代器失效 (~2%)

#### Rust 禁止，Dast 允许但危险

```rust
// ❌ Rust: 编译错误
let mut vec = vec![1, 2, 3];
for item in &vec {
    vec.push(4);  // 编译错误: 迭代时不能修改
}
```

```dast
// ⚠️ Dast: 编译通过，但运行时可能崩溃
let vec = Vec.from([1, 2, 3])
for item in &vec {
    vec.push(4)  // Debug: panic，Release: UB
}

// ✅ 正确写法
let vec = Vec.from([1, 2, 3])
let snapshot = vec.clone()
for item in &snapshot {
    vec.push(item + 10)
}
```

**影响**: 约 2% 的代码在 debug 模式会 panic

---

### 5. 并发数据竞争 (~1%)

#### Send/Sync 自动推导的风险

```dast
struct Counter {
    value: i32,  // 非原子
}

// Dast 自动推导: Counter 是 Send (可以跨线程传递)
// 但没有 Sync (不能跨线程共享引用)

fn race_condition() {
    let counter = Counter { value: 0 }

    // ✅ OK: 移动到另一个线程
    spawn(move || {
        counter.value += 1
    })

    // ❌ 如果用 Shared<Counter>
    let shared = Shared.new(Counter { value: 0 })
    for _ in 0..10 {
        let c = shared.clone()
        spawn(move || {
            c.value += 1  // 数据竞争！
        })
    }
}

// ✅ 正确: 使用原子类型
struct Counter {
    value: Atomic<i32>,
}
```

**Rust 的处理**: Sync trait 强制使用 Mutex/Atomic
**Dast 的处理**:
- 编译器尽力推导
- 边缘情况可能漏检
- 依赖 ThreadSanitizer 等工具

**影响**: 约 1% 的并发代码需要额外测试

---

### 6. 整数溢出 (~1%)

```dast
fn calculate(x: i32) -> i32 {
    return x + 1  // 如果 x = i32::MAX 呢？
}

// Debug 模式: panic
// Release 模式: 回绕 (wrapping)
```

**Rust 的处理**: 默认 panic (debug) / wrapping (release)
**Dast 的处理**: 相同，但可配置

**影响**: 需要显式使用 `checked_add` / `saturating_add`

---

## 10% 的分布

```
┌─────────────────────────────────────────────────────────┐
│              那 10% 不安全的来源                         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  1. 复杂生命周期推断失败           ~3%                   │
│     ├─ 需要重构设计                                     │
│     └─ 或使用 unsafe                                    │
│                                                         │
│  2. 非重叠借用误判                 ~2%                   │
│     ├─ Debug: 运行时检查                                │
│     └─ Release: 需手动 assert                           │
│                                                         │
│  3. 自引用结构限制                 ~1%                   │
│     └─ @pinned 限制移动                                 │
│                                                         │
│  4. 迭代器失效                     ~2%                   │
│     ├─ Debug: panic                                     │
│     └─ Release: UB                                      │
│                                                         │
│  5. 并发数据竞争                   ~1%                   │
│     └─ 依赖测试工具检测                                 │
│                                                         │
│  6. 整数溢出                       ~1%                   │
│     └─ 需显式使用 checked_* 方法                        │
│                                                         │
│  总计                              ~10%                  │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## 缓解策略

### 1. 分层安全模式

```bash
# 严格模式 (接近 Rust)
$ dast build --strict
# - 禁用非重叠借用优化
# - 强制显式生命周期标注 (边缘情况)
# - 整数溢出总是 panic

# 标准模式 (默认)
$ dast build
# - 90% 编译期检查
# - Debug 运行时检查
# - Release 假设正确

# 宽松模式 (性能优先)
$ dast build --unsafe-optimizations
# - 禁用所有运行时检查
# - 假设程序员正确
```

### 2. 工具链支持

```bash
# 静态分析
$ dast check --pedantic
Warning: Potential iterator invalidation at src/main.dast:42

# 运行时检测
$ dast test --sanitize=address,thread,memory
# - AddressSanitizer: 内存错误
# - ThreadSanitizer: 数据竞争
# - MemorySanitizer: 未初始化内存

# Fuzzing
$ dast fuzz target_function
```

### 3. 渐进式采用

```dast
// 标记模块的安全级别
@safety(strict)  // 此模块使用严格检查
module critical {
    // 关键代码，接近 Rust 级别检查
}

@safety(standard)  // 默认
module app {
    // 普通应用代码
}

@safety(unsafe)  // 性能关键
module hot_path {
    // 手动优化，程序员保证安全
}
```

---

## 与 Rust 的对比

| 场景 | Rust | Dast | 权衡 |
|------|------|------|------|
| 复杂生命周期 | ✅ 编译通过 | ❌ 需重构/unsafe | 简单 vs 表达力 |
| 非重叠借用 | ❌ 保守拒绝 | ✅ 允许 + 运行时检查 | 灵活 vs 绝对安全 |
| 自引用结构 | Pin (复杂) | @pinned (简单) | 易用 vs 灵活性 |
| 迭代器失效 | ✅ 编译禁止 | ⚠️ 运行时检测 | 性能 vs 安全 |
| 并发安全 | ✅ 完全保证 | ⚠️ 尽力推导 | 简单 vs 保证 |

---

## 实际影响评估

### 对于不同类型项目

| 项目类型 | 受影响程度 | 建议 |
|---------|-----------|------|
| CLI 工具 | < 5% | 标准模式即可 |
| Web 服务 | < 8% | 标准模式 + 测试 |
| 系统编程 | ~10% | 严格模式 + 审计 |
| 嵌入式 | < 5% | 无堆模式 + 静态分析 |
| 游戏引擎 | ~10% | 混合模式 (热路径 unsafe) |

### 学习曲线对比

```
Rust:  ████████████████████ (难度 10/10)
       ↓ 需要完全理解生命周期、借用检查

Dast:  ████████░░░░░░░░░░░░ (难度 4/10)
       ↓ 大部分情况自动处理
       ↓ 边缘情况有明确错误提示
```

---

## 总结

**那 10% 是**:
1. 编译器无法自动推断的复杂场景
2. 简化规则允许但可能不安全的操作
3. 需要运行时检查或程序员保证的情况

**缓解方法**:
1. Debug 模式运行时检查
2. 静态分析工具
3. 测试 + Sanitizers
4. 可选的严格模式

**核心理念**:
> 让 90% 的代码简单易写，剩余 10% 用 unsafe + 工具保障

---

## 待讨论

- [ ] 是否需要默认启用严格模式？
- [ ] 运行时检查的性能开销是否可接受？
- [ ] 是否需要 `@safety` 注解系统？
