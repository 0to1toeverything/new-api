#!/bin/bash
# 开发机执行：构建镜像并推送到本地 Registry
set -e
cd "$(dirname "$0")/.."

TAG="${1:-latest}"
REGISTRY="192.168.6.247:5000"

echo "==> 构建镜像 ${REGISTRY}/new-api:${TAG} ..."
docker build -t ${REGISTRY}/new-api:${TAG} .

echo "==> 推送到 Registry ..."
docker push ${REGISTRY}/new-api:${TAG}

echo "==> 完成，生产机执行更新:"
echo "    cd ~/new-api && docker compose pull && docker compose up -d"
