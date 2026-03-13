#!/usr/bin/env bash
set -euo pipefail

K3D_CLUSTER_NAME="${SMITH_K3D_CLUSTER_NAME:-smith-int}"
VCLUSTER_NAME="${SMITH_VCLUSTER_NAME:-smith-vc}"
VCLUSTER_NAMESPACE="${SMITH_VCLUSTER_NAMESPACE:-smith-vcluster}"
ETCD_NAMESPACE="${SMITH_ETCD_NAMESPACE:-smith-system}"
ETCD_RELEASE_NAME="${SMITH_ETCD_RELEASE_NAME:-smith-etcd}"
USE_VCLUSTER="${SMITH_USE_VCLUSTER:-false}"

info() { echo "[env-down] $*"; }

run_with_timeout() {
  local seconds="$1"
  shift
  (
    "$@" &
    local pid=$!
    (
      sleep "$seconds"
      kill -TERM "$pid" >/dev/null 2>&1 || true
    ) &
    local watcher=$!
    wait "$pid" >/dev/null 2>&1 || true
    kill -TERM "$watcher" >/dev/null 2>&1 || true
  )
}

if command -v helm >/dev/null 2>&1; then
  run_with_timeout 60 helm uninstall "$ETCD_RELEASE_NAME" -n "$ETCD_NAMESPACE"
fi

if [[ "$USE_VCLUSTER" == "true" ]] && command -v vcluster >/dev/null 2>&1; then
  run_with_timeout 90 vcluster delete "$VCLUSTER_NAME" -n "$VCLUSTER_NAMESPACE"
fi

if command -v kubectl >/dev/null 2>&1; then
  run_with_timeout 45 kubectl delete namespace "$ETCD_NAMESPACE" --ignore-not-found
  if [[ "$USE_VCLUSTER" == "true" ]]; then
    run_with_timeout 45 kubectl delete namespace "$VCLUSTER_NAMESPACE" --ignore-not-found
  fi
fi

if command -v k3d >/dev/null 2>&1; then
  info "deleting k3d cluster ${K3D_CLUSTER_NAME}"
  run_with_timeout 90 k3d cluster delete "$K3D_CLUSTER_NAME"
fi

info "environment removed"
