#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

SOAK_ITERATIONS="${SMITH_SOAK_ITERATIONS:-3}"
SOAK_INTERVAL_SECONDS="${SMITH_SOAK_INTERVAL_SECONDS:-60}"
ARTIFACTS_DIR="${SMITH_TEST_ARTIFACTS_DIR:-/tmp/smith-staging-artifacts}"

mkdir -p "$ARTIFACTS_DIR"

log() {
  echo "[staging-soak-chaos] $*"
}

for i in $(seq 1 "$SOAK_ITERATIONS"); do
  iter_dir="$ARTIFACTS_DIR/iter-$i"
  mkdir -p "$iter_dir"

  log "iteration ${i}/${SOAK_ITERATIONS}: run integration watch/reconcile"
  ./scripts/integration/test-watch-reconcile.sh | tee "$iter_dir/watch-reconcile.log"

  log "iteration ${i}/${SOAK_ITERATIONS}: run failure-injection scenarios"
  ./scripts/test/failure-injection.sh | tee "$iter_dir/failure-injection.log"

  log "iteration ${i}/${SOAK_ITERATIONS}: run matrix with cluster checks enabled"
  SMITH_ENABLE_CLUSTER_TESTS=true \
  SMITH_TEST_ARTIFACTS_DIR="$iter_dir/matrix-artifacts" \
  SMITH_FIXTURE_DIR="/tmp/smith-test-repo-${i}" \
  ./scripts/test/run-matrix.sh | tee "$iter_dir/run-matrix.log"

  if [[ "$i" -lt "$SOAK_ITERATIONS" ]]; then
    log "sleep ${SOAK_INTERVAL_SECONDS}s before next iteration"
    sleep "$SOAK_INTERVAL_SECONDS"
  fi
done

log "staging soak/chaos complete"
