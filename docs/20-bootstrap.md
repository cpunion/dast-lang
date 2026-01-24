# Dast 编译器自举规划

## 概述

Dast 采用**分阶段自举**策略：

- **Stage0**：Go 实现的 bootstrap 编译器
- **Stage1**：Dast 实现，由 Stage0 驱动
- **Stage2**：Dast 实现，完整模块化版本

```
Stage 0 (Go)              Stage 1 (Dast)            Stage 2 (Dast)
┌────────────────────┐    ┌────────────────────┐    ┌────────────────────┐
│  compiler/         │ →  │  bootstrap/stage1/ │ →  │  compiler/stage2/  │
│  bootstrap/stage0/ │    │                    │    │                    │
└────────────────────┘    └────────────────────┘    └────────────────────┘
     编译 Stage1               编译 Stage2              完整编译器
```

## 目录结构

### Stage0 (Go 实现)

```
compiler/bootstrap/stage0/
├── cmd/dast/
│   └── main.go              # 主入口
├── internal/
│   ├── ast/                 # AST 定义
│   ├── lexer/               # 词法分析
│   ├── parser/              # 语法分析
│   ├── typecheck/           # 类型检查
│   ├── compile/             # AST → IR
│   ├── ir/                  # IR 定义、解析、格式化
│   └── interp/              # IR 解释器
├── tests/                   # Stage0 测试
└── dast-stage0              # 编译好的二进制
```

### Stage1 (Dast 实现) - 新结构

```
compiler/bootstrap/stage1/
│
├── compiler/                    # 编译器 (Dast → IR)
│   ├── token.dast              # Token 定义
│   ├── lexer.dast              # 词法分析
│   ├── ast.dast                # AST 定义
│   ├── parser.dast             # 语法分析
│   ├── typecheck.dast          # 类型检查
│   ├── compile.dast            # 编译 (AST → IR)
│   └── main.dast               # 入口点 (run/ir/build 命令)
│
├── irv0/                        # IR v0 模块 (可共享)
│   ├── types.dast              # IR 数据结构 (IrProgram, IrFunction, IrVar...)
│   ├── parser.dast             # IR 文本解析
│   ├── format.dast             # IR 格式化输出
│   └── validate.dast           # IR 验证
│
├── interp/                      # IR 解释器
│   ├── runtime.dast            # 运行时值、内置函数
│   ├── eval.dast               # 指令求值
│   └── interp.dast             # 解释器主逻辑
│
├── codegen/                     # 代码生成器
│   ├── c_codegen.dast          # IR → C 代码生成
│   ├── c_runtime.h             # C 运行时头文件
│   └── c_runtime.c             # C 运行时实现
│
└── tests/                       # 测试
    ├── ir/                      # IR 测试文件
    │   ├── valid.ir
    │   ├── hello.ir
    │   └── typed.ir
    ├── run-pass/                # 编译+运行测试
    └── compile-fail/            # 编译错误测试
```

### Stage2 (Dast 实现) - 模块化版本

```
compiler/stage2/
├── frontend/                # 前端
│   ├── token.dast
│   ├── lexer.dast
│   ├── ast.dast
│   ├── parser.dast
│   └── typecheck.dast
├── middle/                  # 中端
│   ├── compile.dast
│   └── ir/                  # Stage2 IR (可扩展)
├── backend/                 # 后端
│   ├── interp/              # IR 解释器
│   └── codegen-c/           # C 代码生成
├── driver/                  # 驱动程序
│   └── main.dast
├── stdlib/                  # 标准库
└── tests/                   # 测试
```

## IR v0 模块共享设计

### 设计目标

IR v0 定义在 `stage1/irv0/` 中，可被多个阶段共用：

1. **Stage0**：Go 代码中有独立的 IR 实现（`internal/ir/`）
2. **Stage1**：直接使用 `stage1/irv0/` 源文件
3. **Stage2**：可以导入 `stage1/irv0/` 作为模块

### 使用方式

**Stage0/Stage1**（不支持模块，直接列文件）：

