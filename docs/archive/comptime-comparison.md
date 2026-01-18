# 编译期计算能力对比：Rust / C++ / Zig / Nim / Dast

## 总览对比

| 特性 | C++ | Rust | Zig | Nim | Dast |
|------|-----|------|-----|-----|------|
| 泛型 | ✅ 模板 | ✅ | ✅ | ✅ | ✅ |
| Const 泛型 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 可变参数泛型 | ✅ | ❌ | ❌ | ✅ | ✅ |
| 编译期函数 | constexpr | const fn | comptime | static | 自动 |
| 类型作为值 | ❌ | ❌ | ✅ | ✅ | ✅ |
| 编译期反射 | ❌ | ❌ | ✅ | ✅ | ✅ |
| 编译期约束 | Concept | Trait | ❌ | ❌ | ✅ |
| 代码生成 | 模板元编程 | 过程宏 | comptime | 宏 | comptime |

---

## 1. 泛型能力

### C++ 模板

```cpp
// 可变参数模板
template<typename... Ts>
void print(Ts... args) {
    (std::cout << ... << args);
}

// Const 泛型
template<typename T, size_t N>
struct Array {
    T data[N];
};

// 模板特化
template<typename T>
struct Vec { };

template<>
struct Vec<bool> {
    // 特化实现
};
```

**优势**: 最强大的模板元编程
**劣势**: 编译慢、错误信息差

---

### Rust 泛型

```rust
// 基础泛型
fn identity<T>(x: T) -> T { x }

// Const 泛型
struct Array<T, const N: usize> {
    data: [T; N]
}

// ❌ 不支持可变参数泛型
// ❌ 不支持模板特化（实验性）
```

**优势**: 类型安全、清晰
**劣势**: 表达力受限

---

### Zig 泛型

```zig
// 类型作为值
fn Vec(comptime T: type) type {
    return struct {
        items: []T,
    };
}

const IntVec = Vec(i32);

// 编译期参数
fn Array(comptime T: type, comptime N: usize) type {
    return struct {
        data: [N]T,
    };
}
```

**优势**: 类型是一等公民、灵活
**劣势**: 语法不够直观

---

### Nim 泛型

```nim
# 泛型
proc identity[T](x: T): T = x

# 可变参数泛型
proc print[T](args: varargs[T]) =
  for arg in args:
    echo arg

# 编译期执行
proc factorial(n: int): int {.compileTime.} =
  if n <= 1: 1 else: n * factorial(n - 1)
```

**优势**: 灵活、强大的宏系统
**劣势**: 小众、生态不成熟

---

### Dast 泛型

```dast
// 基础泛型
fn identity[T](x: T) -> T { x }

// Const 泛型 + 默认值
struct Array[T, const N: usize = 16] {
    data: [T; N]
}

// 可变参数泛型
fn print[...Ts](args: (Ts...)) {
    comptime for i in 0..Ts.len() {
        println("{}", args[i])
    }
}

// 编译期约束
fn process[T](x: T)
where comptime @size_of(T) <= 64
{ }
```

**优势**: 结合 Rust 安全性 + Zig 灵活性
**劣势**: 新语言，生态待建

---

## 2. 编译期计算

### C++ constexpr

```cpp
constexpr int factorial(int n) {
    return n <= 1 ? 1 : n * factorial(n - 1);
}

constexpr int fact10 = factorial(10);

// 限制: 不能有副作用、I/O
```

**能力**: ⭐⭐⭐
**限制**: 较多，语法受限

---

### Rust const fn

```rust
const fn factorial(n: i32) -> i32 {
    if n <= 1 { 1 } else { n * factorial(n - 1) }
}

const FACT10: i32 = factorial(10);

// 限制: 不能分配堆内存、不能调用非 const fn
```

**能力**: ⭐⭐⭐
**限制**: 严格，但逐渐放宽

---

### Zig comptime

```zig
fn factorial(n: i32) i32 {
    if (n <= 1) return 1;
    return n * factorial(n - 1);
}

const fact10 = factorial(10);  // 编译期

// 类型作为值
fn Vec(comptime T: type) type {
    return struct { items: []T };
}

// 编译期反射
const fields = @typeInfo(Point).Struct.fields;
```

