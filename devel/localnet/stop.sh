#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_DIR="$SCRIPT_DIR/logs"
PID_FILE="$LOG_DIR/pipeline.pid"

echo "[stop] phase 1: terminate tee processes first"
TEE_TERMINATED=0
for pat in "tee .*devel/localnet/logs/blockfetcher.log" "tee .*devel/localnet/logs/console-reader.log"; do
  while read -r p; do
    [[ -n "$p" ]] && { echo "[stop] TERM tee pid=$p ($pat)"; kill -TERM "$p" 2>/dev/null || true; TEE_TERMINATED=1; }
  done < <(pgrep -f "$pat" 2>/dev/null || true)
done
sleep 0.25
for pat in "tee .*devel/localnet/logs/blockfetcher.log" "tee .*devel/localnet/logs/console-reader.log"; do
  while read -r p; do
    [[ -n "$p" ]] && { echo "[stop] KILL tee pid=$p ($pat)"; kill -KILL "$p" 2>/dev/null || true; }
  done < <(pgrep -f "$pat" 2>/dev/null || true)
done
if [[ $TEE_TERMINATED -eq 1 ]]; then echo "[stop] tee processes terminated"; fi

# 2. Use PID file for process group shutdown
echo "[stop] phase 2: pipeline group shutdown via pid file"
if [[ -f "$PID_FILE" ]]; then
  PID=$(cat "$PID_FILE")
  if ps -p "$PID" > /dev/null 2>&1; then
    PGID=$(ps -o pgid= -p "$PID" | tr -d ' ' || echo "")
    echo "[stop] TERM pipeline PID=$PID PGID=${PGID:-unknown}"
    if [[ -n "$PGID" ]]; then
      kill -TERM -"$PGID" 2>/dev/null || true
    else
      kill -TERM "$PID" 2>/dev/null || true
    fi
    sleep 0.8
    if ps -p "$PID" > /dev/null 2>&1; then
      echo "[stop] KILL pipeline PID=$PID"
      if [[ -n "$PGID" ]]; then
        kill -KILL -"$PGID" 2>/dev/null || true
      else
        kill -KILL "$PID" 2>/dev/null || true
      fi
    fi
  else
    echo "[stop] process $PID from pid file not running"
  fi
  rm -f "$PID_FILE"
else
  echo "[stop] no pid file ($PID_FILE)"
fi

# 3. Final safety cleanup of remaining blockfetcher/console-reader
echo "[stop] phase 3: safety cleanup for lingering blockfetcher/console-reader"
for pat in "bin/blockfetcher" "bin/console-reader"; do
  while read -r p; do
    [[ -n "$p" ]] && { echo "[stop] TERM leftover pid=$p ($pat)"; kill -TERM "$p" 2>/dev/null || true; }
  done < <(pgrep -f "$pat" 2>/dev/null || true)
done
sleep 0.4
for pat in "bin/blockfetcher" "bin/console-reader"; do
  while read -r p; do
    [[ -n "$p" ]] && { echo "[stop] KILL leftover pid=$p ($pat)"; kill -KILL "$p" 2>/dev/null || true; }
  done < <(pgrep -f "$pat" 2>/dev/null || true)
done

echo "[stop] done"
