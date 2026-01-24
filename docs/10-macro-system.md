# 宏系统设计

## 目标

1. **卫生宏**：默认不捕获、避免名称冲突（Racket/Nim 风格）
2. **人体工学**：像普通函数一样可测试、可组合、可复用
3. **类型安全**：宏展开后正常类型检查
4. **语法扩展**：支持 DSL 与代码生成

> 说明：宏系统与 `comptile`、泛型常量参数等编译期计算并行发展，设计分开维护，不互相耦合。

---

## 核心模型

### 1) AST 宏（唯一形态）

宏是编译期函数，返回 AST 值：

- `AstExpr` / `AstStmt` / `AstItem` / `AstBlock`

```dast
macro fn gen_add(a: AstExpr, b: AstExpr) -> AstExpr {
    quote { $a + $b }
}

fn main() {
    let v = gen_add!(1, 2)
    println("v", v)
}
```

### 2) 生成 AST 的两种方式

**结构化：`quote`（推荐）**

```dast
macro fn mk_stmt(x: AstExpr) -> AstStmt {
    quote { let v = $x; }
}
```

**字符串：`ast_*`（低阶/调试用途）**

```dast
macro fn mk_stmt(x: AstExpr) -> AstStmt {
    ast_stmt("let v = 1;")
}
```

### 3) AST 插回代码（splice）

**隐式：`macro_name!()`**

- 表达式/语句/顶层位置均视作隐式 unquote

```dast
let x = add1!(1)
log_stmt!()
gen_const!(MAGIC, 7)
```

**显式：`compile!(ast)`**

- 用于非宏函数返回的 AST 值，或需要显式插入的场景

```dast
let ast = some_macro_like_fn()
let v = compile!(ast)
```

---

## 语法细节

### quote

`quote` 是结构化 AST 构造器，推荐使用显式目标形式以消除歧义：

```dast
quote expr { 1 + 2 }        // -> AstExpr
quote stmt { let x = 1; }   // -> AstStmt
quote item { const X: i64 = 1 } // -> AstItem
quote block { { let x = 1; x } } // -> AstBlock
```

> `quote{...}` 的简写也允许，但其返回类型应由上下文决定（宏签名的返回类型）。

### unquote/splice

在 `quote` 里用 `$` 插入 AST：

```dast
quote expr { $a + $b }
quote stmt { let $name = $value; }
quote item { struct $name { value: i64 } }
quote expr { $(bind("tmp")) + 1 }
```

规则：

- `$x` 允许直接插入标识符
- `$(expr)` 允许插入任意表达式（常用于 `bind`/函数调用）
- `$x` 与 `$(expr)` 的 AST 类型需与当前位置匹配
- 类型不匹配时编译期报错
- `$$` 表示字面量 `$`

### macro 调用的插入规则

`macro_name!()` 在三种位置都视为 **隐式 unquote**：

| 位置 | 期望 AST 类型 |
|------|--------------|
| 表达式 | `AstExpr` / `AstBlock` / `AstStmt(仅表达式语句)` |
| 语句 | `AstStmt` / `AstExpr` / `AstBlock` |
| 顶层 | `AstItem` |

显式插入使用 `compile!(ast)`，用于非宏函数返回的 AST 值。

> **参数默认按表达式语法捕获为 AST**。  
> 若你已经有 AST 值（例如 `quote` / `ast_expr` / `gensym` / `bind` / 其它宏返回），可直接传入，编译器不会再包一层。

---

## 卫生性（Racket/Nim 风格）

### 默认规则（卫生）

- 宏内部“新引入的名字”不会捕获调用点变量
- 宏参数插入的标识符保留调用点语义

这避免了大多数宏陷阱，读写体验最好。

### 显式破坏卫生（必要时）

提供两类逃逸工具：

- `gensym(prefix) -> AstExpr`：生成唯一标识符（避免冲突）
- `bind(name) -> AstExpr`：显式绑定调用点同名标识符（故意捕获）

示例：

```dast
macro fn with_tmp(x: AstExpr) -> AstExpr {
    let t = gensym("tmp")
    quote { let $t = $x; $t + 1 }
}

macro fn use_caller_tmp() -> AstExpr {
    quote { $(bind("tmp")) + 1 }
}
```

---

## 模块与可见性

宏遵循普通可见性规则：

```dast
pub macro fn gen_const(...) -> AstItem { ... }

foo.gen_const!(...)
```

---

## 编译流程

```
parse
  -> macro expand (quote/unquote, !)
  -> typecheck
  -> compile
```

宏展开阶段会多轮执行（可设置上限），直到 AST 中不再出现宏调用。

---

## 宏展开规则

### 展开顺序

1. **先内后外**：优先展开最内层的宏调用
2. **同层从左到右**：按源代码顺序处理
3. **多轮迭代**：一轮展开可能引入新的宏，直到不再出现或达到上限

### 递归与上限

- 允许宏展开宏（嵌套/递归）
- 设定最大展开深度与总步数（避免无限递归）
- 触顶时报错，提示可能的递归宏

### 错误定位

- 展开产生的错误应尽量回溯到宏调用点
- 可选提供“展开栈”用于诊断（宏调用链）

---

## 示例

### 1) 生成表达式
```dast
macro fn add1(x: AstExpr) -> AstExpr {
    quote { $x + 1 }
}

fn main() {
    let v = add1!(41)
    println("v", v)
}
```

### 2) 生成语句
```dast
macro fn log_stmt() -> AstStmt {
    quote { println("hello") }
}

fn main() {
    log_stmt!()
}
```

### 3) 生成全局定义
```dast
macro fn gen_counter(name: AstExpr) -> AstItem {
    quote {
        struct $name { value: i64 }
    }
}

gen_counter!(Counter)
```

---

## 未来扩展

- `quote` 支持更丰富的插值与模式
- typed macro（可选）
- 宏调试/展开追踪工具
