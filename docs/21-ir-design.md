# Dast IR 设计（纯底层）

> **核心理念**：IR 只关心内存和计算，所有高级概念在前端完全展开。

## 前端已处理（IR 不可见）

| 概念 | 前端展开方式 |
|------|-------------|
| `mut`/`const` | 只是类型检查，IR 中所有内存可读写 |
| `enum` | 展开为 struct { tag, payload } |
| `closure` | 展开为 struct + 函数 |
| `&T`/`&mut T` | 展开为指针 `*T` |
| `trait` | 单态化或 vtable 指针 |
| `泛型` | 完全单态化 |
| `match` | 展开为 switch + 字段访问 |
| `方法调用` | 展开为 `Type.method(self, ...)` |

## IR 数据模型

### 类型

```
Type ::=
    | i8 | i16 | i32 | i64       // 有符号整数
    | u8 | u16 | u32 | u64       // 无符号整数
    | f32 | f64                   // 浮点
    | bool                        // 布尔
    | *T                          // 指针
    | [T; N]                      // 数组
    | { T1, T2, ... }             // 匿名结构体 (按偏移访问)
```

### Place（内存位置）

```
Place ::=
    | local(x)                    // 局部变量/参数
    | deref(p)                    // *p
    | field(p, offset)            // p + offset
    | index(p, i)                 // p + i * elem_size
```

### Operand（值）

```
Operand ::=
    | use(place)                  // 读取内存
    | const(value)                // 常量
    | place                       // 作为指针传递
```

> **简化**：`place` 直接作为 operand 表示取地址，无需 `addr_of`。

## 指令集（7 类）

### 1. 赋值
```
place = operand
```

### 2. 二元运算
```
place = binop(op, a, b)

op ∈ { +, -, *, /, %,
       ==, !=, <, <=, >, >=,
       &, |, ^, <<, >> }
```

### 3. 一元运算
```
place = unop(op, a)

op ∈ { -, ! }
```

### 4. 调用
```
place = call(func, args...)
```

### 5. 内存
```
place = alloca(size, align)     // 栈分配
memcpy(dst, src, size)
memset(dst, val, size)
```

### 6. 原子
```
place = atomic(op, ptr, val, ordering)
```

### 7. Intrinsic
```
place = intrinsic(name, args...)
```

## 终结符（3 种）

```
return [operand]
goto(block)
switch(operand, [(val, block)...], default)
```

## 示例

### 引用参数

```dast
fn inc(x: &mut i32) {
    *x = *x + 1
}

let mut n = 10
inc(&mut n)
```

展开为：

```
fn inc(x: *i32) {
    block entry:
        t0 = use(deref(local(x)))      // *x
        t1 = binop(+, t0, const 1)
        deref(local(x)) = t1           // *x = t1
        return
}

fn main() {
    block entry:
        local(n) = const 10
        call(inc, local(n))            // 直接传 place，即地址
        return
}
```

### enum

```dast
enum Option { Some(i32), None }
let x = Option.Some(42)
```

展开为：

```
// Option = { tag: u8, payload: i32 } @size(8) @align(4)
fn main() {
    block entry:
        // Some = tag 0
        field(local(x), 0) = const u8 0     // tag
        field(local(x), 4) = const i32 42   // payload
        return
}
```

### closure

```dast
let y = 10
let f = |x| x + y
f(5)
```

展开为：

```
// 生成的闭包函数
fn closure_0(env: *{ i32 }, x: i32) -> i32 {
    block entry:
        t0 = use(field(deref(local(env)), 0))  // env.y
        t1 = binop(+, use(local(x)), t0)
        return t1
}

fn main() {
    block entry:
        field(local(env), 0) = const 10       // 捕获 y
        t0 = call(closure_0, local(env), const 5)
        return
}
```

### 结构体方法

```dast
impl Point {
    fn length(&self) -> f64 { ... }
}
p.length()
```

展开为：

```
t0 = call(Point.length, local(p))   // 直接传 place
```

## 总结

```
┌────────────────────────────────────────────┐
│  IR 核心                                   │
├────────────────────────────────────────────┤
│  类型: 标量 + 指针 + 数组 + 结构体         │
│  指令: 7 类                                 │
│  终结符: 3 种                               │
├────────────────────────────────────────────┤
│  不存在: mut/const/enum/closure/ref/trait   │
│  不存在: addr_of (直接用 place)              │
└────────────────────────────────────────────┘
```

| 层 | 职责 |
|----|------|
| **前端** | 类型检查、借用检查、泛型/trait/enum/closure 展开 |
| **IR** | 内存、指针、计算、控制流 |
| **后端** | 寄存器分配、指令选择、目标代码生成 |
