#!/bin/bash
# Quick single-shot test for new-api gateway.
# Usage:
#   KEY=sk-your-key bash scripts/quick_test.sh [concurrency] [total]
#   or source .env first (if KEY is set there)

set -euo pipefail
cd "$(dirname "$0")/.."

URL="${URL:-http://192.168.6.114}"
# KEY must be set via env var or .env
if [ -z "${KEY:-}" ]; then
  echo "❌ 请设置 KEY 环境变量，例如:"
  echo "   KEY=sk-your-key bash scripts/quick_test.sh"
  echo "   或先在 .env 中配置 KEY=..."
  exit 1
fi
MODEL="${MODEL:-gpt-3.5-turbo-0613}"
CONCURRENCY="${1:-10}"
TOTAL="${2:-100}"

echo "==> 并发 $CONCURRENCY, 总计 $TOTAL 请求 -> $URL/v1/chat/completions (model=$MODEL)"

go run scripts/concurrency.go \
  -url "$URL" \
  -key "$KEY" \
  -model "$MODEL" \
  -c "$CONCURRENCY" \
  -n "$TOTAL"
