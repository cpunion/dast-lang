# Stage0 语言特性（扩展版）

> 本文描述 **Stage0（Go 实现）当前实际支持的语言能力**，包含最初 Stage0 规划的子集 + 近期扩展能力。该文档用于“实现现状”的基准说明与测试验收参考。

## 1. 设计目标与边界

- **目标**：可自举的最小编译器内核，输出 IR v0，保持实现简单、稳定。
- **扩展**：为 Stage2 开发效率，提前实现模块/依赖/测试、宏系统、泛型/trait/type alias 等特性。
- **不包含**（仍留给 Stage2）：async/await、unsafe、comptime 泛型参数、过程宏、完整标准库扩展等。

## 2. 命令与输出

Stage0 CLI（`dast-stage0`）：

```bash
# 构建/运行/测试
./compiler/bootstrap/stage0/dast-stage0 build [--emit-ir|--emit-qbe] <dir|file.dast ...> [-o output]
./compiler/bootstrap/stage0/dast-stage0 run   <file.dast> [more.dast ...] [-- args...]
./compiler/bootstrap/stage0/dast-stage0 test  [dir|file.dast ...]

# IR v0 流水线
./compiler/bootstrap/stage0/dast-stage0 ir        <file.dast> [more.dast ...]
./compiler/bootstrap/stage0/dast-stage0 ir-run    <file.ir> [-- args...]
./compiler/bootstrap/stage0/dast-stage0 ir-verify <file.ir>
./compiler/bootstrap/stage0/dast-stage0 ir-opt    <file.ir>
./compiler/bootstrap/stage0/dast-stage0 ir-qbe    <file.ir>
```

说明：
- `dast build` 默认输出**可执行文件**（`-o` 指定路径；未指定默认为 `./a.out`）。
- 使用 `--emit-ir` 时，`-o` 输出 IR 文件；未指定则输出到 stdout。
- 使用 `--emit-qbe` 时，`-o` 输出 QBE 文件；未指定则输出到 stdout。
- Stage0 可执行输出走 **IR → QBE → cc** 流水线（需要本机可用的 `qbe` 与 `cc/clang`）。

## 3. 包与模块

### 3.1 目录结构

- 包根目录包含 `dast.toml`
- 代码默认在 `src/` 下
- 同一目录下多个 `.dast` 文件视为同一个模块集合
 - 支持工作空间（workspace）根目录：`dast.toml` 中包含 `[workspace]`

示例：
```
package/
├── dast.toml
└── src/
    ├── main.dast
    ├── math.dast
    └── util/
        └── sub/
            └── ops.dast
```

### 3.2 import 语法

```dast
import "utils"          // 导入目录模块，导入后可直接使用其中符号
import "util.sub" as sub  // 路径点号分隔，使用别名访问
```

- `"a.b.c"` 对应 `src/a/b/c/` 目录
- `import "path" as alias` 后，通过 `alias.name` 访问导入符号
- `import "std/xxx"` 从标准库根目录解析
  - Stage0: 优先 `DAST_STDLIB`，否则自动定位到 `compiler/bootstrap/stage0/stdlib`
  - 若 stdlib 不存在，导入失败

### 3.3 依赖（路径依赖）

`dast.toml` 支持路径依赖：

```toml
[dependencies]
util = { path = "../util" }
math = { path = "../math" }
```

- 仅支持 `path` 依赖
- 依赖包的 `src/` 作为模块根

### 3.4 Workspace 依赖

```toml
# workspace 根 dast.toml
[workspace]
members = ["app", "util"]

[workspace.dependencies]
util = { path = "util" }

# app/dast.toml
[dependencies]
util = { workspace = true }
```

- 支持 `workspace = true` 形式引用 workspace 根统一声明的路径依赖
- Stage0 的 workspace `members` 解析为**单行数组字面量**（暂不支持多行展开）

## 4. 基础类型与表达式

### 4.1 基础类型

- 整型：`int`, `i8/i16/i32/i64`, `u8/u16/u32/u64`, `isize/usize`, `char`
- 布尔：`bool`
- 字符串：`String`/`string`
- 单元：`unit`
- 数组：`[T]`
- 引用：`&T`, `&mut T`（IR 统一为 `*T`）

### 4.2 表达式

- 二元：`+ - * / % == != < <= > >= && ||`
- 一元：`- ! * & &mut`
- 索引：`arr[i]`
- 字段访问：`obj.field`
- 块表达式：`{ ... }`
- if/match 表达式（可有返回值）

## 5. 结构体与枚举

### 5.1 结构体

