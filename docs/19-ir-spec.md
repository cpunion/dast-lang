# Dast IR v0 规范（稳定核心）

> 目标：**稳定**且**最小**的 IR，使 stage0 只需支持 v0 即可运行 stage1，即便 stage1 未来大幅扩充语法。

## 设计原则

1. **语法糖前端展开**
   绝大多数语言特性在前端阶段降解为 v0 IR（泛型、trait、async、宏、解构等）。
2. **IR 指令集固定**
   v0 不随语法扩展频繁变化，必要新增指令走 v1+。
3. **版本显式**
   Program 带 `version` 与 `features`，stage0 只接收 `v0`。
4. **语义稳定**
   IR 语义优先“可解释器运行”，便于自举与调试。

## Program 结构

```
Program {
  version: "v0",
  features: [String],   // 可选
  functions: [Function],
  entry: String,
  meta?: {...}          // 可选，stage0 可忽略
}
```

### Function

```
Function {
  name: String,
  params: [String],
  blocks: [Block],
  temp_count: i32
}
```

### Block

```
Block {
  label: String,
  instrs: [Instr],
  term: Term
}
```

## IR v0 文本格式（ir_program_format）

> 该文本格式是 **stage0/stage1 的互操作桥梁**：
>
> - `dast ir` 输出此格式  
> - `dast ir-run` 读取并解释执行  
> - stage2 可直接生成该格式，交给 stage1/stage0 运行

### 顶层结构

```
ir v0
fn <name>(<param0>, <param1>, ...)
  block <label>:
    <instr>
    <term>

fn <name>(...)
  block <label>:
    ...
```

- 以 `ir v0` 开头  
- 每个函数以 `fn name(params)` 开始  
- 每个 block 以 `block label:` 开始  
- 指令/终结符是缩进行  
- 函数之间用空行分隔  

### IR 校验（ir-verify）

可用 `dast ir-verify <file.ir>` 对 IR v0 做静态校验，主要规则：

- `version` 必须为 `v0`，`features` 必须为空  
- `entry` 若存在，必须指向已定义函数  
- 函数名/块标签不能为空且唯一  
- 每个 block 必须有终结符（`jump/branch/return`）  
- `jump/branch` 目标必须存在  
- `tN` 必须满足 `0 <= N < temp_count`（`call` 的 `dst` 与 `enum` 的 `payload` 允许 `-1` 表示无值）  
- `binop/unary` 操作符必须属于 v0 定义集合  
- `struct` 字段名不能为空且不可重复

### IR 优化（ir-opt）

`ir-opt` 是一个**保守优化**工具，保证 v0 语义不变：

- 常量折叠：`unary/binop` 在常量输入时折叠为 `const`
- 分支折叠：`branch` 条件为常量 `bool` 时改写为 `jump`
- 删除不可达块：从函数首块出发的可达性分析

### 指令文本形态（与 v0 指令一一对应）

```
tN = const <value>
tN = load <name>
store <name>, tN
tN = addr_of <name>
tN = load_ref tM
store_ref tM, tN
tN = <op> tA, tB
tN = <op> tA              # 一元
tN = call <callee>(tA, tB)
call <callee>(tA, tB)      # 无返回值
tN = array [tA, tB]
tN = index tA[tB]
set_index tA[tB] = tC
tN = struct <Name> { field: tA, other: tB }
tN = get_field tA.field
set_field tA.field = tB
tN = enum <Name>.<Variant>(tA)
tN = enum <Name>.<Variant>
tN = enum_tag tA
tN = enum_payload tA
```

终结符：

```
jump <label>
branch tA, <then>, <else>
return
return tA
```

常量 `value`：
- `int`（十进制）
- `true` / `false`
- `"string"`（**不做转义**，保持原样）
- `unit`
- `&N` / `struct#N` / `enum#N` / `array#N`（仅用于调试输出）

> 注意：文本格式不转义字符串内容，若字符串包含换行/引号，会破坏行结构；  
> v0 规范建议 **避免在 IR 文本中出现换行或引号字符**。

