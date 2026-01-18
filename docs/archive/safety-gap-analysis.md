# Dast 与 Rust 安全性差距分析

## 概述

Dast 目标是 **90% 编译期安全**，与 Rust 的 **100% 编译期安全** 存在差距。本文档详细分析这些差距。

---

## 安全性对比矩阵

| 安全类别 | Rust | Dast | 差距 |
|---------|------|------|------|
| 内存安全 | 100% 编译期 | ~90% 编译期 + 运行时检查 | ⚠️ 中等 |
| 线程安全 | 100% 编译期 | ~85% 编译期 + 工具检测 | ⚠️ 较大 |
| 类型安全 | 100% | 100% | ✅ 相同 |
| 空指针安全 | 100% | 100% | ✅ 相同 |
| 生命周期安全 | 100% | ~90% | ⚠️ 中等 |
| 数据竞争 | 100% 编译期 | ~80% 编译期 + TSan | ⚠️ 较大 |

---

## 具体差距详解

### 1. 生命周期检查 (中等差距)

#### Rust: 完整的生命周期系统

```rust
// ✅ Rust 可以表达复杂的生命周期关系
struct Parser<'input, 'ctx>
where 'ctx: 'input  // 'ctx 比 'input 活得更久
{
    input: &'input str,
    context: &'ctx Context,
}

impl<'input, 'ctx> Parser<'input, 'ctx> {
    fn parse(&mut self) -> &'input str {
        self.context.log("parsing");
        &self.input[0..5]
    }
}

// 编译器完全理解生命周期关系
fn complex_lifetimes<'a, 'b>(x: &'a str, y: &'b str) -> &'a str
where 'b: 'a
{
    // 可以安全地返回 x，即使 y 的生命周期更长
    x
}
```

#### Dast: 简化的生命周期推断

```dast
// ❌ Dast 无法表达复杂关系
struct Parser {
    input: &str,
    context: &Context,
}

impl Parser {
    fn parse(self: &mut Self) -> &str {
        self.context.log("parsing")
        return self.input[0..5]  // 编译错误: 无法推断生命周期
    }
}

// 需要重构或 unsafe
```

**差距**: 约 10% 的 Rust 代码需要显式生命周期标注，Dast 无法编译

**影响**:
- 需要重构数据结构
- 或使用 unsafe + 手动保证
- 或返回拥有的值 (性能损失)

---

### 2. 借用检查精确度 (中等差距)

#### Rust: 精确的借用检查

```rust
// ✅ Rust 可以证明安全性
fn split_first_mut(slice: &mut [i32]) -> Option<(&mut i32, &mut [i32])> {
    if slice.is_empty() {
        return None;
    }

    let (first, rest) = slice.split_at_mut(1);
    Some((&mut first[0], rest))
}

// 编译器理解: first 和 rest 不重叠
```

#### Dast: 简化的借用检查

```dast
// ⚠️ Dast 允许但可能不安全
fn get_two_mut(arr: &mut [i32], i: usize, j: usize) -> (&mut i32, &mut i32) {
    return (&mut arr[i], &mut arr[j])
    // 如果 i == j，两个引用指向同一位置
    // Debug: 运行时检查
    // Release: UB
}
```

**差距**: Rust 通过 `split_at_mut` 等 API 证明不重叠，Dast 依赖运行时检查

**影响**:
- Debug 模式有性能开销
- Release 模式可能 UB
- 需要程序员手动保证

---

### 3. 线程安全 (较大差距)

#### Rust: Send/Sync 强制检查

```rust
// ✅ Rust 强制实现 Sync 才能跨线程共享
struct Counter {
    value: i32,  // 非原子
}

// 编译错误: Counter 没有实现 Sync
let counter = Arc::new(Counter { value: 0 });
for _ in 0..10 {
    let c = counter.clone();
    thread::spawn(move || {
        c.value += 1;  // 编译错误
    });
}

// 必须使用同步原语
struct Counter {
    value: Mutex<i32>,  // 或 AtomicI32
}
// 现在可以编译
```

