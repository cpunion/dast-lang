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

## Language Features Implementation Status

### Core Language Features

| Feature | Status | Stage | Notes |
|---------|--------|-------|-------|
| **Basic Types** | ✅ Complete | Stage 0 | int, bool, String, arrays |
| **Structs** | ✅ Complete | Stage 0 | Definition, instantiation, field access |
| **Enums** | ✅ Complete | Stage 0 | Variants, pattern matching |
| **Functions** | ✅ Complete | Stage 0 | Parameters, return types, recursion |
| **Generics** | ✅ Complete | Stage 0 | Generic functions and structs |
| **References** | ✅ Complete | Stage 0 | `&T`, `&mut T`, borrow checking |
| **Pattern Matching** | ✅ Complete | Stage 0 | match, if-let, guards, ranges |
| **Closures** | ✅ Complete | Stage 2 | Immutable and mutable captures |
| **Traits** | ✅ Complete | Stage 2 | Definition, impl, methods |
| **Associated Types** | ✅ Complete | Stage 2 | Trait associated types |
| **Trait Bounds** | ✅ Complete | Stage 2 | Single and multiple bounds |
| **Type Aliases** | ⚠️ Partial | Stage 2 | Parser done, resolution bug |
| **Impl Blocks** | ✅ Complete | Stage 0 | Methods, static methods, Self |
| **Modules** | ✅ Complete | Stage 0 | Nested modules, visibility |
| **Imports** | ✅ Complete | Stage 0 | Selective, aliased, re-exports |
| **Const** | ✅ Complete | Stage 0 | Compile-time constants |
| **Type Inference** | ✅ Complete | Stage 0 | Local variable types |
| **Lifetimes** | 🚧 Planned | - | Explicit lifetime annotations |
| **Async/Await** | 🚧 Planned | - | Async functions, futures |
| **Macros** | 🚧 Planned | - | Declarative and procedural |

### Memory & Safety

| Feature | Status | Stage | Notes |
|---------|--------|-------|-------|
| **RAII** | ✅ Complete | Stage 0 | Automatic resource management |
| **Borrow Checker** | ✅ Complete | Stage 0 | Compile-time safety |
| **Move Semantics** | ✅ Complete | Stage 0 | Ownership transfer |
| **Mutable Closure Capture** | 🚧 In Progress | Stage 2 | Variable promotion needed |
| **Full Lifetimes** | 🚧 Planned | - | Explicit annotations |

### Standard Library

| Feature | Status | Stage | Notes |
|---------|--------|-------|-------|
| **Prelude** | ✅ Package Created | Stage 2 | I/O, arrays, strings, system |
| **Collections** | 🚧 Planned | - | Vec, HashMap, etc. |
| **Error Handling** | 🚧 Planned | - | Result, Option |
| **Iterators** | 🚧 Planned | - | Iterator trait, adapters |
| **String Utilities** | 🚧 Planned | - | String manipulation |

### Tooling & Infrastructure

| Feature | Status | Stage | Notes |
|---------|--------|-------|-------|
| **Package Management** | ✅ Complete | Stage 2 | Directory-based packages |
| **Build System** | ✅ Complete | Stage 0 | dast build/run/test |
| **LSP Server** | ✅ Production | Stage 3 | Diagnostics, go-to-def |
| **VSCode Extension** | ✅ Complete | Stage 3 | Syntax highlighting, LSP |
| **IR Optimizer** | 🚧 Planned | Stage 2 | Optimization passes |
| **Better Errors** | 🚧 Planned | Stage 2 | Improved error messages |
| **Incremental Compilation** | 🚧 Planned | - | Dependency tracking |

### Test Coverage

- **Stage 0**: 12 tests passing
- **Stage 1**: Self-hosting IR check passing
- **Stage 2**: 44/45 tests passing (type-alias bug)
- **LSP**: 35/35 tests passing


## Language Server Protocol (LSP) Support

**Status**: ✅ **Production Ready** (35/35 tests passing)

The Dast LSP provides real-time language support in VSCode and other LSP-compatible editors.

### Features

| Feature | Status | Description |
|---------|--------|-------------|
| **Diagnostics** | ✅ Complete | Real-time syntax and type error detection |
| **Go to Definition** | ✅ MVP | Jump to function/struct definitions (F12) |
| **Hover** | 🚧 Planned | Type information on hover |
| **Find References** | 🚧 Planned | Find all symbol usages |
| **Code Completion** | 🚧 Planned | Context-aware suggestions |

### Installation

1. **Install VSCode Extension**:
   ```bash
   cd compiler/stage3/vscode-ext
   ./build-and-install.sh
   ```

2. **Reload VSCode** and open any `.dast` file

3. **Enjoy** real-time diagnostics and navigation!

### Architecture

- **Self-hosting**: LSP written in Dast, runs via Stage 0 → Stage 2 → Stage 3 bootstrap chain
- **Real compiler integration**: Uses actual Stage 2 lexer, parser, and type checker
- **Zero dependencies**: Native JSON parser and JSON-RPC implementation

See [`.vscode/LSP_SETUP.md`](.vscode/LSP_SETUP.md) for detailed setup and usage.


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
- ✅ Stage 2 compiler (self-hosting capable)
- ✅ LSP with real-time diagnostics (35/35 tests)

**Next**:
- Stage 2 optimization and stabilization
- LSP advanced features (hover, completion)
- Standard library implementation

## Implementation Layout

- `compiler/bootstrap/stage0/`: Go bootstrap compiler/runtime (IR + interpreter)
- `compiler/bootstrap/stage1/`: Dast implementation (bootstrap in progress)
- `compiler/core/`: Shared compiler modules
- `compiler/stage2/`: Full language compiler (self-hosting target)
- `compiler/stage3/`: Tooling + optimization
