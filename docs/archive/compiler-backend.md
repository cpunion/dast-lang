# 编译后端架构

## 设计原则

1. **多后端架构**: 可切换不同代码生成器
2. **统一 IR**: 自定义 SSA 中间表示
3. **按需选择**: 开发/生产/嵌入式场景使用不同后端

---

## 架构总览

```
    源码 (.dast)
         │
         ▼
    ┌─────────┐
    │  前端    │  Parser + AST + 类型检查
    └────┬────┘
         │
         ▼
    ┌─────────┐
    │ Dast IR │  SSA 中间表示
    └────┬────┘
         │
    ┌────┴────┬──────────┬──────────┐
    ▼         ▼          ▼          ▼
┌───────┐ ┌────────┐ ┌───────┐ ┌────────┐
│  QBE  │ │Cranelift│ │ LLVM │ │字节码  │
└───┬───┘ └───┬────┘ └───┬───┘ └───┬────┘
    │         │          │         │
    ▼         ▼          ▼         ▼
 Native    WASM+      全平台     虚拟机
(快速编译) Native    (极致优化) (热更新)
```

---

## 后端对比

| 后端 | 优势 | 适用场景 |
|------|------|---------|
| **QBE** | ~10K 行 C，编译极快 | 开发迭代、嵌入式 |
| **Cranelift** | WASM 原生、Rust 安全 | 生产 WASM、JIT |
| **LLVM** | 极致优化、全平台 | 性能关键、GPU |
| **字节码** | 即时执行、易热更 | 开发调试、插件 |

---

## 平台与后端映射

| 目标 | 推荐后端 |
|------|---------|
| 开发调试 | 字节码 / QBE |
| Linux/macOS/Windows | Cranelift / LLVM |
| WASM | Cranelift |
| 嵌入式裸机 | QBE / LLVM |
| iOS/Android | Cranelift / LLVM |

---

## 编译命令

```bash
# 默认 (Cranelift)
$ dast build

# 指定后端
$ dast build --backend=qbe      # 快速编译
$ dast build --backend=llvm     # 极致优化
$ dast build --backend=bytecode # 解释执行

# 混合模式
$ dast build --backend=cranelift \
    --bytecode=plugins/*        # 插件用字节码
```

---

## 待讨论

- [ ] 自定义 IR 指令集设计
- [ ] 后端切换的 API 稳定性
- [ ] 调试信息在各后端的支持
- [ ] 增量编译策略
