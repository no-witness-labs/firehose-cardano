#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_DIR="$SCRIPT_DIR/logs"
PID_FILE="$LOG_DIR/pipeline.pid"

if [[ ! -f "$PID_FILE" ]]; then
  echo "No pid file found ($PID_FILE)"
  exit 0
fi

PID=$(cat "$PID_FILE")
if ps -p "$PID" > /dev/null 2>&1; then
  echo "Stopping pipeline PID=$PID"
  kill "$PID" || true
  sleep 1
  if ps -p "$PID" > /dev/null 2>&1; then
    echo "Force killing PID=$PID"
    kill -9 "$PID" || true
  fi
else
  echo "Process $PID not running"
fi

rm -f "$PID_FILE"
echo "Stopped"
