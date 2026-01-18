# 包管理与依赖系统

## 包配置文件 (dast.toml)

### 基础配置

```toml
[package]
name = "my_project"
version = "0.1.0"
edition = "2026"
authors = ["Your Name <you@example.com>"]
license = "MIT"
description = "A sample Dast project"
repository = "https://github.com/user/my_project"

[dependencies]
# 中央仓库依赖
json = "1.0"
http = { version = "2.0", features = ["tls"] }

# Git 依赖
my_lib = { git = "https://github.com/user/my_lib", branch = "main" }
forked_lib = { git = "https://github.com/myuser/forked_lib", rev = "abc123" }

# 本地路径依赖
local_lib = { path = "../local_lib" }

[dev-dependencies]
test_utils = "0.5"

[build-dependencies]
build_script = "1.0"
```

---

## Git 依赖支持

### 灵活的 Git 引用

```toml
[dependencies]
# 分支
lib1 = { git = "https://github.com/user/lib1", branch = "develop" }

# 标签
lib2 = { git = "https://github.com/user/lib2", tag = "v1.2.3" }

# 提交哈希
lib3 = { git = "https://github.com/user/lib3", rev = "abc123def" }

# 默认分支（main/master）
lib4 = { git = "https://github.com/user/lib4" }

# 子目录
lib5 = { git = "https://github.com/user/monorepo", path = "packages/lib5" }
```

### Fork 工作流

```toml
# 原始依赖
# json = "1.0"

# 直接使用 Fork（库名不变）
json = { git = "https://github.com/myuser/json", branch = "my-fixes" }

# 代码中的 import 不需要改变
# import json  // 仍然是库名
```

**优势**:
- import 使用库名，不是路径
- 无需修改代码
- 比 Go 简单（Go 需要改 import 路径）

---

## 依赖解析

### 版本约束

```toml
[dependencies]
# 精确版本
lib1 = "=1.2.3"

# 兼容版本（默认）
lib2 = "1.2.3"     # >= 1.2.3, < 2.0.0
lib3 = "^1.2.3"    # 同上

# 次版本兼容
lib4 = "~1.2.3"    # >= 1.2.3, < 1.3.0

# 范围
lib5 = ">= 1.2, < 2.0"

# 通配符
lib6 = "1.*"       # >= 1.0.0, < 2.0.0
```

---

## 包发布

### 发布到中央仓库

```bash
# 登录
$ dast login

# 发布
$ dast publish

# 指定版本
$ dast publish --version 1.0.0

# 干运行
$ dast publish --dry-run
```

### 发布配置

```toml
[package]
name = "my_lib"
version = "1.0.0"
publish = true  # 或 false 禁止发布

# 排除文件
exclude = [
    "tests/",
    "examples/",
    "*.tmp"
]

# 包含文件（优先级更高）
include = [
    "src/**/*.dast",
    "README.md",
    "LICENSE"
]
```

---

## 工作空间 (Workspace)

### 单仓库多包

```toml
# 根目录 dast.toml
[workspace]
members = [
    "core",
    "utils",
    "cli",
]

# 共享依赖版本
[workspace.dependencies]
serde = "1.0"
tokio = "1.0"

# 成员包可以使用
# core/dast.toml
[dependencies]
serde = { workspace = true }
```

---

## 与其他语言对比

| 特性 | Cargo (Rust) | Go Modules | npm | Dast |
|------|-------------|-----------|-----|------|
| Git 依赖 | ✅ | ⚠️ 复杂 | ✅ | ✅ |
| 版本约束 | ✅ | ⚠️ 最小版本 | ✅ | ✅ |
| Fork 工作流 | ✅ | ❌ 需改路径 | ✅ | ✅ |
| Workspace | ✅ | ✅ | ✅ | ✅ |
| 中央仓库 | ✅ | ✅ | ✅ | ✅ |
| 锁文件 | ✅ | ✅ | ✅ | ✅ |

### Cargo vs Dast

```toml
# Cargo
[dependencies]
serde = { git = "https://github.com/myuser/serde", branch = "fix" }

# Dast (相同)
[dependencies]
json = { git = "https://github.com/myuser/json", branch = "fix" }
```

**相似度**: 非常高，语法几乎相同

### Go Modules 的问题

#### 1. Git 依赖复杂

```go
// Go: 需要 replace
replace github.com/original/lib => github.com/myuser/lib v0.0.0-20230101000000-abc123def

// Dast: 直接指定
[dependencies]
lib = { git = "https://github.com/myuser/lib", branch = "main" }
```

#### 2. Fork 需要改 import 路径

```go
// Go: 需要修改所有 import
import "github.com/myuser/lib"  // 而不是 original/lib

// Dast: import 使用库名，不需要改
import lib  // 库名不变
```

---

## 依赖锁文件 (dast.lock)

### 自动生成

```toml
# 此文件由 dast 自动生成，不要手动编辑

[[package]]
name = "json"
version = "1.0.5"
source = "registry+https://crates.dast.io"
checksum = "abc123..."

[[package]]
name = "my_lib"
version = "0.1.0"
source = "git+https://github.com/user/my_lib?branch=main#abc123def"

[[package]]
name = "local_lib"
version = "0.1.0"
source = "path+../local_lib"
```

### 版本控制

```bash
# 提交到 Git
$ git add dast.lock

# 确保团队使用相同版本
$ dast build  # 使用 dast.lock 中的精确版本
```

---

## 私有仓库

### 配置私有源

```toml
# ~/.dast/config.toml
[registries]
my-company = { index = "https://registry.company.com" }

[registries.my-company]
token = "secret-token"
```

### 使用私有依赖

```toml
[dependencies]
internal_lib = { version = "1.0", registry = "my-company" }
```

---

## 缓存和离线模式

```bash
# 下载所有依赖到本地缓存
$ dast fetch

# 离线构建
$ dast build --offline

# 清理缓存
$ dast clean --cache
```

---

## 建议

| 特性 | 决策 |
|------|------|
| 配置格式 | TOML (类似 Cargo) |
| Git 依赖 | ✅ 完整支持 |
| Patch 机制 | ✅ 支持 |
| 中央仓库 | ✅ 类似 crates.io |
| 工作空间 | ✅ 支持 |
| 锁文件 | ✅ 自动生成 |

**核心**: 结合 Cargo 的优点，避免 Go Modules 的复杂性
