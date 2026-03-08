#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <make-target>" >&2
  exit 2
fi

target="$1"
repo_root="$(git rev-parse --show-toplevel)"
image="${SMITH_HOOKS_DOCKER_IMAGE:-golang:1.24-bookworm}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required for containerized hooks" >&2
  exit 1
fi

container_id="$(docker create \
  --workdir /workspace \
  --env HOME=/tmp \
  --env SKIP_GIT_HOOKS=1 \
  "${image}" \
  /bin/bash -lc "make ${target}")"

cleanup() {
  docker rm -f "${container_id}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker cp "${repo_root}/." "${container_id}:/workspace"
docker start -a "${container_id}"
