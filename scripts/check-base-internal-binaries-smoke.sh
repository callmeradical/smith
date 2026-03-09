#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:-loop-base:local}"
BINARY_LIST_FILE="${BINARY_LIST_FILE:-docker/base-internal-binaries.txt}"

if [[ ! -f "$BINARY_LIST_FILE" ]]; then
  echo "missing binary list file: $BINARY_LIST_FILE" >&2
  exit 1
fi

required_binaries="$(
  awk '
    /^[[:space:]]*#/ { next }
    /^[[:space:]]*$/ { next }
    { print $1 }
  ' "$BINARY_LIST_FILE"
)"

if [[ -z "$required_binaries" ]]; then
  echo "no required binaries found in: $BINARY_LIST_FILE" >&2
  exit 1
fi

required_binaries="$(echo "$required_binaries" | tr '\n' ' ')"

docker run --rm "$IMAGE" sh -lc "
missing=0
for bin in $required_binaries; do
  if ! command -v \"\$bin\" >/dev/null 2>&1; then
    echo \"missing internal binary: \$bin\" >&2
    missing=1
    continue
  fi
  if ! \"\$bin\" --version >/dev/null 2>&1; then
    echo \"internal binary version check failed: \$bin\" >&2
    missing=1
  fi
done

if [ \"\$missing\" -ne 0 ]; then
  exit 1
fi

echo \"internal binary smoke check passed\"
"
