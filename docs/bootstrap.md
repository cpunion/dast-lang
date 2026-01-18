# Bootstrap Strategy

目标：让 **stage0 只自举一次**，随后通过 IR v0 快照滚动升级 stage2 语法。

## 基本链路

```
stage0 (Go) -> stage1 (Dast 源码，多文件) -> stage2 (Dast，完整规范)
```

## 核心约束

- stage0 始终只支持 **IR v0**。
- stage1 起步为 **stage0 语法子集**，可被 stage0 直接编译运行。
- stage2 实现完整语言规范，但 **输出 IR v0** 以保持可运行。

## 滚动升级策略

当需要升级 stage2 的“编译器源码语法”时：

1) 用 stage1 编译 stage2  
2) 将 stage2 输出的 **单文件 IR v0** 作为 stage1 快照  
3) 由 stage0 直接运行该 IR 快照（等价于新的 stage1）

这样 stage0 永远不变，语法升级通过 **IR v0 快照**滚动推进。

## 示例命令

```
# stage0 编译 stage1（源码多文件）
./compiler/bootstrap/stage0/dast-stage0 run compiler/bootstrap/stage1/*.dast -- run examples/hello.dast

# 用 stage1 编译 stage2，生成 IR v0 快照
make build-stage1-ir

# stage0 运行 stage1 快照（IR v0 单文件）
./compiler/bootstrap/stage0/dast-stage0 ir-run compiler/bootstrap/stage1/stage1.ir -- run examples/hello.dast
```

> 注意：`compiler/bootstrap/stage1/stage1.ir` 是**阶段性快照**，只有在升级 stage2 语法时才生成与更新。

