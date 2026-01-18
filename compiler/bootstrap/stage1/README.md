# Stage1 (Dast)

Stage1 is a Dast implementation of the stage0 compiler (frontend + IR v0 + interpreter). It is executed by the Go stage0 runtime.

## Run with stage0

From repo root:

```bash
# build stage0
( cd compiler/bootstrap/stage0 && go build ./cmd/dast )

# run stage1 on a program (multi-file)
./compiler/bootstrap/stage0/dast run compiler/bootstrap/stage1/*.dast -- run examples/hello.dast
```

Current stage1 supports the same Stage0 feature set: multi-file single module, `struct/enum/const/impl/self`, arrays/refs, `if/while/match`, implicit tail return, IR v0 lowering, and IR interpreter builtins.
