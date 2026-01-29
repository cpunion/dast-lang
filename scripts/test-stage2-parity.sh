#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
stage0="$root/compiler/stage0/dast-stage0"

if [ ! -x "$stage0" ]; then
  (cd "$root" && make build-stage0)
fi

timeout_cmd=""
if command -v timeout >/dev/null 2>&1; then
  timeout_cmd="timeout"
elif command -v gtimeout >/dev/null 2>&1; then
  timeout_cmd="gtimeout"
fi

stage2_dirs=(
  "$root/compiler/stage2/driver"
  "$root/compiler/stage2/frontend"
  "$root/compiler/stage2/middle"
  "$root/compiler/stage2/util"
)

report_dir="$root/compiler/stage2/target"
report="$report_dir/parity-report.txt"
mkdir -p "$report_dir"
echo "stage2 parity report" > "$report"
echo "timestamp: $(date)" >> "$report"
echo >> "$report"

pass=0
fail=0

run_case() {
  local name="$1"
  local expect="$2"
  shift 2
  local out status
  local cmd=("$stage0" run "${stage2_dirs[@]}" -- "$@")
  if [ -n "$timeout_cmd" ]; then
    set +e
    out="$("$timeout_cmd" 90s "${cmd[@]}" 2>&1)"
    status=$?
    set -e
  else
    set +e
    out="$("${cmd[@]}" 2>&1)"
    status=$?
    set -e
  fi
  local ok=1
  if [ "$expect" = "pass" ]; then
    if [ $status -ne 0 ]; then ok=0; fi
  else
    if [ $status -eq 0 ]; then ok=0; fi
  fi
  if [ $ok -eq 1 ]; then
    pass=$((pass+1))
    echo "[ok] $name" >> "$report"
  else
    fail=$((fail+1))
    echo "[fail] $name" >> "$report"
    echo "$out" | head -50 >> "$report"
    echo >> "$report"
  fi
}

for f in "$root"/compiler/tests/run-pass/*.dast; do
  run_case "run-pass:$f" pass run "$f"
done

for f in "$root"/compiler/tests/compile-fail/*.dast; do
  run_case "compile-fail:$f" fail run "$f"
done

run_case "module-basic:build" pass build --emit-ir "$root/compiler/tests/integration/module-basic"
run_case "module-basic:run" pass run "$root/compiler/tests/integration/module-basic"
run_case "module-basic:test" pass test "$root/compiler/tests/integration/module-basic"

run_case "test-fail:test" fail test "$root/compiler/tests/integration/test-fail"
run_case "test-fail-compile:test" fail test "$root/compiler/tests/integration/test-fail-compile"

run_case "deps:build" pass build --emit-ir "$root/compiler/tests/integration/deps/app"
run_case "deps:run" pass run "$root/compiler/tests/integration/deps/app"
run_case "deps:test" pass test "$root/compiler/tests/integration/deps/app"

run_case "workspace:run" pass run "$root/compiler/tests/integration/workspace/app"
run_case "workspace:test" pass test "$root/compiler/tests/integration/workspace/app"

echo >> "$report"
echo "pass: $pass" >> "$report"
echo "fail: $fail" >> "$report"

echo "stage2 parity report: $report"
if [ $fail -ne 0 ]; then
  exit 1
fi
