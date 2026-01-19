# Stage1 (Dast)

Stage1 is a Dast implementation of the stage0 compiler (frontend + IR v0 + interpreter). It is executed by the Go stage0 runtime.

## Run with stage0

From repo root:

```bash
# build stage0
( cd compiler/bootstrap/stage0 && go build -o dast-stage0 ./cmd/dast )

# run stage1 on a program (multi-file)
./compiler/bootstrap/stage0/dast-stage0 run compiler/bootstrap/stage1/*.dast -- run compiler/bootstrap/stage0/examples/hello/main.dast
```

## IR v0

```bash
# compile to IR (stdout)
./compiler/bootstrap/stage0/dast-stage0 run compiler/bootstrap/stage1/*.dast -- ir compiler/bootstrap/stage0/examples/hello/main.dast > /tmp/hello.ir

# run IR
./compiler/bootstrap/stage0/dast-stage0 run compiler/bootstrap/stage1/*.dast -- ir-run /tmp/hello.ir

# verify IR
./compiler/bootstrap/stage0/dast-stage0 run compiler/bootstrap/stage1/*.dast -- ir-verify /tmp/hello.ir

# optimize IR
./compiler/bootstrap/stage0/dast-stage0 run compiler/bootstrap/stage1/*.dast -- ir-opt /tmp/hello.ir
```

Current stage1 supports the same Stage0 feature set: multi-file single module, `struct/enum/const/impl/self`, arrays/refs, `if/while/match`, implicit tail return, IR v0 lowering, and IR interpreter builtins.
