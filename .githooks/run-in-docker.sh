#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <make-target>" >&2
  exit 2
fi

target="$1"
repo_root="$(git rev-parse --show-toplevel)"
image="${SMITH_HOOKS_DOCKER_IMAGE:-smith-hooks:local}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required for containerized hooks" >&2
  exit 1
fi

# Ensure hooks image is built if using the default
if [[ "$image" == "smith-hooks:local" ]] && ! docker image inspect "$image" >/dev/null 2>&1; then
  echo "building hooks image $image..." >&2
  docker build -f docker/hooks.Dockerfile -t "$image" "$repo_root" >&2
fi

gomodcache="${GOMODCACHE:-${GOPATH:-$HOME/go}/pkg/mod}"
npmcache="${HOME}/.npm"
mkdir -p "$npmcache"

container_id="$(docker create \
  --workdir /workspace \
  -v "${gomodcache}:/go/pkg/mod" \
  -v "${npmcache}:/tmp/.npm" \
  --env HOME=/tmp \
  --env GOMODCACHE=/go/pkg/mod \
  --env npm_config_cache=/tmp/.npm \
  --env SKIP_GIT_HOOKS=1 \
  --env GOFLAGS=-buildvcs=false \
  "${image}" \
  /bin/bash -lc "export PATH=/usr/local/go/bin:\$PATH; find . -name '._*' -delete; make ${target}")"

cleanup() {
  docker rm -f "${container_id}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Copy source excluding large/platform-specific artifacts and macOS metadata
# Use tar to handle exclusion and avoid macOS extended attribute errors in Linux containers
# --no-xattr is required on macOS to avoid lsetxattr errors in the Linux container
tar -C "${repo_root}" --no-xattr --exclude="node_modules" --exclude=".svelte-kit" --exclude="build" --exclude="output" --exclude="._*" -cf - . | docker cp - "${container_id}:/workspace"

docker start -a "${container_id}"
