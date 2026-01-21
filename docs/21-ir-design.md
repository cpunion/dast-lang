# Dast IR 设计（纯底层）

> **核心理念**：IR 只关心内存和计算，所有高级概念在前端完全展开。

## 前端已处理（IR 不可见）

| 概念 | 前端展开方式 |
|------|-------------|
| `let x = 42` | 纯值 `%n` 或提升为 `var`（若需地址） |
| `let mut x` | 栈变量 `var` |
| `&x` | 直接传 `x`（var 就是地址） |
| `enum/closure/trait` | 展开为 struct + 函数 |

## IR 数据模型

### 变量

```
fn foo(a: i32, b: *i32) -> i32 {
    var x: i32         // 栈变量，本身就是地址
    ...
}
```

- `var x` 是一个栈槽地址
- `%n` 是 SSA 临时值（寄存器）

### Operand

```
Operand ::=
    | %n              // 临时值
    | x               // 变量（作为地址）
    | const(value)    // 常量
```

## 指令集（5 类）

### 1. 内存
```
%0 = load(ptr)              // 从地址读
store(ptr, val)             // 写入地址
%0 = offset(ptr, n)         // 指针偏移
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
%0 = alloca(size, align)    // 动态栈分配
memcpy(dst, src, size)
memset(dst, val, size)
```

### 5. 原子/Intrinsic
```
%0 = atomic(op, ptr, val, ordering)
%0 = intrinsic(name, args...)
```

## 终结符（3 种）

```
return [val]
goto(block)
switch(val, [(v1, b1), ...], default)
```

## 示例

### 取地址

```dast
fn inc(x: &mut i32) { *x = *x + 1 }
let mut n = 10
inc(&mut n)
```

展开为：

```
fn inc(x: *i32) {
    var x: *i32            // 参数也是变量
    block entry:
        %0 = load(x)       // 读 x 得到指针
        %1 = load(%0)      // *x
        %2 = binop(+, %1, const 1)
        store(%0, %2)      // *x = ...
        return
}

fn main() {
    var n: i32
    block entry:
        store(n, const 10)  // n = 10
        call(inc, n)        // 直接传 n（它就是地址）
        return
}
```

### 结构体字段

```dast
struct Point { x: i32, y: i32 }
let p = Point { x: 1, y: 2 }
let v = p.x
```

展开为：

```
fn main() {
    var p: { i32, i32 }
    var v: i32
    block entry:
        %0 = offset(p, 0)      // &p.x
        store(%0, const 1)
        %1 = offset(p, 4)      // &p.y
        store(%1, const 2)
        %2 = load(%0)
        store(v, %2)
        return
}
```

## 总结

```
┌──────────────────────────────────────┐
│  IR 核心                             │
├──────────────────────────────────────┤
│  5 类指令 + 3 种终结符               │
├──────────────────────────────────────┤
│  var x → 栈槽地址，直接作为指针使用   │
│  %n → SSA 临时值                     │
├──────────────────────────────────────┤
│  无 addr 指令：var 本身就是地址       │
└──────────────────────────────────────┘
```
