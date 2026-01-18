# Stage2 Compiler (Full Language)

Goal: implement the complete language spec and self-host using the latest language features.

Submodules:
- `frontend/`: lexer → parser → AST → typecheck
- `middle/`: HIR/MIR/IR lowering and passes
- `backend/`: codegen targets
- `driver/`: CLI glue and build orchestration
