#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FIXTURE_DIR="${SMITH_FIXTURE_DIR:-/tmp/smith-test-repo}"
ARTIFACTS_DIR="${SMITH_TEST_ARTIFACTS_DIR:-/tmp/smith-test-artifacts}"

mkdir -p "$ARTIFACTS_DIR"

cd "$ROOT_DIR"

./scripts/fixtures/provision-smith-test-repo.sh "$FIXTURE_DIR"
go test ./internal/source/e2e -run TestConcurrentLoopsIsolation -count=1

./scripts/test/verify-completion.sh "$FIXTURE_DIR" "concurrent-safe-a"
./scripts/test/verify-completion.sh "$FIXTURE_DIR" "concurrent-safe-b"

{
  echo "concurrent_e2e:pass"
  echo "fixture_dir:$FIXTURE_DIR"
  echo "timestamp:$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} > "$ARTIFACTS_DIR/e2e-concurrent-summary.txt"

echo "concurrent loops e2e complete"
