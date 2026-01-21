# Dast IR 设计（最小化核心）

> 目标：**底层、通用、跨平台**的最小 IR，类似 Rust MIR 但更简洁。

## 设计原则

1. **正交性**：每个概念只有一种表达方式
2. **可组合**：通过组合而非特化指令达到表达力
3. **显式语义**：所有行为都是显式的

## 类型系统

```
Type ::=
    // 标量
    | i8 | i16 | i32 | i64 | i128
    | u8 | u16 | u32 | u64 | u128
    | f32 | f64
    | bool | char | unit | never
    
    // 复合
    | *T                    // 裸指针
    | &T | &mut T           // 引用
    | [T; N]                // 数组
    | struct { fields... }  // 结构体
    | enum { variants... }  // 枚举
    | fn(args) -> ret       // 函数
```

## 内存模型

### Place（内存位置）

```
Place ::=
    | local(x)              // 局部变量
    | deref(p)              // *p
    | field(p, f)           // p.f  
    | index(p, i)           // p[i]
    | downcast(p, v)        // p as Variant
```

### Operand（值）

```
Operand ::=
    | use(place)            // 使用（编译器决定 copy/move）
    | const(value)          // 常量
    | ref(place)            // &place
    | ref_mut(place)        // &mut place
```

> **关键简化**：`copy` 和 `move` 合并为 `use`，编译器根据类型自动判断。

## 指令集（核心 12 类）

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

op ∈ { -, !, ~ }
```

### 4. 类型转换
```
place = cast(operand, T)
```

### 5. 聚合构造
```
place = aggregate(T, fields...)
```

结构体、枚举、数组统一用 `aggregate`：
```
local(p) = aggregate(Point, x: t0, y: t1)
local(a) = aggregate([i32; 3], t0, t1, t2)
local(o) = aggregate(Option::Some, t0)
local(n) = aggregate(Option::None)
```

### 6. 判别式
```
place = discriminant(operand)
```

### 7. 调用
```
place = call(func, args...)
```

所有调用统一形式：
- 直接调用：`call(add, t0, t1)`
- 间接调用：`call(deref(fn_ptr), t0, t1)`
- 方法调用：`call(Point.distance, t_self, t0)`

### 8. Drop
```
drop(place)
```

### 9. 内存分配
```
place = alloc(T)         // 栈分配
place = alloc_heap(T)    // 堆分配
free(place)              // 释放
```

### 10. 原子操作
```
place = atomic(op, place, operand, ordering)

op ∈ { load, store, swap, add, sub, and, or, xor, cmpxchg }
ordering ∈ { relaxed, acquire, release, acqrel, seqcst }
```

### 11. Intrinsic
```
place = intrinsic(name, args...)
```

用于无法用普通指令表达的操作：
```
t0 = intrinsic(size_of, T)
t0 = intrinsic(transmute, t1)
t0 = intrinsic(sqrt_f64, t1)
t0 = intrinsic(memcpy, dst, src, len)
```

### 12. 无操作
```
nop
```

## 终结符（4 种）

```
Terminator ::=
    | return [operand]                    // 返回
    | goto(block)                         // 无条件跳转  
    | switch(operand, [(val, block)...], default)  // 分支
    | unreachable                         // 不可达
```

> **简化**：`branch` 是 `switch` 的特例（`switch(c, [(true, then)], else)`）

## 溢出/边界检查通过属性控制

不是特化指令，而是指令属性：

```
place = binop(+, a, b) @overflow(wrap)     // 环绕
place = binop(+, a, b) @overflow(saturate) // 饱和
place = binop(+, a, b) @overflow(trap)     // panic (默认)
place = binop(+, a, b) @overflow(undef)    // 未定义

place = use(index(arr, i)) @bounds(check)   // 检查 (默认)
place = use(index(arr, i)) @bounds(unsafe)  // 不检查
```

## 完整示例

### Dast 源码

```dast
fn sum(arr: &[i32]) -> i32 {
    let mut total = 0
    let mut i = 0
    while i < len(arr) {
        total = total + arr[i]
        i = i + 1
    }
    total
}
```

### IR

```
fn sum(arr: &[i32]) -> i32 {
    block entry:
        local(total) = const 0
        local(i) = const 0
        goto loop_cond
    
    block loop_cond:
        t0 = intrinsic(len, use(local(arr)))
        t1 = binop(<, use(local(i)), t0)
        switch t1, [(true, loop_body)], loop_exit
    
    block loop_body:
        t2 = use(index(deref(local(arr)), use(local(i))))
        t3 = binop(+, use(local(total)), t2)
        local(total) = t3
        t4 = binop(+, use(local(i)), const 1)
        local(i) = t4
        goto loop_cond
    
    block loop_exit:
        return use(local(total))
}
```

## 指令统计对比

| 类别 | 原设计 | 新设计 | 简化方式 |
|------|--------|--------|----------|
| 算术 | add/add_unchecked/add_wrapping/... | binop + @overflow | 属性代替特化 |
| 索引 | index/index_unchecked | use(index(...)) + @bounds | 属性代替特化 |
| 引用 | ref/ref_mut/addr_of/addr_of_mut | ref/ref_mut operand | 统一为 operand |
| 调用 | call/call_indirect | call | 统一形式 |
| 使用 | copy/move | use | 编译器推断 |
| 聚合 | make_struct/make_enum/make_array | aggregate | 统一构造 |
| 分支 | branch/switch | switch | switch 包含 branch |

**结果**：
- 指令类型：30+ → **12**
- 终结符：6 → **4**

## 与 IR v0 的关系

```
┌───────────────────────────────────────────┐
│  AST                                      │
│    ↓                                       │
│  IR (本文档) ─── 12 指令 + 4 终结符        │
│    ↓                                       │
│  IR v0 ──────── 展开为简化指令 (解释器)  │
│    ↓                                       │
│  Backend ────── C / LLVM / Cranelift     │
└───────────────────────────────────────────┘
```

**Lowering 示例**：
```
IR:     place = binop(+, a, b) @overflow(wrap)
IR v0:  t0 = + a, b  // 解释器自然环绕

IR:     place = aggregate(Point, x: t0, y: t1)
IR v0:  t2 = struct Point { x: t0, y: t1 }

IR:     drop(local(x))
IR v0:  call Type.drop(x)  // 展开为调用
```
