#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:-knqyf263/vuln-image:1.2.3}"

printf '==> trivy image --severity CRITICAL --exit-code 1 %s (expected to fail)\n' "$IMAGE"
if trivy image --severity CRITICAL --exit-code 1 "$IMAGE"; then
  echo "expected trivy gate to fail for known vulnerable image: $IMAGE" >&2
  exit 1
fi

echo "trivy negative check passed: critical gate failed as expected"
