#!/usr/bin/env bash
set -euo pipefail

# ── local-dev.sh ──
# Build the backend and frontend(s) locally without Docker.
# Dependencies: Go 1.25+ (backend), bun (frontend).
#
# Usage:
#   ./local-dev.sh build          Build backend only (./new-api)
#   ./local-dev.sh build-web      Build both frontends (web/default/dist/ + web/classic/dist/)
#   ./local-dev.sh build-web-default  Build default frontend (web/default/dist/)
#   ./local-dev.sh build-web-classic  Build classic frontend (web/classic/dist/)
#   ./local-dev.sh build-all      Build both frontends + backend
#   ./local-dev.sh run            Build backend and run on :3000
#   ./local-dev.sh run-with-web   Build default frontend + backend and run
#   ./local-dev.sh clean          Remove build artifacts

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

BINARY="./new-api"

: "${SQLITE_PATH:=one-api.db}"
: "${SESSION_SECRET:=local-dev-change-me}"
: "${MEMORY_CACHE_ENABLED:=true}"
: "${BATCH_UPDATE_ENABLED:=true}"

version() {
  if [ -f VERSION ]; then
    cat VERSION
  else
    echo "dev"
  fi
}

ensure_embed_dirs() {
  mkdir -p web/default/dist web/classic/dist
  for d in web/default/dist web/classic/dist; do
    if [ ! -f "$d/index.html" ]; then
      echo '<!doctype html><html><head><title>dev</title></head><body>use frontend dev server</body></html>' > "$d/index.html"
    fi
  done
}

build_web_classic() {
  echo "==> building classic frontend ..."
  if ! command -v bun &>/dev/null; then
    echo "ERROR: bun is not installed. Install it from https://bun.sh"
    exit 1
  fi
  cd "$SCRIPT_DIR/web" && bun install --frozen-lockfile
  cd "$SCRIPT_DIR/web/classic"
  DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION="$(version)" bun run build
  echo "==> frontend build complete -> web/classic/dist/"
}

build_web_default() {
  echo "==> building default frontend ..."
  if ! command -v bun &>/dev/null; then
    echo "ERROR: bun is not installed. Install it from https://bun.sh"
    exit 1
  fi
  cd "$SCRIPT_DIR/web" && bun install --frozen-lockfile
  cd "$SCRIPT_DIR/web/default"
  bun run build
  echo "==> default frontend build complete -> web/default/dist/"
}

build_web() {
  build_web_default
  build_web_classic
}

build_backend() {
  echo "==> building new-api ($(version)) ..."
  cd "$SCRIPT_DIR"
  ensure_embed_dirs
  go build \
    -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(version)'" \
    -o "$BINARY" \
    .
  echo "==> build complete: $BINARY"
}

run() {
  if [ ! -x "$BINARY" ]; then
    build_backend
  fi
  echo "==> starting new-api on :3000 ..."
  echo "    DB:       SQLite -> $SQLITE_PATH"
  echo "    Cache:    memory (Redis disabled)"
  echo ""
  export SQLITE_PATH SESSION_SECRET MEMORY_CACHE_ENABLED BATCH_UPDATE_ENABLED
  exec "$BINARY" --port 3000 --log-dir ./logs
}

run_with_web() {
  build_web_default
  build_backend
  run
}

clean() {
  rm -f "$BINARY"
  echo "==> cleaned $BINARY"
}

case "${1:-build}" in
  build)              build_backend && build_web_classic;;
  build-web)          build_web ;;
  build-web-default)  build_web_default ;;
  build-web-classic)  build_web_classic ;;
  build-all)          build_web && build_backend ;;
  run)                run ;;
  run-with-web)       run_with_web ;;
  clean)              clean ;;
  *)
    echo "Usage: $0 {build|build-web|build-web-default|build-web-classic|build-all|run|run-with-web|clean}"
    exit 1
    ;;
esac
