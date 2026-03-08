#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ARTIFACTS_DIR="${SMITH_TEST_ARTIFACTS_DIR:-/tmp/smith-test-artifacts}"
mkdir -p "$ARTIFACTS_DIR"

cd "$ROOT_DIR"

run() {
  local label="$1"
  shift
  echo "[RUN] $label"
  "$@"
}

# Recovery-path validation: completion saga and reconcile drift behavior.
run "recovery-failure-injection" ./scripts/test/failure-injection.sh

# Override-path validation: ensure operator cancel/override request path remains wired.
run "override-cli-path" go test ./cmd/smithctl -run TestLoopCancelBatchPostsOverride -count=1

{
  echo "recovery_override_validation:pass"
  echo "timestamp:$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} > "$ARTIFACTS_DIR/recovery-override-validation.txt"

echo "recovery + override validation complete"
