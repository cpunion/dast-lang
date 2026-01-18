# 错误处理模型

## 设计目标

1. **显式** - 错误必须被处理，不能忽略
2. **零成本** - 无运行时开销
3. **简洁** - 语法直观
4. **可组合** - 错误可传播

---

## 核心类型

### Result<T, E>

```dast
enum Result<T, E> {
    Ok(T),
    Err(E),
}

fn divide(a: i32, b: i32) -> Result<i32, String> {
    if b == 0 {
        return .Err("division by zero")
    }
    return .Ok(a / b)
}
```

### Option<T>

```dast
enum Option<T> {
    Some(T),
    None,
}

fn find(arr: &[i32], target: i32) -> Option<usize> {
    for i, v in arr.enumerate() {
        if v == target { return .Some(i) }
    }
    return .None
}
```

---

## 错误传播

### ? 操作符

```dast
fn process() -> Result<Data, Error> {
    let file = File.open("data.txt")?   // 错误自动返回
    let content = file.read_all()?
    let data = parse(content)?
    return .Ok(data)
}
```

### 展开形式

```dast
// ? 等价于
let file = match File.open("data.txt") {
    .Ok(f) => f,
    .Err(e) => return .Err(e.into()),
}
```

---

## 错误类型设计

### 方案 A: 自定义 Error Trait

```dast
trait Error {
    fn message(self: &Self) -> String
    fn source(self: &Self) -> Option<&dyn Error> { .None }
}

struct IoError { code: i32, msg: String }
struct ParseError { line: usize, msg: String }

impl Error for IoError {
    fn message(self: &Self) -> String { self.msg.clone() }
}
```

### 方案 B: 简单错误类型

```dast
// 使用 String 或 enum
fn divide(a: i32, b: i32) -> Result<i32, String>
fn parse(s: &str) -> Result<Data, ParseError>
```

### 方案 C: 错误码 (嵌入式)

```dast
fn read_sensor() -> Result<i32, i32>  // 错误码
```

---

## 模式匹配

```dast
let result = divide(10, 0)

match result {
    .Ok(value) => println("Result: {}", value),
    .Err(msg) => println("Error: {}", msg),
}

// 简写
if let .Ok(value) = result {
    println("Got: {}", value)
}

// unwrap (panic if error)
let value = result.unwrap()

// 默认值
let value = result.unwrap_or(0)

// 链式处理
let value = result
    .map(|x| x * 2)
    .unwrap_or(0)
```

---

## Panic (不可恢复错误)

```dast
fn must_have(opt: Option<i32>) -> i32 {
    match opt {
        .Some(v) => v,
        .None => panic("value required"),
    }
}

// 或使用 unwrap
let value = opt.unwrap()  // None 时 panic
```

### Panic 策略

```bash
# 默认: 栈展开，调用 Drop
$ dast build

# 嵌入式: 直接终止
$ dast build --panic=abort
```

---

## 与其他语言对比

| 特性 | C++ | Go | Rust | Dast |
|------|-----|-----|------|------|
| 异常 | ✅ try/catch | ❌ | ❌ | ❌ |
| 错误码 | ✅ | ✅ | ❌ | ⚠️ 可选 |
| Result 类型 | ⚠️ optional | ⚠️ (val, err) | ✅ | ✅ |
| ? 操作符 | ❌ | ❌ | ✅ | ✅ |
| panic | ❌ | ✅ | ✅ | ✅ |

---

## 可选: throws 语法糖

```dast
// 方案: 类似 Swift 的 throws
fn divide(a: i32, b: i32) throws -> i32 {
    if b == 0 { throw "division by zero" }
    return a / b
}

// 调用
fn caller() throws -> i32 {
    let result = try divide(10, 2)
    return result
}

// 等价于
fn divide(a: i32, b: i32) -> Result<i32, Error> { ... }
fn caller() -> Result<i32, Error> { ... }
```

---

## 最终决策

| 特性 | 决策 |
|------|------|
| 核心机制 | `Result<T, E>` + `Option<T>` |
| 错误传播 | `?` 操作符 |
| try 块 | ✅ 支持 |
| 不可恢复 | `panic` |
| Panic 策略 | unwinding (可选 abort) |
| throws 语法 | ❌ 不支持 (用 Result) |

### try 块语法

```dast
fn main() {
    let result: Result<Data, Error> = try {
        let file = File.open("x.txt")?
        let content = file.read()?
        parse(content)?
    }

    match result {
        .Ok(data) => use(data),
        .Err(e) => println("Error: {}", e),
    }
}
```
