# AI Agent Guidelines for Dast Language Development

> This document provides guidelines for AI coding assistants (Claude, GPT, etc.) working on the Dast language project.

See also: [CLAUDE.md](CLAUDE.md) for Claude-specific instructions.
Key docs: [Overview](docs/00-overview.md) • [Implementation Roadmap](docs/implementation-roadmap.md) • [Correctness Testing Standard](docs/23-correctness-testing.md) • [Pre-Macro Readiness Checklist](docs/24-pre-macro-checklist.md)

---

## Project Overview

**Dast** is a modern systems programming language with ~99% compile-time safety, designed for extreme platform coverage from embedded to desktop.

**Current Status**: Design phase complete, ready for Stage 0 bootstrap compiler implementation.

---

## Design Documents

All design documents are in `docs/`:

### Core Documents (00-18)

| Range | Topic |
|-------|-------|
| 00 | Overview |
| 01-06 | Type system, generics, comptime |
| 07-08 | Memory management, thread safety |
| 09-12 | Async, macros, packages, testing |
| 13-15 | Standard library, syntax, toolchain |
| 16-18 | Platform support, FFI, hot reload |

### Historical Documents

Early design discussions are in `docs/archive/` for reference only.

---

## Key Design Decisions

### Syntax

```dast
// Generics: square brackets
fn identity[T](x: T) -> T { x }
struct Vec[T] { ... }

// Attributes: @ prefix
@test
@derive(Clone)
@repr(C)

// Testing: Go-style
// File: math_test.dast
fn test_add() {
    assert_eq(add(1, 2), 3)
}
```

### Memory Model

- **RAII**: Automatic resource cleanup via Drop trait
- **Ownership**: Unique ownership with move semantics
- **Borrowing**: `&T` (immutable), `&mut T` (mutable)
- **No GC**: No garbage collector, no reference counting
- **Lifetime inference**: Automatic, no explicit `'a` annotations

### Thread Safety

