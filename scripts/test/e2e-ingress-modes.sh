#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ARTIFACTS_DIR="${SMITH_TEST_ARTIFACTS_DIR:-/tmp/smith-test-artifacts}"

mkdir -p "$ARTIFACTS_DIR"

cd "$ROOT_DIR"

go test ./internal/source/e2e -run TestIngressModesLoopCreationAndExecution -count=1

{
  echo "ingress_modes_e2e:pass"
  echo "timestamp:$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} > "$ARTIFACTS_DIR/e2e-ingress-summary.txt"

echo "ingress modes e2e complete"
