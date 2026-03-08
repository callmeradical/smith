#!/usr/bin/env bash
set -euo pipefail

REPO_PATH="${1:-/tmp/smith-test-repo}"
SCENARIO_ID="${2:-single-loop-success}"
EXPECTED_PATH="${3:-test/fixtures/smith-repo/spec/expected-outcomes.json}"
HANDOFF_PATH="${SMITH_VERIFY_HANDOFF_PATH:-}"
PHASE_PATH="${SMITH_VERIFY_PHASE_PATH:-}"
OUTPUT_PATH="${SMITH_VERIFY_OUTPUT_PATH:-/tmp/smith-test-artifacts/completion-report-${SCENARIO_ID}.json}"

mkdir -p "$(dirname "$OUTPUT_PATH")"

go run ./cmd/smith-verify-completion \
  -repo "$REPO_PATH" \
  -scenario "$SCENARIO_ID" \
  -expected "$EXPECTED_PATH" \
  -handoff "$HANDOFF_PATH" \
  -phase "$PHASE_PATH" \
  -output "$OUTPUT_PATH"
