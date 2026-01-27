#!/usr/bin/env bash
set -euo pipefail

mem_mb=128
interval_s=1
verbose="${SAFE_RUN_VERBOSE:-}"

usage() {
  cat <<USAGE
Usage: $(basename "$0") [options] -- <cmd> [args...]

Options:
  --mem-mb N     Kill process if RSS exceeds N MiB (default: 128)
  --interval S   Polling interval seconds (default: 1)
  -h, --help     Show this help
USAGE
}

if [ $# -eq 0 ]; then
  usage >&2
  exit 2
fi

while [ $# -gt 0 ]; do
  case "$1" in
    --mem-mb)
      mem_mb="${2:-}"
      shift 2
      ;;
    --interval)
      interval_s="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      break
      ;;
    *)
      break
      ;;
  esac
done

if [ $# -eq 0 ]; then
  usage >&2
  exit 2
fi

cmd=("$@")

mem_limit_kb=$((mem_mb * 1024))

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

set +e
"${cmd[@]}" &
pid=$!
set -e

if [ -n "$verbose" ]; then
  echo "[safe-run] pid=$pid limit=${mem_mb}MiB interval=${interval_s}s cmd=${cmd[*]}"
fi

peak=0
killed=0
while kill -0 "$pid" 2>/dev/null; do
  rss="$(get_rss_kb "$pid")"
  if [ "$rss" -gt "$peak" ]; then
    peak="$rss"
  fi
  if [ "$rss" -ge "$mem_limit_kb" ]; then
    echo "[safe-run] rss=${rss}KiB exceeded limit=${mem_limit_kb}KiB; killing $pid" >&2
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

status=0
if wait "$pid"; then
  status=0
else
  status=$?
fi

if [ -n "$verbose" ]; then
  peak_mb=$((peak / 1024))
  echo "[safe-run] peak_rss=${peak_mb}MiB status=$status" >&2
fi

if [ "$killed" -eq 1 ]; then
  exit 99
fi
exit "$status"