#### Dast: 自动推导 Send/Sync

```dast
// ⚠️ Dast 自动推导，可能漏检
struct Counter {
    value: i32,
}

// 编译通过 (Counter 自动推导为 Send)
let counter = Shared.new(Counter { value: 0 })
for _ in 0..10 {
    let c = counter.clone()
    spawn(move || {
        c.value += 1  // 数据竞争！
    })
}
```

**差距**: Rust 强制显式标记，Dast 自动推导可能出错

**影响**:
- 需要 ThreadSanitizer 检测
- 并发 bug 难以复现
- 生产环境可能出现问题

**缓解**:
```bash
# 开发阶段必须运行
$ dast test --sanitize=thread
```

---

### 4. 内部可变性 (中等差距)

#### Rust: 严格的内部可变性规则

```rust
use std::cell::RefCell;

// ✅ Rust 运行时检查借用规则
let cell = RefCell::new(42);

let r1 = cell.borrow();       // 不可变借用
let r2 = cell.borrow();       // OK: 多个不可变借用
let r3 = cell.borrow_mut();   // Panic: 已有不可变借用

// 编译期知道需要运行时检查
```

#### Dast: 类似但可能更宽松

```dast
// ⚠️ Dast Cell 可能检查不够严格
let cell = Cell.new(42)

let r1 = cell.borrow()
let r2 = cell.borrow()
let r3 = cell.borrow_mut()  // 应该 panic，但可能漏检
```

**差距**: Rust 的 RefCell 有完善的运行时检查，Dast 可能实现不够严格

---

### 5. 未定义行为 (较大差距)

#### Rust: 最小化 UB

Rust 的 UB 仅限于 unsafe 块内：
- 解引用悬垂/未对齐指针
- 数据竞争
- 破坏不变量

```rust
// Safe Rust 保证无 UB
fn safe_code() {
    let vec = vec![1, 2, 3];
    // 无论怎么写，都不会 UB (最多 panic)
}
```

#### Dast: 更多潜在 UB

Dast 的 UB 可能出现在 safe 代码中 (Release 模式):
- 非重叠借用假设被违反
- 迭代器失效
- 整数溢出 (可配置)

```dast
// ⚠️ Release 模式可能 UB
fn potentially_ub() {
    let vec = Vec.from([1, 2, 3])
    for item in &vec {
        vec.push(4)  // Release: UB
    }
}
```

**差距**: Rust 将 UB 限制在 unsafe，Dast 在 Release 模式允许更多 UB

---

### 6. 类型系统表达力 (小差距)

#### Rust: 高级类型特性

```rust
// ✅ Rust 支持高级类型特性
trait Iterator {
    type Item;  // 关联类型
    fn next(&mut self) -> Option<Self::Item>;
}

// 高阶 trait bound
fn process<I>(iter: I)
where
    I: Iterator,
    I::Item: Display + Clone,
{
    ...
}

// 生命周期约束
fn complex<'a, T: 'a>(x: &'a T) -> Box<dyn Trait + 'a> {
    ...
}
```

#### Dast: 简化的类型系统

```dast
// ⚠️ Dast 可能不支持某些高级特性
trait Iterator<T> {  // 泛型参数而非关联类型
    fn next(self: &mut Self) -> Option<T>
}

// 简化的约束
fn process<T: Display + Clone>(iter: Iterator<T>) {
    ...
}

// 不支持生命周期约束
fn complex<T>(x: &T) -> Box<dyn Trait> {
    // 无法表达 'a 约束
}
```

**差距**: Dast 类型系统更简单，某些 Rust 模式无法表达

---

### 7. 编译器优化保证 (小差距)

#### Rust: 严格的别名规则

```rust
// ✅ Rust 保证 &mut 独占，编译器可以激进优化
fn optimize(x: &mut i32, y: &i32) {
    *x += 1;
    *x += 1;
    // 编译器知道 x 和 y 不重叠，可以优化为 *x += 2
}
```

#### Dast: 可能更保守

