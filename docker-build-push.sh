#!/usr/bin/env bash
set -euo pipefail

IMAGE="ghcr.io/0to1toeverything/new-api"
TAG="${TAG:-latest}"
FULL_IMAGE="${IMAGE}:${TAG}"

# ---------- preflight ----------
if ! command -v docker &>/dev/null; then
  echo "❌ docker is not installed or in PATH"
  exit 1
fi

echo "==> Building ${FULL_IMAGE} ..."
docker build -t "${FULL_IMAGE}" "$(dirname "$0")"

# ---------- login ----------
if ! docker pull "${FULL_IMAGE}" &>/dev/null; then
  echo "==> Not logged in to ghcr.io (or image not yet published). Logging in..."
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    echo "${GITHUB_TOKEN}" | docker login ghcr.io -u zerotoeverything --password-stdin
  elif [ -n "${GH_TOKEN:-}" ]; then
    echo "${GH_TOKEN}" | docker login ghcr.io -u zerotoeverything --password-stdin
  else
    echo "==> Paste your GitHub Personal Access Token (needs write:packages scope):"
    echo "    Generate one at: https://github.com/settings/tokens"
    read -rsp "Token: " TOKEN
    echo
    echo "${TOKEN}" | docker login ghcr.io -u zerotoeverything --password-stdin
  fi
fi

# ---------- push ----------
echo "==> Pushing ${FULL_IMAGE} ..."
docker push "${FULL_IMAGE}"

# ---------- done ----------
echo
echo "✅ Done: ${FULL_IMAGE}"
echo
echo "   If this is the first push, set the package to Public:"
echo "   https://github.com/0to1toeverything/new-api/pkgs/container/new-api/settings"
echo
echo "   Remote device command:"
echo "   docker compose up -d"

fi
