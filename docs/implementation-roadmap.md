# 实现路线图

> 目标：以 **IR v0 稳定核心** 为基座，逐步完成自举与完整工具链。

## Stage 0 — Bootstrap 编译器（Go）

**目标**：最小可运行编译器 + IR v0 + 解释器  
**语言子集**：基础类型、struct/enum、函数、match、借用/引用、数组；无泛型/宏/async/comptime  
**输出**：IR v0（稳定）  

完成标准：
- stage0 可运行 stage2（bootstrap）
- 基础示例可以运行（examples）

## Stage 2 — 完整语言规范自举

**目标**：实现完整语言规范；输出 IR v0  
**新增能力**：泛型、trait/comptime、宏系统、async/await、模块与包、标准库扩展  
**输出**：降解到 IR v0

完成标准：
- stage2 编译器使用**完整规范**实现  
- stage2 可以编译 stage2（最新规范自举）
- stage2 可输出单文件 IR v0，供 stage0 运行

## Stage 3 — 工具链与优化

**目标**：工程化完善 + 性能优化  
**范围**：fmt/lint/LSP、增量编译、优化器、调试符号、跨平台后端、包管理增强  

完成标准：
- 工具链稳定（fmt/lint/LSP/包管理）
- 编译性能与运行性能可满足生产需求

---

## IR 稳定性约束（适用于所有阶段）

- stage0 只支持 **IR v0**。  
- stage2 应尽可能将新语法**前端降解**为 v0 IR。  

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
