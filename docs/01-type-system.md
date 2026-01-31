# 类型系统设计

## 设计目标

1. **静态强类型** - 编译期类型检查
2. **类型推断** - 减少显式标注
3. **泛型** - 参数化多态
4. **Trait** - 接口抽象
5. **代数数据类型** - enum + struct
6. **简洁** - 比 Rust 更少的语法噪音

---

## 基础类型

### 数值类型

```dast
// 整数
i8, i16, i32, i64          // 有符号
u8, u16, u32, u64          // 无符号
isize, usize               // 指针大小
int                        // 等价于 i64

// 预留（尚未实现）
// i128, u128

// 浮点
f32, f64

// 布尔
bool  // true, false

// 字符
char  // Unicode 码点

> 注：整数字面量与浮点字面量初始为 `untyped-int` / `untyped-float`，
> 在上下文约束时收敛到具体类型；若无约束，默认收敛为 `i64` / `f64`。
```

### 复合类型

```dast
// 数组 (固定大小)
[T; N]       // [i32; 10]

// 切片 (动态大小视图)
[T]          // 只能作为引用 &[T]

// 元组
(T, U, V)    // (i32, String, bool)

// 字符串
str          // 只能作为引用 &str
String       // 拥有的字符串
```

### 位宽 / 对齐（与 C 一致）

- **整数位宽**：`i8/u8=8`，`i16/u16=16`，`i32/u32=32`，`i64/u64=int=64`。
- **布尔**：`bool` 为 **1 字节**，对齐 **1**。
- **字符**：`char` 为 **32 位 Unicode 码点**（4 字节，对齐 4）。
- **浮点**：`f32` 为 4 字节，对齐 4；`f64` 为 8 字节，对齐 8。
- **指针大小**：`isize/usize`、引用、数组头、`String/str` 的布局为 **目标指针宽度**。
- **结构体布局**：与 C 一致（字段按声明顺序放置，按字段对齐对齐，整体大小向最大对齐取整）。

> 指针宽度来自目标平台（例如 `--target` 或环境变量 `DAST_TARGET_PTR_WIDTH`）。

---

## 结构体

```dast
struct Point {
    x: f32,
    y: f32,
}

// 实例化
let p = Point { x: 1.0, y: 2.0 }

// 简写 (变量名与字段名相同)
let x = 1.0
let y = 2.0
let p = Point { x, y }

// 更新语法
let p2 = Point { x: 3.0, ..p }
```

### 元组结构体

```dast
struct Color(u8, u8, u8)

let red = Color(255, 0, 0)
let r = red.0
```

### 单元结构体

```dast
struct Marker

let m = Marker
```

---

## 枚举 (代数数据类型)

```dast
// 简单枚举
enum Direction {
    North,
    South,
    East,
    West,
}

// 带数据的枚举
enum Option[T] {
    Some(T),
    None,
}

enum Result[T, E] {
    Ok(T),
    Err(E),
}

// 复杂数据
enum Message {
    Quit,
    Move { x: i32, y: i32 },
    Write(String),
    Color(u8, u8, u8),
}
```

### 模式匹配

```dast
let msg = Message.Move { x: 10, y: 20 }

match msg {
    .Quit => println("quit"),
    .Move { x, y } => println("move to {}, {}", x, y),
    .Write(text) => println("write: {}", text),
    .Color(r, g, b) => println("color: {}, {}, {}", r, g, b),
}

// 简写: 点号前缀表示当前枚举
let opt: Option[i32] = .Some(42)
let opt: Option[i32] = .None
```

---

## 泛型

### 泛型函数

```dast
fn identity[T](x: T) -> T {
    return x
}

fn swap[T, U](pair: (T, U)) -> (U, T) {
    let (a, b) = pair
    return (b, a)
}
```

### 泛型结构体

```dast
struct Pair[T, U] {
    first: T,
    second: U,
}

struct Vec[T] {
    data: *mut T,
    len: usize,
    cap: usize,
}
```

### 泛型枚举

```dast
enum Option[T] {
    Some(T),
    None,
}

enum Result[T, E] {
    Ok(T),
    Err(E),
}
```

---

## Trait (接口)

### 定义

```dast
trait Display {
    fn display(self: &Self) -> String
}

trait Default {
    fn default() -> Self
}

trait Clone {
    fn clone(self: &Self) -> Self
}
```

### 实现

```dast
impl Display for Point {
    fn display(self: &Self) -> String {
        return format("({}, {})", self.x, self.y)
    }
}

impl Default for Point {
    fn default() -> Self {
        return Point { x: 0.0, y: 0.0 }
    }
}
```

### 混合模式: 自动满足 + 显式扩展

```dast
trait Printable {
    fn print(self: &Self)
}

struct Foo {}

impl Foo {
    fn print(self: &Self) { println("foo") }
}

// ✅ 自动满足 Printable，无需显式 impl Printable for Foo
fn use_printable[T: Printable](t: T) { t.print() }
use_printable(Foo{})  // 直接可用

// 仍可为无方法的类型显式实现
impl Printable for ThirdPartyType {
    fn print(self: &Self) { println("{}", self.value) }
}
```

### 泛型约束

```dast
fn print[T: Display](value: T) {
    println("{}", value.display())
}

// 多个约束
fn process[T: Clone + Display](value: T) {
    let copy = value.clone()
    println("{}", copy.display())
}

// trait 带类型参数
fn iter_sum[T: Iterator[i32]](it: T) -> i32 {
    // ...
}

// where 子句
fn complex[T, U](x: T, y: U) -> T
where
    T: Clone + Into[U],
    U: Display,
{
    println("{}", y.display())
    return x.clone()
}
```

