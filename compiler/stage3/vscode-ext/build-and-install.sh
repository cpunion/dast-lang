#!/bin/bash
# Build and install Dast VSCode extension

set -e

cd "$(dirname "$0")"

echo "📦 Building Dast VSCode Extension..."

# Install dependencies
echo "Installing npm dependencies..."
npm install

# Compile TypeScript
echo "Compiling TypeScript..."
npm run compile

# Package extension
echo "Packaging extension..."
npx -y @vscode/vsce package --allow-missing-repository --no-dependencies

# Find the generated .vsix file
VSIX_FILE=$(ls -t *.vsix 2>/dev/null | head -1)

if [ -z "$VSIX_FILE" ]; then
    echo "❌ Error: No .vsix file generated"
    exit 1
fi

echo "✅ Extension packaged: $VSIX_FILE"
echo ""
echo "To install in VSCode:"
echo "  1. Open VSCode"
echo "  2. Press Cmd+Shift+P (Mac) or Ctrl+Shift+P (Windows/Linux)"
echo "  3. Type 'Extensions: Install from VSIX'"
echo "  4. Select: $(pwd)/$VSIX_FILE"
echo ""
echo "Or run:"
echo "  code --install-extension $(pwd)/$VSIX_FILE"
