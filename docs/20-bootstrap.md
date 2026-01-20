# Dast 编译器自举规划

## 概述

Dast 采用**分阶段自举**策略，使用 Rust 编写的 Stage0 bootstrap 编译器来编译用 Dast 编写的 Stage1 编译器，最终实现完全自举。

## 自举阶段

```
Stage 0 (Bootstrap)          Stage 1                    Stage 2+
┌────────────────────┐      ┌────────────────────┐      ┌────────────────────┐
│  Rust 实现         │  →   │  Dast 实现          │  →   │  Dast 实现          │
│  dastlang-stage0/  │      │  stage1/            │      │  (自编译)           │
│  stage0/           │      │                    │      │                    │
└────────────────────┘      └────────────────────┘      └────────────────────┘
     编译 Dast 代码              用 Stage0 编译            用 Stage1 编译
```

## 仓库结构

```
dastlang/                    # 语言规范与文档
├── docs/                    # 设计文档
└── README.md

dastlang-stage0/             # 编译器实现
├── stage0/                  # Rust bootstrap 编译器 (永久保留)
│   ├── src/
│   ├── examples/
│   └── Cargo.toml
└── stage1/                  # Dast 自举编译器
    ├── lexer.dast
    ├── parser.dast
    └── ...
```

## 三阶段验证 (Triple Test)

每次编译器发布前必须通过三阶段验证：

```bash
# Stage1 编译自身 → Stage2
./stage0/dastc stage1/ -o stage2

# Stage2 编译 Stage1 源码 → Stage3
./stage2 stage1/ -o stage3

# 验证 Stage2 和 Stage3 二进制一致
diff stage2 stage3  # 必须相同！
```

这确保编译器的正确性：如果 Stage2 能正确编译自身产生相同结果，则编译器是可信的。

## 自举路线图

### Phase 1: Stage0 完善 ✅
- [x] 词法分析器
- [x] 递归下降解析器
- [x] 类型推断
- [x] IR 生成
- [x] 解释器执行
- [x] 结构体支持
- [x] 动态数组 (Vec)

### Phase 2: Stage1 核心
- [x] 词法分析 (lexer.dast)
- [x] 表达式解析 (parser.dast)
- [x] 递归下降 + 优先级 (compiler.dast)
- [x] 变量绑定 (mini.dast)
- [x] 函数调用 (full.dast)
- [ ] 完整类型检查
- [ ] IR 生成
- [ ] 代码输出

### Phase 3: 自举验证
- [ ] Stage1 编译 Stage1 → Stage2
- [ ] 三阶段验证通过
- [ ] 性能基准测试

### Phase 4: 生产就绪
- [ ] LLVM 后端
- [ ] 标准库
- [ ] 包管理器

## 开发工作流

### 日常开发
```bash
# 使用 Stage0 编译和运行 Stage1 代码
cd dastlang-stage0/stage0
cargo run -- run ../stage1/xxx.dast
```

### 添加新语言特性
1. 先在 Stage0 (Rust) 中实现该特性
2. 验证现有 Stage1 代码仍能编译运行
3. 在 Stage1 中使用新特性
4. 运行三阶段验证

### 发布流程
1. 冻结 Stage1 代码变更
2. 运行三阶段验证
3. 更新版本号
4. 标记 Git tag

## 关键设计原则

1. **永久保留 Stage0**: Bootstrap 编译器永远可用，确保任何人能从源码重建
2. **增量添加特性**: 先在 Stage0 测试，再在 Stage1 使用
3. **三阶段验证**: 每次发布必须自编译验证
4. **版本锁定**: 记录每个 Stage1 版本对应的 Stage0 提交

## 回滚策略

如果 Stage1 出现严重 bug：

```bash
# 1. 使用 Stage0 重新编译已知良好的 Stage1 版本
git checkout v1.0.0 -- stage1/
cargo run -- build stage1/ -o dastc-fixed

# 2. 用修复的编译器继续开发
./dastc-fixed build stage1-dev/ -o dastc-new
```

## 参考

- [Bootstrapping a Compiler](https://en.wikipedia.org/wiki/Bootstrapping_(compilers))
- [Trusting Trust Attack](https://www.cs.cmu.edu/~rdriley/487/papers/Thompson_1984_ResearchingATrustedCompiler.pdf)
- Go/Rust/Zig 自举实践
