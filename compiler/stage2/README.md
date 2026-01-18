# Stage2 (Dast)

Stage2 is the **full-spec** compiler, still lowering to IR v0 for bootstrapping.

## Bootstrap chain

```
stage0 (Go) -> stage1 (Dast, multi-file) -> stage2 (Dast, full spec) -> IR v0
```

Stage1 can compile stage2 into a **single IR v0 file**. That IR file can replace stage1 as a snapshot so stage0 can always run stage1, even after stage2 syntax evolves.

## Build Stage1 IR snapshot from Stage2

From repo root:

```bash
make build-stage1-ir

# run stage1 snapshot
./compiler/bootstrap/stage0/dast-stage0 ir-run compiler/bootstrap/stage1/stage1.ir -- run examples/hello.dast
```

