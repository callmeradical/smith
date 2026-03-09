#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:-loop-base:local}"
REQUIRED_TOOLS="${REQUIRED_TOOLS:-git curl jq make node npm pnpm python3 pip rg bash}"

docker run --rm "$IMAGE" sh -lc "
missing=0
for tool in $REQUIRED_TOOLS; do
  if ! command -v \"\$tool\" >/dev/null 2>&1; then
    echo \"missing binary: \$tool\" >&2
    missing=1
  fi
done

if [ \"\$missing\" -ne 0 ]; then
  exit 1
fi

echo \"tooling smoke check passed\"
"
