# Stage0 (Go)

Stage0 is the bootstrap compiler/runtime written in Go.

## Build

```bash
cd compiler/bootstrap/stage0

go build -o dast-stage0 ./cmd/dast
```

## Run examples

```bash
./dast-stage0 run examples/hello.dast
./dast-stage0 run examples/struct_enum.dast
./dast-stage0 run examples/array.dast
```

## Run Stage1 (Dast)

From repo root:

```bash
./compiler/bootstrap/stage0/dast-stage0 run compiler/bootstrap/stage1/main.dast compiler/bootstrap/stage1/token.dast compiler/bootstrap/stage1/lexer.dast compiler/bootstrap/stage1/ast.dast compiler/bootstrap/stage1/parser.dast -- compiler/bootstrap/stage1/main.dast
```
