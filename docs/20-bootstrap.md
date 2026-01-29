# Dast 编译器自举规划

## 概述

Dast 采用**分阶段自举**策略：

- **Stage0**：Go 实现的 bootstrap 编译器/运行时（IR v0 + 解释器）
- **Stage2**：Dast 实现的完整编译器（自举目标）
- **Stage3**：工具链与优化（fmt/lint/LSP/opt）

```
Stage 0 (Go)                 Stage 2 (Dast)
┌──────────────────────┐     ┌──────────────────────┐
│  compiler/stage0/    │  →  │  compiler/stage2/    │
└──────────────────────┘     └──────────────────────┘
         运行 Stage2                   完整编译器
```

## 目录结构

### Stage0（Go）

```
compiler/stage0/
├── cmd/dast/              # 主入口
├── internal/              # AST/Parser/Typecheck/IR/Interp
├── runtime/               # 运行时（C）
├── stdlib/                # 标准库（Stage0）
├── tests/                 # Stage0 测试
└── dast-stage0            # 编译后的二进制
```

### Stage2（Dast）

```
compiler/stage2/
├── frontend/              # 词法/语法/AST/类型检查
├── middle/                # IR/中端
├── backend/               # 解释器/QBE/C 后端
├── driver/                # 入口
├── stdlib/                # 标准库
└── tests/                 # 测试
```

## IR v0

- 规范：`docs/19-ir-spec.md`  
- 实现：
  - Stage0：`compiler/stage0/internal/ir/`
  - Stage2：`compiler/stage2/middle/ir/`

Stage2 以 IR v0 为稳定输出目标，Stage0 保持对 IR v0 的完整支持。

## 使用方式

```bash
# 运行 Stage2（由 Stage0 驱动）
make test-stage2

# Stage0 编译/运行
./compiler/stage0/dast-stage0 build src/ -o app.out
./compiler/stage0/dast-stage0 run main.dast
```

## 关键设计原则

1. **永久保留 Stage0**：bootstrap 编译器永远可用  
2. **IR v0 稳定**：Stage2 目标输出 IR v0  
3. **跨阶段一致**：Stage0/Stage2 的测试与行为尽量一致
