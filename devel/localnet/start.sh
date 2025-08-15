#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/../../" && pwd)"
BIN_DIR="$REPO_DIR/bin"
LOG_DIR="$REPO_DIR/devel/localnet/logs"
mkdir -p "$BIN_DIR" "$LOG_DIR"

CARDANO_NODE_ADDRESS=${CARDANO_NODE_ADDRESS:-"backbone.cardano.iog.io:3001"}
CARDANO_NETWORK=${CARDANO_NETWORK:-"mainnet"}
PIPELINE_LIMIT=${PIPELINE_LIMIT:-10}

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

echo "[build] Compiling firecardano"
go build -o "$BIN_DIR/firecardano" ./cmd/firecardano

echo "[env] CARDANO_NODE_ADDRESS=$CARDANO_NODE_ADDRESS"
echo "[env] CARDANO_NETWORK=$CARDANO_NETWORK"
echo "[env] PIPELINE_LIMIT=$PIPELINE_LIMIT"

if [[ $BUILD_ONLY -eq 1 ]]; then
  echo "[exit] Build only requested (-b)"
  exit 0
fi

BLOCK_LOG="$LOG_DIR/blockfetcher.log"
READER_LOG="$LOG_DIR/console-reader.log"

# Start blockfetcher -> console-reader pipeline
# tee to keep raw FIRE lines log while console-reader consumes

echo "[run] Launching pipeline"
(
  BLOCK_FETCH_ADDRESS="$CARDANO_NODE_ADDRESS" \
  BLOCK_FETCH_NETWORK="$CARDANO_NETWORK" \
  BLOCK_FETCH_PIPELINE_LIMIT="$PIPELINE_LIMIT" \
  "$BIN_DIR/firecardano" blockfetcher 2>&1 | tee "$BLOCK_LOG" | "$BIN_DIR/firecardano" console-reader 2>&1 | tee "$READER_LOG"
) &
PID=$!

echo $PID > "$LOG_DIR/pipeline.pid"
echo "[ok] Pipeline started (PID=$PID)"
echo "Logs: $BLOCK_LOG, $READER_LOG"
echo "To stop: kill $(cat $LOG_DIR/pipeline.pid)"
