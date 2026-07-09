#!/bin/bash
# 一键部署 HA 到生产机
# 通过滚动式重启实现零宕机升级：先拉取新镜像，然后逐个重建 new-api 实例
set -euo pipefail

cd "$(dirname "$0")"

# 加载 .env（如果存在），PROD_IP 优先用命令行参数
if [ -f ../.env ]; then
  set -a
  source ../.env
  set +a
fi
PROD_IP="${1:-${PROD_IP:-192.168.6.114}}"
REGISTRY="${REGISTRY:-192.168.6.247:5000}"

echo "==> 1/4 检查镜像是否已构建 ..."
SHORT_HASH=$(git -C .. rev-parse --short HEAD 2>/dev/null || echo "unknown")
IMAGE="${REGISTRY}/new-api"
if docker images --format '{{.Tag}}' "${IMAGE}" 2>/dev/null | grep -qxF "${SHORT_HASH}"; then
  echo "  ✅ 镜像 ${IMAGE}:${SHORT_HASH} 已存在，跳过构建"
else
  echo "  🔨 镜像不存在，开始构建并推送 ..."
  ./2-docker-build-push.sh
fi

echo "==> 2/4 拷贝配置到生产机 ..."
SSH_CMD="ssh -F /dev/null -i /home/zhuxi/.ssh/id_ed25519 -o PasswordAuthentication=no zhuxi@${PROD_IP}"
SCP_CMD="scp -F /dev/null -i /home/zhuxi/.ssh/id_ed25519 -o PasswordAuthentication=no"

${SSH_CMD} 'mkdir -p ~/allinone/new-api'
${SCP_CMD} ../.env ./prod/docker-compose.yml ./prod/nginx.conf zhuxi@${PROD_IP}:~/allinone/new-api/

echo "==> 2.5/4 同步根目录 docker-compose.yml（供远程操作参考）..."
${SCP_CMD} ../docker-compose.yml zhuxi@${PROD_IP}:~/allinone/new-api/docker-compose.root.yml 2>/dev/null || true

echo "==> 3/4 拉取最新镜像 ..."
# 仅拉取 new-api 镜像（基础镜像已缓存，且生产机无法访问 Docker Hub）
${SSH_CMD} "cd ~/allinone/new-api && sudo docker compose pull new-api-1 new-api-2"

echo "==> 4/4 轮替升级（零宕机）..."
ssh -tt -F /dev/null -i /home/zhuxi/.ssh/id_ed25519 zhuxi@${PROD_IP} "cd ~/allinone/new-api && \
  echo '--- 先升级 new-api-2（此时 new-api-1 继续接受请求）---' && \
  sudo docker compose up -d --no-deps new-api-2 && \
  sleep 8 && \
  echo '--- new-api-2 就绪，升级 new-api-1 ---' && \
  sudo docker compose up -d --no-deps new-api-1 && \
  sleep 2 && \
  echo '--- 重载 nginx 配置 ---' && \
  sudo docker compose up -d --no-deps nginx"

echo "==> 验证就绪 ..."
sleep 5
RESULT=$(curl -sf "http://${PROD_IP}/api/status" 2>/dev/null) || {
  echo "  ⚠️  无法访问 http://${PROD_IP}/api/status，请手动检查"
  echo "==> 部署完成（跳过验证）"
  exit 0
}
echo "  响应: $RESULT" | head -1
if echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d.get('success'), 'not ok'" 2>/dev/null; then
  echo "  ✅ 服务健康"
fi
echo "==> 部署完成！"
