# Stage0 (Go)

Stage0 is the bootstrap compiler/runtime written in Go.

## Build

```bash
cd compiler/bootstrap/stage0

go build -o dast-stage0 ./cmd/dast
```

## Run examples

```bash
./dast-stage0 run examples/hello/main.dast
./dast-stage0 run examples/struct_enum/main.dast
./dast-stage0 run examples/array/main.dast
```

## IR v0

```bash
# compile to IR (stdout)
./dast-stage0 ir examples/hello/main.dast > /tmp/hello.ir

# run IR
./dast-stage0 ir-run /tmp/hello.ir

# verify IR
./dast-stage0 ir-verify /tmp/hello.ir

# optimize IR
./dast-stage0 ir-opt /tmp/hello.ir
```

## Run Stage1 (Dast)

From repo root:

```bash
./compiler/bootstrap/stage0/dast-stage0 run compiler/bootstrap/stage1/main.dast compiler/bootstrap/stage1/token.dast compiler/bootstrap/stage1/lexer.dast compiler/bootstrap/stage1/ast.dast compiler/bootstrap/stage1/parser.dast -- compiler/bootstrap/stage1/main.dast
```
