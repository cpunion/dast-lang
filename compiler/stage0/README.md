# Stage0 (Go)

Stage0 is the bootstrap compiler/runtime written in Go.

## Build

```bash
cd compiler/stage0

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
