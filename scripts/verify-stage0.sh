#!/usr/bin/env bash
set -euo pipefail

if ! command -v qbe >/dev/null 2>&1; then
  echo "error: qbe not found in PATH"
  exit 1
fi

if ! command -v cc >/dev/null 2>&1 && ! command -v clang >/dev/null 2>&1; then
  echo "error: cc/clang not found in PATH"
  exit 1
fi

timeout_cmd=""
if command -v timeout >/dev/null 2>&1; then
  timeout_cmd="timeout"
elif command -v gtimeout >/dev/null 2>&1; then
  timeout_cmd="gtimeout"
else
  echo "warning: timeout not found; running without timeout"
fi

make build-stage0

if [ -n "$timeout_cmd" ]; then
  "$timeout_cmd" 10m make test-stage0
else
  make test-stage0
fi

./compiler/stage0/dast-stage0 run compiler/stage0/examples/hello/main.dast
./compiler/stage0/dast-stage0 build compiler/tests/integration/module-basic -o /tmp/dast_module_basic
