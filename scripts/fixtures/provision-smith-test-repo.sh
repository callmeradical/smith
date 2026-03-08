#!/usr/bin/env bash
set -euo pipefail

TARGET_DIR="${1:-/tmp/smith-test-repo}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

"$ROOT_DIR/test/fixtures/smith-repo/scripts/provision.sh" "$TARGET_DIR"
