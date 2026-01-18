# 高级泛型特性

## 1. Const 泛型默认值

### 基础语法

```dast
// 带默认值的 const 泛型参数
struct Buffer[T, const SIZE: usize = 1024] {
    data: [T; SIZE]
}

// 使用默认值
let buf1: Buffer[u8]           // SIZE = 1024
let buf2: Buffer[u8, 2048]     // SIZE = 2048
```

### 编译期计算默认值

```dast
const DEFAULT_SIZE: usize = 64

struct Array[T, const N: usize = DEFAULT_SIZE * 2] {
    data: [T; N]
}

// N = 128
let arr: Array[i32]

// 更复杂的计算
const fn next_power_of_two(n: usize) -> usize {
    let mut p = 1
    while p < n { p *= 2 }
    return p
}

struct AlignedBuffer[T, const SIZE: usize = next_power_of_two(100)] {
    data: [T; SIZE]  // SIZE = 128
}
```

### 依赖其他泛型参数

```dast
struct Matrix[T, const ROWS: usize, const COLS: usize = ROWS] {
    data: [T; ROWS * COLS]
}

// 方阵
let m1: Matrix[f32, 4]        // 4x4
let m2: Matrix[f32, 3, 5]     // 3x5
```

---

## 2. 编译期约束 (Comptime Where)

### 基础约束

```dast
// 约束类型大小
fn process[T](value: T)
where
    comptime @size_of(T) <= 64
{
    // 只接受 <= 64 字节的类型
}

process(i32)      // ✅ 4 字节
process([u8; 64]) // ✅ 64 字节
process([u8; 65]) // ❌ 编译错误
```

### 多个编译期约束

```dast
fn optimized[T](arr: &[T])
where
    comptime @is_integer(T),
    comptime @size_of(T) <= 8
{
    // 只接受 <= 8 字节的整数类型
}
```

### Const 参数约束

```dast
struct SmallArray[T, const N: usize]
where
    comptime N > 0,
    comptime N <= 256
{
    data: [T; N]
}

let arr1: SmallArray[i32, 10]   // ✅
let arr2: SmallArray[i32, 0]    // ❌ N > 0
let arr3: SmallArray[i32, 300]  // ❌ N <= 256
```

### 复杂表达式约束

```dast
struct Matrix[T, const ROWS: usize, const COLS: usize]
where
    comptime ROWS * COLS <= 1024,  // 总元素数限制
    comptime ROWS > 0 && COLS > 0
{
    data: [T; ROWS * COLS]
}
```

---

## 3. 可变参数泛型 (Variadic Generics)

### 基础语法

```dast
// 可变数量的类型参数
fn print_all[...Ts](values: (Ts...)) {
    comptime for i in 0..Ts.len() {
        println("{}", values[i])
    }
}

print_all((42, "hello", 3.14))
// Ts = (i32, &str, f64)
```

### 元组操作

```dast
// 元组求和（类型必须相同）
fn tuple_sum[T, const N: usize](values: (T, T, T...)) -> T
where
    T: Add
{
    let mut sum = values.0
    comptime for i in 1..N {
        sum = sum + values[i]
    }
    return sum
}

let result = tuple_sum((1, 2, 3, 4))  // 10
```

### 可变参数结构体

```dast
struct Tuple[...Ts] {
    comptime for i in 0..Ts.len() {
        field_{i}: Ts[i]
    }
}

// 等价于
struct Tuple[T0, T1, T2] {
    field_0: T0,
    field_1: T1,
    field_2: T2,
}
```

### 类型映射

```dast
// 将所有类型包装为 Option
fn wrap_all[...Ts](values: (Ts...)) -> (Option[Ts]...) {
    comptime for i in 0..Ts.len() {
        result[i] = Some(values[i])
    }
    return result
}

let wrapped = wrap_all((42, "hi"))
// 返回: (Option[i32], Option[&str])
```

---

## 与其他语言对比

| 特性 | C++ | Rust | Dast |
|------|-----|------|------|
| Const 泛型默认值 | ✅ | ⚠️ 实验性 | ✅ |
| 编译期约束 | Concept | ❌ | ✅ |
| 可变参数泛型 | ✅ | ❌ | ✅ |

---

## 实际应用

### 1. 固定大小向量

```dast
struct Vec[T, const N: usize = 3]
where
    comptime N > 0,
    comptime N <= 4
{
    data: [T; N]
}

impl[T, const N: usize] Vec[T, N] {
    fn dot(self: &Self, other: &Self) -> T
    where T: Mul + Add
    {
        let mut sum = self.data[0] * other.data[0]
        comptime for i in 1..N {
            sum = sum + self.data[i] * other.data[i]
        }
        return sum
    }
}

let v1 = Vec { data: [1.0, 2.0, 3.0] }
let v2 = Vec { data: [4.0, 5.0, 6.0] }
let dot = v1.dot(&v2)  // 32.0
```

### 2. 类型安全的 printf

```dast
fn printf[...Args](format: &str, args: (Args...)) {
    // 编译期检查格式字符串
    comptime {
        let placeholders = count_placeholders(format)
        if placeholders != Args.len() {
            @compile_error("参数数量不匹配")
        }
    }

    // 运行时格式化
    comptime for i in 0..Args.len() {
        print_arg(args[i])
    }
}

printf("x={}, y={}", (10, 20))  // ✅
printf("x={}", (10, 20))        // ❌ 编译错误
```

---

## 建议

| 特性 | 优先级 | 复杂度 |
|------|--------|--------|
| Const 默认值 | 高 | 低 |
| 编译期约束 | 高 | 中 |
| 可变参数泛型 | 中 | 高 |

**第一阶段**: Const 默认值 + 编译期约束
**第二阶段**: 可变参数泛型
