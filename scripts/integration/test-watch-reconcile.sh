#!/usr/bin/env bash
set -euo pipefail

VCLUSTER_NAME="${SMITH_VCLUSTER_NAME:-smith-vc}"
VCLUSTER_NAMESPACE="${SMITH_VCLUSTER_NAMESPACE:-smith-vcluster}"
ETCD_NAMESPACE="${SMITH_ETCD_NAMESPACE:-smith-system}"
ETCD_RELEASE_NAME="${SMITH_ETCD_RELEASE_NAME:-smith-etcd}"

cleanup() {
  if [[ -n "${PF_PID:-}" ]]; then
    kill "$PF_PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

vcluster connect "$VCLUSTER_NAME" -n "$VCLUSTER_NAMESPACE" --update-current=true --background-proxy=true

kubectl -n "$ETCD_NAMESPACE" get svc "$ETCD_RELEASE_NAME" >/dev/null
kubectl -n "$ETCD_NAMESPACE" port-forward svc/"$ETCD_RELEASE_NAME" 2379:2379 >/tmp/smith-it-port-forward.log 2>&1 &
PF_PID=$!

for _ in $(seq 1 20); do
  if (echo > /dev/tcp/127.0.0.1/2379) >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

SMITH_IT_ENABLE=true \
SMITH_IT_NAMESPACE="$ETCD_NAMESPACE" \
SMITH_IT_ETCD_ENDPOINTS="http://127.0.0.1:2379" \
go test -tags integration ./internal/source/integration -run TestVClusterWatchReconcile -count=1 -v
