#!/bin/bash
set -e

# List of required workflows
WORKFLOWS=(
  "build-smith.yml"
  "build-api.yml"
  "build-chat.yml"
  "build-core.yml"
  "build-replica.yml"
  "build-ctl.yml"
  "build-verify-completion.yml"
  "build-console.yml"
  "platform-gate.yml"
  "helm-publish.yml"
  "images-build-publish.yml"
)

MISSING=0
for wf in "${WORKFLOWS[@]}"; do
  if [ ! -f ".github/workflows/$wf" ]; then
    echo "ERROR: Missing workflow .github/workflows/$wf"
    MISSING=1
  fi
done

if [ $MISSING -eq 1 ]; then
  echo "Validation failed: Some workflows are missing."
  exit 1
fi

echo "All required workflows are present."
