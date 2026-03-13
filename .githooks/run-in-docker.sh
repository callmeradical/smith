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

gomodcache="${GOMODCACHE:-${GOPATH:-$HOME/go}/pkg/mod}"

container_id="$(docker create \
  --workdir /workspace \
  -v "${gomodcache}:/go/pkg/mod" \
  --env HOME=/tmp \
  --env GOMODCACHE=/go/pkg/mod \
  --env SKIP_GIT_HOOKS=1 \
  --env GOFLAGS=-buildvcs=false \
  "${image}" \
  /bin/bash -lc "export PATH=/usr/local/go/bin:\$PATH; \
    # WORKAROUND: Delete macOS metadata files that can cause Playwright syntax errors in Linux containers
    # TODO: Refine tar/cp logic to prevent these from ever entering the container
    find . -name '._*' -delete; \
    make ${target}")"

cleanup() {
  docker rm -f "${container_id}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Copy source excluding large/platform-specific artifacts and macOS metadata
# Use tar to handle exclusion and avoid macOS extended attribute errors in Linux containers
# --no-xattr is required on macOS to avoid lsetxattr errors in the Linux container
tar -C "${repo_root}" --no-xattr --exclude="node_modules" --exclude=".svelte-kit" --exclude="build" --exclude="output" --exclude="._*" -cf - . | docker cp - "${container_id}:/workspace"

docker start -a "${container_id}"