## 值模型（Value）

v0 仅支持 8 种运行时值：

- `int`（有符号整数，v0 统一为 64-bit）
- `bool`
- `string`
- `unit`
- `ref`（指向 heap slot）
- `struct`
- `enum`
- `array`

> 注意：Dast 的 `i32/u32/...` 等基础类型在 v0 统一降为 `int`。
> 若未来需要保留位宽/溢出语义，则需前端显式插入检查或进入 v1+。

## 指令集（v0）

### 常量/变量

- `Const dst, value`
- `LoadVar dst, name`
- `StoreVar name, src`

### 引用与解引用

- `AddrOf dst, name`
  取局部变量地址（heap slot）。
- `LoadRef dst, src`
  `src` 必须为 `ref`。
- `StoreRef ref, src`
  `ref` 必须为 `ref`。

### 一元/二元运算

- `UnaryOp dst, op, src`
- `BinOp dst, op, lhs, rhs`

`op` 取值：`+ - * / % == != < <= > >= && || !`

### 调用

- `Call dst, callee, args`

> 约定：方法调用降为 `Type.method`，并把 `self` 作为第一个参数。

### 数组

- `MakeArray dst, elems`
- `Index dst, array, index`
- `SetIndex array, index, src`

### 结构体

- `MakeStruct dst, name, fields`
- `GetField dst, src, field`
- `SetField src, field, value`

> 字段以**名称**索引，保证语义稳定。
> 布局与偏移属于可选 meta（见下文）。

### 枚举

- `MakeEnum dst, name, variant, payload`
- `EnumTag dst, src`   → `string`（variant 名称）
- `EnumPayload dst, src`

> v0 使用**字符串 tag**。
> `@repr(...)` 与判别值仅做前端校验，不改变 IR 语义。
> 若要支持数值 tag，需 v1 引入 `EnumTagInt` 或元信息强制约束。

### 控制流

- `Jump target`
- `Branch cond, then_label, else_label`
- `Return [value]`

> IR 是块图结构，不强制 SSA。

## Builtins（固定语义）

- `print(...)`
- `println(...)`
- `len(x)`
- `push(&mut array, value)`
- `pop(&mut array)`
- `read_file(path)`
- `read_dir(path)`
- `write_file(path, data)`
- `args()`
- `char_at(str, index)`
- `substr(str, start, len)`

## 语法特性降解指南（面向未来）

> 原则：**只要能展开，就不要扩展 IR**。

### impl / method / self
- 前端将 `impl` 方法降为 `Type.method(self, ...)` 的普通函数。
- 调用 `obj.method(a)` 降为 `Call(Type.method, [obj, a])`。

### 泛型
- 仅在前端做单态化（monomorphization）。
- IR 中只出现具体类型实例。

### trait / interface
- 优先降为单态化或显式 vtable 结构体 + `Call`。
- 若需要动态分发，前端生成 `struct { vtable, data }` 形式。

### async / await
- 降为状态机结构体 + 显式驱动函数。
- IR 只看到普通 `struct/enum` + `Call` + `Match`（由前端展开）。

### 宏 / comptime
- 仅在前端展开，IR 不出现宏语义。

### 模式匹配
- 降为 `EnumTag` + `EnumPayload` + `Branch`。

### 构造/析构与 Drop
- `Drop` 在前端展开为显式调用。

### 模块/包/导入
- 前端完成名字解析与重命名，IR 中只保留**全限定函数名**。

### 常量与全局
- v0 推荐**常量内联**。
- 若出现全局常量/静态数据，可放入 `Program.meta`，stage0 可忽略。

## 元信息（可选，v0 可忽略）

```
meta {
  types?: ...
  layouts?: ...
  debug?: ...
  source_map?: ...
}
```

- `types/layouts` 供优化器或后端使用
- `debug/source_map` 供调试与错误提示使用

## 兼容性策略

- stage0 只支持 `version = v0`。
- stage1 若启用新特性，必须**先降为 v0**；无法降解的特性应报告 “需要 v1 IR”。
