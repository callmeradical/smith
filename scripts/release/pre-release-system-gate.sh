#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROFILE="${1:-}"
ARTIFACTS_DIR="${SMITH_TEST_ARTIFACTS_DIR:-/tmp/smith-pre-release-artifacts}"
FIXTURE_DIR="${SMITH_FIXTURE_DIR:-/tmp/smith-pre-release-repo}"

if [[ -z "$PROFILE" ]]; then
  echo "usage: $0 <vcluster|parity>" >&2
  exit 2
fi

mkdir -p "$ARTIFACTS_DIR"
: > "$ARTIFACTS_DIR/summary.txt"

cd "$ROOT_DIR"

run() {
  local label="$1"
  shift
  echo "[RUN] $label" | tee -a "$ARTIFACTS_DIR/summary.txt"
  "$@"
}

case "$PROFILE" in
  vcluster)
    run "vcluster-watch-reconcile" ./scripts/integration/test-watch-reconcile.sh
    run "vcluster-matrix" env \
      SMITH_ENABLE_CLUSTER_TESTS=true \
      SMITH_TEST_ARTIFACTS_DIR="$ARTIFACTS_DIR" \
      SMITH_FIXTURE_DIR="$FIXTURE_DIR" \
      ./scripts/test/run-matrix.sh
    ;;
  parity)
    run "non-vcluster-parity" env \
      SMITH_TEST_ARTIFACTS_DIR="$ARTIFACTS_DIR" \
      SMITH_FIXTURE_DIR="$FIXTURE_DIR" \
      ./scripts/test/parity-spot-check.sh
    ;;
  *)
    echo "invalid profile: $PROFILE (expected vcluster|parity)" >&2
    exit 2
    ;;
esac

run "recovery-override-validation" env \
  SMITH_TEST_ARTIFACTS_DIR="$ARTIFACTS_DIR" \
  ./scripts/test/validate-recovery-override.sh

echo "profile=$PROFILE" >> "$ARTIFACTS_DIR/summary.txt"
echo "pre-release system gate complete" | tee -a "$ARTIFACTS_DIR/summary.txt"
