# Dast 编译器 Bootstrap 开发计划

## 总体架构

Dast 采用三阶段自举策略,每个阶段都有明确的职责和边界:

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Bootstrap Pipeline                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  Stage0 (Go)          Stage1 (Dast)         Stage2 (Dast)           │
│  ┌──────────┐         ┌──────────┐          ┌──────────┐            │
│  │  Lexer   │────────▶│  Lexer   │─────────▶│  Lexer   │            │
│  │  Parser  │  编译   │  Parser  │   编译   │  Parser  │            │
│  │ Compiler │         │ Compiler │          │ Compiler │            │
│  │ IR v0    │         │ IR v0    │          │ IR v2    │            │
│  │ Interp   │         │ Interp   │          │ Backends │            │
│  └──────────┘         └──────────┘          └──────────┘            │
│      ↓                     ↓                      ↓                  │
│   解释执行               解释执行            多后端编译              │
│   Stage1                Stage2              (IR v0/Interp/C)        │
└─────────────────────────────────────────────────────────────────────┘
```

## Stage0: Go Bootstrap 编译器

**状态**: ✅ **已完成** (Commit 27b0e7b)

### 职责
- 编译并解释执行 Stage1 的 Dast 代码
- 提供完整的 IR v0 支持(生成、解析、优化、解释)
- 作为整个 bootstrap 流程的起点

### 关键特性
- ✅ 完整的 Dast 语法支持
- ✅ IR v0 生成与常量内联
- ✅ IR 解释器
- ✅ 结构体、枚举、引用支持
- ✅ 基础类型检查

### 目录结构
```
compiler/bootstrap/stage0/
├── cmd/dast/main.go           # CLI 入口
├── internal/
│   ├── ast/                   # AST 定义
│   ├── lexer/                 # 词法分析
│   ├── parser/                # 语法分析
│   ├── compile/               # AST → IR v0
│   ├── ir/                    # IR v0 (定义/解析/优化)
│   ├── interp/                # IR v0 解释器
│   ├── typecheck/             # 类型检查
│   └── diag/                  # 诊断信息
├── examples/                  # 示例程序
└── tests/                     # 测试套件
```

## Stage1: Dast 实现的编译器

**状态**: 🔄 **开发中**

### 职责
- 用 Dast 重新实现 Stage0 的功能
- 编译并解释执行 Stage2
- 验证 Dast 语言的自举能力

### 关键约束

#### 1. IR v0 模块的双重用途

**问题**: `irv0/` 模块需要同时支持两种使用方式:
- **Stage1 内部**: 通过 Makefile 文件列表方式编译(所有符号全局唯一)
- **Stage2 依赖**: 作为独立包通过 `dast.toml` 导入

**解决方案**:

```
compiler/bootstrap/stage1/
├── irv0/                          # IR v0 模块
│   ├── dast.toml                 # 包配置(供 Stage2 使用)
│   ├── types.dast                # IR 数据结构
│   ├── parser.dast               # IR 解析
│   ├── format.dast               # IR 格式化
│   └── validate.dast             # IR 验证
│
├── compiler/                      # 编译器模块
│   ├── main.dast                 # 入口(run/ir/build 命令)
│   ├── token.dast                # Token 定义
│   ├── lexer.dast                # 词法分析
│   ├── ast.dast                  # AST 定义
│   ├── parser.dast               # 语法分析
│   ├── typecheck.dast            # 类型检查
│   └── compile.dast              # AST → IR v0
│
├── interp/                        # 解释器模块
│   ├── runtime.dast              # 运行时
│   ├── eval.dast                 # 指令求值
│   └── interp.dast               # 解释器主逻辑
│
└── Makefile                       # 构建配置
```

**Makefile 示例**:
```makefile
# Stage1 编译 - 文件列表方式
STAGE1_FILES = \
    irv0/types.dast \
    irv0/parser.dast \
    irv0/format.dast \
    irv0/validate.dast \
    compiler/token.dast \
    compiler/lexer.dast \
    compiler/ast.dast \
    compiler/parser.dast \
    compiler/typecheck.dast \
    compiler/compile.dast \
    compiler/main.dast \
    interp/runtime.dast \
    interp/eval.dast \
    interp/interp.dast

