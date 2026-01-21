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

## IR 只有什么

```
类型：
  标量: i8 i16 i32 i64 u8 u16 u32 u64 f32 f64 bool
  指针: *T
  数组: [T; N]
  结构体: { field: T, ... }  (匿名，带布局)

内存：
  place: local(x) | deref(p) | field(p, offset) | index(p, i)
  
值：
  operand: use(place) | const(value)
```

## 指令集（8 类）

### 1. 赋值
```
place = operand
```

### 2. 二元运算
```
place = binop(op, a, b)
```

### 3. 一元运算
```
place = unop(op, a)
```

### 4. 指针运算
```
place = addr_of(place)    // 取地址
place = offset(ptr, n)    // 指针偏移
```

### 5. 调用
```
place = call(func, args...)
```

### 6. 内存
```
place = alloca(size, align)    // 栈分配
memcpy(dst, src, size)
memset(dst, val, size)
```

### 7. 原子
```
place = atomic(op, ptr, val, ordering)
```

### 8. Intrinsic
```
place = intrinsic(name, args...)
```

## 终结符（3 种）

```
return [operand]
goto(block)
switch(operand, [(val, block)...], default)
```

## 展开示例

### enum -> struct

```dast
enum Option[T] { Some(T), None }
let x = Option.Some(42)
match x {
    .Some(v) => v,
    .None => 0,
}
```

展开为：

```
// Option 展开为 { tag: u8, payload: T }
local(x) = const { tag: 0, payload: 42 }  // Some = tag 0

t0 = use(field(local(x), 0))  // 读 tag
switch t0, [(0, some_arm), (1, none_arm)], unreachable

block some_arm:
    t1 = use(field(local(x), 8))  // 读 payload (offset 8)
    return t1

block none_arm:
    return const 0
```

### closure -> struct + fn

```dast
let y = 10
let f = |x| x + y
f(5)
```

展开为：

```
// 闭包结构体
local(env) = const { y: 10 }

// 闭包函数 (env 作为第一个参数)
fn closure_0(env: *{ y: i32 }, x: i32) -> i32 {
    t0 = use(field(deref(env), 0))  // env.y
    t1 = binop(+, use(local(x)), t0)
    return t1
}

// 调用
t2 = call(closure_0, addr_of(local(env)), const 5)
```

### &mut T -> *T

```dast
fn inc(x: &mut i32) {
    *x = *x + 1
}
```

展开为：

```
fn inc(x: *i32) {
    t0 = use(deref(local(x)))
    t1 = binop(+, t0, const 1)
    deref(local(x)) = t1
    return
}
```

### 方法 -> 普通函数

```dast
impl Point {
    fn distance(&self) -> f64 { ... }
}
p.distance()
```

展开为：

```
t0 = call(Point.distance, addr_of(local(p)))
```

## 完整示例

### Dast

```dast
struct Point { x: i32, y: i32 }

fn manhattan(p: &Point) -> i32 {
    let ax = if p.x < 0 { -p.x } else { p.x }
    let ay = if p.y < 0 { -p.y } else { p.y }
    ax + ay
}
```

### IR

```
fn manhattan(p: *{ i32, i32 }) -> i32 {
    block entry:
        t0 = use(field(deref(local(p)), 0))    // p.x
        t1 = binop(<, t0, const 0)
        switch t1, [(1, neg_x)], pos_x
    
    block neg_x:
        t2 = unop(-, t0)
        goto have_ax(t2)
    
    block pos_x:
        goto have_ax(t0)
    
    block have_ax(ax: i32):
        t3 = use(field(deref(local(p)), 4))    // p.y (offset 4)
        t4 = binop(<, t3, const 0)
        switch t4, [(1, neg_y)], pos_y
    
    block neg_y:
        t5 = unop(-, t3)
        goto have_ay(t5)
    
    block pos_y:
        goto have_ay(t3)
    
    block have_ay(ay: i32):
        t6 = binop(+, use(local(ax)), use(local(ay)))
        return t6
}
```

## 总结

| 层 | 关心什么 | 不关心什么 |
|----|---------|----------|
| **前端** | 类型安全、借用检查、泛型、trait | 内存布局 |
| **IR** | 内存、指针、计算 | mut/const/enum/closure/trait |
| **后端** | 寄存器、指令选择 | 高级抽象 |

**IR 只有**：
- 8 类指令
- 3 种终结符
- 标量 + 指针 + 数组 + 匿名结构体
