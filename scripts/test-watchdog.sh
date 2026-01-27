#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
stage0="$root/compiler/bootstrap/stage0/dast-stage0"
stage2_bin="$root/compiler/stage2/target/dast-stage2"

mem_gb=10
interval_s=1
do_build=1
do_sample="${WATCHDOG_SAMPLE:-}"
do_lsof="${WATCHDOG_LSOF:-1}"

usage() {
  cat <<EOF
Usage: $(basename "$0") [options] <file.dast>

Options:
  --mem-gb N        Kill process if RSS exceeds N GiB (default: 10)
  --interval S      Watchdog polling interval seconds (default: 1)
  --no-build        Skip rebuilding stage2 binary
  -h, --help        Show this help
EOF
}

input=""
while [ $# -gt 0 ]; do
  case "$1" in
    --mem-gb)
      mem_gb="${2:-}"
      shift 2
      ;;
    --interval)
      interval_s="${2:-}"
      shift 2
      ;;
    --no-build)
      do_build=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      if [ -z "$input" ]; then
        input="$1"
        shift
      else
        echo "unexpected argument: $1" >&2
        usage >&2
        exit 2
      fi
      ;;
  esac
done

if [ -z "$input" ]; then
  usage >&2
  exit 2
fi

if [ ! -f "$input" ]; then
  echo "input not found: $input" >&2
  exit 2
fi

if [ ! -x "$stage0" ]; then
  (cd "$root" && make build-stage0)
fi

# ps rss is in KiB on macOS/Linux.
mem_limit_kb=$((mem_gb * 1024 * 1024))

get_rss_kb() {
  local pid="$1"
  local rss
  rss="$(ps -o rss= -p "$pid" 2>/dev/null | awk '{print $1}')"
  if [ -z "$rss" ]; then
    echo 0
  else
    echo "$rss"
  fi
}

run_with_watchdog() {
  local label="$1"
  shift
  local -a cmd=("$@")

  echo "[$label] cmd: ${cmd[*]}"
  set +e
  "${cmd[@]}" &
  local pid=$!
  set -e

  echo "[$label] pid=$pid limit=${mem_gb}GiB interval=${interval_s}s"

  local peak=0
  local killed=0

  while kill -0 "$pid" 2>/dev/null; do
    local rss
    rss="$(get_rss_kb "$pid")"
    if [ "$rss" -gt "$peak" ]; then
      peak="$rss"
    fi
    if [ "$rss" -ge "$mem_limit_kb" ]; then
      echo "[$label] rss=${rss}KiB exceeded limit=${mem_limit_kb}KiB; killing $pid"
      if [ -n "$do_sample" ] && command -v sample >/dev/null 2>&1; then
        local sample_out="/tmp/dast-sample-${label}-$$.txt"
        echo "[$label] capturing sample -> $sample_out"
        sample "$pid" 2 -file "$sample_out" >/dev/null 2>&1 || true
      fi
      if [ "${do_lsof}" != "0" ] && command -v lsof >/dev/null 2>&1; then
        echo "[$label] lsof snapshot:"
        lsof -p "$pid" | head -n 20 || true
      fi
      kill -TERM "$pid" 2>/dev/null || true
      killed=1
      break
    fi
    sleep "$interval_s"
  done

  if [ "$killed" -eq 1 ]; then
    sleep 1
    if kill -0 "$pid" 2>/dev/null; then
      kill -KILL "$pid" 2>/dev/null || true
    fi
  fi

  local status=0
  if wait "$pid"; then
    status=0
  else
    status=$?
  fi

  local peak_mb=$((peak / 1024))
  echo "[$label] peak_rss=${peak_mb}MiB status=$status"

  if [ "$killed" -eq 1 ]; then
    return 99
  fi
  return "$status"
}

mapfile -t stage2_files < <(find "$root/compiler/stage2" -name '*.dast' \
  -not -path "$root/compiler/stage2/tests/*" \
  -not -path "$root/compiler/stage2/stdlib/*" \
  -not -path "$root/compiler/stage2/backend/codegen-c/*" \
  -not -path "$root/compiler/stage2/backend/interp/*" | sort)
stage2_files+=("$root/compiler/stage2/backend/interp/interp.dast")
stage2_files+=("$root/compiler/stage2/backend/interp/quote.dast")

mkdir -p "$(dirname "$stage2_bin")"

if [ "$do_build" -eq 1 ]; then
  run_with_watchdog "stage2-build" \
    "$stage0" build -o "$stage2_bin" "${stage2_files[@]}"
fi

if [ ! -x "$stage2_bin" ]; then
  echo "stage2 binary not found: $stage2_bin" >&2
  exit 1
fi

run_with_watchdog "stage2-run" "$stage2_bin" run "$input"
