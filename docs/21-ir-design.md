# Dast IR 设计（Rust 级能力）

> 目标：设计一个**底层、通用、跨平台**的 IR，具备 Rust MIR/LLVM IR 级别的表达能力。

## 设计原则

1. **底层表示**：接近机器模型，显式内存布局
2. **类型完备**：完整类型系统，支持泛型单态化
3. **所有权语义**：显式 move/borrow/drop
4. **跨平台**：平台无关 + 平台特定 intrinsics
5. **可优化**：SSA 形式，支持标准优化 passes

## 与现有 IR v0 的关系

```
┌─────────────────────────────────────────────────────────┐
│                    IR 层次架构                          │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Dast AST                                               │
│      ↓                                                  │
│  IR (本文档) ─── 高级 IR，保留类型/所有权信息           │
│      ↓                                                  │
│  IR v0 ───────── 低级 IR，解释器/C codegen 目标         │
│      ↓                                                  │
│  Backend ─────── C / LLVM / Cranelift / 解释器          │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

- **IR**（本文档）：编译器内部表示，保留完整语义信息
- **IR v0**：稳定的低级 IR，用于 bootstrap 和简单后端

## 类型系统

### 基础类型

```
// 整数类型（显式大小）
i8, i16, i32, i64, i128
u8, u16, u32, u64, u128
isize, usize

// 浮点类型
f32, f64

// 其他
bool
char        // Unicode scalar value (u32)
unit        // zero-sized type
never       // ! type, 无返回
```

### 复合类型

```
// 指针类型
*const T    // 不可变裸指针
*mut T      // 可变裸指针
&T          // 不可变引用
&mut T      // 可变引用

// 数组和切片
[T; N]      // 固定长度数组
[T]         // 切片（unsized）

// 结构体（带布局信息）
struct Point {
    x: i32 @offset(0),
    y: i32 @offset(4),
} @size(8) @align(4)

// 枚举（带判别式）
enum Option[T] @repr(u8) {
    None @tag(0),
    Some(T) @tag(1),
}

// 函数指针
fn(i32, i32) -> i32

// 闭包类型
closure[env: Env, fn(i32) -> i32]
```

### 泛型单态化

```
// 源码
fn identity[T](x: T) -> T { x }

// IR 中单态化
fn identity$i32(x: i32) -> i32 { x }
fn identity$String(x: String) -> String { x }
```

## 内存模型

### 布局信息

```
type_layout {
    size: usize,
    align: usize,
    fields: [(name, offset, type)],  // 结构体
    discriminant: (offset, type),     // 枚举
}
```

### Place 表达式（左值）

```
place ::=
    | local(name)              // 局部变量
    | deref(place)             // *place
    | field(place, field)      // place.field
    | index(place, operand)    // place[index]
    | downcast(place, variant) // (place as Variant)
```

### Operand（右值）

```
operand ::=
    | copy(place)    // 按位复制（Copy 类型）
    | move(place)    // 移动（非 Copy 类型）
    | const(value)   // 常量
```

## 所有权与生命周期

### Move/Copy 语义

```
// Move: 转移所有权
t1 = move local(x)     // x 之后不可用

// Copy: 按位复制
t1 = copy local(x)     // x 仍可用

// 编译器根据类型自动选择
// Copy 类型: i32, bool, (T, U) where T: Copy, U: Copy
// Move 类型: String, Vec[T], Box[T]
```

### 借用操作

```
// 不可变借用
t1 = ref local(x)      // &x

// 可变借用
t1 = ref_mut local(x)  // &mut x

// 解引用
t1 = copy deref(t0)    // *t0 (Copy)
t1 = move deref(t0)    // *t0 (Move)
```

### Drop 调用

```
// 显式 drop 点
drop local(x)

// 条件 drop（部分移动）
drop_if_alive local(x)

// scope 结束自动插入
block exit:
    drop local(z)
    drop local(y)
    drop local(x)
    return
```

## 指令集

### 赋值

```
// 基本赋值
place = operand

