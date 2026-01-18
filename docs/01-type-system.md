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
i8, i16, i32, i64, i128    // 有符号
u8, u16, u32, u64, u128    // 无符号
isize, usize               // 指针大小

// 浮点
f32, f64

// 布尔
bool  // true, false

// 字符
char  // Unicode 码点
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
let x = 42          // 推断为 i32
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
