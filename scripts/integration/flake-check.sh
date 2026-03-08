#!/usr/bin/env bash
set -euo pipefail

RUNS="${SMITH_IT_FLAKE_RUNS:-3}"
PASSED=0
FAILED=0

for i in $(seq 1 "$RUNS"); do
  echo "[flake-check] run ${i}/${RUNS}"
  if ./scripts/integration/test-watch-reconcile.sh; then
    PASSED=$((PASSED + 1))
  else
    FAILED=$((FAILED + 1))
  fi
done

echo "[flake-check] passed=${PASSED} failed=${FAILED} total=${RUNS}"
if [[ "$FAILED" -gt 0 ]]; then
  exit 1
fi
