#!/bin/bash
# Quick LSP diagnostic script

echo "=== Dast LSP Diagnostic ==="
echo ""

# 1. Check extension
echo "1. Checking VSCode extension..."
if code --list-extensions | grep -q "dast-language"; then
    echo "   ✅ Extension installed: $(code --list-extensions | grep dast-language)"
else
    echo "   ❌ Extension NOT installed"
    echo "   Run: make vscode-ext-install"
    exit 1
fi
echo ""

# 2. Check LSP script
echo "2. Checking LSP script..."
if [ -x "compiler/stage3/lsp/dast-lsp.sh" ]; then
    echo "   ✅ LSP script exists and is executable"
else
    echo "   ❌ LSP script missing or not executable"
    echo "   Run: chmod +x compiler/stage3/lsp/dast-lsp.sh"
    exit 1
fi
echo ""

# 3. Check Stage 0 compiler
echo "3. Checking Stage 0 compiler..."
if [ -x "compiler/stage0/dast-stage0" ]; then
    echo "   ✅ Stage 0 compiler exists"
else
    echo "   ❌ Stage 0 compiler missing"
    echo "   Run: make build-stage0"
    exit 1
fi
echo ""

# 4. Check settings
echo "4. Checking VSCode settings..."
if [ -f ".vscode/settings.json" ]; then
    echo "   ✅ Settings file exists"
    if grep -q "dast.server.path" .vscode/settings.json; then
        echo "   ✅ LSP server path configured"
    else
        echo "   ❌ LSP server path NOT configured"
        exit 1
    fi
else
    echo "   ❌ Settings file missing"
    exit 1
fi
echo ""

# 5. Run LSP tests
echo "5. Running LSP tests..."
./compiler/stage0/dast-stage0 run \
    compiler/stage2/frontend/token.dast \
    compiler/stage2/frontend/lexer.dast \
    compiler/stage2/frontend/ast.dast \
    compiler/stage2/frontend/parser.dast \
    compiler/stage2/frontend/typecheck.dast \
    compiler/stage2/middle/ir.dast \
    compiler/stage2/middle/compile.dast \
    compiler/stage2/backend/interp.dast \
    compiler/stage2/lsp_bridge.dast \
    compiler/stage3/lsp/json.dast \
    compiler/stage3/lsp/jsonrpc.dast \
    compiler/stage3/lsp/protocol.dast \
    compiler/stage3/lsp/server.dast \
    compiler/stage3/lsp/diagnostics.dast \
    compiler/stage3/lsp/diagnostics_test.dast \
    compiler/stage3/lsp/definition.dast \
    compiler/stage3/lsp/definition_test.dast \
    compiler/stage3/lsp/hover.dast \
    compiler/stage3/lsp/hover_test.dast \
    compiler/stage3/lsp/completion.dast \
    compiler/stage3/lsp/completion_test.dast \
    compiler/stage3/lsp/test_main.dast 2>&1 | grep -E "(PASS|FAIL|complete)"

echo ""
echo "=== Next Steps ==="
echo ""
echo "If all checks passed:"
echo "  1. Open VSCode"
echo "  2. Press Cmd+Shift+P (Mac) or Ctrl+Shift+P"
echo "  3. Type 'Reload Window' and press Enter"
echo "  4. Open a .dast file"
echo "  5. Check 'View → Output' and select 'Dast Language Server'"
echo ""
echo "If LSP still doesn't work, check the Output panel for errors."
