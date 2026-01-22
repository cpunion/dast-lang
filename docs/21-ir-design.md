# IR v0 简化记录

## 已完成的简化

### 1. 合并 IndexUnchecked/SetIndexUnchecked

**修改前**：
```
Index dst, array, index
SetIndex array, index, src
IndexUnchecked dst, array, index
SetIndexUnchecked array, index, src
```

**修改后**：
```
Index dst, array, index [, @unchecked]
SetIndex array, index, src [, @unchecked]
```

- Index/SetIndex 结构体添加 `Unchecked bool` 字段
- 指令数: 22 → 20 (减少 2 个)

### 2. Enum 展开为 Struct

**修改前** (3 个指令)：
```
t0 = enum Option.None@1:i32
t1 = enum Option.Some@0:i32(t0)
t2 = enum_tag t1
t3 = enum_payload t1
```

**修改后** (使用现有 struct 指令)：
```
t0 = i32 1           // tag 值
t1 = unit            // payload 值
t2 = struct Option { _tag: t0, _payload: t1 }
t3 = t2._tag         // GetField
t4 = t2._payload     // GetField
```

- 移除 MakeEnum, EnumTag, EnumPayload 三个指令
- Enum 在 IR 层面表示为 Struct，字段为 `_tag` 和 `_payload`
- 指令数: 20 → 17 (减少 3 个)

## 当前 IR v0 指令集 (17 个)

```
// 常量/变量 (4)
Const dst, value
LoadVar dst, name
StoreVar name, src
AddrOf dst, name

// 引用 (2)
LoadRef dst, src
StoreRef ref, src

// 运算 (2)
BinOp dst, op, lhs, rhs
UnaryOp dst, op, src

// 调用 (1)
Call dst, callee, args

// 数组 (3) - 已简化
MakeArray dst, elems
Index dst, array, index [, @unchecked]
SetIndex array, index, src [, @unchecked]

// 结构体 (3) - 也用于 enum
MakeStruct dst, name, fields
GetField dst, src, field
SetField src, field, val

// 控制流 (3)
Jump target
Branch cond, then, else
Return [value]
```

## 待简化（可选）

### 3. 移除 AddrOf

```
// 现在
t0 = addr_of x
call foo(t0)

// 可简化为
call foo(x)   // x 直接作为地址
```

影响：减少 1 个指令，但需更改调用约定

## 下一步

- M5 后端测试完善
- 可选：移除 AddrOf 指令
