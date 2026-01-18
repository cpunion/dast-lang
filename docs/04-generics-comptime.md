# 泛型、编译期执行与类型推断

## 泛型 (Generics)

### 基础泛型

```dast
// 泛型函数
fn identity[T](x: T) -> T {
    return x
}

// 泛型结构体
struct Pair[T, U] {
    first: T,
    second: U,
}

// 泛型枚举
enum Option[T] {
    Some(T),
    None,
}
```

### 泛型约束

```dast
// 单个约束
fn print[T: Display](value: T) {
    println("{}", value.display())
}

// 多个约束
fn process[T: Clone + Display](value: T) {
    let copy = value.clone()
    println("{}", copy.display())
}

// where 子句 (复杂场景)
fn complex[T, U](x: T, y: U) -> T
where
    T: Clone + Into[U],
    U: Display,
{
    println("{}", y.display())
    return x.clone()
}
```

### 泛型特化 (可选)

```dast
// 通用实现
impl[T] Vec[T] {
    fn len(self: &Self) -> usize { self.len }
}

// 特化实现 (为特定类型优化)
impl Vec[u8] {
    fn as_bytes(self: &Self) -> &[u8] {
        // 零复制转换
    }
}
```

---

## 编译期执行 (comptime)

### 设计理念

**类似 Zig 的 comptime** - 用普通函数语法，编译期执行

### comptime 函数

```dast
// 编译期函数
comptime fn factorial(n: i32) -> i32 {
    if n <= 1 { return 1 }
    return n * factorial(n - 1)
}

// 使用
const FACT_10: i32 = factorial(10)  // 编译期计算
```

### comptime 变量

```dast
comptime let size = 1024 * 1024  // 编译期常量

let buffer: [u8; size]  // 使用编译期值
```

### 编译期代码生成

```dast
// 编译期生成数组
comptime fn generate_table() -> [i32; 256] {
    let mut table: [i32; 256] = undefined
    for i in 0..256 {
        table[i] = i * i
    }
    return table
}

const SQUARE_TABLE: [i32; 256] = generate_table()
```

### 类型作为值

```dast
// 类型可以作为 comptime 参数
comptime fn size_of(T: type) -> usize {
    return @size_of(T)
}

const I32_SIZE: usize = size_of(i32)  // 4
```

---

## 类型推断

### 局部变量推断

```dast
let x = 42          // 推断为 i32
let y = 3.14        // 推断为 f64
let s = "hello"     // 推断为 &str

// 需要上下文
let v = Vec.new()   // 类型未知
v.push(42)          // 现在推断为 Vec[i32]
```

### 泛型实例化推断

```dast
fn identity[T](x: T) -> T { x }

// 从参数推断
let a = identity(42)      // T = i32
let b = identity("hi")    // T = &str

// 从返回类型推断
let c: f64 = identity(3.14)  // T = f64

// 显式指定 (turbofish)
let d = identity::[i32](42)
```

### 返回类型推断

```dast
fn parse[T: FromStr](s: &str) -> Result[T, Error] {
    T.from_str(s)
}

// 延迟推断
let x = parse("42")        // 类型未知
let y: i32 = x?            // 推断 T = i32

// 或直接推断
let num: i32 = parse("42")?

// 无法推断时使用 turbofish
let num = parse::[i32]("42")?
```

### Turbofish 语法 `::[T]`

显式指定泛型参数，避免歧义：

```dast
// 无法推断时必须使用
parse::[i32]("42")
Vec::[i32].new()
collect::[Vec[i32]]()

// 避免与索引冲突
funcs[0](42)               // 闭包数组索引 + 调用
parse::[i32](42)           // 泛型参数 + 调用
```

**命名来源**: `::` + `[` 看起来像鱼 🐟 (Rust 的 `::<>` 叫 turbofish)

---

### 闭包类型推断

```dast
// 参数和返回类型自动推断
let add = |a, b| a + b

let result = add(1, 2)     // 推断 a: i32, b: i32, 返回 i32
```

---

## 与其他语言对比

### 泛型

| 语言 | 泛型语法 | 特化 | 约束 |
|------|---------|------|------|
| C++ | `template<T>` | ✅ | Concept |
| Rust | `<T>` | ⚠️ 实验性 | Trait bound |
| Dast | `[T]` | ⚠️ 可选 | Trait bound |

### 编译期执行

| 语言 | 机制 | 能力 |
|------|------|------|
| C++ | `constexpr` | 受限 |
| Rust | `const fn` | 受限 |
| Zig | `comptime` | 完整 |
| Dast | `comptime` | 完整 (类似 Zig) |

### 类型推断

| 语言 | 局部变量 | 泛型 | 返回类型 |
|------|---------|------|---------|
| C++ | `auto` | ✅ | ❌ |
| Rust | ✅ | ✅ | ⚠️ 部分 |
| Dast | ✅ | ✅ | ⚠️ 部分 |

---

## comptime 高级用法

### 编译期反射

```dast
comptime fn field_count(T: type) -> usize {
    return @field_count(T)
}

struct Point { x: f32, y: f32 }
const FIELDS: usize = field_count(Point)  // 2
```

### 编译期条件

```dast
fn process[T](value: T) {
    comptime if @is_integer(T) {
        // 整数特定代码
        return value * 2
    } else {
        // 其他类型代码
        return value
    }
}
```

### 编译期循环展开

```dast
comptime fn unroll_sum(arr: &[i32]) -> i32 {
    let mut sum = 0
    comptime for i in 0..arr.len() {
        sum += arr[i]  // 编译期展开为 N 个加法
    }
    return sum
}
```

---

## 待讨论

1. **泛型特化** - 是否支持？优先级？
2. **comptime 边界** - 哪些操作允许编译期执行？
3. **类型推断限制** - 何时必须显式标注？
4. **编译期反射** - 支持到什么程度？

---

## 最终语法决策

### 泛型与数组语法

```dast
// 数组（分号）
[i32; 256]             // 固定大小数组，值类型
&[i32]                 // 切片引用

// 泛型（逗号 + 方括号）
Vec[i32]               // 单参数
Map[string, i32]       // 多参数
Matrix[T, const N: usize]  // 类型 + const 参数
```

**规则**:
- 数组用分号 `;` 分隔
- 泛型用逗号 `,` 分隔
- 泛型用方括号 `[]` 而非尖括号 `<>`

---

## 最终决策

| 特性 | 决策 |
|------|------|
| 泛型符号 | `[]` (方括号) |
| 数组语法 | `[T; N]` (Rust 风格) |
| Turbofish | `::[T]` (显式泛型参数) |
| 类型推断 | 延迟推断 (Rust 风格) |
| 闭包泛型 | ✅ 支持 |
| 泛型约束 | Trait bound |
| 泛型特化 | 可选支持 |
| comptime | 类似 Zig，完整支持 |
| 编译期反射 | 基础支持 |
