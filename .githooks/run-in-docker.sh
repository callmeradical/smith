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

mkdir -p "${repo_root}/.cache/go-build" "${repo_root}/.cache/go-mod"

exec docker run --rm \
  --user "$(id -u):$(id -g)" \
  --workdir /workspace \
  --volume "${repo_root}:/workspace" \
  --env HOME=/tmp \
  --env GOCACHE=/workspace/.cache/go-build \
  --env GOMODCACHE=/workspace/.cache/go-mod \
  --env SKIP_GIT_HOOKS=1 \
  "${image}" \
  /bin/bash -lc "make ${target}"
