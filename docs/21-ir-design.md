# IR 设计讨论

## 结论：IR v0 已足够

经过讨论，新 IR 设计与 IR v0 几乎一样：

| 特性 | IR v0 | 新 IR 设计 |
|------|-------|----------|
| 类型名 | ✓ 保留 | ✓ 保留 |
| 字段名 | ✓ `p.x` | ✓ `field(p, x)` |
| 偏移 | 后端计算 | 后端计算 |
| enum | 特殊指令 | 可展开为 struct |
| closure | 特殊指令 | 可展开为 struct |

**主要区别**：
- enum/closure 的处理方式
- IR v0 用特殊指令，新设计展开为 struct

## 决策：继续使用 IR v0

IR v0 已经足够稳定且实用：

1. **保留类型和字段名**：跨平台，可调试
2. **后端计算布局**：不在 IR 中硬编码偏移
3. **解释器友好**：便于 bootstrap
4. **C codegen 友好**：直接映射

## 可能的小优化

如果未来需要简化，可以：

### 1. enum 展开为 struct

```
// 现在
MakeEnum dst, Option, Some, payload, 0, u8
EnumTag dst, src
EnumPayload dst, src

// 可以简化为
type Option = { tag: u8, payload: i32 }
MakeStruct dst, Option, { tag: 0, payload: v }
GetField dst, src, tag
GetField dst, src, payload
```

影响：减少 3 个指令类型

### 2. closure 展开为 struct + fn

已经是这样做的（前端展开）。

### 3. 统一 Load/Store

```
// 现在
LoadVar dst, name
StoreVar name, src
LoadRef dst, src
StoreRef ref, src

// 可以统一为
Load dst, ptr
Store ptr, val
```

影响：减少 4 个指令类型，但需要显式区分变量和引用。

## 当前状态

**继续使用 IR v0**，原因：

- 已经稳定，Stage0/1/2 都在使用
- 满足 bootstrap 需求
- enum/closure 特殊指令不影响后端生成
- 优化可以渐进式进行

下一步待解决问题：M5 后端测试