// 示例
local(x) = const 42
local(y) = copy local(x)
local(z) = move local(s)
deref(local(p)) = const 10
field(local(point), x) = const 0
```

### 算术/逻辑运算

```
// 二元运算（checked/unchecked）
t0 = add(t1, t2)              // 可能 panic（溢出）
t0 = add_unchecked(t1, t2)    // 未定义行为（溢出）
t0 = add_wrapping(t1, t2)     // 环绕
t0 = add_saturating(t1, t2)   // 饱和

// 运算符
add, sub, mul, div, rem       // 算术
eq, ne, lt, le, gt, ge        // 比较
and, or, xor                  // 位运算
shl, shr                      // 移位
land, lor, lnot               // 逻辑

// 一元运算
t0 = neg(t1)                  // -x
t0 = not(t1)                  // !x (按位/逻辑)
```

### 类型转换

```
// 数值转换
t0 = cast(t1, i32 -> i64)         // 有符号扩展
t0 = cast(t1, u32 -> u64)         // 零扩展
t0 = cast(t1, i64 -> i32)         // 截断
t0 = cast(t1, f64 -> i32)         // 浮点到整数

// 指针转换
t0 = ptr_to_int(t1)               // *T -> usize
t0 = int_to_ptr(t1, *T)           // usize -> *T
t0 = bitcast(t1, *const T, *const U)  // 重解释

// 引用转换
t0 = reborrow(t1)                 // &mut T -> &T
t0 = addr_of(place)               // &place
t0 = addr_of_mut(place)           // &mut place
```

### 聚合类型操作

```
// 结构体
t0 = aggregate Point { x: t1, y: t2 }
t0 = get_field(t1, x)
set_field(place, x, t0)

// 枚举
t0 = make_enum Option::Some(t1)
t0 = enum_discriminant(t1)        // 获取 tag
t0 = enum_payload(t1, Some)       // 获取 payload

// 数组
t0 = aggregate_array [t1, t2, t3]
t0 = index(t1, t2)                // 带边界检查
t0 = index_unchecked(t1, t2)      // 无边界检查
set_index(place, t1, t2)
```

### 内存操作

```
// 分配/释放
t0 = alloca(T)                    // 栈分配
t0 = heap_alloc(T)                // 堆分配
heap_free(t0)                     // 堆释放

// 内存操作
memcpy(dst, src, size)
memmove(dst, src, size)
memset(dst, value, size)

// 原子操作
t0 = atomic_load(place, ordering)
atomic_store(place, t0, ordering)
t0 = atomic_rmw(op, place, t1, ordering)  // read-modify-write
t0 = atomic_cmpxchg(place, expected, desired, success_ord, fail_ord)

// ordering: relaxed, acquire, release, acq_rel, seq_cst
```

### 函数调用

```
// 直接调用
t0 = call func_name(t1, t2, t3)

// 间接调用（函数指针）
t0 = call_indirect(t_fn_ptr, t1, t2)

// 方法调用（已脱糖）
t0 = call Type.method(t_self, t1, t2)

// Intrinsic 调用
t0 = intrinsic size_of[T]()
t0 = intrinsic transmute[T, U](t1)
```

## 控制流

### 基本块

```
fn example(x: i32) -> i32 {
    block entry:
        t0 = const 0
        t1 = lt(x, t0)
        branch t1, negative, non_negative
    
    block negative:
        t2 = neg(x)
        jump exit(t2)
    
    block non_negative:
        jump exit(x)
    
    block exit(result: i32):
        return result
}
```

### 终结符

```
term ::=
    | return [operand]                      // 返回
    | jump target(args...)                  // 无条件跳转
    | branch cond, then_target, else_target // 条件分支
    | switch operand, [case -> target], default_target  // 多路分支
    | call func(args...) -> target, unwind_target       // 可能 panic 的调用
    | unreachable                           // 不可达
    | abort                                 // 异常终止
```

### Switch（用于 match）

```
// match opt {
//     Some(x) => ...,
//     None => ...,
// }

block match_entry:
    t0 = enum_discriminant(opt)
    switch t0, [0 -> some_arm, 1 -> none_arm], unreachable_block

block some_arm:
    t1 = enum_payload(opt, Some)
    // ...

block none_arm:
    // ...
```

## Panic 与 Unwinding

```
// 可能 panic 的调用
block bb1:
    t0 = call may_panic() -> bb2, unwind_bb

block bb2:           // 正常路径
    ...

