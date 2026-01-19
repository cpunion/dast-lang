# Stage2 (Dast)

Stage2 is the **full-spec** compiler, still lowering to IR v0 for bootstrapping.

## Bootstrap chain

```
stage0 (Go) -> stage1 (Dast, multi-file) -> stage2 (Dast, full spec) -> IR v0
```

Stage1 can compile stage2 into a **single IR v0 file**. That IR file can replace stage1 as a snapshot so stage0 can always run stage1, even after stage2 syntax evolves.

## Module loading (current)

- 同目录下的 `*.dast` 视为同一模块（无需 `mod.dast`）
- `import "foo"` 解析为目录 `foo/`（相对于包根）
- `import "./foo"` / `import "../foo"` 相对于当前文件所在目录
- `import "a.b"` 等价于 `import "a/b"`
- `mod name` 等价于 `import name`（相对于当前目录）

包根由 `dast.toml` 决定，若存在 `src/` 则以 `src/` 作为代码根目录。

## Build Stage1 IR snapshot from Stage2

From repo root:

```bash
make build-stage1-ir

# run stage1 snapshot
./compiler/bootstrap/stage0/dast-stage0 ir-run compiler/bootstrap/stage1/stage1.ir -- run compiler/bootstrap/stage0/examples/hello/main.dast
```

## IR verify

```bash
./compiler/bootstrap/stage0/dast-stage0 run $(find compiler/stage2 -name '*.dast' -not -path 'compiler/stage2/tests/*' | sort) -- ir-verify /tmp/hello.ir
```

## IR opt

```bash
./compiler/bootstrap/stage0/dast-stage0 run $(find compiler/stage2 -name '*.dast' -not -path 'compiler/stage2/tests/*' | sort) -- ir-opt /tmp/hello.ir
```
