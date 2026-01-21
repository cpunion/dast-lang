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
    | { T1, T2, ... }             // 匿名结构体
```

### 变量定义

每个函数开头声明所有局部变量：

```
fn foo(a: i32, b: *i32) -> i32 {
    var x: i32           // 栈槽，有地址
    var tmp: i32         // 栈槽
    ...
}
```

- `var` 声明栈上的内存槽位，有地址
- 参数也是变量，有地址
- 临时值用 `%0, %1, ...` 表示，无地址

### Operand（值）

```
Operand ::=
    | %n                 // 临时值（无地址，纯值）
    | const(value)       // 常量
```

### 内存访问

```
// 读内存 -> 临时值
%0 = load(ptr)                    // 从指针读

// 写内存
store(ptr, value)                 // 写入指针位置

// 取地址 -> 指针值
%0 = addr(var)                    // 取变量地址
%0 = offset(ptr, n)               // 指针偏移
```

## 指令集（6 类）

### 1. 内存
```
%0 = load(ptr)                    // 读
store(ptr, val)                   // 写
%0 = addr(var)                    // 取地址
%0 = offset(ptr, n)               // 指针偏移
```

### 2. 算术/逻辑
```
%0 = binop(op, a, b)
%0 = unop(op, a)
```

### 3. 调用
```
%0 = call(func, args...)
```

### 4. 分配
```
%0 = alloca(size, align)          // 动态栈分配
memcpy(dst, src, size)
memset(dst, val, size)
```

### 5. 原子
```
%0 = atomic(op, ptr, val, ordering)
```

### 6. Intrinsic
```
%0 = intrinsic(name, args...)
```

## 终结符（3 种）

```
return [val]
goto(block)
switch(val, [(v1, b1), ...], default)
```

## 示例

### 值变量 vs 引用

```dast
let x = 42          // 值变量
let r = &x          // 引用（指针）
let v = *r          // 解引用
```

展开为：

```
fn main() {
    var x: i32
    var r: *i32
    var v: i32
    
    block entry:
        store(addr(x), const 42)     // x = 42
        store(addr(r), addr(x))      // r = &x (指针值)
        %0 = load(addr(r))           // 读 r 得到指针
        %1 = load(%0)                // 解引用
        store(addr(v), %1)           // v = *r
        return
}
```

### 可变引用参数

```dast
fn inc(x: &mut i32) {
    *x = *x + 1
}

let mut n = 10
inc(&mut n)
```

展开为：

```
fn inc(x: *i32) {               // &mut i32 -> *i32
    var x: *i32                  // 参数也是变量
    
    block entry:
        %0 = load(addr(x))       // 读取指针参数
        %1 = load(%0)            // *x
        %2 = binop(+, %1, const 1)
        store(%0, %2)            // *x = %2
        return
}

fn main() {
    var n: i32
    
    block entry:
        store(addr(n), const 10)
        call(inc, addr(n))       // 传递 n 的地址
        return
}
```

### 结构体

```dast
struct Point { x: i32, y: i32 }
let p = Point { x: 1, y: 2 }
let v = p.x
```

展开为：

```
// Point = { i32, i32 } @size(8) @align(4)
fn main() {
    var p: { i32, i32 }
    var v: i32
    
    block entry:
        %0 = offset(addr(p), 0)      // &p.x
        store(%0, const 1)           // p.x = 1
        %1 = offset(addr(p), 4)      // &p.y
        store(%1, const 2)           // p.y = 2
        %2 = load(%0)                // 读 p.x
        store(addr(v), %2)           // v = p.x
        return
}
```

### enum

```dast
enum Option { Some(i32), None }
let x = Option.Some(42)
match x { .Some(v) => v, .None => 0 }
```

展开为：

```
// Option = { u8, i32 } @size(8) tag@0 payload@4
fn main() {
    var x: { u8, i32 }
    
    block entry:
        %0 = offset(addr(x), 0)      // &x.tag
        store(%0, const u8 0)        // Some = 0
        %1 = offset(addr(x), 4)      // &x.payload
        store(%1, const 42)
        %2 = load(%0)                // 读 tag
        switch %2, [(0, some_arm), (1, none_arm)], unreachable
    
    block some_arm:
        %3 = load(%1)                // 读 payload
        return %3
    
    block none_arm:
        return const 0
}
```

## 总结

```
┌───────────────────────────────────────────┐
│  IR 核心                                  │
├───────────────────────────────────────────┤
│  变量: var x: T (栈槽，有地址)             │
│  临时值: %n (无地址，纯值)                  │
├───────────────────────────────────────────┤
│  内存: load / store / addr / offset       │
│  计算: binop / unop / call                │
│  分配: alloca / memcpy / memset            │
│  原子: atomic                              │
│  扩展: intrinsic                           │
├───────────────────────────────────────────┤
│  终结符: return / goto / switch            │
└───────────────────────────────────────────┘
```

**关键区分**：
- `var x` → 栈槽，有地址，用 `addr(x)` 取地址
- `%n` → 临时值，无地址，不能取地址
- `load/store` → 通过指针访问内存