stage1: $(STAGE1_FILES)
	../stage0/dast-stage0 ir $(STAGE1_FILES) > stage1.ir
	../stage0/dast-stage0 ir-run stage1.ir
```

**符号命名规范**:
- 所有顶层函数/类型必须全局唯一
- 使用模块前缀: `IrV0_Program`, `Compiler_parse`, `Interp_eval`
- 避免符号冲突

#### 2. dast.toml 配置

**irv0/dast.toml**:
```toml
[package]
name = "irv0"
version = "0.1.0"

[dependencies]
# 无外部依赖
```

### 开发计划

**Phase 1: IR v0 模块** (优先级最高)
- [ ] `irv0/types.dast` - IR 数据结构
- [ ] `irv0/parser.dast` - IR 文本解析
- [ ] `irv0/format.dast` - IR 格式化输出
- [ ] `irv0/validate.dast` - IR 验证
- [ ] 创建 `irv0/dast.toml`

**Phase 2: 编译器核心**
- [ ] `compiler/token.dast` - Token 定义
- [ ] `compiler/lexer.dast` - 词法分析
- [ ] `compiler/ast.dast` - AST 定义
- [ ] `compiler/parser.dast` - 语法分析
- [ ] `compiler/compile.dast` - AST → IR v0
- [ ] `compiler/main.dast` - CLI 入口

**Phase 3: 解释器**
- [ ] `interp/runtime.dast` - 运行时值定义
- [ ] `interp/eval.dast` - 指令求值
- [ ] `interp/interp.dast` - 解释器主逻辑

**Phase 4: 集成测试**
- [ ] 编译 Stage2 的简单模块
- [ ] 运行 Stage2 的基础测试
- [ ] 验证 IR v0 兼容性

## Stage2: 自举编译器

**状态**: 🔄 **开发中**

### 职责
- 完整的 Dast 编译器实现
- 支持多种后端
- 模块化架构,支持扩展

### 关键特性

#### 1. IR 版本策略

**IR v2** (Stage2 内部使用):
- 更丰富的类型系统
- 更多优化机会
- SSA 形式(可选)

**IR v0** (后端目标):
- 通过 lowering 从 IR v2 转换
- 保持与 Stage0/Stage1 兼容
- 作为稳定的编译目标

#### 2. 多后端架构

```
Stage2 Frontend → IR v2 → Lowering → IR v0 → Backends
                                              ├─ IR v0 输出
                                              ├─ Interp (解释执行)
                                              └─ C Codegen
```

**Backend 1: IR v0 输出**
- 生成 `.ir` 文件
- 可由 Stage0/Stage1 解释执行
- 用于调试和验证

**Backend 2: Interpreter**
- 直接解释执行 IR v0
- 快速原型开发
- 交互式 REPL

**Backend 3: C Codegen**
- IR v0 → C 代码
- 生成独立可执行文件
- 最佳性能

### 目录结构

```
compiler/stage2/
├── frontend/                      # 前端
│   ├── token.dast
│   ├── lexer.dast
│   ├── ast.dast
│   ├── parser.dast
│   └── typecheck.dast
│
├── middle/                        # 中端
│   ├── compile.dast              # AST → IR v2
│   ├── ir/                       # IR v2 定义
│   │   ├── types.dast
│   │   ├── builder.dast
│   │   └── opt.dast
│   └── lower.dast                # IR v2 → IR v0
│
├── backend/                       # 后端
│   ├── irv0/                     # IR v0 输出
│   │   └── emit.dast
│   ├── interp/                   # 解释器
│   │   ├── runtime.dast
│   │   └── eval.dast
│   └── codegen-c/                # C 代码生成
│       ├── codegen-c.dast
│       ├── c_runtime.h
│       └── c_runtime.c
│
├── driver/                        # 驱动程序
│   └── main.dast
│
├── stdlib/                        # 标准库
│   ├── core.dast
│   ├── io.dast
│   └── ...
│
├── dast.toml                     # 包配置
└── tests/                        # 测试
```

### 依赖关系

**dast.toml**:
```toml
[package]
name = "dast-compiler"
version = "2.0.0"

