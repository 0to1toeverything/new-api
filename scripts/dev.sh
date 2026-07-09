#!/bin/bash
# ================================================================
# dev.sh — 本地开发环境一键启动脚本
#
# 用法:
#   ./scripts/dev.sh              重新初始化 DB 并启动服务
#   ./scripts/dev.sh --reset      强制重新初始化（删除已有数据库）
#   ./scripts/dev.sh --build      仅编译后端 + 前端
# ================================================================
set -e

cd "$(dirname "$0")/.."

# 加载 .env（如果存在）
if [ -f .env ]; then
  set -a
  source .env
  set +a
fi

# ----- 配置 -----
DB_FILE="one-api.db"
PORT="${PORT:-3000}"
MODE="${MODE:-debug}"

# ----- 颜色辅助 -----
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

info()  { echo -e "${CYAN}[INFO]${NC} $1"; }
ok()    { echo -e "${GREEN}[OK]${NC}   $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
fail()  { echo -e "${RED}[FAIL]${NC} $1"; exit 1; }

# ----- 参数解析 -----
RESET=false
BUILD_ONLY=false

for arg in "$@"; do
  case "$arg" in
    --reset) RESET=true ;;
    --build) BUILD_ONLY=true ;;
    --help|-h)
      echo "用法: $0 [--reset] [--build] [--help]"
      echo ""
      echo "  --reset    强制重新初始化数据库（删除已有）"
      echo "  --build    仅编译后端和前端，不启动服务"
      exit 0
      ;;
  esac
done

# ----- 检查前置依赖 -----
info "检查依赖..."
command -v go >/dev/null 2>&1 || fail "需要 Go (>=1.22)，请先安装"
go version

if command -v bun >/dev/null 2>&2; then
  info "Bun 可用 (版本: v$(bun --version))"
fi

# ----- 停止已有服务 -----
if [ -f /tmp/new-api.pid ]; then
  OLD_PID=$(cat /tmp/new-api.pid)
  if kill -0 "$OLD_PID" 2>/dev/null; then
    info "停止已有服务 (PID: $OLD_PID)..."
    kill "$OLD_PID" 2>/dev/null
    sleep 1
  fi
  rm -f /tmp/new-api.pid
fi

# ----- 数据库初始化 -----
if [ "$RESET" = true ]; then
  info "强制重置模式: 删除已有数据库..."
  rm -f "$DB_FILE"
fi

if [ ! -f "$DB_FILE" ] || [ "$RESET" = true ]; then
  info "初始化测试数据库..."
  go run scripts/init_test_db.go
  ok "数据库初始化完成: $DB_FILE"
else
  info "数据库已存在: $DB_FILE (使用 --reset 可重新初始化)"
fi

# ----- 后端编译验证 -----
info "编译后端..."
go build -o /tmp/new-api-server . 2>&1 || fail "后端编译失败"
ok "后端编译成功"

# ----- 前端编译（如有需要） -----
if [ -d "web/default" ] && [ -d "web/default/node_modules" ]; then
  info "编译前端 (web/default)..."
  (cd web/default && bun run build 2>/dev/null) && ok "前端编译成功" || warn "前端编译失败，使用已有构建产物"
fi

# ----- 仅编译模式 -----
if [ "$BUILD_ONLY" = true ]; then
  info "仅编译模式完成"
  exit 0
fi

# ----- 启动服务 -----
info "启动服务 (端口: $PORT, 模式: $MODE)..."
export GIN_MODE="$MODE"
/tmp/new-api-server &
SERVER_PID=$!
echo "$SERVER_PID" > /tmp/new-api.pid

# 等待服务就绪
for i in $(seq 1 10); do
  if curl -s -o /dev/null "http://localhost:${PORT}/" 2>/dev/null; then
    ok "服务已就绪 → http://localhost:${PORT}/"
    echo ""
    echo -e "${CYAN}============================================${NC}"
    echo -e "  管理后台: ${GREEN}http://localhost:${PORT}/${NC}"
    echo -e "  管理员:   ${YELLOW}root / root1234${NC}"
    echo -e "  测试令牌: ${YELLOW}sk-${TEST_TOKEN_KEY:-...}${NC}"
    echo -e ""
    echo -e "  并发测试:"
    echo -e "    ${GREEN}go run scripts/concurrency.go \\${NC}"
    echo -e "      ${GREEN}-url http://localhost:${PORT} \\${NC}"
    echo -e "      ${GREEN}-key ${TEST_TOKEN_KEY:-concur...} \\${NC}"
    echo -e "      ${GREEN}-model test-gpt-concurrency -c 20 -n 200${NC}"
    echo -e "${CYAN}============================================${NC}"
    echo ""
    info "提示: 按 Ctrl+C 停止服务，或运行 kill \$(cat /tmp/new-api.pid)"
    exit 0
  fi
  sleep 1
done

fail "服务启动超时，请检查日志"
