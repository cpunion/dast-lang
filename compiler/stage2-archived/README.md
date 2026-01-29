# Stage2 (Dast)

Stage2 is the **full-spec** compiler, still lowering to IR v0 for bootstrapping.

## Bootstrap chain

```
stage0 (Go) -> stage2 (Dast, full spec) -> IR v0
```

## Module loading (current)

- 同目录下的 `*.dast` 视为同一模块（无需 `mod.dast`）
- `import "foo"` 解析为目录 `foo/`（相对于包根）
- `import "./foo"` / `import "../foo"` 相对于当前文件所在目录
- `import "a.b"` 等价于 `import "a/b"`
- `mod name` 等价于 `import name`（相对于当前目录）

包根由 `dast.toml` 决定，若存在 `src/` 则以 `src/` 作为代码根目录。

## IR verify

```bash
./compiler/stage0/dast-stage0 run $(find compiler/stage2 -name '*.dast' -not -path 'compiler/stage2/tests/*' | sort) -- ir-verify /tmp/hello.ir
```

## IR opt

```bash
./compiler/stage0/dast-stage0 run $(find compiler/stage2 -name '*.dast' -not -path 'compiler/stage2/tests/*' | sort) -- ir-opt /tmp/hello.ir
```
