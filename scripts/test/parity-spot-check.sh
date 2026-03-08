#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

ARTIFACTS_DIR="${SMITH_TEST_ARTIFACTS_DIR:-/tmp/smith-parity-artifacts}"
FIXTURE_DIR="${SMITH_FIXTURE_DIR:-/tmp/smith-parity-repo}"
mkdir -p "$ARTIFACTS_DIR"

# Non-vCluster parity spot-check: same harness without cluster mode.
SMITH_ENABLE_CLUSTER_TESTS=false \
SMITH_TEST_ARTIFACTS_DIR="$ARTIFACTS_DIR" \
SMITH_FIXTURE_DIR="$FIXTURE_DIR" \
./scripts/test/run-matrix.sh
