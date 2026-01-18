# Comptime 编译期执行详细设计

## 核心理念

**类似 Zig 的 comptime** - 用普通代码语法，编译期执行

---

## Comptime 的能力边界

### 允许的操作

```dast
comptime fn factorial(n: i32) -> i32 {
    // ✅ 控制流
    if n <= 1 { return 1 }

    // ✅ 递归
    return n * factorial(n - 1)

    // ✅ 循环
    let mut sum = 0
    for i in 0..n {
        sum += i
    }
    return sum
}

comptime fn generate_table() -> [i32; 256] {
    // ✅ 数组操作
    let mut table: [i32; 256] = undefined
    for i in 0..256 {
        table[i] = i * i
    }
    return table
}
```

### 禁止的操作

```dast
comptime fn bad() {
    // ❌ I/O 操作
    let file = File.open("x.txt")

    // ❌ 网络
    let response = http.get("...")

    // ❌ 系统调用
    let time = std.time.now()

    // ❌ 随机数
    let r = random()
}
```

**规则**: 编译期执行必须是确定性的、纯函数式的

---

## Comptime 变量

```dast
// 编译期常量
comptime let SIZE = 1024 * 1024
let buffer: [u8; SIZE]

// 编译期计算
comptime let FACTORIAL_10 = factorial(10)

// 类型作为值
comptime let T = i32
let x: T = 42
```

---

## 编译期反射

### 基础反射

```dast
// 类型信息
comptime fn size_of(T: type) -> usize {
    return @size_of(T)
}

comptime fn align_of(T: type) -> usize {
    return @align_of(T)
}

// 使用
const I32_SIZE: usize = size_of(i32)      // 4
const POINT_SIZE: usize = size_of(Point)  // 8
```

### 类型检查

```dast
comptime fn is_integer(T: type) -> bool {
    return @is_integer(T)
}

comptime fn is_float(T: type) -> bool {
    return @is_float(T)
}

// 编译期条件
fn process[T](value: T) {
    comptime if is_integer(T) {
        return value * 2
    } else {
        return value
    }
}
```

### 结构体反射

```dast
struct Point {
    x: f32,
    y: f32,
}

comptime fn field_count(T: type) -> usize {
    return @field_count(T)
}

comptime fn field_name(T: type, index: usize) -> &str {
    return @field_name(T, index)
}

// 使用
const FIELDS: usize = field_count(Point)  // 2
const NAME: &str = field_name(Point, 0)   // "x"
```

---

## 编译期代码生成

### 展开循环

```dast
comptime fn unroll_add(arr: &[i32; 4]) -> i32 {
    let mut sum = 0
    comptime for i in 0..4 {
        sum += arr[i]  // 编译期展开为 4 个加法
    }
    return sum
}

// 生成代码等价于:
// sum = arr[0] + arr[1] + arr[2] + arr[3]
```

### 生成函数

```dast
comptime fn make_getter(T: type, field_name: &str) -> fn(&T) -> FieldType {
    // 编译期生成 getter 函数
    return |obj: &T| @field(obj, field_name)
}
```

---

## 与 Rust/Zig 对比

| 特性 | Rust const fn | Zig comptime | Dast comptime |
|------|--------------|--------------|---------------|
| 控制流 | ✅ | ✅ | ✅ |
| 循环 | ✅ | ✅ | ✅ |
| 递归 | ✅ | ✅ | ✅ |
| 类型作为值 | ❌ | ✅ | ✅ |
| 反射 | ❌ | ✅ | ✅ |
| 代码生成 | ❌ | ✅ | ✅ |

---

## 内置 Comptime 函数

### 类型操作

```dast
@size_of(T: type) -> usize
@align_of(T: type) -> usize
@type_name(T: type) -> &str
```

### 类型检查

```dast
@is_integer(T: type) -> bool
@is_float(T: type) -> bool
@is_pointer(T: type) -> bool
@is_struct(T: type) -> bool
@is_enum(T: type) -> bool
```

### 结构体反射

```dast
@field_count(T: type) -> usize
@field_name(T: type, index: usize) -> &str
@field_type(T: type, index: usize) -> type
@field(obj: &T, name: &str) -> FieldType
```

---

## 实际应用场景

### 1. 编译期查表

```dast
comptime fn crc_table() -> [u32; 256] {
    let mut table: [u32; 256] = undefined
    for i in 0..256 {
        let mut crc = i as u32
        for _ in 0..8 {
            if crc & 1 != 0 {
                crc = (crc >> 1) ^ 0xEDB88320
            } else {
                crc = crc >> 1
            }
        }
        table[i] = crc
    }
    return table
}

const CRC_TABLE: [u32; 256] = crc_table()
```

### 2. 泛型序列化

```dast
fn serialize[T](obj: &T) -> Vec[u8] {
    let mut bytes = Vec.new()

    comptime for i in 0..@field_count(T) {
        let field_name = @field_name(T, i)
        let field_value = @field(obj, field_name)
        bytes.extend(serialize_field(field_value))
    }

    return bytes
}
```

### 3. 单元测试生成

```dast
comptime fn generate_tests() {
    comptime for test_case in TEST_CASES {
        @test(test_case.name, || {
            assert_eq(test_case.input, test_case.expected)
        })
    }
}
```

---

## 待确认

1. **反射范围** - 支持到什么程度？
2. **代码生成** - 是否支持动态生成函数？
3. **性能** - 编译期执行的时间限制？
4. **错误处理** - comptime 函数如何报错？

---

## 建议

| 特性 | 决策 |
|------|------|
| 基础 comptime | ✅ 完整支持 |
| 类型作为值 | ✅ 支持 |
| 基础反射 | ✅ 支持 (@size_of, @field_count 等) |
| 高级反射 | ⚠️ 可选 (动态生成函数) |
| 确定性 | ✅ 强制 (禁止 I/O) |
