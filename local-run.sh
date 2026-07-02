#!/usr/bin/env bash
set -euo pipefail

# ── local-run.sh ──
# Run the pre-built backend binary (no build step).
# Usage: ./local-run.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

BINARY="./new-api"

if [ ! -x "$BINARY" ]; then
  echo "ERROR: $BINARY not found. Run ./local-dev.sh build first."
  exit 1
fi

: "${SQLITE_PATH:=one-api.db}"
: "${SESSION_SECRET:=local-dev-change-me}"
: "${MEMORY_CACHE_ENABLED:=true}"
: "${BATCH_UPDATE_ENABLED:=true}"

echo "==> starting new-api on :3000 ..."
echo "    DB:       SQLite -> $SQLITE_PATH"
echo "    Cache:    memory (Redis disabled)"
echo ""

export SQLITE_PATH SESSION_SECRET MEMORY_CACHE_ENABLED BATCH_UPDATE_ENABLED
exec "$BINARY" --port 3000 --log-dir ./logs
