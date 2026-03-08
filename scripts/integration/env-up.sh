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
USE_VCLUSTER="${SMITH_USE_VCLUSTER:-true}"

ETCD_NAMESPACE="${SMITH_ETCD_NAMESPACE:-smith-system}"
ETCD_RELEASE_NAME="${SMITH_ETCD_RELEASE_NAME:-smith-etcd}"
ETCD_CHART="${SMITH_ETCD_CHART:-bitnami/etcd}"
ETCD_VERSION="${SMITH_ETCD_VERSION:-}"
ETCD_STORAGE_CLASS="${SMITH_ETCD_STORAGE_CLASS:-local-path}"
ETCD_PERSISTENCE_ENABLED="${SMITH_ETCD_PERSISTENCE_ENABLED:-false}"
ETCD_WAIT_TIMEOUT="${SMITH_ETCD_WAIT_TIMEOUT:-8m}"
ETCD_MODE="${SMITH_ETCD_MODE:-simple}"
ETCD_IMAGE="${SMITH_ETCD_IMAGE:-quay.io/coreos/etcd:v3.5.17}"

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
if [[ "$USE_VCLUSTER" == "true" ]]; then
  need_cmd vcluster
fi

if ! k3d cluster list | awk 'NR>1 {print $1}' | grep -q "^${K3D_CLUSTER_NAME}$"; then
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
if kubectl get nodes -o jsonpath='{range .items[*].spec.taints[*]}{.key}{"\n"}{end}' 2>/dev/null | grep -q '^node.kubernetes.io/disk-pressure$'; then
  fail "cluster nodes still report disk-pressure taints; free Docker disk space and retry"
fi

if [[ "$USE_VCLUSTER" == "true" ]]; then
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
    export SMITH_VCLUSTER_KUBECONFIG="${SMITH_VCLUSTER_KUBECONFIG:-/tmp/${VCLUSTER_NAME}-kubeconfig.yaml}"
    vcluster connect "$VCLUSTER_NAME" -n "$VCLUSTER_NAMESPACE" --print > "$SMITH_VCLUSTER_KUBECONFIG"
    export KUBECONFIG="$SMITH_VCLUSTER_KUBECONFIG"
  fi
else
  info "SMITH_USE_VCLUSTER=false; using direct k3d namespace deployment profile"
fi

kubectl create namespace "$ETCD_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null

if [[ "$ETCD_MODE" == "helm" ]]; then
  info "installing/upgrading etcd via Helm release ${ETCD_RELEASE_NAME} in namespace ${ETCD_NAMESPACE}"
  helm repo add bitnami https://charts.bitnami.com/bitnami >/dev/null 2>&1 || true
  helm repo update >/dev/null

  version_args=()
  if [[ -n "$ETCD_VERSION" ]]; then
    version_args+=(--version "$ETCD_VERSION")
  fi
  persistence_args=(--set persistence.enabled="$ETCD_PERSISTENCE_ENABLED")
  if [[ "$ETCD_PERSISTENCE_ENABLED" == "true" ]]; then
    persistence_args+=(--set persistence.storageClass="$ETCD_STORAGE_CLASS" --set persistence.size=2Gi)
  fi

  helm upgrade --install "$ETCD_RELEASE_NAME" "$ETCD_CHART" \
    --namespace "$ETCD_NAMESPACE" \
    "${version_args[@]}" \
    "${persistence_args[@]}" \
    --set auth.rbac.create=false \
    --set auth.token.enabled=false \
    --set replicaCount=1 \
    --set service.ports.client=2379 \
    --wait \
    --timeout "$ETCD_WAIT_TIMEOUT" >/dev/null
else
  info "installing/upgrading standalone etcd deployment ${ETCD_RELEASE_NAME} (${ETCD_IMAGE}) in namespace ${ETCD_NAMESPACE}"
  cat <<YAML | kubectl -n "$ETCD_NAMESPACE" apply -f - >/dev/null
apiVersion: v1
kind: Service
metadata:
  name: ${ETCD_RELEASE_NAME}
spec:
  selector:
    app.kubernetes.io/name: ${ETCD_RELEASE_NAME}
  ports:
    - name: client
      port: 2379
      targetPort: 2379
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${ETCD_RELEASE_NAME}
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: ${ETCD_RELEASE_NAME}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: ${ETCD_RELEASE_NAME}
    spec:
      containers:
        - name: etcd
          image: ${ETCD_IMAGE}
          imagePullPolicy: IfNotPresent
          command: ["/usr/local/bin/etcd"]
          args:
            - --name=default
            - --data-dir=/etcd-data
            - --listen-client-urls=http://0.0.0.0:2379
            - --advertise-client-urls=http://${ETCD_RELEASE_NAME}.${ETCD_NAMESPACE}.svc.cluster.local:2379
          ports:
            - containerPort: 2379
              name: client
          readinessProbe:
            tcpSocket:
              port: 2379
            initialDelaySeconds: 5
            periodSeconds: 5
          livenessProbe:
            tcpSocket:
              port: 2379
            initialDelaySeconds: 10
            periodSeconds: 10
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi
          volumeMounts:
            - name: data
              mountPath: /etcd-data
      volumes:
        - name: data
          emptyDir: {}
YAML
fi

if [[ "$ETCD_MODE" == "helm" ]]; then
  kubectl -n "$ETCD_NAMESPACE" rollout status statefulset/${ETCD_RELEASE_NAME} --timeout=300s >/dev/null || {
    echo "[env-up] etcd failed to become ready; diagnostics:" >&2
    kubectl -n "$ETCD_NAMESPACE" get pods -o wide >&2 || true
    kubectl -n "$ETCD_NAMESPACE" describe statefulset "$ETCD_RELEASE_NAME" >&2 || true
    kubectl -n "$ETCD_NAMESPACE" describe pods >&2 || true
    kubectl -n "$ETCD_NAMESPACE" logs statefulset/"$ETCD_RELEASE_NAME" --all-containers --tail=200 >&2 || true
    exit 1
  }
else
  kubectl -n "$ETCD_NAMESPACE" rollout status deployment/${ETCD_RELEASE_NAME} --timeout=300s >/dev/null || {
    echo "[env-up] etcd failed to become ready; diagnostics:" >&2
    kubectl -n "$ETCD_NAMESPACE" get pods -o wide >&2 || true
    kubectl -n "$ETCD_NAMESPACE" describe deployment "$ETCD_RELEASE_NAME" >&2 || true
    kubectl -n "$ETCD_NAMESPACE" describe pods >&2 || true
    kubectl -n "$ETCD_NAMESPACE" logs deployment/"$ETCD_RELEASE_NAME" --all-containers --tail=200 >&2 || true
    exit 1
  }
fi

if [[ "$ETCD_MODE" != "helm" ]]; then
  kubectl -n "$ETCD_NAMESPACE" get svc "$ETCD_RELEASE_NAME" >/dev/null
fi

info "environment ready"
info "k3d cluster: ${K3D_CLUSTER_NAME}"
if [[ "$USE_VCLUSTER" == "true" ]]; then
  info "vcluster: ${VCLUSTER_NAME} (${VCLUSTER_NAMESPACE})"
else
  info "vcluster: disabled (direct k3d profile)"
fi
info "etcd endpoint (inside cluster): http://${ETCD_RELEASE_NAME}.${ETCD_NAMESPACE}.svc.cluster.local:2379"
