# Dast 编译器 Bootstrap 开发计划

## 总体架构

当前自举链路简化为两阶段：

```
Stage0 (Go)  ──▶  Stage2 (Dast)
```

- **Stage0**：Go 编写的 bootstrap 编译器/运行时（IR v0 + 解释器）
- **Stage2**：Dast 编写的编译器（当前以 QBE 后端为主）

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
- 逐步扩展语言覆盖面
- 当前直接输出 QBE（不输出 IR v0）

目录：
```
compiler/stage2/
```

## IR v0

- 规范：`docs/19-ir-spec.md`
- 实现：Stage0 `compiler/stage0/internal/ir/`

## 测试与验证

- `make test-stage0`
- `make test-stage2`
- `make test-ir`

## 关键约束

1. **Stage0 永久可用**
2. **IR v0 稳定（Stage0 依赖）**
3. **Stage2 当前不输出 IR v0**
