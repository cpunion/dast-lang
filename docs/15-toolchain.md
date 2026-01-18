# 工具链设计

## 核心工具

### dast - 主命令

```bash
$ dast --version
$ dast --help

# 构建
$ dast build
$ dast build --release

# 运行
$ dast run
$ dast run --release

# 测试
$ dast test
$ dast bench
$ dast fuzz

# 包管理
$ dast init
$ dast new my_project
$ dast add json
$ dast publish
```

---

## 格式化工具 (dastfmt)

### 基础用法

```bash
# 格式化文件
$ dast fmt src/main.dast

# 格式化整个项目
$ dast fmt

# 检查格式（不修改）
$ dast fmt --check

# 显示差异
$ dast fmt --diff
```

### 配置文件 (dast.toml)

```toml
[fmt]
# 缩进
indent_style = "spaces"
indent_size = 4

# 行宽
max_width = 100

# 导入排序
imports_granularity = "crate"

# 尾随逗号
trailing_comma = "vertical"
```

### 格式化规则

```dast
// 自动格式化前
fn add(a:i32,b:i32)->i32{a+b}

// 自动格式化后
fn add(a: i32, b: i32) -> i32 {
    a + b
}
```

---

## Linter (dastlint)

### 基础用法

```bash
# 运行 lint
$ dast lint

# 自动修复
$ dast lint --fix

# 指定级别
$ dast lint --deny warnings
```

### Lint 规则

#### 代码质量

```dast
// ❌ 未使用的变量
fn foo() {
    let x = 42  // warning: unused variable
}

// ✅ 使用下划线
fn foo() {
    let _x = 42  // OK
}
```

#### 性能

```dast
// ❌ 不必要的克隆
fn process(data: &Vec[i32]) {
    let copy = data.clone()  // warning: unnecessary clone
    // ...
}

// ✅ 直接使用引用
fn process(data: &Vec[i32]) {
    // ...
}
```

#### 风格

```dast
// ❌ 不一致的命名
fn MyFunction() { }  // warning: function names should be snake_case

// ✅ 正确命名
fn my_function() { }
```

### 配置 (dast.toml)

```toml
[lint]
# 警告级别
level = "warn"  # deny, warn, allow

# 禁用特定规则
disabled = [
    "unused_variables",
]

# 启用额外规则
enabled = [
    "pedantic",
]
```

### Lint 类别

| 类别 | 说明 |
|------|------|
| correctness | 正确性（默认 deny） |
| performance | 性能（默认 warn） |
| style | 风格（默认 warn） |
| complexity | 复杂度（默认 warn） |
| pedantic | 严格检查（默认 allow） |

---

## 文档生成 (dastdoc)

### 基础用法

```bash
# 生成文档
$ dast doc

# 打开文档
$ dast doc --open

# 生成私有项文档
$ dast doc --document-private-items
```

### 文档注释

```dast
/// 计算两个数的和
///
/// # 参数
/// * `a` - 第一个数
/// * `b` - 第二个数
///
/// # 返回
/// 两个数的和
///
/// # 示例
/// ```
/// let result = add(2, 3)
/// assert_eq!(result, 5)
/// ```
pub fn add(a: i32, b: i32) -> i32 {
    a + b
}
```

### 文档特性

- **Markdown 支持** - 完整的 Markdown 语法
- **代码示例** - 自动测试代码示例
- **搜索** - 全文搜索
- **跨引用** - 自动链接类型和函数

### 输出格式

```
target/doc/
├── index.html
├── my_crate/
│   ├── index.html
│   ├── fn.add.html
│   └── struct.Point.html
└── search-index.js
```

---

## 其他工具

### LSP 服务器（内置）

```bash
# LSP 服务器内置在编译器中
$ dast lsp

# 编辑器自动调用
# 无需单独安装

# 功能
- 代码补全
- 跳转定义
- 查找引用
- 重命名
- 实时诊断
- 悬停提示
- 代码操作
```

**优势**:
- 无需单独安装
- 与编译器共享代码
- 始终保持同步
- 类似 Go 的 `gopls`

### dast expand (宏展开)

```bash
# 展开宏
$ dast expand src/main.dast

# 展开特定宏
$ dast expand --macro vec!
```

### dast check (快速检查)

```bash
# 只检查，不生成代码
$ dast check

# 比 build 快，用于 IDE
```

---

## 与其他语言对比

| 工具 | Rust | Go | Dast |
|------|------|-----|------|
| 格式化 | rustfmt | gofmt | dastfmt |
| Linter | clippy | golint | dastlint |
| 文档 | rustdoc | godoc | dastdoc |
| LSP | rust-analyzer (独立) | gopls (内置) | 内置 |
| 包管理 | cargo | go mod | dast |

**优势**: LSP 内置在编译器中，无需单独安装

---

## 编辑器集成

### VS Code

```json
{
  "dast.lsp.enable": true,
  "dast.fmt.onSave": true,
  "dast.lint.onSave": true
}
```

**LSP 自动启动**: 编辑器调用 `dast lsp`

### Vim/Neovim

```vim
" 使用内置 LSP
lua << EOF
require'lspconfig'.dast.setup{
  cmd = {'dast', 'lsp'}
}
EOF
```

---

## CI/CD 集成

```yaml
# GitHub Actions
name: CI
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: dast-lang/setup-dast@v1

      - name: Format check
        run: dast fmt --check

      - name: Lint
        run: dast lint

      - name: Test
        run: dast test

      - name: Build
        run: dast build --release
```

---

## 最终设计

| 工具 | 功能 | 位置 |
|------|------|------|
| dastfmt | 代码格式化 | 内置 |
| dastlint | 代码检查 | 内置 |
| dastdoc | 文档生成 | 内置 |
| LSP | 语言服务器 | 内置 |

**核心**: 所有工具内置在 `dast` 命令中，开箱即用
