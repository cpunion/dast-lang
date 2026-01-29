#!/bin/bash
# Dast LSP Server Launcher
# Usage: ./dast-lsp [workspace_root]
#
# This script launches the Dast LSP server via the bootstrap chain:
# Stage 0 (Go) runs Stage 2 (Dast) which runs Stage 3 LSP (Dast)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_ROOT="${1:-$(pwd)}"

# Find compiler directories relative to script location
COMPILER_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
STAGE0_BIN="$COMPILER_ROOT/stage0/dast-stage0"
STAGE2_DIR="$COMPILER_ROOT/stage2"
LSP_DIR="$SCRIPT_DIR"

# Build Stage 0 if needed
if [ ! -f "$STAGE0_BIN" ]; then
    echo "Building Stage 0 compiler..." >&2
    (cd "$COMPILER_ROOT/stage0" && go build -o dast-stage0 ./cmd/dast) >&2
fi

# Collect Stage 2 files
STAGE2_FILES=$(find "$STAGE2_DIR" -name '*.dast' -not -path '*/tests/*' | sort | tr '\n' ' ')

# Collect LSP files
LSP_FILES="$LSP_DIR/json.dast $LSP_DIR/jsonrpc.dast $LSP_DIR/protocol.dast $LSP_DIR/server.dast $LSP_DIR/main.dast"

# Launch the LSP server
exec "$STAGE0_BIN" run $STAGE2_FILES -- run $LSP_FILES
