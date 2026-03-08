#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ARTIFACTS_DIR="${SMITH_TEST_ARTIFACTS_DIR:-/tmp/smith-test-artifacts}"
FIXTURE_DIR="${SMITH_FIXTURE_DIR:-/tmp/smith-test-repo}"
ENABLE_CLUSTER_TESTS="${SMITH_ENABLE_CLUSTER_TESTS:-false}"

mkdir -p "$ARTIFACTS_DIR"
: > "$ARTIFACTS_DIR/summary.txt"

run() {
  local label="$1"
  shift
  echo "[RUN] $label" | tee -a "$ARTIFACTS_DIR/summary.txt"
  "$@"
}

cd "$ROOT_DIR"

run "unit-core" go test ./internal/source/core/...
run "unit-locking" go test ./internal/source/locking/...
run "unit-completion-failure-injection" go test ./internal/source/completion/...
run "unit-reconcile-failure-injection" go test ./internal/source/reconcile/...
run "model-helpers" go test ./internal/source/model/...

run "fixture-provision" ./scripts/fixtures/provision-smith-test-repo.sh "$FIXTURE_DIR"
run "fixture-verify" ./scripts/fixtures/verify-smith-test-repo.sh "$FIXTURE_DIR"
run "completion-verify-single" ./scripts/test/verify-completion.sh "$FIXTURE_DIR" "single-loop-success"
run "completion-verify-concurrent-a" ./scripts/test/verify-completion.sh "$FIXTURE_DIR" "concurrent-safe-a"
run "completion-verify-concurrent-b" ./scripts/test/verify-completion.sh "$FIXTURE_DIR" "concurrent-safe-b"
run "completion-verify-merge-conflict" ./scripts/test/verify-completion.sh "$FIXTURE_DIR" "merge-conflict"
run "e2e-single-loop" ./scripts/test/e2e-single-loop.sh
run "e2e-concurrent-loops" ./scripts/test/e2e-concurrent-loops.sh
run "e2e-ingress-modes" ./scripts/test/e2e-ingress-modes.sh
run "e2e-environment-modes" ./scripts/test/e2e-environment-modes.sh
run "e2e-skill-mounts" ./scripts/test/e2e-skill-mounts.sh

cp "$ROOT_DIR/test/fixtures/smith-repo/spec/expected-outcomes.json" "$ARTIFACTS_DIR/expected-outcomes.json"
git -C "$FIXTURE_DIR" branch --list > "$ARTIFACTS_DIR/fixture-branches.txt"
git -C "$FIXTURE_DIR" log --oneline --decorate --all > "$ARTIFACTS_DIR/fixture-git-log.txt"

if [[ "$ENABLE_CLUSTER_TESTS" == "true" ]]; then
  if command -v kubectl >/dev/null 2>&1 && kubectl cluster-info >/dev/null 2>&1; then
    echo "[RUN] cluster-tests placeholder: environment is reachable" | tee -a "$ARTIFACTS_DIR/summary.txt"
  else
    echo "[WARN] cluster tests requested but no reachable Kubernetes cluster" | tee -a "$ARTIFACTS_DIR/summary.txt"
    exit 1
  fi
else
  echo "[INFO] cluster tests skipped (SMITH_ENABLE_CLUSTER_TESTS=false)" | tee -a "$ARTIFACTS_DIR/summary.txt"
fi

echo "matrix complete" | tee -a "$ARTIFACTS_DIR/summary.txt"
