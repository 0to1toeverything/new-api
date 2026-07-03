#!/usr/bin/env bash
# Mirror Docker image from ghcr.io to Gitee registry (for Chinese users)
# Usage: GITEE_TOKEN=xxx ./mirror-to-gitee.sh [tag]

set -euo pipefail
TAG="${1:-latest}"

SRC="ghcr.io/zerotoeverything/new-api:${TAG}"
DST="registry.gitee.com/zerotoeverything/new-api:${TAG}"

echo "==> Pulling ${SRC} ..."
docker pull "${SRC}"

echo "==> Tagging for Gitee ..."
docker tag "${SRC}" "${DST}"

echo "==> Logging in to Gitee ..."
if [ -n "${GITEE_TOKEN:-}" ]; then
  echo "${GITEE_TOKEN}" | docker login registry.gitee.com -u zerotoeverything --password-stdin
else
  docker login registry.gitee.com -u zerotoeverything
fi

echo "==> Pushing ${DST} ..."
docker push "${DST}"
echo "✅ Done: ${DST}"
