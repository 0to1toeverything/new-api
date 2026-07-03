#!/usr/bin/env bash
# Build Docker image locally and push to Gitee (for Chinese users)
# Usage: ./build-and-push-gitee.sh [tag]
set -euo pipefail
TAG="${1:-latest}"
IMAGE="registry.gitee.com/zerotoeverything/new-api:${TAG}"

echo "==> Building ${IMAGE} ..."
docker build -t "${IMAGE}" "$(dirname "$0")"

echo "==> Logging in to Gitee ..."
if [ -n "${GITEE_TOKEN:-}" ]; then
  echo "${GITEE_TOKEN}" | docker login registry.gitee.com -u zerotoeverything --password-stdin
else
  docker login registry.gitee.com -u zerotoeverything
fi

echo "==> Pushing ${IMAGE} ..."
docker push "${IMAGE}"
echo "✅ Done: ${IMAGE}"
