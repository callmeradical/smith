#!/usr/bin/env bash
set -euo pipefail

# Transitional wrapper for Go-native harness migration.
# RETIRE_AFTER=2026-06-30

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ARTIFACTS_DIR="${SMITH_TEST_ARTIFACTS_DIR:-/tmp/smith-test-artifacts}"

mkdir -p "$ARTIFACTS_DIR"

cd "$ROOT_DIR"

go test ./internal/source/e2e -run TestLoopSkillMountBehavior -count=1

{
  echo "skill_mounts_e2e:pass"
  echo "timestamp:$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} > "$ARTIFACTS_DIR/e2e-skill-mounts-summary.txt"

echo "skill mounts e2e complete"
