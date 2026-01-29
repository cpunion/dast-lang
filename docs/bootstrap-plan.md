# Dast 编译器 Bootstrap 开发计划

## 总体架构

当前自举链路简化为两阶段：

```
Stage0 (Go)  ──▶  Stage2 (Dast)
```

- **Stage0**：Go 编写的 bootstrap 编译器/运行时（IR v0 + 解释器）
- **Stage2**：Dast 编写的完整编译器（自举目标）

## Stage0：Go Bootstrap 编译器

职责：
- 支持 IR v0（生成、解析、验证、解释执行）
- 作为 Stage2 的运行/验证基座

目录：
```
compiler/stage0/
```

## Stage2：Dast 编译器

职责：
- 实现完整语言规范
- 输出 IR v0，供 Stage0 运行/验证

目录：
```
compiler/stage2/
```

## IR v0

- 规范：`docs/19-ir-spec.md`
- 实现：Stage0 `compiler/stage0/internal/ir/`，Stage2 `compiler/stage2/middle/ir/`

## 测试与验证

- `make test-stage0`
- `make test-stage2`
- `make test-ir`

## 关键约束

1. **Stage0 永久可用**
2. **IR v0 稳定**
3. **Stage2 以 IR v0 作为稳定输出**
