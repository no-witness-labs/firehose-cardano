#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/../../" && pwd)"
BIN_DIR="$REPO_DIR/bin"
LOG_DIR="$REPO_DIR/devel/localnet/logs"
mkdir -p "$BIN_DIR" "$LOG_DIR"

CONFIG_FILE=${CONFIG_FILE:-"blockfetcher.toml"}

BUILD_ONLY=0
CLEAN=0
while getopts ":bc" opt; do
  case $opt in
    b) BUILD_ONLY=1 ;;
    c) CLEAN=1 ;;
    *) echo "Usage: $0 [-b (build only)] [-c (clean)]" >&2; exit 1 ;;
  esac
done

if [[ $CLEAN -eq 1 ]]; then
  echo "[clean] Removing build artifacts and logs"
  rm -rf "$BIN_DIR" "$LOG_DIR"
  mkdir -p "$BIN_DIR" "$LOG_DIR"
fi

echo "[build] Compiling firecardano services"
go build -o "$BIN_DIR/firecardano" ./cmd/firecardano
go build -o "$BIN_DIR/blockfetcher" ./cmd/blockfetcher

echo "[env] CONFIG_FILE=$CONFIG_FILE"

if [[ $BUILD_ONLY -eq 1 ]]; then
  echo "[exit] Build only requested (-b)"
  exit 0
fi

BLOCK_LOG="$LOG_DIR/blockfetcher.log"
FIREHOSE_LOG="$LOG_DIR/firehose.log"

# Start blockfetcher -> firehose pipeline
# tee to keep raw FIRE lines log while firehose processes them

echo "[run] Launching pipeline"
(
  "$BIN_DIR/blockfetcher" -config "$REPO_DIR/$CONFIG_FILE" 2>&1 | tee "$BLOCK_LOG" | "$BIN_DIR/firecardano" start 2>&1 | tee "$FIREHOSE_LOG"
  # "$BIN_DIR/blockfetcher" -config "$REPO_DIR/$CONFIG_FILE" 2>&1 | tee "$BLOCK_LOG"
) &
PID=$!

echo $PID > "$LOG_DIR/pipeline.pid"
echo "[ok] Pipeline started (PID=$PID)"
echo "Logs: $BLOCK_LOG, $FIREHOSE_LOG"
echo "To stop: kill $(cat $LOG_DIR/pipeline.pid)"
