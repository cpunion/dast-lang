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

## 待简化（可选）

### 2. Enum 展开为 Struct

```
// 现在 (3 个指令)
MakeEnum dst, name, variant, payload, tag, tag_type
EnumTag dst, src
EnumPayload dst, src

// 可简化为 (使用现有 struct 指令)
MakeStruct dst, Option, { _tag: 0, _payload: v }
GetField dst, src, _tag
GetField dst, src, _payload
```

影响：减少 3 个指令，但需修改前端展开 enum

### 3. 移除 AddrOf

```
// 现在
t0 = addr_of x
call foo(t0)

// 可简化为
call foo(x)   // x 直接作为地址
```

影响：减少 1 个指令，但需更改调用约定

## 当前 IR v0 指令集 (20 个)

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

// 结构体 (3)
MakeStruct dst, name, fields
GetField dst, src, field
SetField src, field, val

// 枚举 (3) - 可简化
MakeEnum dst, name, variant, payload, tag, tag_type
EnumTag dst, src
EnumPayload dst, src

// 控制流 (3)
Jump target
Branch cond, then, else
Return [value]
```

## 下一步

- M5 后端测试完善
- 可选：Enum 展开为 Struct
