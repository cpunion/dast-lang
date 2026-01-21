# IR v0 规范

## 概述

IR v0 是 Dast Stage0/Stage1 编译器的中间表示格式。设计原则：
- **值语义**：参数和 let 变量是值，直接使用
- **显式可变**：mut 变量需要 store/load
- **静态类型**：所有 temp 都有类型标注
- **保留命名**：参数保留原名，temp 用 `tN` 避开冲突

## 文件结构

```
ir v0

type Point = { x: i64, y: i64 }
type Option = { _tag: u32, _value: i64 }

fn add(a: i64, b: i64) -> i64
  block entry0:
    t0: i64 = + a, b
    return t0

fn main() -> unit
  ...
```

## 类型声明

### 结构体
```
type Point = { x: i64, y: i64 }
type Person = { name: String, age: i32 }
```

### 枚举（展开为 struct）

枚举在 IR 中展开为带 tag 的 struct：
```dast
enum Option { Some(i64), None }
```
展开为：
```
type Option = { _tag: u32, _value: i64 }
```

`_tag` 值：Some=0, None=1



### 类型别名
```
type Int = i64
type IntList = [i64; 10]
```

### 泛型实例化命名（Stage1）

泛型类型在 IR 中单态化，名称与 Dast 语法一致：
```
# Dast / IR 名称
Option[i64]
Result[String, Error]
Map[String, i64]
```

## 函数签名

```
fn name(param: Type, ...) -> ReturnType
```

- 参数保留原名
- 返回类型始终标注（无返回值为 `unit`）

## 基本类型

| 类型 | 说明 |
|-----|------|
| `i32`, `i64` | 整数 |
| `bool` | 布尔 |
| `String` | 字符串 |
| `unit` | 空类型 |
| `*T` | 指针/引用 |
| `[T; N]` | 静态数组（固定长度） |

## 指令格式

### 常量
```
t0: i64 = 42
t1: String = "hello"
t2: bool = true
```

### 算术/逻辑运算
```
t0: i64 = + a, b
t1: i64 = - x, y
t2: bool = == a, b
t3: i64 = - x           # 一元取负
t4: bool = ! flag       # 逻辑非
```

### 变量访问

**值变量**（param 和 let）直接使用：
```
fn add(a: i64, b: i64) -> i64
  block entry0:
    t0: i64 = + a, b    # 直接使用 a, b
    return t0
```

**可变变量**（let mut）用 store/load：
```
fn counter() -> i64
  block entry0:
    t0: i64 = 0
    store x, t0         # 写入
    t1: i64 = load x    # 读取
    ...
```

### 引用操作
```
t0: *i64 = addr_of x       # 取地址
t1: i64 = load_ref t0      # 解引用读取
store_ref t0, t1           # 解引用写入
```

### 函数调用
```
t0: i64 = call add(t1, t2)
t1: unit = call println(t0)
```

### 结构体
```
# 创建（常量直接内联）
t0: Point = { x: 0, y: 0 }

# 字段访问（值语义）
t0: i64 = p.x

# 字段赋值（引用）
p.x = val
```

### 枚举（展开为 struct）
```
# Option.Some(42)
t0: Option = { _tag: 0, _value: 42 }

# Option.None
t1: Option = { _tag: 1, _value: 0 }

# 获取 tag
t2: u32 = opt._tag

# 获取 payload
t3: i64 = opt._value
```

### 静态数组
```
# 创建（常量直接内联）
t0: [i64; 3] = [1, 2, 3]

# 索引（常量索引）
t1: i64 = arr[0]

# 索引（变量索引）
t2: i64 = arr[idx]

# 赋值
arr[0] = t1
```

## 控制流

### 无条件跳转
```
jump block_label
```

### 条件分支
```
branch t0, then_block, else_block
```

### 返回
```
return t0      # 返回值
return         # 返回 unit
```

## 完整示例

```
ir v0

type Point = { x: i64, y: i64 }

fn origin() -> Point
  block entry0:
    t0: i64 = 0
    t1: i64 = 0
    t2: Point = { x: t0, y: t1 }
    return t2

fn distance(p: Point) -> i64
  block entry0:
    t0: i64 = p.x
    t1: i64 = p.y
    t2: i64 = * t0, t0
    t3: i64 = * t1, t1
    t4: i64 = + t2, t3
    return t4

fn abs(x: i64) -> i64
  block entry0:
    t0: i64 = 0
    t1: bool = < x, t0
    branch t1, then0, else0

  block then0:
    t2: i64 = - x
    return t2

  block else0:
    return x

fn count() -> i64
  block entry0:
    t0: i64 = 0
    store i, t0
    jump cond0

  block cond0:
    t1: i64 = load i
    t2: i64 = 3
    t3: bool = < t1, t2
    branch t3, body0, after0

  block body0:
    t4: i64 = load i
    t5: i64 = 1
    t6: i64 = + t4, t5
    store i, t6
    jump cond0

  block after0:
    t7: i64 = load i
    return t7
```

## 与 Dast 语法对照

| Dast | IR |
|------|-----|
| `let x = 1` | `t0: i64 = 1` (x → t0) |
| `let mut x = 1` | `store x, t0` |
| `x = x + 1` | `load x` → `+` → `store x` |
| `&mut x` | `addr_of x` |
| `*p` | `load_ref p` |
| `p.x` | `p.x` (值) 或 `load → .x` (mut) |
| `Point { x: 0 }` | `{ x: t0, y: t1 }` |
| `if c { } else { }` | `branch` + blocks |
| `while c { }` | `jump` + `branch` 循环 |

## Stage0/Stage1 覆盖范围

| 特性 | Stage0 | Stage1 |
|-----|--------|--------|
| 基本类型 | ✓ | ✓ |
| 结构体 | ✓ | ✓ |
| 枚举 | ✓ | ✓ |
| 数组 | ✓ | ✓ |
| 引用 | ✓ | ✓ |
| 控制流 | ✓ | ✓ |
| 函数调用 | ✓ | ✓ |
| 闭包 | - | ✓ (展开为 env 参数) |
| 泛型 | - | ✓ (单态化) |