block unwind_bb:     // panic 路径
    drop local(x)    // 清理资源
    resume           // 继续 unwinding

// 或者 abort 模式
block bb1 @panic(abort):
    t0 = call may_panic() -> bb2
```

## 平台抽象

### Intrinsics

```
// 类型信息
size_of[T]() -> usize
align_of[T]() -> usize
needs_drop[T]() -> bool
type_id[T]() -> TypeId

// 内存
transmute[T, U](x: T) -> U
read_unaligned[T](ptr: *const T) -> T
write_unaligned[T](ptr: *mut T, val: T)

// 数学
sqrt_f32(f32) -> f32
sqrt_f64(f64) -> f64
sin_f64(f64) -> f64
// ...

// 位操作
ctpop(x: T) -> u32      // count ones
ctlz(x: T) -> u32       // leading zeros
cttz(x: T) -> u32       // trailing zeros
bswap(x: T) -> T        // byte swap
rotl(x: T, n: u32) -> T // rotate left
rotr(x: T, n: u32) -> T // rotate right

// 原子
atomic_fence(ordering)
```

### 平台特定

```
// 条件编译已在 IR 之前展开
// IR 中是单一目标

// 目标信息
target {
    arch: "x86_64",
    os: "linux",
    pointer_width: 64,
    endian: "little",
    features: ["sse2", "avx"],
}

// ABI 标注
fn extern_func(a: i32, b: i32) -> i32 @abi("C")
fn win_func(a: i32) -> i32 @abi("stdcall")
```

## 完整示例

### Dast 源码

```dast
fn sum(vec: &Vec[i32]) -> i32 {
    let mut total = 0
    for x in vec {
        total = total + *x
    }
    total
}
```

### IR 表示

```
fn sum(vec: &Vec$i32) -> i32 {
    block entry:
        local(total) = const i32 0
        t0 = call Vec$i32.iter(vec) -> iter_ready, unwind_cleanup
    
    block iter_ready:
        local(iter) = move t0
        jump loop_header
    
    block loop_header:
        t1 = call Iterator$i32.next(&mut local(iter)) -> loop_check, unwind_cleanup
    
    block loop_check:
        t2 = enum_discriminant(t1)
        switch t2, [0 -> loop_body, 1 -> loop_exit], unreachable
    
    block loop_body:
        t3 = enum_payload(t1, Some)   // &i32
        t4 = copy deref(t3)           // i32
        t5 = copy local(total)
        t6 = add(t5, t4)
        local(total) = move t6
        drop t1
        jump loop_header
    
    block loop_exit:
        drop t1
        drop local(iter)
        t7 = copy local(total)
        return t7
    
    block unwind_cleanup:
        drop_if_alive local(iter)
        drop local(total)
        resume
}
```

## IR 到 IR v0 的 Lowering

```
IR (高级)                          IR v0 (低级)
─────────────────────────────────────────────────────
copy(place)                   →    load/get_field
move(place)                   →    load/get_field + 标记无效
drop(x)                       →    call Type.drop(x)
aggregate Point{x,y}          →    struct Point{x,y}
enum_discriminant             →    enum_tag
atomic_load                   →    call __atomic_load
call f() -> ok, unwind        →    call f() (简化，abort 模式)
```

## 与 Rust MIR 对比

| 特性 | Rust MIR | Dast IR |
|------|----------|----------|
| SSA | Partial SSA | Full SSA |
| Place 表达式 | ✅ | ✅ |
| Move/Copy | ✅ | ✅ |
| Drop 标记 | ✅ | ✅ |
| Unwinding | ✅ | ✅ |
| 泛型 | 单态化 | 单态化 |
| 生命周期 | Borrow checker 之后 | 类似 |
| Unsafe | 标记块 | 标记块 |

## 下一步

1. **阶段划分**
   - Stage0/1: 继续使用 IR v0（简单，足够 bootstrap）
   - Stage2+: 引入完整 IR，支持优化和多后端

2. **实现路径**
   - 先定义 IR 数据结构
   - 实现 AST → IR 编译
   - 实现 IR → IR v0 lowering
   - 实现 IR 优化 passes

3. **优先级**
   - P0: 类型系统、控制流
   - P1: 所有权/drop
   - P2: 原子操作、unwinding
   - P3: 优化 passes
