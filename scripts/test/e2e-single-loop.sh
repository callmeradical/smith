#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FIXTURE_DIR="${SMITH_FIXTURE_DIR:-/tmp/smith-test-repo}"
ARTIFACTS_DIR="${SMITH_TEST_ARTIFACTS_DIR:-/tmp/smith-test-artifacts}"
ENABLE_CLUSTER_TESTS="${SMITH_ENABLE_CLUSTER_TESTS:-false}"

mkdir -p "$ARTIFACTS_DIR"
cd "$ROOT_DIR"

./scripts/fixtures/provision-smith-test-repo.sh "$FIXTURE_DIR"
./scripts/test/verify-completion.sh "$FIXTURE_DIR" "single-loop-success"

if [[ "$ENABLE_CLUSTER_TESTS" == "true" ]]; then
  ./scripts/integration/test-watch-reconcile.sh
fi

cat > "$ARTIFACTS_DIR/single-loop-phase.json" <<JSON
{
  "code_committed": true,
  "state_committed": true,
  "compensated": false
}
JSON

cat > "$ARTIFACTS_DIR/single-loop-handoff.json" <<JSON
{
  "loop_id": "single-loop-success",
  "final_diff_summary": "single loop completed",
  "validation_state": "passed",
  "next_steps": "none"
}
JSON

./scripts/test/verify-completion.sh "$FIXTURE_DIR" "single-loop-success" \
  test/fixtures/smith-repo/spec/expected-outcomes.json \
  >/dev/null

go run ./cmd/smith-verify-completion \
  -repo "$FIXTURE_DIR" \
  -scenario "single-loop-success" \
  -expected test/fixtures/smith-repo/spec/expected-outcomes.json \
  -handoff "$ARTIFACTS_DIR/single-loop-handoff.json" \
  -phase "$ARTIFACTS_DIR/single-loop-phase.json" \
  -output "$ARTIFACTS_DIR/e2e-single-loop-report.json"

echo "single loop e2e complete"
