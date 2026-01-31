# Dast Language

A modern systems programming language with ~99% compile-time safety.

## Design Status

✅ **Design Phase Complete** - All core language features, standard library, tooling, and platform support have been fully designed.

## Documentation

🌐 **Website**: https://cpunion.github.io/dast-lang-website/

See [docs/00-overview.md](docs/00-overview.md) for the complete design overview.

### Core Design Documents (01-18)

- **Type System & Generics** (01-06): Type system, error handling, modules, generics, comptime
- **Memory & Safety** (07-08): Memory management (RAII), thread safety (Sendable/Shareable)
- **Advanced Features** (09-12): Async model, macro system, package management, testing
- **Engineering** (13-15): Standard library, syntax details, toolchain
- **Platform & Interop** (16-18): Platform support, FFI, hot reload

### Implementation Roadmap

See [docs/implementation-roadmap.md](docs/implementation-roadmap.md)

**Stage 0** (9 months): Bootstrap compiler with minimal feature set for self-hosting
- Implementation language: Go
- Target: Self-hosting capability

## Key Features

- **Syntax**: `[T]` for generics, `@attr` for attributes
- **Memory**: RAII, no GC, no reference counting, borrow checking
- **Safety**: ~99% compile-time safety, automatic lifetime inference
- **Concurrency**: Sendable/Shareable traits (simplified Send/Sync)
- **Testing**: Go-style (`*_test.dast`, `test_*` prefix)
- **Tooling**: `dast` CLI (build/run/test); LSP is a stage3 prototype (not integrated yet)
- **Runtime ABI**: C runtime lives in `compiler/stage0/runtime/c_runtime.c` (see `docs/20-bootstrap.md` for the ABI list)

## Implementation Status (high level)

- **Stage0** (Go) implements a broad, tested language subset and runs all shared tests.
- **Stage2** (Dast) implements the same subset and is driven by Stage0 (bootstrap path).
- **Stage3** tooling (LSP/VSCode) is **prototype** code under `compiler/stage3/` and is not wired into the `dast` CLI yet.

For exact behavior, prefer `make test` and the design docs; avoid relying on hard-coded counts.


## Platform Support

- Embedded (ARM Cortex-M, RISC-V, AVR)
- Edge Computing (Raspberry Pi, Jetson)
- Desktop (Linux, macOS, Windows)
- WebAssembly (Browser, WASI)
- Mobile (iOS, Android)

## License

TBD

## Status

**Current**:
- ✅ Design phase complete
- ✅ Stage0 + Stage2 compilers with shared test suite
- 🧪 Stage3 tooling prototypes (LSP/VSCode)

**Next**:
- Stage2 optimization and stabilization
- Tooling integration (fmt/lint/LSP)
- Standard library implementation

## Implementation Layout

- `compiler/stage0/`: Go bootstrap compiler/runtime (IR + interpreter)
- `compiler/core/`: Shared compiler modules
- `compiler/stage2/`: Dast compiler (bootstrap target, subset today)
- `compiler/stage3/`: Tooling prototypes (LSP/VSCode)