```bash
# Stage0 运行 Stage1 编译器
dast-stage0 run \
  compiler/bootstrap/stage1/compiler/ \
  compiler/bootstrap/stage1/irv0/ \
  compiler/bootstrap/stage1/interp/ \
  -- run test.dast

# Stage0 运行 IR 解释器
dast-stage0 run \
  compiler/bootstrap/stage1/irv0/ \
  compiler/bootstrap/stage1/interp/ \
  -- ir-run tests/ir/hello.ir
```

**Stage2**（支持模块，可以 import）：

```dast
// stage2/backend/irv0_output.dast
import "../../bootstrap/stage1/irv0" as irv0

fn emit_ir(prog: irv0::IrProgram) -> String {
    return irv0::ir_program_format(prog)
}
```

### 目录参数支持

Stage0 已支持目录作为参数，自动展开为该目录下所有 `.dast` 文件：

```bash
# 以下两种方式等价
dast-stage0 run compiler/bootstrap/stage1/compiler/

dast-stage0 run \
  compiler/bootstrap/stage1/compiler/token.dast \
  compiler/bootstrap/stage1/compiler/lexer.dast \
  ...
```

### Stage0 build 输出

Stage0 的 `build` 默认输出**可执行文件**（通过 **IR → QBE → cc** 流水线生成）；仅在显式指定 `--emit-ir`/`--emit-qbe` 时输出 IR/QBE：

```bash
dast-stage0 build src/ -o app.out
dast-stage0 build --emit-ir src/ -o app.ir
dast-stage0 build --emit-qbe src/ -o app.qbe
```

> 依赖本机 `qbe` 与 `cc/clang`（用于从 QBE 生成可执行文件）。

## IR v0 静态类型

新版 IR v0 采用静态类型，详见 [irv0-newspec.md](./irv0-newspec.md)。

关键特性：
- 每个 temp 有类型：`t0: String = const "Hello"`
- 支持局部变量声明：`var counter: int`
- 生成强类型 C 代码

## Stage0 语言特性（扩展版）

当前 Stage0 实现已超出最初最小子集，包含模块/依赖/测试、宏系统、泛型/trait/type alias 等能力。
详见：[Stage0 语言特性（扩展版）](./22-stage0-language.md)

## 编译流程

### 日常开发

```bash
# 使用 Stage0 编译并运行 Stage1
make run-stage1

# 使用 Stage0 → Stage1 编译并运行 Stage2
make run-stage2

# 运行测试
make test-stage2
```

### 二进制构建流程

```
Dast 源码
    ↓ (Stage1 编译器)
IR v0 (静态类型)
    ↓ (C 代码生成器)
C 源码
    ↓ (GCC/Clang)
可执行文件
```

## Makefile 配置

```makefile
# Stage1 目录
STAGE1_FILES := compiler/bootstrap/stage1/compiler/ \
                compiler/bootstrap/stage1/irv0/ \
                compiler/bootstrap/stage1/interp/

# Stage2 文件（使用 find）
STAGE2_FILES := $(shell find compiler/stage2 -name '*.dast' ...)

# 运行 Stage1
run-stage1:
    $(STAGE0_BIN) run $(STAGE1_FILES) -- $(ARGS)

# 测试
test-stage1:
    @for f in compiler/bootstrap/stage1/tests/ir/*.ir; do \
        $(STAGE0_BIN) run $(STAGE1_FILES) -- ir-run $$f; \
    done
```

## 自举验证

```bash
# Stage1 编译 Stage2 源码 → 生成 IR
./dast-stage0 run stage1/ -- ir stage2/ > stage2.ir

# IR 解释执行
./dast-stage0 run stage1/ -- ir-run stage2.ir

# 或者生成 C 代码编译执行
./dast-stage0 run stage1/ -- ir-c stage2.ir > stage2.c
gcc -o stage2 stage2.c c_runtime.c
./stage2
```

## 关键设计原则

1. **永久保留 Stage0**: Bootstrap 编译器永远可用
2. **IR v0 共享**: Stage1 的 IR 模块可被 Stage2 复用
3. **静态类型 IR**: 支持强类型 C 代码生成
4. **目录参数**: 简化命令行调用

## 参考

- [IR v0 新规范](./irv0-newspec.md)
- [Bootstrapping a Compiler](https://en.wikipedia.org/wiki/Bootstrapping_(compilers))
