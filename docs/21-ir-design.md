# Dast IR 设计（纯底层）

> **核心理念**：IR 只关心内存和计算，所有高级概念在前端完全展开。

## 前端已处理（IR 不可见）

| 概念 | 前端展开方式 |
|------|-------------|
| `let x = 42` | 纯值，直接内联或分配寄存器 |
| `let x = 42; &x` | **提升为栈变量**（需要地址时） |
| `let mut x` | 栈变量 |
| `enum` | 展开为 struct { tag, payload } |
| `closure` | 展开为 struct + 函数 |
| `&T`/`&mut T` | 展开为指针 `*T` |

## IR 数据模型

### 变量

```
fn foo(a: i32, b: *i32) -> i32 {
    var x: i32         // 栈变量（有地址）
    ...
}
```

- 函数参数和 `var` 声明都是栈变量
- 只有栈变量可以取地址
- 临时值 `%n` 是 SSA 值，无地址

### Operand

```
Operand ::=
    | %n              // 临时值（寄存器）
    | const(value)    // 常量
```

## 指令集（6 类）

### 1. 内存
```
%0 = load(ptr)              // 从指针读
store(ptr, val)             // 写入指针
%0 = addr(var)              // 取栈变量地址（唯一能取地址的方式）
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

## 前端提升示例

### 值变量不需要地址

```dast
let x = 42
let y = x + 1
```

展开为（无栈变量）：

```
fn main() {
    block entry:
        %0 = const 42
        %1 = binop(+, %0, const 1)
        return
}
```

### 值变量需要地址时提升

```dast
let x = 42
let r = &x      // 需要地址，x 提升为栈变量
```

展开为：

```
fn main() {
    var x: i32           // 前端发现需要地址，提升为 var
    var r: *i32
    
    block entry:
        store(addr(x), const 42)
        store(addr(r), addr(x))
        return
}
```

### mut 变量始终是栈变量

```dast
let mut x = 42
x = x + 1
```

展开为：

```
fn main() {
    var x: i32           // mut 总是栈变量
    
    block entry:
        store(addr(x), const 42)
        %0 = load(addr(x))
        %1 = binop(+, %0, const 1)
        store(addr(x), %1)
        return
}
```

## 总结

```
┌─────────────────────────────────────────────────┐
│  取地址规则                                      │
├─────────────────────────────────────────────────┤
│  addr(var) ✓   只能对栈变量取地址               │
│  addr(%n)  ✗   临时值没有地址                   │
├─────────────────────────────────────────────────┤
│  前端职责                                        │
│  - let x = v; &x → 将 x 提升为 var              │
│  - let mut x → 总是 var                         │
│  - let x = v (无 &x) → 纯值 %n                  │
└─────────────────────────────────────────────────┘
```

**指令总计**：
- 6 类指令
- 3 种终结符
- 无冗余：`addr` 只能用于 `var`，不能用于临时值
