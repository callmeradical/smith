#!/usr/bin/env bash
set -euo pipefail

SMITH_ETCD_NAMESPACE="${SMITH_ETCD_NAMESPACE:-smith-system}"
SMITH_ETCD_RELEASE_NAME="${SMITH_ETCD_RELEASE_NAME:-smith-etcd}"
SMITH_DR_ARTIFACTS_DIR="${SMITH_DR_ARTIFACTS_DIR:-/tmp/smith-dr}"
SMITH_DR_TIMEOUT="${SMITH_DR_TIMEOUT:-180s}"
SMITH_DR_DRY_RUN="${SMITH_DR_DRY_RUN:-false}"

info() { echo "[dr-drill] $*"; }
fail() { echo "[dr-drill] ERROR: $*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

run_or_echo() {
  if [[ "$SMITH_DR_DRY_RUN" == "true" ]]; then
    echo "[dry-run] $*"
    return 0
  fi
  "$@"
}

need_cmd kubectl
need_cmd date

mkdir -p "$SMITH_DR_ARTIFACTS_DIR"
snapshot_file="${SMITH_DR_ARTIFACTS_DIR}/snapshot-$(date -u +%Y%m%dT%H%M%SZ).db"
report_file="${SMITH_DR_ARTIFACTS_DIR}/dr-report.json"
canary_loop="dr-loop-$(date -u +%Y%m%d%H%M%S)"

detect_etcd_pod() {
  local pod=""
  pod="$(kubectl -n "$SMITH_ETCD_NAMESPACE" get pods -l app.kubernetes.io/name="$SMITH_ETCD_RELEASE_NAME" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [[ -n "$pod" ]]; then
    echo "$pod"
    return
  fi
  pod="$(kubectl -n "$SMITH_ETCD_NAMESPACE" get pods -l app.kubernetes.io/instance="$SMITH_ETCD_RELEASE_NAME" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  echo "$pod"
}

if [[ "$SMITH_DR_DRY_RUN" == "true" ]]; then
  etcd_pod="dry-run-etcd-pod"
else
  etcd_pod="$(detect_etcd_pod)"
  [[ -n "$etcd_pod" ]] || fail "could not find etcd pod in namespace ${SMITH_ETCD_NAMESPACE}"
fi

info "using etcd pod: ${etcd_pod}"
info "artifacts: ${SMITH_DR_ARTIFACTS_DIR}"

snapshot_started_at="$(date -u +%s)"
run_or_echo kubectl -n "$SMITH_ETCD_NAMESPACE" exec "$etcd_pod" -- sh -ceu \
  "ETCDCTL_API=3 etcdctl --endpoints=http://127.0.0.1:2379 put /smith/v1/anomalies/${canary_loop} '{\"id\":\"${canary_loop}\",\"title\":\"DR Canary\",\"description\":\"dr canary\",\"source_type\":\"dr\",\"source_ref\":\"dr/${canary_loop}\",\"provider_id\":\"codex\",\"model\":\"gpt-5\",\"environment\":{},\"policy\":{\"max_attempts\":1,\"backoff_initial\":0,\"backoff_max\":0,\"timeout\":0,\"terminate_on_error\":false},\"correlation_id\":\"corr-${canary_loop}\",\"schema_version\":\"v1\"}' >/dev/null && \
   ETCDCTL_API=3 etcdctl --endpoints=http://127.0.0.1:2379 put /smith/v1/state/${canary_loop} '{\"loop_id\":\"${canary_loop}\",\"state\":\"overwriting\",\"attempt\":1,\"reason\":\"dr-canary\",\"correlation_id\":\"corr-${canary_loop}\",\"schema_version\":\"v1\"}' >/dev/null && \
   ETCDCTL_API=3 etcdctl --endpoints=http://127.0.0.1:2379 snapshot save /tmp/smith-dr-snapshot.db >/dev/null"

run_or_echo kubectl -n "$SMITH_ETCD_NAMESPACE" cp "${etcd_pod}:/tmp/smith-dr-snapshot.db" "$snapshot_file"
snapshot_completed_at="$(date -u +%s)"

outage_started_at="$(date -u +%s)"
run_or_echo kubectl -n "$SMITH_ETCD_NAMESPACE" delete pod "$etcd_pod" --wait=true
run_or_echo kubectl -n "$SMITH_ETCD_NAMESPACE" wait --for=condition=Ready "pod/${etcd_pod}" --timeout="$SMITH_DR_TIMEOUT" || true
if [[ "$SMITH_DR_DRY_RUN" == "true" ]]; then
  replacement_pod="dry-run-etcd-pod-recovered"
else
  replacement_pod="$(detect_etcd_pod)"
  [[ -n "$replacement_pod" ]] || fail "etcd pod did not recover after outage simulation"
fi

run_or_echo kubectl -n "$SMITH_ETCD_NAMESPACE" wait --for=condition=Ready "pod/${replacement_pod}" --timeout="$SMITH_DR_TIMEOUT"
outage_recovered_at="$(date -u +%s)"

restore_validation_started_at="$(date -u +%s)"
run_or_echo kubectl -n "$SMITH_ETCD_NAMESPACE" exec "$replacement_pod" -- sh -ceu "
  rm -rf /tmp/smith-dr-restore-data /tmp/smith-dr-restore.log
  ETCDCTL_API=3 etcdutl snapshot restore /tmp/smith-dr-snapshot.db --data-dir=/tmp/smith-dr-restore-data --skip-hash-check >/tmp/smith-dr-restore.log 2>&1
  etcd --data-dir=/tmp/smith-dr-restore-data \
    --name=dr-restore \
    --listen-client-urls=http://127.0.0.1:22379 \
    --advertise-client-urls=http://127.0.0.1:22379 \
    --listen-peer-urls=http://127.0.0.1:22380 \
    --initial-advertise-peer-urls=http://127.0.0.1:22380 \
    --initial-cluster=dr-restore=http://127.0.0.1:22380 \
    --initial-cluster-state=new >/tmp/smith-dr-restore-etcd.log 2>&1 &
  pid=\$!
  trap 'kill \$pid >/dev/null 2>&1 || true' EXIT
  for _ in \$(seq 1 30); do
    ETCDCTL_API=3 etcdctl --endpoints=http://127.0.0.1:22379 endpoint health >/dev/null 2>&1 && break
    sleep 1
  done
  ETCDCTL_API=3 etcdctl --endpoints=http://127.0.0.1:22379 get /smith/v1/state/${canary_loop} >/tmp/smith-dr-canary.txt
  grep -q '${canary_loop}' /tmp/smith-dr-canary.txt
"
restore_validation_completed_at="$(date -u +%s)"

rto_seconds="$((outage_recovered_at - outage_started_at))"
rpo_seconds="$((outage_started_at - snapshot_completed_at))"

cat >"$report_file" <<JSON
{
  "canary_loop_id": "${canary_loop}",
  "snapshot_file": "${snapshot_file}",
  "etcd_namespace": "${SMITH_ETCD_NAMESPACE}",
  "etcd_pod_before": "${etcd_pod}",
  "etcd_pod_after": "${replacement_pod}",
  "snapshot_started_at_unix": ${snapshot_started_at},
  "snapshot_completed_at_unix": ${snapshot_completed_at},
  "outage_started_at_unix": ${outage_started_at},
  "outage_recovered_at_unix": ${outage_recovered_at},
  "restore_validation_started_at_unix": ${restore_validation_started_at},
  "restore_validation_completed_at_unix": ${restore_validation_completed_at},
  "rto_seconds": ${rto_seconds},
  "rpo_seconds": ${rpo_seconds}
}
JSON

info "dr drill completed"
info "snapshot: ${snapshot_file}"
info "report: ${report_file}"
info "RTO=${rto_seconds}s RPO=${rpo_seconds}s"
