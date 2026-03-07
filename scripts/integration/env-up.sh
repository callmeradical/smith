#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

K3D_CLUSTER_NAME="${SMITH_K3D_CLUSTER_NAME:-smith-int}"
K3D_SERVERS="${SMITH_K3D_SERVERS:-1}"
K3D_AGENTS="${SMITH_K3D_AGENTS:-2}"
K3D_PORT_HTTP="${SMITH_K3D_PORT_HTTP:-8080:80@loadbalancer}"
K3D_PORT_HTTPS="${SMITH_K3D_PORT_HTTPS:-8443:443@loadbalancer}"

VCLUSTER_NAME="${SMITH_VCLUSTER_NAME:-smith-vc}"
VCLUSTER_NAMESPACE="${SMITH_VCLUSTER_NAMESPACE:-smith-vcluster}"
VCLUSTER_CONNECT="${SMITH_VCLUSTER_CONNECT:-true}"

ETCD_NAMESPACE="${SMITH_ETCD_NAMESPACE:-smith-system}"
ETCD_RELEASE_NAME="${SMITH_ETCD_RELEASE_NAME:-smith-etcd}"
ETCD_CHART="${SMITH_ETCD_CHART:-bitnami/etcd}"
ETCD_VERSION="${SMITH_ETCD_VERSION:-}"
ETCD_STORAGE_CLASS="${SMITH_ETCD_STORAGE_CLASS:-local-path}"

info() { echo "[env-up] $*"; }
fail() { echo "[env-up] ERROR: $*" >&2; exit 1; }

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

need_cmd kubectl
need_cmd helm
need_cmd k3d
need_cmd vcluster

if ! k3d cluster list | awk 'NR>1 {print $1}' | rg -q "^${K3D_CLUSTER_NAME}$"; then
  info "creating k3d cluster ${K3D_CLUSTER_NAME}"
  k3d cluster create "$K3D_CLUSTER_NAME" \
    --servers "$K3D_SERVERS" \
    --agents "$K3D_AGENTS" \
    --port "$K3D_PORT_HTTP" \
    --port "$K3D_PORT_HTTPS"
else
  info "k3d cluster ${K3D_CLUSTER_NAME} already exists"
fi

kubectl cluster-info >/dev/null

# Some local Docker environments mark k3d nodes with disk-pressure taints.
# Clear these taints to keep local integration bootstrap deterministic.
kubectl taint nodes --all node.kubernetes.io/disk-pressure:NoSchedule- >/dev/null 2>&1 || true
sleep 2
if kubectl get nodes -o jsonpath='{range .items[*].spec.taints[*]}{.key}{"\n"}{end}' 2>/dev/null | rg -q '^node.kubernetes.io/disk-pressure$'; then
  fail "cluster nodes still report disk-pressure taints; free Docker disk space and retry"
fi

kubectl create namespace "$VCLUSTER_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
if ! kubectl -n "$VCLUSTER_NAMESPACE" get statefulset "$VCLUSTER_NAME" >/dev/null 2>&1 \
  && ! kubectl -n "$VCLUSTER_NAMESPACE" get statefulset "vc-${VCLUSTER_NAME}" >/dev/null 2>&1; then
  info "creating vcluster ${VCLUSTER_NAME}"
  vcluster create "$VCLUSTER_NAME" -n "$VCLUSTER_NAMESPACE" --connect=false
else
  info "vcluster ${VCLUSTER_NAME} already exists"
fi

if [[ "$VCLUSTER_CONNECT" == "true" ]]; then
  info "connecting kubectl context to vcluster"
  vcluster connect "$VCLUSTER_NAME" -n "$VCLUSTER_NAMESPACE" --update-current=false --background-proxy=false
  kubectl config use-context "vcluster_${VCLUSTER_NAME}_${VCLUSTER_NAMESPACE}_k3d-${K3D_CLUSTER_NAME}" >/dev/null
fi

helm repo add bitnami https://charts.bitnami.com/bitnami >/dev/null 2>&1 || true
helm repo update >/dev/null

kubectl create namespace "$ETCD_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null

info "installing/upgrading etcd release ${ETCD_RELEASE_NAME} in namespace ${ETCD_NAMESPACE}"
version_args=()
if [[ -n "$ETCD_VERSION" ]]; then
  version_args+=(--version "$ETCD_VERSION")
fi

helm upgrade --install "$ETCD_RELEASE_NAME" "$ETCD_CHART" \
  --namespace "$ETCD_NAMESPACE" \
  "${version_args[@]}" \
  --set auth.rbac.create=false \
  --set auth.token.enabled=false \
  --set replicaCount=1 \
  --set persistence.enabled=true \
  --set persistence.storageClass="$ETCD_STORAGE_CLASS" \
  --set persistence.size=2Gi \
  --set service.ports.client=2379 \
  --wait \
  --timeout 5m >/dev/null

kubectl -n "$ETCD_NAMESPACE" rollout status statefulset/${ETCD_RELEASE_NAME} --timeout=180s >/dev/null

info "environment ready"
info "k3d cluster: ${K3D_CLUSTER_NAME}"
info "vcluster: ${VCLUSTER_NAME} (${VCLUSTER_NAMESPACE})"
info "etcd endpoint (inside cluster): http://${ETCD_RELEASE_NAME}.${ETCD_NAMESPACE}.svc.cluster.local:2379"
