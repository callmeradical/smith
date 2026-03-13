#!/usr/bin/env bash
set -euo pipefail

TARGET_DIR="${1:-/tmp/smith-test-repo}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"$SCRIPT_DIR/provision.sh" "$TARGET_DIR" >/dev/null

echo "fixture reset complete: $TARGET_DIR"
