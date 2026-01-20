# 模块与包系统

## 设计目标

1. **简洁** - 比 Rust 更直观的模块声明
2. **清晰的可见性** - 默认私有，显式公开
3. **扁平化** - 减少嵌套层次
4. **包管理** - 内置依赖管理

---

## 模块系统

### 目录即模块（Go 风格）

```
project/
├── src/
│   ├── main.dast        # 主入口（根模块）
│   ├── utils/           # utils 模块目录
│   │   ├── lib.dast
│   │   └── io.dast
│   └── parser/          # parser 模块目录
│       ├── lexer.dast
│       └── ast.dast
```

### 导入

> 规则：import 路径必须使用字符串字面量（支持包根与相对路径）。

```dast
// 整个模块
import "utils"
utils.helper()

// 具体项
import { helper, format } from "utils"
helper()

// 别名
import "utils" as u
u.helper()

// 项别名
import { helper as h } from "utils"
h()

// 子模块
import "parser/lexer"
import { Token } from "parser/lexer"
```

### 导出 (pub)

```dast
// 默认私有
fn internal() { }

// 公开
pub fn public_api() { }

pub struct Point {
    pub x: f32,  // 公开字段
    pub y: f32,
    private: i32,  // 私有字段
}

// 公开整个模块
pub mod submodule
```

---

## 可见性级别

```dast
// 私有 (默认)
fn private_fn() { }

// 模块公开
pub fn public_fn() { }

// 包内公开 (可选)
pub(crate) fn crate_fn() { }

// 父模块公开 (可选)
pub(super) fn parent_fn() { }
```

---

## 包 (Crate/Package)

### 项目结构

```
my_project/
├── dast.toml          # 包配置
├── src/
│   ├── main.dast      # 可执行包入口
│   └── lib.dast       # 库包入口
└── examples/
    └── demo.dast
```

### dast.toml

```toml
[package]
name = "my_project"
version = "0.1.0"
edition = "2026"

[dependencies]
json = "1.0"
http = { version = "2.0", features = ["tls"] }

[dev-dependencies]
test_utils = "0.5"
```

---

## 与其他语言对比

| 特性 | Rust | Go | Dast |
|------|------|-----|------|
| 模块声明 | `mod x;` | 目录即包 | 文件即模块 |
| 导入语法 | `use` | `import` | `import` |
| 默认可见性 | 私有 | 首字母大写 | 私有 |
| 公开标记 | `pub` | 首字母大写 | `pub` |
| 包管理 | Cargo | go mod | dast.toml |

---

## 模块声明简化

### Rust 需要

```rust
// lib.rs
mod utils;     // 声明
mod parser;

// main.rs
use crate::utils;
use crate::parser::Token;
```

### Dast 简化

```dast
// main.dast
import "utils"           // 直接导入
import { Token } from "parser"
```

**无需 `mod` 声明** - 目录存在即为模块

---

## 循环导入

```dast
// a/lib.dast
import "b"  // ❌ 如果 b 也导入 a，循环错误

// 编译器检测并报错
// Error: Circular import detected: a -> b -> a
```

---

## 重导出

```dast
// parser/mod.dast
pub import { Token, Lexer } from "lexer"
pub import { Ast, Node } from "ast"

// 用户可以直接
import { Token, Ast } from "parser"
```

也可以直接重导出整个模块的公开项：

```dast
pub import "lexer"
```

---

## 条件编译

```dast
@cfg(target_os = "windows")
fn platform_specific() { }

@cfg(debug)
fn debug_only() { }

// 或使用 if
if @cfg(wasm) {
    // WASM 特定代码
}
```

---

## 总结

| 特性 | 决策 |
|------|------|
| 模块模型 | 目录即模块 |
| 导入语法 | `import "x"` + `import { y } from "x"` |
| 循环导入 | 禁止 |
| 条件编译 | `@cfg` |

### 可见性规则

| 项目 | 私有 | pub |
|------|------|-----|
| 类型 | 包内可见 | 外部可见 |
| 字段/方法 | 类型内可见 | 外部可见 |

```dast
struct Helper { }          // 包内可用

pub struct Api {
    cache: Helper,          // 私有字段 (类型内)
    pub name: String,       // 公开字段
}
```

**比 Rust 简化**: 无需 `mod` 声明，无需 `pub(crate)`
