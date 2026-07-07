#!/usr/bin/env bash
set -euo pipefail

# ── local-run.sh ──
# Manage the backend binary as a background service.
#
# Usage:
#   ./local-run.sh            Restart backend (default: stop + start)
#   ./local-run.sh restart    Same as default
#   ./local-run.sh start      Start backend in the background
#   ./local-run.sh stop       Stop the running backend
#   ./local-run.sh status     Show whether the backend is running

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

BINARY="./new-api"
PID_FILE="./.new-api.pid"

: "${SQLITE_PATH:=one-api.db}"
: "${SESSION_SECRET:=local-dev-change-me}"
: "${MEMORY_CACHE_ENABLED:=true}"
: "${BATCH_UPDATE_ENABLED:=true}"

is_running() {
  if [ ! -f "$PID_FILE" ]; then
    return 1
  fi
  local pid
  pid="$(cat "$PID_FILE")"
  if kill -0 "$pid" 2>/dev/null; then
    return 0
  else
    rm -f "$PID_FILE"
    return 1
  fi
}

do_start() {
  if is_running; then
    echo "==> new-api is already running (pid $(cat "$PID_FILE")) on :3000"
    return 0
  fi

  if [ ! -x "$BINARY" ]; then
    echo "ERROR: $BINARY not found. Run ./local-dev.sh build first."
    exit 1
  fi

  echo "==> starting new-api on :3000 ..."
  echo "    DB:       SQLite -> $SQLITE_PATH"
  echo "    Cache:    memory (Redis disabled)"
  echo ""

  export SQLITE_PATH SESSION_SECRET MEMORY_CACHE_ENABLED BATCH_UPDATE_ENABLED
  nohup "$BINARY" --port 3000 --log-dir ./logs >> ./logs/service.log 2>&1 &
  local pid=$!
  echo "$pid" > "$PID_FILE"
  echo "==> started new-api (pid $pid)"
}

do_stop() {
  if ! is_running; then
    echo "==> new-api is not running"
    rm -f "$PID_FILE"
    return 0
  fi

  local pid
  pid="$(cat "$PID_FILE")"
  echo "==> stopping new-api (pid $pid) ..."
  kill "$pid" 2>/dev/null || true

  local waited=0
  while kill -0 "$pid" 2>/dev/null && [ "$waited" -lt 10 ]; do
    sleep 0.5
    waited=$((waited + 1))
  done

  if kill -0 "$pid" 2>/dev/null; then
    echo "==> force killing new-api (pid $pid) ..."
    kill -9 "$pid" 2>/dev/null || true
    sleep 1
  fi

  rm -f "$PID_FILE"
  echo "==> new-api stopped"
}

do_restart() {
  do_stop
  sleep 1
  do_start
}

do_status() {
  if is_running; then
    echo "==> new-api is running (pid $(cat "$PID_FILE")) on :3000"
  else
    echo "==> new-api is not running"
  fi
}

case "${1:-restart}" in
  start)    do_start ;;
  stop)     do_stop ;;
  restart)  do_restart ;;
  status)   do_status ;;
  *)
    echo "Usage: $0 {start|stop|restart|status}"
    echo "  Default (no argument): restart"
    exit 1
    ;;
esac
