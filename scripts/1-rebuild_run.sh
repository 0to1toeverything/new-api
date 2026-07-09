#!/bin/bash
# ================================================================
# 1_rebuild_run.sh — 修改代码后编译前后端并启动本地服务
#
# 流程:
#   1. 编译前端 web/classic（bun run build）
#   2. 编译后端 Go 二进制
#   3. 任一编译失败则退出，不启动
#   4. 全部通过 → 杀掉旧服务 → 启动新服务
# ================================================================
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$PWD"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC}  $1"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }
fail()  { echo -e "${RED}[FAIL]${NC}  $1"; exit 1; }

# ---- 1. 前端编译 ----
info "编译前端 (web/classic)..."
if ! command -v bun &>/dev/null; then
  fail "bun 未安装，请先安装 bun"
fi
pushd web/classic >/dev/null
if bun run build 2>&1 | tail -3; then
  ok "前端编译成功"
else
  fail "前端编译失败，已终止"
fi
popd >/dev/null

# ---- 2. 后端编译 ----
info "编译后端..."
BINARY="/tmp/new-api-server"
if go build -ldflags "-s -w" -o "$BINARY" . 2>&1 | tail -5; then
  ok "后端编译成功 → $BINARY ($(du -h "$BINARY" | cut -f1))"
else
  fail "后端编译失败，已终止"
fi

# ---- 3. 停止旧服务 ----
info "停止旧服务 (端口 3000)..."
fuser -k 3000/tcp 2>/dev/null || true
sleep 1

# ---- 4. 启动新服务 ----
info "启动服务..."
export GIN_MODE="${GIN_MODE:-debug}"
nohup "$BINARY" > /tmp/new-api-server.log 2>&1 &
PID=$!
echo "$PID" > /tmp/new-api.pid

# 等待就绪
for i in $(seq 1 15); do
  if curl -s -o /dev/null "http://localhost:3000/" 2>/dev/null; then
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "  ✅ 服务已就绪 (PID: $PID)"
    echo -e "  🌐 ${CYAN}http://localhost:3000/${NC}"
    echo -e "  👤 ${YELLOW}root / root1234${NC}"
    echo -e "${GREEN}========================================${NC}"
    exit 0
  fi
  sleep 1
done

fail "服务启动超时，请查看日志: tail -50 /tmp/new-api-server.log"
