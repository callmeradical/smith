#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:-loop-base:local}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARTIFACT_DIR="${REPO_ROOT}/artifacts"
SBOM_PATH="${ARTIFACT_DIR}/sbom-loop-base.spdx.json"

cd "$REPO_ROOT"
mkdir -p "$ARTIFACT_DIR" tmp-skills

printf '\n==> docker build -t %s .\n' "$IMAGE"
docker build -t "$IMAGE" .

printf '\n==> docker run --rm %s sh -lc '\''codex --version && git --version && node --version && python3 --version && rg --version'\''\n' "$IMAGE"
docker run --rm "$IMAGE" sh -lc 'codex --version && git --version && node --version && python3 --version && rg --version'

printf '\n==> docker run --rm -v %s/tmp-skills:/home/dev/.codex/skills %s sh -lc '\''test -d /home/dev/.codex/skills && ls -la /home/dev/.codex/skills'\''\n' "$REPO_ROOT" "$IMAGE"
docker run --rm -v "$(pwd)/tmp-skills:/home/dev/.codex/skills" "$IMAGE" sh -lc 'test -d /home/dev/.codex/skills && ls -la /home/dev/.codex/skills'

printf '\n==> hadolint Dockerfile\n'
hadolint Dockerfile

printf '\n==> shellcheck scripts/*.sh\n'
shellcheck scripts/*.sh

printf '\n==> trivy image --severity CRITICAL --exit-code 1 %s\n' "$IMAGE"
trivy image --severity CRITICAL --exit-code 1 "$IMAGE"

printf '\n==> syft packages %s -o spdx-json > artifacts/sbom-loop-base.spdx.json\n' "$IMAGE"
syft packages "$IMAGE" -o spdx-json > "$SBOM_PATH"

printf '\nAll base-image quality gates passed. SBOM written to %s\n' "$SBOM_PATH"
