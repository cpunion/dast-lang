# Dast Language

A modern systems programming language with ~99% compile-time safety.

## Design Status

✅ **Design Phase Complete** - All core language features, standard library, tooling, and platform support have been fully designed.

## Documentation

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
- **Memory**: RAII, no GC, no reference counting, full borrow checking
- **Safety**: ~99% compile-time safety, automatic lifetime inference
- **Concurrency**: Sendable/Shareable traits (simplified Send/Sync)
- **Testing**: Go-style (`*_test.dast`, `test_*` prefix)
- **Tooling**: Unified `dast` CLI with built-in LSP

## Platform Support

- Embedded (ARM Cortex-M, RISC-V, AVR)
- Edge Computing (Raspberry Pi, Jetson)
- Desktop (Linux, macOS, Windows)
- WebAssembly (Browser, WASI)
- Mobile (iOS, Android)

## License

TBD

## Status

**Current**: Design phase complete, ready for implementation
**Next**: Stage 0 bootstrap compiler development

## Implementation Layout

- `compiler/bootstrap/stage0/`: Go bootstrap compiler/runtime (IR + interpreter)
- `compiler/bootstrap/stage1/`: Dast implementation (bootstrap in progress)
- `compiler/core/`: Shared compiler modules
- `compiler/stage2/`: Full language compiler (self-hosting target)
- `compiler/stage3/`: Tooling + optimization
