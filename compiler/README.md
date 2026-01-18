# Compiler Layout

This directory groups the compiler implementations by stage and shared modules.

- `bootstrap/stage0`: Go bootstrap compiler/runtime (IR v0 + interpreter)
- `bootstrap/stage1`: Dast bootstrap compiler (stage0-aligned)
- `core`: Shared compiler components (lexer/parser/ast/typecheck/borrow/ir/diag)
- `stage2`: Full language compiler (self-hosting target)
- `stage3`: Tooling + optimization (fmt/lint/LSP/opt/debug)
