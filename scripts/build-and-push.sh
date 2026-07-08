#!/bin/bash
# 开发机执行：构建镜像并推送到本地 Registry
set -e
cd "$(dirname "$0")/.."

REGISTRY="192.168.6.247:5000"
SHORT_HASH=$(git rev-parse --short HEAD)
TAG="${1:-${SHORT_HASH}}"
IMAGE="${REGISTRY}/new-api"

# --no-cache on first arg "nocache"
BUILD_FLAGS="--pull"
if [[ "$1" == "nocache" ]]; then
    BUILD_FLAGS="--pull --no-cache"
    TAG="${2:-${SHORT_HASH}}"
fi

echo "==> 构建 ${IMAGE}:${TAG} ..."
docker build \
    ${BUILD_FLAGS} \
    -t "${IMAGE}:${TAG}" \
    -t "${IMAGE}:latest" \
    .

echo "==> 推送 ${IMAGE}:${TAG} ..."
docker push "${IMAGE}:${TAG}"

echo "==> 推送 ${IMAGE}:latest ..."
docker push "${IMAGE}:latest"

echo "==> 完成"
echo "    生产机执行:"
echo "    cd ~/new-api && docker compose pull && docker compose up -d"