[dependencies]
irv0 = { path = "../bootstrap/stage1/irv0" }
```

### 开发计划

**Phase 1: 前端复用**
- [ ] 从 Stage1 迁移前端代码
- [ ] 增强类型系统
- [ ] 模块系统支持

**Phase 2: IR v2 设计**
- [ ] 定义 IR v2 数据结构
- [ ] 实现 IR v2 builder
- [ ] 基础优化 pass

**Phase 3: Lowering**
- [ ] IR v2 → IR v0 转换
- [ ] 类型擦除
- [ ] 控制流简化

**Phase 4: 后端实现**
- [ ] IR v0 输出后端
- [ ] 解释器后端
- [ ] C 代码生成后端

**Phase 5: 标准库**
- [ ] 核心库
- [ ] I/O 库
- [ ] 集合库

## 构建流程

### 完整 Bootstrap 流程

```bash
# 1. 构建 Stage0 (Go)
cd compiler/bootstrap/stage0
go build -o dast-stage0 ./cmd/dast

# 2. 使用 Stage0 编译 Stage1
cd ../stage1
make stage1  # 生成 stage1.ir

# 3. 使用 Stage0 运行 Stage1 编译 Stage2
../stage0/dast-stage0 ir-run stage1.ir -- \
    compile ../../stage2 -o stage2.ir

# 4. 使用 Stage0 运行 Stage2 (IR v0 后端)
../stage0/dast-stage0 ir-run stage2.ir -- \
    compile <source> -o output.ir

# 5. 使用 Stage2 (C 后端) 生成独立可执行文件
../stage0/dast-stage0 ir-run stage2.ir -- \
    compile <source> --backend=c -o output
gcc output.c -o output
./output
```

## 测试策略

### Stage0 测试
- ✅ 单元测试 (Go)
- ✅ IR 生成测试
- ✅ 解释器测试

### Stage1 测试
- [ ] 自编译测试 (编译自己)
- [ ] IR v0 兼容性测试
- [ ] 与 Stage0 输出对比

### Stage2 测试
- [ ] 自编译测试
- [ ] 多后端一致性测试
- [ ] 性能基准测试
- [ ] 标准库测试

## 里程碑

### M1: Stage0 完成 ✅
- [x] 完整 Dast 语法支持
- [x] IR v0 生成与常量内联
- [x] IR 解释器
- [x] 测试覆盖

### M2: Stage1 IR v0 模块
- [ ] IR v0 数据结构
- [ ] IR v0 解析器
- [ ] IR v0 格式化
- [ ] dast.toml 配置

### M3: Stage1 编译器
- [ ] 词法/语法分析
- [ ] AST → IR v0 编译
- [ ] 解释器实现
- [ ] 能编译 Stage2 简单模块

### M4: Stage2 基础架构
- [ ] 前端实现
- [ ] IR v2 设计
- [ ] IR v2 → IR v0 lowering
- [ ] IR v0 输出后端

### M5: Stage2 多后端
- [ ] 解释器后端
- [ ] C 代码生成后端
- [ ] 后端测试

### M6: 自举完成
- [ ] Stage2 能编译自己
- [ ] 生成的编译器能再次编译自己
- [ ] 性能优化
- [ ] 文档完善

## 当前状态

- ✅ **Stage0**: 完成 (Commit 27b0e7b)
- 🔄 **Stage1**: IR v0 模块开发中
- 🔄 **Stage2**: 架构设计中

## 下一步行动

1. **立即**: 完成 Stage1 的 `irv0/types.dast`
2. **本周**: 完成 IR v0 模块全部文件
3. **本月**: 完成 Stage1 编译器核心
4. **下月**: 开始 Stage2 开发