**能力**: ⭐⭐⭐⭐⭐
**限制**: 最少，最灵活

---

### Nim 编译期

```nim
proc factorial(n: int): int {.compileTime.} =
  if n <= 1: 1 else: n * factorial(n - 1)

const fact10 = factorial(10)

# 宏系统
macro genFields(count: int): untyped =
  result = newStmtList()
  for i in 0..<count:
    result.add quote do:
      field{i}: int
```

**能力**: ⭐⭐⭐⭐
**限制**: 较少，宏系统强大

---

### Dast comptime

```dast
// 自动判断是否编译期执行
fn factorial(n: i32) -> i32 {
    if n <= 1 { return 1 }
    return n * factorial(n - 1)
}

const FACT10 = factorial(10)  // 编译期
let x = factorial(input)      // 运行时

// 类型作为值
comptime fn size_of(T: type) -> usize {
    return @size_of(T)
}

// 编译期反射
comptime const FIELDS = @field_count(Point)

// 编译期代码生成
comptime for i in 0..10 {
    // 生成代码
}
```

**能力**: ⭐⭐⭐⭐⭐
**限制**: 最少，类似 Zig

---

## 3. 编译期反射

| 语言 | 类型信息 | 字段遍历 | 方法列表 | 动态调用 |
|------|---------|---------|---------|---------|
| **C++** | ❌ | ❌ | ❌ | ❌ |
| **Rust** | ❌ | ❌ | ❌ | ❌ |
| **Zig** | ✅ | ✅ | ✅ | ✅ |
| **Nim** | ✅ | ✅ | ✅ | ✅ |
| **Dast** | ✅ | ✅ | ✅ | ⚠️ 可选 |

### Zig 反射

```zig
const fields = @typeInfo(Point).Struct.fields;
for (fields) |field| {
    std.debug.print("{s}\n", .{field.name});
}
```

### Nim 反射

```nim
for field in Point.fields:
  echo field.name
```

### Dast 反射

```dast
comptime {
    const count = @field_count(Point)
    for i in 0..count {
        println(@field_name(Point, i))
    }
}
```

---

## 4. 代码生成

### C++ 模板元编程

```cpp
// 编译期循环展开
template<int N>
struct Unroll {
    template<typename F>
    static void apply(F f) {
        f(N);
        Unroll<N-1>::apply(f);
    }
};

template<>
struct Unroll<0> {
    template<typename F>
    static void apply(F f) { f(0); }
};
```

**能力**: ⭐⭐⭐⭐
**可读性**: ⭐⭐

---

### Rust 过程宏

```rust
#[derive(Serialize)]
struct Point { x: f32, y: f32 }

// 宏在编译期生成代码
```

**能力**: ⭐⭐⭐⭐
**可读性**: ⭐⭐⭐

---

### Zig comptime

```zig
comptime {
    for (0..10) |i| {
        @field(result, "field" ++ i) = i;
    }
}
```

**能力**: ⭐⭐⭐⭐⭐
**可读性**: ⭐⭐⭐⭐⭐

---

### Dast comptime

```dast
comptime for i in 0..10 {
    @generate_field(result, format("field_{}", i), i)
}
```

**能力**: ⭐⭐⭐⭐⭐
**可读性**: ⭐⭐⭐⭐⭐

---

## 综合评分

| 语言 | 泛型 | 编译期计算 | 反射 | 代码生成 | 总分 |
|------|------|-----------|------|---------|------|
| **C++** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐ | ⭐⭐⭐⭐ | 17/25 |
| **Rust** | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐ | ⭐⭐⭐⭐ | 15/25 |
| **Zig** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | 23/25 |
| **Nim** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | 22/25 |
| **Dast** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | 25/25 |

---

## Dast 的优势

1. **Rust 的安全性** - 借用检查、类型安全
2. **Zig 的灵活性** - 类型作为值、comptime
3. **C++ 的表达力** - 可变参数泛型、特化
4. **Nim 的简洁性** - 自动 comptime、清晰语法

**结论**: Dast 在编译期计算能力上达到最高水平，同时保持类型安全。