```dast
// ⚠️ Dast 可能无法保证，优化受限
fn optimize(x: &mut i32, y: &i32) {
    *x += 1
    *x += 1
    // 编译器可能需要保守处理
}
```

**差距**: Rust 的别名规则更严格，允许更激进的优化

---

## 差距总结

### 按严重程度排序

1. **线程安全 (最大差距)**
   - Rust: 100% 编译期保证
   - Dast: ~80% + ThreadSanitizer
   - 影响: 并发 bug 可能进入生产

2. **未定义行为范围 (较大差距)**
   - Rust: 仅 unsafe 块
   - Dast: Release 模式 safe 代码也可能 UB
   - 影响: 需要更严格的测试

3. **生命周期表达力 (中等差距)**
   - Rust: 完整的生命周期系统
   - Dast: 简化推断，~10% 代码无法编译
   - 影响: 需要重构或 unsafe

4. **借用检查精确度 (中等差距)**
   - Rust: 精确证明不重叠
   - Dast: 运行时检查
   - 影响: Debug 性能开销

5. **类型系统 (小差距)**
   - Rust: 关联类型、高阶 trait
   - Dast: 简化泛型
   - 影响: 某些模式无法表达

---

## 缓解策略

### 1. 工具链强化

```bash
# 强制使用所有检查
$ dast build --strict --sanitize=all

# CI/CD 流程
$ dast test --sanitize=thread,address,memory
$ dast check --pedantic
$ dast audit unsafe
```

### 2. 分层安全模式

```dast
// 关键模块使用严格模式
@safety(strict)
module crypto {
    // 接近 Rust 级别检查
}

// 普通模块使用标准模式
@safety(standard)
module app {
    // 90% 安全
}
```

### 3. 运行时检查

```dast
// Debug 模式默认启用所有检查
#[cfg(debug)]
const ENABLE_CHECKS = true

// Release 可选启用
$ dast build --release --enable-runtime-checks
```

### 4. 静态分析

```bash
# 使用外部工具
$ dast-analyzer --strict src/
$ dast-miri test  # 类似 Rust Miri 的解释器
```

---

## 量化对比

| 指标 | Rust | Dast | 差距 |
|------|------|------|------|
| 编译期内存安全 | 100% | ~90% | -10% |
| 编译期线程安全 | 100% | ~80% | -20% |
| UB 可能性 (safe 代码) | 0% | ~5% (Release) | +5% |
| 学习曲线 (1-10) | 10 | 4 | -6 |
| 开发速度 | 中 | 快 | +30% |
| 运行时开销 (Debug) | 低 | 中 | +20% |
| 运行时开销 (Release) | 无 | 无 | 0% |

---

## 适用场景建议

### Rust 更适合
- 操作系统内核
- 浏览器引擎
- 加密库
- 底层运行时
- 对安全性要求极高的场景

### Dast 更适合
- 应用层软件
- 游戏引擎
- Web 服务
- CLI 工具
- 快速原型开发
- 需要热更新的场景

---

## 未来改进方向

### 短期 (可行)
- [ ] 改进 Send/Sync 推导算法
- [ ] 增强 ThreadSanitizer 集成
- [ ] 提供更好的错误信息
- [ ] 实现 `dast-miri` 解释器

### 中期 (需要研究)
- [ ] 可选的完整生命周期标注
- [ ] 更精确的借用检查 (NLL)
- [ ] 编译期常量求值

### 长期 (探索性)
- [ ] 形式化验证集成
- [ ] 依赖类型 (Dependent Types)
- [ ] 效应系统 (Effect System)

---

## 结论

**Dast 与 Rust 的安全性差距**:
- **编译期**: 90% vs 100% (~10% 差距)
- **关键差距**: 线程安全、生命周期表达力
- **权衡**: 简单性 vs 绝对安全

**核心理念**:
> 用 10% 的安全性换取 60% 的简单性，
> 通过工具和最佳实践弥补差距

**适用原则**:
- 关键基础设施 → 用 Rust
- 应用层软件 → 用 Dast
- 混合使用 → Rust 写库，Dast 写应用
