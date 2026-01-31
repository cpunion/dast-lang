# Dast 编程语言设计概览

> 本文档记录 Dast 语言的设计讨论与决策

## 设计目标

Dast 是一门**编译型系统编程语言**，目标是:

1. **极致的平台覆盖**: 从嵌入式裸机到主机、WASM、Web、移动端
2. **极致的二进制体积**: 边缘计算、嵌入式、WASM 场景下极小输出
3. **深度图形支持**: OpenGL、DirectX、Vulkan、Metal、WebGPU、GPU 计算
4. **模块级热更新**: 类似动态语言的开发体验，编译型语言的运行性能
5. **高安全性**: ~99% 编译期安全，接近 Rust，无需生命周期标注

## 核心设计决策

| 领域 | 决策 |
|------|------|
| 泛型语法 | `[T]` (简洁) |
| 属性语法 | `@attr` (统一) |
| 内存管理 | RAII、无 GC、无引用计数 |
| 借用检查 | 完整实现，无需生命周期标注 |
| 线程安全 | Sendable/Shareable |
| 测试框架 | Go 风格 (`*_test.dast`, `test_*` 前缀) |
| 异步模型 | async/await Pull 模式 |
| 宏系统 | Comptime + AST 宏 |
| FFI | C FFI 核心，WASM 互操作 |
| 工具链 | 统一 `dast` CLI（build/run/test）；LSP 为 stage3 原型 |

## 安全性定位

```
C++: 20% → Dast: 99% → Rust: 100%
```

| 安全领域 | 方案 |
|---------|------|
| 借用检查 | 完整 (Rust 级别) |
| 生命周期 | 自动推断 (无需 `'a`) |
| 线程安全 | Sendable/Shareable |
| 内存安全 | RAII + Drop |

## 文档结构

### 核心设计文档 (01-18)

| 编号 | 文档 | 主题 |
|------|------|------|
| 01 | [type-system.md](01-type-system.md) | 类型系统 |
| 02 | [error-handling.md](02-error-handling.md) | 错误处理 |
| 03 | [module-package.md](03-module-package.md) | 模块系统 |
| 04 | [generics-comptime.md](04-generics-comptime.md) | 泛型与 Comptime |
| 05 | [comptime-detailed.md](05-comptime-detailed.md) | Comptime 详解 |
| 06 | [advanced-generics.md](06-advanced-generics.md) | 高级泛型 |
| 07 | [memory-management.md](07-memory-management.md) | 内存管理 |
| 08 | [thread-safety.md](08-thread-safety.md) | 线程安全 |
| 09 | [async-model.md](09-async-model.md) | 异步模型 |
| 10 | [macro-system.md](10-macro-system.md) | 宏系统 |
| 11 | [package-management.md](11-package-management.md) | 包管理 |
| 12 | [testing-framework.md](12-testing-framework.md) | 测试框架 |
| 13 | [standard-library.md](13-standard-library.md) | 标准库 |
| 14 | [syntax-details.md](14-syntax-details.md) | 语法细节 |
| 15 | [toolchain.md](15-toolchain.md) | 工具链 |
| 16 | [platform-support.md](16-platform-support.md) | 平台支持 |
| 17 | [ffi-interop.md](17-ffi-interop.md) | FFI 与互操作 |
| 18 | [hot-reload.md](18-hot-reload.md) | 热更新 |
| 19 | [ir-spec.md](19-ir-spec.md) | IR 规范（稳定核心） |

### 早期讨论文档 (archive/)

早期设计讨论、对比分析和高级特性文档已移入 `archive/` 目录，作为设计演进的历史记录保留。

## 实现计划

详见 [实现路线图](implementation-roadmap.md)

**核心策略**: 渐进式自举

- **Stage 0** (Go): Bootstrap 编译器（最小可用子集）
- **Stage 2** (Dast): 逐步覆盖完整语言规范（当前仍为子集）
- **Stage 3**: 工具链与优化（LSP/fmt/lint/优化等原型阶段）

**目标**: 以 Stage0 的 IR v0 稳定基座为支撑，持续扩展 Stage2 覆盖面与工具链能力（Stage2 当前直接生成 QBE）

## 设计完成度

✅ **核心语言设计**: 100%
✅ **标准库设计**: 100%
✅ **工具链设计**: 100%
✅ **平台支持设计**: 100%
✅ **实现计划**: 100%

**状态**: 设计阶段完成，准备进入实现阶段
