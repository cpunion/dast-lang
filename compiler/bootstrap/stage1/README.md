# Stage1 (Dast)

Stage1 is a Dast implementation of the compiler front-end. It is executed by the Go stage0 runtime.

## Run with stage0

From repo root:

```bash
# build stage0
( cd compiler/bootstrap/stage0 && go build ./cmd/dast )

# run stage1 lexer on its own source
./compiler/bootstrap/stage0/dast run compiler/bootstrap/stage1/main.dast compiler/bootstrap/stage1/token.dast compiler/bootstrap/stage1/lexer.dast compiler/bootstrap/stage1/ast.dast compiler/bootstrap/stage1/parser.dast -- compiler/bootstrap/stage1/main.dast
```

Current stage1 parses top-level `struct`/`enum`/`const`/`fn`/`impl` declarations (names/fields/variants/params/return types), supports `@repr(...)` on enums and enum discriminants, and a minimal function body (let/return/if/while/match/assign/expr). Expressions are parsed into a small AST. Basic diagnostics are collected and printed; `--tokens` dumps the token stream.
