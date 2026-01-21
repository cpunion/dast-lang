# IR v0 简化提案

## 概要

将 IR v0 从 22 个指令简化到 16 个，减少冗余。

## 简化 1：Load/Store 统一

**现在 (4 个)**：
```
LoadVar dst, name      // 读变量
StoreVar name, src     // 写变量
LoadRef dst, src       // 解引用读
StoreRef ref, src      // 解引用写
```

**简化后 (2 个)**：
```
Load dst, ptr          // 从地址读
Store ptr, val         // 写入地址
```

**变量处理**：
- `var x` 声明栈变量，`x` 本身就是地址
- `Load dst, x` 读取变量
- `Store x, val` 写入变量
- `Load dst, t0` 解引用（t0 是指针值）

## 简化 2：Enum 展开为 Struct

**现在 (3 个)**：
```
MakeEnum dst, name, variant, payload, tag, tag_type
EnumTag dst, src
EnumPayload dst, src
```

**简化后 (0 个)**：

前端将 enum 展开为 struct：
```dast
enum Option { Some(i32), None }
```
展开为：
```
type Option = { _tag: u8, _payload: i32 }
```

使用现有 struct 指令：
```
MakeStruct dst, Option, { _tag: 0, _payload: v }
GetField dst, src, _tag
GetField dst, src, _payload
```

## 简化 3：移除 AddrOf

**现在 (1 个)**：
```
AddrOf dst, name       // 取变量地址
```

**简化后 (0 个)**：

变量名直接作为地址使用：
```
// 现在
t0 = addr_of x
call foo(t0)

// 简化后
call foo(x)            // x 就是地址
```

## 简化前后对比

| 类别 | 现在 | 简化后 | 减少 |
|------|------|--------|------|
| 常量/变量 | Const, LoadVar, StoreVar, AddrOf | Const, Load, Store | -2 |
| 引用 | LoadRef, StoreRef | (合并到 Load/Store) | -2 |
| 运算 | BinOp, UnaryOp | BinOp, UnaryOp | 0 |
| 调用 | Call | Call | 0 |
| 数组 | MakeArray, Index, SetIndex, IndexUnchecked, SetIndexUnchecked | MakeArray, Index, SetIndex | -2 |
| 结构体 | MakeStruct, GetField, SetField | MakeStruct, GetField, SetField | 0 |
| 枚举 | MakeEnum, EnumTag, EnumPayload | (展开为 struct) | -3 |
| 控制流 | Jump, Branch, Return | Jump, Branch, Return | 0 |

**总计**：22 → 13 指令 (减少 9 个)

## 新 IR v0 指令集

```
// 常量
Const dst, value

// 内存
Load dst, ptr
Store ptr, val

// 运算
BinOp dst, op, lhs, rhs
UnaryOp dst, op, src

// 调用
Call dst, callee, args

// 数组
MakeArray dst, elems
Index dst, array, index
SetIndex array, index, src

// 结构体
MakeStruct dst, name, fields
GetField dst, src, field
SetField src, field, val

// 控制流
Jump target
Branch cond, then, else
Return [value]
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

**现在**：
```
fn inc(x)
  block entry:
    t0 = load_ref x
    t1 = + t0, 1
    store_ref x, t1
    return

fn main()
  block entry:
    store n, 10
    t0 = addr_of n
    call inc(t0)
    return
```

**简化后**：
```
fn inc(x: *i32)
  var x: *i32
  block entry:
    t0 = load x        // 读参数（指针值）
    t1 = load t0       // 解引用
    t2 = + t1, 1
    store t0, t2       // 写入引用位置
    return

fn main()
  var n: i32
  block entry:
    store n, 10        // n = 10
    call inc(n)        // n 就是地址
    return
```

### Enum

```dast
enum Option { Some(i32), None }
let x = Option.Some(42)
match x { .Some(v) => v, .None => 0 }
```

**现在**：
```
t0 = enum Option.Some@0:u8(42)
t1 = enum_tag t0
branch t1 == 0, some_arm, check_none
...
t2 = enum_payload t0
```

**简化后**：
```
type Option = { _tag: u8, _payload: i32 }

t0 = struct Option { _tag: 0, _payload: 42 }
t1 = get_field t0._tag
branch t1 == 0, some_arm, check_none
...
t2 = get_field t0._payload
```
