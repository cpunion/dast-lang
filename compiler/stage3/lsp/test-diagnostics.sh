#!/bin/bash
# Test LSP diagnostics with real Stage 2 integration

cd "$(dirname "$0")/../../.."

# Create file list
cat > /tmp/stage2_files.txt << 'EOF'
compiler/stage2/frontend/token.dast
compiler/stage2/frontend/lexer.dast
compiler/stage2/frontend/ast.dast
compiler/stage2/frontend/parser.dast
compiler/stage2/frontend/typecheck.dast
compiler/stage2/backend/ir.dast
compiler/stage2/backend/interp.dast
compiler/stage2/driver/driver.dast
EOF

STAGE2_FILES=$(cat /tmp/stage2_files.txt | tr '\n' ' ')
LSP_FILES="compiler/stage3/lsp/json.dast compiler/stage3/lsp/jsonrpc.dast compiler/stage3/lsp/protocol.dast compiler/stage3/lsp/server.dast compiler/stage3/lsp/diagnostics.dast compiler/stage3/lsp/diagnostics_test.dast"

echo "Running LSP diagnostics tests with Stage 2 integration..."
./compiler/stage0/dast-stage0 run $STAGE2_FILES -- run $LSP_FILES