- **Sendable**: Can be sent across threads (like Rust's Send)
- **Shareable**: Can be shared across threads (like Rust's Sync)
- **Auto-derived**: Compiler automatically derives these traits
- **Compile-time checked**: 100% thread safety at compile time

---

## Implementation Roadmap

See [docs/implementation-roadmap.md](docs/implementation-roadmap.md) for full details.

### Stage 0: Bootstrap Compiler (9 months)

**Goal**: Minimal Dast compiler written in Go, capable of self-hosting.

**Minimal Feature Set**:
- Basic types: `i32`, `i64`, `bool`, `String`, etc.
- Structs, enums, arrays, references
- Functions, `impl`/`self` (basic)
- Pattern matching
- Borrow checking (simplified)
- No: comptime, macros, async, unsafe, generics

**Components**:
1. Frontend: Lexer → Parser → AST → Type Checker → Borrow Checker (simplified)
2. Middle-end: IR v0 (stable core)
3. Backend: IR interpreter (bootstrap)

**Commands**:
```bash
dast build main.dast
dast run main.dast
dast test
```

**Self-hosting verification**:
```bash
# Stage 0 (Go) compiles Stage 1 (Dast bootstrap compiler)
dast run compiler/bootstrap/stage1/*.dast

# Stage 1 (Dast) compiles itself (once feature-parity is reached)
dastc-stage1 compiler/bootstrap/stage1/*.dast -o dastc-stage1
```

### Tooling (Parallel Development)

**Shared Architecture**: All tools share Parser/AST/Type Checker modules.

```
Compiler Core (Parser → AST → Type Checker → Borrow Checker)
    ↓           ↓              ↓
  dastfmt    dastlint        LSP
```

**Always Available**: fmt/lint/LSP should work from day one using shared compiler infrastructure.

---

## Code Style Guidelines

### When Writing Dast Code

```dast
// Use [T] for generics, not <T>
fn map[T, U](vec: Vec[T], f: fn(T) -> U) -> Vec[U]

// Use @attr for attributes, not #[attr]
@derive(Clone, Debug)
struct Point { x: i32, y: i32 }

// Testing: Go-style
// File: utils_test.dast
fn test_helper() {
    assert(helper() == 42)
}

// No explicit lifetimes
fn longest(x: &str, y: &str) -> &str {
    if x.len() > y.len() { x } else { y }
}
```

### When Writing Implementation Code (Go)

Follow standard Go conventions for the bootstrap compiler.

---

## Working with Design Documents

### Reading Documents

1. **Start with** `docs/00-overview.md` for high-level understanding
2. **Refer to specific docs** for detailed design (e.g., `07-memory-management.md`)
3. **Check archive/** only for historical context, not current design

### Updating Documents

- ✅ **DO**: Update core docs (00-18) when design evolves
- ❌ **DON'T**: Modify archive/ documents (historical record)
- ✅ **DO**: Keep syntax consistent (`[T]`, `@attr`)
- ✅ **DO**: Update `00-overview.md` when adding major features

---

## Common Tasks

### Adding a New Language Feature

1. Check if it's already designed in docs/
2. If not, discuss design first
3. Update relevant design documents
4. Implement in compiler
5. Add tests
6. Update tooling (fmt/lint/LSP)

### Implementing Stage 0 Compiler

1. Start with frontend (lexer, parser)
2. Build AST representation
3. Implement type checker
4. Add borrow checker (simplified)
5. Integrate Cranelift backend
6. Test with simple programs
7. Gradually add features until self-hosting

### Adding Standard Library Modules

1. Check `docs/13-standard-library.md` for design
2. Implement in `std/` directory
3. Add comprehensive tests
4. Document public APIs

---

## Testing Strategy

### Unit Tests

```dast
// File: vec_test.dast
fn test_push() {
    let mut v = Vec.new()
    v.push(1)
    assert_eq(v.len(), 1)
}
```

### Integration Tests

```
tests/
├── ui/           # Error message tests
├── run-pass/     # Should compile and run
├── run-fail/     # Should run but fail
└── compile-fail/ # Should fail to compile
```

### Bootstrap Tests

Verify Stage N and Stage N+1 produce identical binaries.

---

## Communication Guidelines

### When Asking Questions

- Reference specific design documents
- Quote relevant sections
- Propose solutions based on existing design

### When Proposing Changes

- Explain rationale
- Show how it fits with existing design
- Consider impact on implementation roadmap
- Update affected documentation

---

## Resources

- **Design Docs**: `docs/00-overview.md` and `docs/01-18/`
- **Implementation Plan**: `docs/implementation-roadmap.md`
- **Historical Context**: `docs/archive/`
- **Similar Projects**: Rust, Zig, C++

---

## Quick Reference

### Syntax Cheat Sheet

```dast
// Variables
let x = 42
let mut y = 10

// Functions
fn add(a: i32, b: i32) -> i32 { a + b }

// Generics
fn identity[T](x: T) -> T { x }

// Structs
struct Point { x: i32, y: i32 }

// Enums
enum Option[T] { Some(T), None }

// Pattern matching
match opt {
    .Some(x) => println("{}", x),
    .None => println("none"),
}

// Borrowing
fn read(x: &i32) { }
fn write(x: &mut i32) { }

// Attributes
@test
@derive(Clone)
@repr(C)
```

### Common Commands

```bash
# Build
dast build

# Run
dast run main.dast

# Test
dast test

# Format
dast fmt

# Lint
dast lint

# LSP (called by editor)
dast lsp
```

---

## Notes for AI Assistants

1. **Always check design docs first** before proposing implementations
2. **Maintain syntax consistency**: `[T]` for generics, `@attr` for attributes
3. **Follow the roadmap**: Stage 0 features only initially
4. **Shared tooling**: Remember fmt/lint/LSP share compiler modules
5. **Self-hosting is the goal**: Every feature must work in the compiler itself
6. **Test thoroughly**: Especially borrow checker and type system
7. **Document as you go**: Update design docs when adding features

---

## Getting Started

1. Read `docs/00-overview.md`
2. Review `docs/implementation-roadmap.md`
3. Understand Stage 0 minimal feature set
4. Start with frontend (lexer/parser)
5. Build incrementally, test continuously
6. Work towards self-hosting

**Remember**: Design is complete. Focus on implementation.
