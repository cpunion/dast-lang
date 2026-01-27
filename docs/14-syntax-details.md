# 语法细节

## 关键字列表

### 声明和定义
```
fn          // 函数
struct      // 结构体
enum        // 枚举
trait       // Trait
impl        // 实现
type        // 类型别名
const       // 常量
comptime    // 编译期
```

### 控制流
```
if          // 条件
else        // 否则
match       // 模式匹配
for         // 循环
while       // 循环
loop        // 无限循环
break       // 跳出
continue    // 继续
return      // 返回
```

### 模块和可见性
```
import      // 导入
from        // 从...导入
pub         // 公开
mod         // 模块（可选）
```

### 内存和所有权
```
let         // 变量绑定
mut         // 可变
ref         // 引用
move        // 移动
unsafe      // 不安全
```

### 异步
```
async       // 异步函数
await       // 等待
```

### 其他
```
as          // 类型转换
in          // 范围/迭代
where       // 约束
self        // 自身
Self        // 类型自身
true        // 真
false       // 假
```

**总计**: ~35 个关键字（比 Rust 少）

---

## 运算符优先级

### 优先级表（从高到低）

| 优先级 | 运算符 | 说明 |
|--------|--------|------|
| 1 | `()` `[]` `.` | 调用、索引、成员访问 |
| 2 | `!` `-` `*` `&` | 一元运算符 |
| 3 | `as` | 类型转换 |
| 4 | `*` `/` `%` | 乘除模 |
| 5 | `+` `-` | 加减 |
| 6 | `<<` `>>` | 位移 |
| 7 | `&` | 位与 |
| 8 | `^` | 位异或 |
| 9 | `\|` | 位或 |
| 10 | `==` `!=` `<` `>` `<=` `>=` | 比较 |
| 11 | `&&` | 逻辑与 |
| 12 | `\|\|` | 逻辑或 |
| 13 | `..` `..=` | 范围 |
| 14 | `=` `+=` `-=` 等 | 赋值 |

### 示例

```dast
// 优先级示例
let x = 2 + 3 * 4      // 14 (乘法优先)
let y = (2 + 3) * 4    // 20 (括号优先)

// 比较链
if 0 < x && x < 10 { }

// 位运算
let flags = 0b1010 | 0b0101  // 0b1111
```

---

## 元组与解构

### 元组类型与字面量

```dast
let t: (i32, String, bool) = (1, "hi", true)
let unit: () = ()
let single = (42,)  // 单元素元组必须写逗号
```

### 元组访问与解构

```dast
let a = t.0
let b = t.1

let (x, y) = (1, 2)
```

> 说明：`(T)` 是分组表达式；`(T,)` 才是单元素元组。

---

## 循环表达式与标签

### loop 表达式与 break 返回值

```dast
let v = loop {
    if cond { break 1 }
    break 2
}
```

- `loop` 可以作为表达式，结果来自 `break <expr>`。
- `break` 可带值的形式仅用于 `loop` 表达式；`while/for` 中只能写 `break`。

### 标签

```dast
outer: loop {
    while cond {
        break outer: 1
    }
}

continue outer
```

- `label: loop/while/for` 定义标签。
- `break label: expr` 跳出指定循环并返回值（仅 `loop` 表达式）。
- `continue label` 跳到指定循环的下一次迭代（`continue label:` 也被接受）。

### for-in

```dast
for (x, y) in pairs {
    println("{x} {y}")
}
```

---

## 注释风格

### 行注释

```dast
// 单行注释
let x = 42  // 行尾注释
```

### 文档注释

```dast
/// 函数文档注释
/// 支持 Markdown
///
/// # 示例
/// ```
/// let result = add(2, 3)
/// assert_eq!(result, 5)
/// ```
pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

> 说明：函数体最后一条语句可以是表达式语句，作为隐式返回值。若最后一条语句是 `if`/`match`，则分支最后的表达式作为返回值。若函数返回 `unit`，则忽略该隐式返回。

/// 结构体文档
pub struct Point {
    /// X 坐标
    pub x: f32,
    /// Y 坐标
    pub y: f32,
}
```

### 模块文档

```dast
//! 模块级文档
//!
//! 这个模块提供数学函数

pub fn sqrt(x: f64) -> f64 { ... }
```

### 块注释（可选）

```dast
/*
 * 多行注释
 * 可以嵌套
 */
```

---

## 字符串插值

### 基础语法

```dast
let name = "World"
let msg = "Hello, {name}!"  // "Hello, World!"

let x = 42
let s = "x = {x}"           // "x = 42"
```

### 表达式插值

```dast
let x = 10
let y = 20
let s = "sum = {x + y}"     // "sum = 30"

// 方法调用
let name = "alice"
let s = "Name: {name.to_uppercase()}"  // "Name: ALICE"
```

### 格式化

```dast
let pi = 3.14159
let s = "pi = {pi:.2}"      // "pi = 3.14"

let x = 42
let s = "hex = {x:x}"       // "hex = 2a"
let s = "bin = {x:b}"       // "bin = 101010"
```

### 原始字符串

```dast
let path = r"C:\Users\name"  // 原始字符串，不转义
let regex = r"\d+"           // 正则表达式
```

### 多行字符串

```dast
let text = """
    多行字符串
    保留缩进
    """
```

---

## 属性语法

```dast
// 函数属性
@inline
fn fast_function() { }

@deprecated(since: "1.0", note: "use new_function instead")
fn old_function() { }

// 条件编译
@cfg(target_os = "linux")
fn linux_only() { }

// 派生
@derive(Clone, Debug)
struct Point { x: f32, y: f32 }

// 测试
@should_panic
fn test_panic() { }

@ignore(reason: "slow test")
fn test_slow() { }
```

---

## 模式匹配

```dast
match value {
    0 => "zero",
    1 | 2 => "one or two",
    3..=9 => "three to nine",
    x if x > 100 => "large",
    _ => "other",
}

// 解构
match point {
    Point { x: 0, y: 0 } => "origin",
    Point { x, y } => "point",
    Point { x: a, y } => "alias binding",
}

// 元组/数组模式
match value {
    (a, b) => a + b,
    [x, y] => x * y,
    _ => 0,
}

// 结构体模式字段绑定规则：
// - `Point { x }` 等价于 `Point { x: x }`
// - `Point { x: a }` 表示字段名是 `x`，绑定名是 `a`

// if let
if let .Some(x) = optional {
    println("{}", x)
}
```

---

## 与其他语言对比

| 特性 | Rust | Go | Dast |
|------|------|-----|------|
| 关键字数量 | ~50 | ~25 | ~35 |
| 字符串插值 | format! | fmt.Sprintf | `{expr}` |
| 文档注释 | /// | // | /// |
| 属性 | #[attr] | // go:attr | @attr |
| 模式匹配 | ✅ | ❌ | ✅ |

---

## 建议

| 特性 | 决策 |
|------|------|
| 关键字 | ~35 个 |
| 注释 | `//` 和 `///` |
| 字符串插值 | `{expr}` 内置 |
| 属性 | `@attr` |
| 模式匹配 | 完整支持 |