### 关联类型 (可选，简化版)

```dast
trait Iterator {
    type Item
    fn next(self: &mut Self) -> Option[Self.Item]
}

// 或简化为泛型参数
trait Iterator[T] {
    fn next(self: &mut Self) -> Option[T]
}
```

---

## 类型推断

### 局部变量

```dast
let x = 42          // 推断为 i64（未约束时默认 i64）
let y = 3.14        // 推断为 f64
let s = "hello"     // 推断为 &str
let v = Vec.new()   // 需要上下文推断元素类型

v.push(42)          // 现在推断 v: Vec[i32]
```

### 闭包参数

```dast
let add = |a, b| a + b    // 参数类型从使用推断

let result = add(1, 2)     // 推断 a: i32, b: i32
```

### 泛型实例化

```dast
let opt = Option.Some(42)  // Option[i32]
let opt: Option[i32] = .None
```

---

## 数值运算与隐式转换规则（对齐 Zig）

- **整数/浮点二元运算**：两侧类型必须完全一致（含位宽与有/无符号），否则编译错误。
- **untyped-int / untyped-float**：字面量初始为无类型数值，需由上下文约束到具体类型。
  - 未被约束时默认 `untyped-int -> i64`，`untyped-float -> f64`。
  - `untyped-int` 可按上下文收敛为任意整数或浮点类型。
  - `untyped-float` 只能收敛为浮点类型。
- **char**：仅允许比较（`== != < <= > >=`），算术需显式转为整数。
- **不做隐式数值扩展/缩窄**：不同整数宽度或整数/浮点混用必须显式转换。

## 引用与字符串的隐式规则

- 允许 `&mut T -> &T`（只读协变）。
- 允许 `String -> &str` 的只读借用（期望类型为 `&str` 时）。
- 不允许 `String` 直接当作 `str` 值使用（必须显式借用）。
- 字符串字面量默认类型为 `&str`；在**期望 `String`** 的位置允许字面量直接使用（编译器隐式构造 `String`）。

### 隐式转换矩阵（仅在上下文期望类型时生效）

| 来源 | 目标 | 允许 | 说明 |
| --- | --- | --- | --- |
| `untyped-int` | 任意整数类型 | ✅ | 字面量需落入目标范围 |
| `untyped-int` | `f32/f64` | ✅* | **仅字面量**且可精确表示 |
| `untyped-float` | `f32/f64` | ✅ | 
| `String` | `&str` | ✅ | 只读借用；不会产生 `str` 值 |
| `&mut T` | `&T` | ✅ | 只读协变 |
| `"..."` 字面量 | `&str` | ✅ | 字面量默认即 `&str` |
| `"..."` 字面量 | `String` | ✅ | 仅在期望 `String` 时构造 |
| `[lit...]` | `[T]` | ✅ | 若期望为 `[T]`，元素按 `T` 规则收敛 |

> 其它隐式转换一律拒绝；不同整数宽度或 int/float 混用必须显式 `as`。

---

## 类型别名

```dast
type Meters = f64
type Result[T] = Result[T, Error]
type Callback = fn(i32) -> bool
```

---

## 特殊类型

### Never 类型

```dast
fn panic(msg: &str) -> ! {
    // 永不返回
}

fn infinite_loop() -> ! {
    loop { }
}
```

### Unit 类型

```dast
fn no_return() {
    // 隐式返回 ()
}

fn explicit() -> () {
    return ()
}
```

---

## 与 Rust 对比

| 特性 | Rust | Dast | 简化点 |
|------|------|------|--------|
| 泛型语法 | `<T>` | `[T]` | 简化 |
| Trait bound | `T: Clone + Debug` | `T: Clone + Display` | 相同 |
| Where 子句 | 常用 | 少用 | 简化 |
| 关联类型 | 复杂 | 可选/简化 | 简化 |
| 生命周期 | `'a, 'b` | 自动推断 | 大幅简化 |
| 闭包类型 | `Fn/FnMut/FnOnce` | 自动推断 | 简化 |
| dyn Trait | 显式 | 显式 | 相同 |
| impl Trait | 支持 | 支持 | 相同 |

---

## 自动派生

```dast
@derive(Clone, Display, Default)
struct Point {
    x: f32,
    y: f32,
}

// 编译器自动生成实现
```

### 支持的派生

- `Clone` - 克隆
- `Copy` - 按位复制
- `Default` - 默认值
- `Display` - 格式化显示
- `Debug` - 调试显示
- `Eq`, `PartialEq` - 相等比较
- `Ord`, `PartialOrd` - 排序比较
- `Hash` - 哈希
- `Sendable`, `Shareable` - 线程安全

---

## 总结

| 特性 | 状态 |
|------|------|
| 基础类型 | ✅ 与 Rust 类似 |
| 结构体 | ✅ 与 Rust 类似 |
| 枚举 | ✅ 代数数据类型 |
| 泛型 | ✅ 与 Rust 类似 |
| Trait | ✅ 简化版 |
| 类型推断 | ✅ 与 Rust 类似 |
| 生命周期 | ✅ 自动推断 |

**核心简化**: 无显式生命周期标注，其余与 Rust 保持一致。
