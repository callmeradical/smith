#!/usr/bin/env bash
set -euo pipefail

K3D_CLUSTER_NAME="${SMITH_K3D_CLUSTER_NAME:-smith-int}"
K3D_SERVER_CONTAINER="k3d-${K3D_CLUSTER_NAME}-server-0"
K3D_SERVERLB_CONTAINER="k3d-${K3D_CLUSTER_NAME}-serverlb"
SMITH_NAMESPACE="${SMITH_NAMESPACE:-smith-system}"
ENCRYPTION_CONFIG_PATH="/etc/rancher/k3s/encryption-config.yaml"
K3S_CONFIG_FRAGMENT_PATH="/etc/rancher/k3s/config.yaml.d/90-secrets-encryption.yaml"

info() { echo "[enable-secrets-encryption] $*"; }
fail() { echo "[enable-secrets-encryption] ERROR: $*" >&2; exit 1; }

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "missing required command: $1"
  fi
}

need_cmd docker
need_cmd kubectl

if ! docker ps --format '{{.Names}}' | grep -qx "$K3D_SERVER_CONTAINER"; then
  fail "server container '$K3D_SERVER_CONTAINER' not found"
fi
if ! docker ps --format '{{.Names}}' | grep -qx "$K3D_SERVERLB_CONTAINER"; then
  fail "load balancer container '$K3D_SERVERLB_CONTAINER' not found"
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

if docker exec "$K3D_SERVER_CONTAINER" sh -lc "test -s '$ENCRYPTION_CONFIG_PATH'"; then
  info "reusing existing encryption provider config at $ENCRYPTION_CONFIG_PATH"
else
  info "creating new encryption provider config"
  ENC_KEY_B64="$(head -c 32 /dev/urandom | base64 | tr -d '\n')"
  cat >"$TMP_DIR/encryption-config.yaml" <<EOC
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
      - secrets
    providers:
      - aescbc:
          keys:
            - name: key1
              secret: ${ENC_KEY_B64}
      - identity: {}
EOC
  docker cp "$TMP_DIR/encryption-config.yaml" "$K3D_SERVER_CONTAINER:$ENCRYPTION_CONFIG_PATH"
fi

cat >"$TMP_DIR/90-secrets-encryption.yaml" <<EOC
kube-apiserver-arg:
  - encryption-provider-config=${ENCRYPTION_CONFIG_PATH}
EOC
docker exec "$K3D_SERVER_CONTAINER" sh -lc 'mkdir -p /etc/rancher/k3s/config.yaml.d'
docker cp "$TMP_DIR/90-secrets-encryption.yaml" "$K3D_SERVER_CONTAINER:$K3S_CONFIG_FRAGMENT_PATH"
docker exec "$K3D_SERVER_CONTAINER" sh -lc "chmod 600 '$ENCRYPTION_CONFIG_PATH' '$K3S_CONFIG_FRAGMENT_PATH'"

info "restarting k3s server container"
docker restart "$K3D_SERVER_CONTAINER" >/dev/null

# k3d server restart can change container IP; restart serverlb to pick up target IP.
info "restarting k3d server load balancer container"
docker restart "$K3D_SERVERLB_CONTAINER" >/dev/null

info "waiting for kubernetes api availability"
for _ in $(seq 1 60); do
  if kubectl get nodes >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

kubectl wait --for=condition=Ready "node/$K3D_SERVER_CONTAINER" --timeout=180s >/dev/null

if kubectl get namespace "$SMITH_NAMESPACE" >/dev/null 2>&1; then
  info "rewriting secrets in namespace '$SMITH_NAMESPACE'"
  kubectl -n "$SMITH_NAMESPACE" get secret -o name | while read -r secret_name; do
    kubectl -n "$SMITH_NAMESPACE" annotate --overwrite "$secret_name" \
      smith.dev/encryption-rewrite="$(date +%s)" >/dev/null
  done
else
  info "namespace '$SMITH_NAMESPACE' not found; skipping rewrite"
fi

MARKER="smithenc-$(date +%s)-$RANDOM"
MARKER_B64="$(printf '%s' "$MARKER" | base64 | tr -d '\n')"

kubectl -n "$SMITH_NAMESPACE" create secret generic smith-encryption-probe \
  --from-literal=probe="$MARKER" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

PLAINTEXT_MATCH="$(docker exec "$K3D_SERVER_CONTAINER" sh -lc "grep -a -n '$MARKER' /var/lib/rancher/k3s/server/db/state.db /var/lib/rancher/k3s/server/db/state.db-wal 2>/dev/null | head -n1 || true")"
BASE64_MATCH="$(docker exec "$K3D_SERVER_CONTAINER" sh -lc "grep -a -n '$MARKER_B64' /var/lib/rancher/k3s/server/db/state.db /var/lib/rancher/k3s/server/db/state.db-wal 2>/dev/null | head -n1 || true")"

if [[ -n "$PLAINTEXT_MATCH" || -n "$BASE64_MATCH" ]]; then
  fail "encryption probe marker still visible in datastore; plaintext='$PLAINTEXT_MATCH' base64='$BASE64_MATCH'"
fi

info "secret encryption provider enabled and validated"
info "probe secret: smith-encryption-probe"
