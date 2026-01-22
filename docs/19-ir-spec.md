# Dast IR v0 规范（稳定核心）

> 目标：**稳定**且**最小**的 IR，使 stage0 只需支持 v0 即可运行 stage1/2，即便上层语法持续演进。

## 设计原则

1. **语法糖前端展开**
   绝大多数语言特性在前端阶段降解为 v0 IR（泛型、trait、async、宏、解构等）。
2. **指令集稳定**
   v0 指令集保持稳定，必要信息通过**可选元数据**补充（如整数位宽、枚举 tag 位宽）。
3. **版本显式**
   Program 带 `version` 与 `features`，当前只允许 `v0`。
4. **语义稳定**
   IR 语义优先“可解释器运行”，便于自举与调试。

## Program 结构

```
Program {
  version: "v0",
  features: [String],   // 可选，当前必须为空
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
  param_types: [String],   // 可选（与 params 等长）
  return_type: String,     // 可选
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

函数签名支持可选类型标注：

```
fn add(a: i32, b: i32) -> i32
fn main()
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
- `binop` 操作符必须属于 v0 定义集合
- `struct` 字段名不能为空且不可重复
- `term` 必须是 block 的最后一行（终结符后不能再出现指令）
- `load/addr_of` 变量名必须已声明（函数参数或出现过 `store`）
- `load/addr_of` 在所有可达路径上必须已赋值（否则报“可能未初始化”）

### IR 优化（ir-opt）

`ir-opt` 是一个**保守优化**工具，保证 v0 语义不变：

- 常量折叠：`binop` 在常量输入时直接折叠为字面量 operand（不生成指令）
- 分支折叠：`branch` 条件为常量 `bool` 时改写为 `jump`
- 删除不可达块：从函数首块出发的可达性分析
- 变量常量传播（局部）
- 越界检查消除（安全数组）
- 简单内联（单块、无调用的函数）

### 指令文本形态（与 v0 指令一一对应）

```
<operand> := tN | <literal>

tN = load <name>
store <name>, <operand>

tN = addr_of <name>
tN = load_ref tM
store_ref tM, <operand>

tN = <op> <operand>, <operand>

tN = call <callee>(<operand>, <operand>)
call <callee>(<operand>, <operand>)      # 无返回值

tN = array [<operand>, <operand>]
tN = index <operand>[<operand>] [@unchecked]
set_index <operand>[<operand>] = <operand> [@unchecked]

tN = struct <Name> { field: <operand>, other: <operand> }
tN = get_field tA.field
set_field tA.field = <operand>

tN = enum <Name>.<Variant>@<tag>:<tag_type>(<operand>)
tN = enum <Name>.<Variant>@<tag>:<tag_type>

tN = enum_tag <operand>
tN = enum_payload <operand>
```

终结符：

```
jump <label>
branch <operand>, <then>, <else>
return
return <operand>
```

常量 `value`：
- `int`（十进制）
- `true` / `false`
- `"string"`（支持转义：`\n`, `\t`, `\r`, `\"`, `\\`）
- `unit`
- `&N` / `struct#N` / `enum#N` / `array#N`（仅用于调试输出）

> 注意：IR 文本 **需要转义字符串内容**，解析时应进行反转义。

## 值模型（Value）

v0 支持 8 种运行时值：

- `int`（有符号整数，**可选**携带位宽元数据）
- `bool`
- `string`
- `unit`
- `ref`（指向 heap slot）
- `struct`
- `enum`
- `array`

整数类型名集合：

`i8 i16 i32 i64 i128 u8 u16 u32 u64 u128 isize usize char`

> 位宽元数据不改变解释器语义，但可用于后续优化/后端选择。

## 指令集（v0）

### 变量

- 常量通过 operand 直接内联，不再作为指令出现
- `LoadVar dst, name`
  - `@addr`（文本语法 `load_addr`）：返回指向变量的引用
  - `@ref`（文本语法 `load_ref tX`）：从引用 temp 解引用读取
- `StoreVar name, src`
  - `@ref`（文本语法 `store_ref tX, src`）：通过引用 temp 写入

### 二元运算

- `BinOp dst, op, lhs, rhs`

`op` 取值：`+ - * / % == != < <= > >= && ||`

> 一元运算在 IR 中降级为二元：`-x` → `0 - x`，`!x` → `x == false`。

### 调用

- `Call dst, callee, args`

> 约定：方法调用降为 `Type.method`，并把 `self` 作为第一个参数。

### 数组

- `MakeArray dst, elems`
- `Index dst, array, index`（可选 `@unchecked` 表示忽略边界检查）
- `SetIndex array, index, src`（可选 `@unchecked`）

### 结构体

- `MakeStruct dst, name, fields`
- `GetField dst, src, field`
- `SetField src, field, value`

> 字段以**名称**索引，保证语义稳定。
> 布局与偏移属于可选 meta。

### 枚举

- `MakeEnum dst, name, variant, payload, tag, tag_type`
- `EnumTag dst, src`   → `int`（可携带 `tag_type`）
- `EnumPayload dst, src`

> `@repr(...)` 在 IR 中体现为 `tag_type` 元数据。

### 控制流

- `Jump target`
- `Branch cond, then_label, else_label`
- `Return [value]`
