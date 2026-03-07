#!/usr/bin/env bash
set -euo pipefail

VCLUSTER_NAME="${SMITH_VCLUSTER_NAME:-smith-vc}"
VCLUSTER_NAMESPACE="${SMITH_VCLUSTER_NAMESPACE:-smith-vcluster}"

if ! command -v vcluster >/dev/null 2>&1; then
  echo "missing required command: vcluster" >&2
  exit 1
fi

if ! kubectl -n "$VCLUSTER_NAMESPACE" get statefulset "$VCLUSTER_NAME" >/dev/null 2>&1; then
  echo "vcluster not found; run ./scripts/integration/env-up.sh first" >&2
  exit 1
fi

vcluster connect "$VCLUSTER_NAME" -n "$VCLUSTER_NAMESPACE" --update-current=false --background-proxy=false
kubectl config use-context "vcluster_${VCLUSTER_NAME}_${VCLUSTER_NAMESPACE}_k3d-${SMITH_K3D_CLUSTER_NAME:-smith-int}" >/dev/null

SMITH_ENABLE_CLUSTER_TESTS=true ./scripts/test/run-matrix.sh
