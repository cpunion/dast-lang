# Dast VSCode Extension

Language support for the Dast programming language.

## Features

- Syntax highlighting for `.dast` files
- Language Server Protocol (LSP) integration
- Real-time diagnostics (coming soon)
- Code completion (coming soon)
- Go to definition (coming soon)

## Installation

### From Source

1. Build the Stage 0 compiler:
   ```bash
   cd compiler/bootstrap/stage0
   go build -o dast-stage0 ./cmd/dast
   ```

2. Install extension dependencies:
   ```bash
   cd compiler/stage3/vscode-ext
   npm install
   npm run compile
   ```

3. Install the extension in VSCode:
   - Open VSCode
   - Press `Cmd+Shift+P` (Mac) or `Ctrl+Shift+P` (Windows/Linux)
   - Type "Extensions: Install from VSIX"
   - Select the `.vsix` file (or use "Developer: Install Extension from Location" and select the `vscode-ext` folder)

### Development Mode

1. Open the `vscode-ext` folder in VSCode
2. Press `F5` to launch Extension Development Host
3. Open a `.dast` file to activate the extension

## Configuration

The extension can be configured via VSCode settings:

- `dast.server.path`: Path to the `dast-lsp` executable or wrapper script
  - Default: looks for `compiler/stage3/lsp/dast-lsp.sh` in workspace
- `dast.trace.server`: Enable LSP communication tracing
  - Options: `off`, `messages`, `verbose`
  - Default: `off`

## Architecture

The Dast LSP uses a self-hosting bootstrap chain:

```
Stage 0 (Go) → Stage 2 (Dast) → Stage 3 LSP (Dast)
```

The `dast-lsp.sh` wrapper script orchestrates this chain automatically.

## Commands

- `Dast: Restart Language Server` - Restart the LSP server

## Requirements

- VSCode 1.75.0 or higher
- Go 1.19+ (for building Stage 0)
- Dast compiler workspace

## Known Issues

- LSP features are currently in development
- Only basic syntax highlighting is available

## Release Notes

### 0.1.0

- Initial release
- Syntax highlighting
- LSP infrastructure (Phase 1)
