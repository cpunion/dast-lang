# Dast IR v0 规范（稳定核心）

> 目标：**稳定**且**最小**的 IR。当前由 **stage0** 使用；stage2 暂未输出 IR v0（直接生成 QBE），未来如需互操作再对齐。

## 0. 范围与版本

- 仅支持 `v0`。
- 本文档同时包含**规范**与**必要的背景说明**；历史简化过程记录在文末“设计演进记录”。

---

## 1. 文本格式总览（ir v0）

### 顶层结构

```
ir v0

enum Option[T] tag i32
  variant None = 1 : unit
  variant Some = 0 : T

type Point = { x: i64, y: i64 }

fn add(a: i64, b: i64) -> i64
  block entry0:
    t0: i64 = + a, b
    return t0
```

**顺序约定**：
1) `ir v0` 头  
2) `enum` 声明（可选）  
3) `type` 声明（可选）  
4) `fn` 函数（至少一个）  

### 函数签名

```
fn name(param: Type, ...) -> ReturnType
```

- 返回类型缺省时视为 `unit`。  
- 参数名是**变量名**，可在函数体内直接 `load/store`。

### 临时变量与类型标注

指令可带**可选**类型标注：

```
t3: i64 = + t1, t2
t7 = call foo(t0)
```

`tN: <Type>` 只作为**调试/后端提示**；解析器接受无标注形式。

---

## 2. Program 结构（语义模型）

```
Program {
  Version: "v0",
  Features: [],               // 目前必须为空
  Enums:    [EnumDecl],
  TypeDecls: { name -> TypeDecl },
  Functions: { name -> Function },
  Entry: String               // 可选
}
```

### EnumDecl

```
enum Name[T] tag i32
  variant A = 0 : unit
  variant B = 1 : T
```

> 仅作为**类型与 tag 元数据**；IR 指令层面不包含专用 enum 指令（见“枚举表示”）。

### TypeDecl（结构体）

```
type Point = { x: i64, y: i64 }
```

仅用于 **struct** 类型。

---

## 3. 值模型（Value）

IR v0 运行时值（解释器/IR 级语义）：

- `int`（可带位宽标注，如 `i32 7`）
- `float`（文本形式存储，如 `3.14`）
- `bool`
- `string`
- `unit`
- `ref`
- `struct`
- `enum`（解释器层可见；IR 指令表现为 struct）
- `array`
- `ast`（宏系统用）

**整数类型名集合**：
`i8 i16 i32 i64 i128 u8 u16 u32 u64 u128 isize usize char`

---

## 4. 指令集（当前实现）

> 下面为**真实实现**的 IR v0 指令集合与文本语法。

### 4.1 变量与引用

```
tN = load <name>
tN = load_addr <name>
tN = load_ref tM

store <name>, <operand>
store_ref tM, <operand>
```

说明：
- 变量名由**参数**与**首次 store** 引入。
- `load_ref/store_ref` 通过引用 temp 读写。

### 4.2 二元运算

```
tN = <op> <lhs>, <rhs>
```

支持的 `op`：
`+ - * / % == != < <= > >= && || & | ^ << >>`

> 一元运算在 IR 中降级为二元：  
> `-x` → `0 - x`，`!x` → `x == false`。

### 4.3 调用

```
tN = call <callee>(arg0, arg1, ...)
call <callee>(arg0, arg1, ...)

tN = call_closure <closure>(args...)
call_closure <closure>(args...)
```

### 4.4 数组

```
tN = array [e0, e1, ...]
tN = index <array>[<idx>] [@unchecked]
set_index <array>[<idx>] = <value> [@unchecked]

tN = addr_index tM[<idx>]
```

### 4.5 结构体

```
tN = struct <Name> { field: <op>, ... }
tN = tM.field
tM.field = <op>

tN = addr_field tM.field
```

### 4.6 控制流

```
jump <label>
branch <cond>, <then>, <else>
return
return <operand>
```

---

## 5. 枚举表示（重要）

**IR v0 不包含专用 enum 指令**。枚举在 IR 中表示为：

```
type Option = { _tag: i32, _payload: T }
```

构造与访问通过 struct 指令完成：

```
t0 = struct Option { _tag: 0, _payload: x }
t1 = t0._tag
t2 = t0._payload
```

**EnumDecl** 仅提供 tag 与 payload 类型信息，用于验证与后端生成。

---

## 6. 校验规则（ir-verify）

验证器主要检查：

- `version` 必须为 `v0`，`features` 为空
- `entry` 若存在必须指向已定义函数
- block 标签非空且唯一
- 每个 block 必须有终结符
- `jump/branch` 目标必须存在
- temp 号必须合法（`tN` 且 `0 <= N < temp_count`）
- `call/call_closure` 的 `dst` 可为 `-1` 表示无返回值
- `binop` 必须属于 v0 操作符集合
- `struct` 字段名非空且不可重复
- `load` 的变量必须先声明（参数或首次 store）
- `load` 在所有可达路径上必须已赋值（“可能未初始化”检查）

> `temp_count` 在文本格式中不显式出现，由解析器扫描 `tN` 自动推导。

---

## 7. 优化（ir-opt）

`ir-opt` 为保守优化，保持语义不变，当前包含：

- 常量折叠（int/bool/string）
- 分支折叠（常量条件）
- 数组边界检查消除（静态可证安全）
- 删除不可达块
- 单块、无调用函数的内联

---

## 8. 与 Dast 语法对照（示例）

| Dast | IR |
|------|-----|
| `let x = 1` | `store x, 1` |
| `let mut x = 1` | `store x, 1`（变量可变性由前端保证） |
| `x = x + 1` | `t0 = load x` → `t1 = + t0, 1` → `store x, t1` |
| `&mut x` | `t0 = load_addr x` |
| `*p` | `t0 = load_ref p` |
| `p.x` | `t0 = t1.x` |
| `Point { x: 0 }` | `t0 = struct Point { x: 0, ... }` |
| `if c { } else { }` | `branch` + blocks |
| `while c { }` | `jump` + `branch` + blocks |

---

## 9. 设计演进记录（摘要）

IR v0 经历的主要简化：

1. **Index/SetIndex 合并 unchecked 版本**  
   `IndexUnchecked/SetIndexUnchecked` → `@unchecked` 标记
2. **Enum 降解为 Struct**  
   移除 `MakeEnum/EnumTag/EnumPayload`，改用 `struct + _tag/_payload`
3. **地址获取与引用合并**  
   `addr_of` → `load_addr`；`load_ref/store_ref` 通过 `LoadVar/StoreVar` 标记
4. **移除 UnaryOp/Const**  
   一元运算降级为二元；常量直接作为 operand

