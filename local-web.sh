#!/usr/bin/env bash
set -euo pipefail

# ── local-web.sh ──
# Start classic frontend dev server (Rsbuild HMR) on :5174.
# Proxies API calls to the backend on :3000.
# Usage: ./local-web.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

if ! command -v bun &>/dev/null; then
  echo "ERROR: bun is not installed. Install it from https://bun.sh"
  exit 1
fi

cd "$SCRIPT_DIR/web" && bun install --frozen-lockfile

echo "==> starting classic frontend dev server on :5174 ..."
echo "    API proxy -> :3000"
echo ""

cd "$SCRIPT_DIR/web/classic"
exec bun run dev -- --host 0.0.0.0 --port 5174