```dast
struct Point { x: i32, y: i32 }
let p = Point { x: 1, y: 2 }
let v = p.x
p.y = 3
```

### 5.2 枚举

```dast
enum OptionI32 { Some(i32), None }
let v = OptionI32.Some(3)
```

## 6. 模式匹配与控制流

### 6.1 match / if let / while let

```dast
match v {
    0 => 10,
    1 | 2 => 20,
    3..=5 => 30,
    _ => 40,
}

if let .Some(x) = opt { ... }
while let .Some(x) = opt { ... }
```

支持的模式：
- 常量字面量
- `_|` 通配符
- `a | b`（不允许带绑定）
- `a..=b`（仅 int）
- 结构体模式：`Point { x, y }`
- 枚举模式：`.Some(x)` / `.None`
- match guard：`if <bool>`

### 6.2 控制流

- `if / else`
- `while`, `loop`
- `break`, `continue`
- `return`

## 7. 借用与引用

- `&T` 与 `&mut T`
- 解引用 `*p`
- 允许对 **变量/字段/索引** 取引用
- 赋值通过 `*ref = ...`

> Stage0 的借用检查为简化版，但具备基础的只读/可变规则约束。

## 8. 函数、方法与 impl

```dast
fn add(a: i32, b: i32) -> i32 { a + b }

struct Counter { value: i64 }
impl Counter {
    fn inc(self: &mut Counter) { self.value = self.value + 1 }
}
```

## 9. 泛型、trait 与 type alias（扩展）

### 9.1 泛型（单态化）

```dast
struct Box[T] { value: T }
fn identity[T](x: T) -> T { x }

impl[T] Box[T] {
    fn get(self: &Self) -> T { self.value }
}
```

- 编译期单态化
- 仅静态分发，无动态调度

### 9.2 trait 与约束

```dast
trait Cloneable { fn clone(self: &Self) -> Self }
impl Cloneable for Point { ... }

fn use_bound[T: Cloneable](x: T) -> i32 { 1 }
```

### 9.3 type alias

```dast
type IntList = [i32]
```

## 10. 常量表达式

支持：字面量 + 一元/二元运算 + 括号

```dast
const A = 1 + 2 * 3
const B = -(4 + 5)
const C = !false
```

不支持：函数调用等复杂表达式（会报错）。

## 11. 宏系统（扩展）

### 11.1 宏函数

```dast
macro fn add_expr() -> AstExpr {
    ast_expr("1 + 2")
}
```

- 返回类型可为：`AstExpr`, `AstStmt`, `AstItem`, `AstBlock`
- 宏调用：`name!(...)`, `name![...]`, `name!{...}`
  - 参数默认按表达式语法捕获为 AST（可直接写 `add1!(1)`）

### 11.2 quote / splice / bind

```dast
macro fn add1(x: AstExpr) -> AstExpr {
    quote expr { $x + 1 }
}

macro fn make_stmt(x: AstExpr) -> AstStmt {
    quote stmt { let $(bind("v")) = $x; }
}
```

- `quote expr|stmt|item|block { ... }` 生成 AST
- `$x` 进行 splice
- `bind("name")` 提供卫生性绑定
- `gensym()` 生成唯一标识符

### 11.3 compile!

```dast
let v = compile!(ast_expr("1 + 2"))
compile!(ast_item("const X: i64 = 7"))
```

- `compile!(...)` 将 AST 注入当前编译阶段
- 在表达式/语句/顶层均可使用

## 12. 闭包（扩展）

```dast
let mut count = 0
let incr = || { count = count + 1 }
incr()
```

- 支持无参数闭包、以及带参数闭包（`|x, y| { ... }`）
- 捕获外部变量（含可变捕获）
- 编译到 closure + env 结构

## 13. 测试（扩展）

- 测试文件：`*_test.dast`
- 测试函数：`fn test_*()`（**必须无参数**）
- `dast test` 会编译并运行测试
- 使用 `import "std/testing"` 获取断言宏
- 暂不支持 `@test` 标记（已废弃）

## 14. IR v0

- Stage0 输出 **IR v0**（稳定核心）
- 支持 `ir / ir-run / ir-verify / ir-opt / ir-qbe`
- 已提供 IR 生成与 IR→QBE 的单元测试覆盖

---

## 15. 未实现 / 明确不支持

- async/await
- unsafe
- comptime 泛型参数 / const generics
- 过程宏（proc macro）
- 动态 trait 对象（dyn trait）
- 完整标准库

> 若需扩展能力，应优先在 Stage0 中实现**必要最小子集**，并保持 IR v0 兼容。
