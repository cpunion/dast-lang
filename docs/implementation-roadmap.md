# 实现路线图

> 目标：以 **Stage0 的 IR v0 稳定核心** 为基座，逐步完成自举与完整工具链（Stage2 当前直接生成 QBE）。

## Stage 0 — Bootstrap 编译器（Go）

**目标**：最小可运行编译器 + IR v0 + 解释器  
**语言子集**：基础类型、struct/enum、函数、match、借用/引用、数组；无泛型/宏/async/comptime  
**输出**：IR v0（稳定）  

完成标准：
- stage0 可运行 stage2（bootstrap）
- 基础示例可以运行（examples）

## Stage 2 — 语言规范渐进式自举

**目标**：逐步扩展语言覆盖面，保持与 Stage0 的可运行一致性  
**新增能力（逐步推进）**：泛型、trait/comptime、宏系统、async/await、模块与包、标准库扩展  
**输出**：当前直接生成 QBE（不输出 IR v0）

完成标准：
- stage2 覆盖核心语言子集并与 stage0 行为一致  
- 完整自举（stage2 编译 stage2）作为阶段性目标

## Stage 3 — 工具链与优化

**目标**：工程化完善 + 性能优化  
**范围**：fmt/lint/LSP、增量编译、优化器、调试符号、跨平台后端、包管理增强  

完成标准：
- 工具链稳定（fmt/lint/LSP/包管理）
- 编译性能与运行性能可满足生产需求

---

## IR 稳定性约束（适用于 Stage0）

- stage0 只支持 **IR v0**。  
- stage2 当前不输出 IR v0。  

## 目录布局（建议）

```
compiler/
  stage0/
  core/
  stage2/
    frontend/
    middle/
    backend/
    driver/
  stage3/
    fmt/
    lint/
    lsp/
    opt/
    incremental/
    debug/
```
