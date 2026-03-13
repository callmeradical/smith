#!/usr/bin/env bash
set -euo pipefail

SMITH_API_BASE_URL="${SMITH_API_BASE_URL:-http://127.0.0.1:8080}"
SMITH_LAT_LOOP_ID="${SMITH_LAT_LOOP_ID:-latency-loop}"
SMITH_LAT_SAMPLES="${SMITH_LAT_SAMPLES:-40}"
SMITH_LAT_INTERVAL_SEC="${SMITH_LAT_INTERVAL_SEC:-0.20}"
SMITH_OPERATOR_TOKEN="${SMITH_OPERATOR_TOKEN:-}"
SMITH_LAT_TIMEOUT_SEC="${SMITH_LAT_TIMEOUT_SEC:-30}"

info() { echo "[latency] $*"; }
fail() { echo "[latency] ERROR: $*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

api() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local args=(-sS -X "$method" "$SMITH_API_BASE_URL$path")
  if [[ -n "$SMITH_OPERATOR_TOKEN" ]]; then
    args+=(-H "Authorization: Bearer $SMITH_OPERATOR_TOKEN")
  fi
  if [[ -n "$body" ]]; then
    args+=(-H "Content-Type: application/json" --data "$body")
  fi
  curl "${args[@]}"
}

get_state() {
  api GET "/v1/loops/$SMITH_LAT_LOOP_ID" | jq -r '.state.state // .state.State // empty'
}

set_override() {
  local target="$1"
  api POST "/v1/control/override" "$(jq -nc \
    --arg loop "$SMITH_LAT_LOOP_ID" \
    --arg target "$target" \
    '{loop_id:$loop,target_state:$target,reason:"latency-benchmark",actor:"latency-script"}')" >/dev/null
}

ensure_loop() {
  local status
  status="$(curl -s -o /dev/null -w "%{http_code}" "$SMITH_API_BASE_URL/v1/loops/$SMITH_LAT_LOOP_ID")"
  if [[ "$status" == "404" ]]; then
    info "creating loop $SMITH_LAT_LOOP_ID"
    api POST "/v1/loops" "$(jq -nc \
      --arg loop "$SMITH_LAT_LOOP_ID" \
      '{loop_id:$loop,title:"Latency benchmark loop",description:"Console latency measurement loop",source_type:"manual",source_ref:"latency/benchmark"}')" >/dev/null
  fi
}

need_cmd curl
need_cmd jq
need_cmd awk
need_cmd sort

ensure_loop
state="$(get_state)"
[[ "$state" == "unresolved" || "$state" == "running" ]] || fail "loop must be in unresolved/running state, found: $state"

tmp_dir="$(mktemp -d)"
samples_file="$tmp_dir/samples_ms.txt"
stream_log="$tmp_dir/stream.log"

info "capturing $SMITH_LAT_SAMPLES samples on loop=$SMITH_LAT_LOOP_ID"
curl -sN "$SMITH_API_BASE_URL/v1/loops/$SMITH_LAT_LOOP_ID/journal/stream?since_seq=0" >"$stream_log" &
stream_pid=$!
trap 'kill "$stream_pid" >/dev/null 2>&1 || true; rm -rf "$tmp_dir"' EXIT

start_epoch="$(date +%s)"
generated=0
while (( generated < SMITH_LAT_SAMPLES )); do
  current="$(get_state)"
  target="running"
  if [[ "$current" == "running" ]]; then
    target="unresolved"
  fi
  set_override "$target"
  generated=$((generated + 1))
  sleep "$SMITH_LAT_INTERVAL_SEC"
done

while :; do
  awk '/^data: /{sub(/^data: /, ""); print}' "$stream_log" \
    | jq -r 'select((.entry.message // .message) == "manual override applied") | (((now - ((.entry.timestamp // .timestamp) | fromdateiso8601)) * 1000) | floor)' \
    >"$samples_file" || true

  collected="$(wc -l <"$samples_file" | tr -d ' ')"
  if (( collected >= SMITH_LAT_SAMPLES )); then
    break
  fi
  now_epoch="$(date +%s)"
  if (( now_epoch - start_epoch > SMITH_LAT_TIMEOUT_SEC )); then
    fail "timed out waiting for stream samples (collected=$collected expected=$SMITH_LAT_SAMPLES)"
  fi
  sleep 0.2
done

count="$(wc -l <"$samples_file" | tr -d ' ')"
p95="$(sort -n "$samples_file" | awk '{a[NR]=$1} END {idx=int((NR*95+99)/100); if (idx < 1) idx=1; if (idx > NR) idx=NR; print a[idx] }')"
p99="$(sort -n "$samples_file" | awk '{a[NR]=$1} END {idx=int((NR*99+99)/100); if (idx < 1) idx=1; if (idx > NR) idx=NR; print a[idx] }')"

info "samples=$count p95_ms=$p95 p99_ms=$p99"
if (( p95 < 100 )); then
  info "PASS: p95 latency is below 100ms target"
else
  info "WARN: p95 latency is above 100ms target; investigate network/browser/host load"
fi
