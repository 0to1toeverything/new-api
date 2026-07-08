#!/bin/bash
# 一键部署 HA 到生产机
set -e
PROD_IP="${1:-192.168.6.114}"
cd "$(dirname "$0")/prod"

echo "==> 1/4 构建并推送最新镜像 ..."
cd ..
./build-and-push.sh

echo "==> 2/4 拷贝配置到生产机 ..."
ssh zhuxi@${PROD_IP} 'mkdir -p ~/allinone/new-api'
scp docker-compose.yml nginx.conf zhuxi@${PROD_IP}:~/allinone/new-api/

echo "==> 3/4 清理旧容器并启动 ..."
ssh -tt zhuxi@${PROD_IP} "cd ~/allinone/new-api && sudo docker compose down --remove-orphans 2>/dev/null; sudo docker compose up -d"

echo "==> 4/4 等待就绪 ..."
sleep 8
ssh zhuxi@${PROD_IP} 'curl -s http://localhost:3000/api/status | python3 -c "import sys,json; d=json.load(sys.stdin); print(f\"success={d[\"success\"]}\")"'
echo "==> 部署完成！"
