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
- **新增 Enum 元数据**：IR header 记录每个 enum 的 tag 类型与各变体 payload 类型，后端可生成强类型 `tag + union`，但指令仍保持 `GetField/SetField`
- 指令数: 20 → 17 (减少 3 个)

**IR Header 示例**：
```
ir v0
enum Option[T] tag i32
  variant None = 1 : unit
  variant Some = 0 : T
```

### 3. LoadVar 支持取址（移除 AddrOf）

**修改前**：
```
t0 = addr_of x
call foo(t0)
```

**修改后**：
```
t0 = load_addr x
call foo(t0)
```

- LoadVar 指令新增 `@addr`（文本语法 `load_addr`）用于生成引用值
- 独立的 AddrOf 指令删除
- 指令数: 17 → 16 (减少 1 个)

### 4. 合并 LoadRef/StoreRef

**修改前**：
```
t0 = load_ref t1
store_ref t1, t2
```

**修改后**：
```
t0 = load_ref t1        // LoadVar + @ref
store_ref t1, t2        // StoreVar + @ref
```

- LoadVar/StoreVar 新增 `@ref`（文本语法 `load_ref` / `store_ref`）
- 移除 LoadRef、StoreRef 两个指令
- 指令数: 16 → 14 (减少 2 个)

### 5. 移除 UnaryOp

**修改前**：
```
t0 = - t1
t0 = ! t1
```

**修改后**：
```
t0 = - 0, t1
t2 = == t3, false
```

- 一元运算在 IR 中降级为二元：`-x` → `0 - x`，`!x` → `x == false`
- 移除 UnaryOp 指令
- 指令数: 15 → 14 (减少 1 个)

### 6. 移除 Const

**修改前**：
```
t0 = const 42
store x, t0
```

**修改后**：
```
store x, 42
```

- 常量直接以内联 Operand 表示，不再需要 Const 指令
- 简化优化路径，减少额外临时寄存器
- 指令数: 14 → 13 (减少 1 个)

## 当前 IR v0 指令集 (13 个)

```
// 变量 (2)
LoadVar dst, name [, @addr] [, @ref]
StoreVar name, src [, @ref]

// 运算 (1)
BinOp dst, op, lhs, rhs

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

## 下一步

- M5 后端测试完善
- （空）新的 IR 简化需求待定
