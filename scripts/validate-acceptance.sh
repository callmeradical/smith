#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ALLOW_MISSING_TOOLS=false
if [[ "${1:-}" == "--allow-missing-tools" ]]; then
  ALLOW_MISSING_TOOLS=true
fi

failed=0
warned=0

pass() {
  printf 'PASS: %s\n' "$1"
}

warn() {
  printf 'WARN: %s\n' "$1"
  warned=$((warned + 1))
}

fail() {
  printf 'FAIL: %s\n' "$1"
  failed=$((failed + 1))
}

section() {
  printf '\n== %s ==\n' "$1"
}

expect_file() {
  local path="$1"
  local message="$2"
  if [[ -f "$path" ]]; then
    pass "$message"
  else
    fail "$message (missing: $path)"
  fi
}

expect_dir() {
  local path="$1"
  local message="$2"
  if [[ -d "$path" ]]; then
    pass "$message"
  else
    fail "$message (missing: $path)"
  fi
}

expect_rg() {
  local pattern="$1"
  local target="$2"
  local message="$3"
  if rg -q "$pattern" "$target"; then
    pass "$message"
  else
    fail "$message"
  fi
}

check_command() {
  local cmd="$1"
  local message="$2"
  if command -v "$cmd" >/dev/null 2>&1; then
    pass "$message"
    return 0
  fi

  if $ALLOW_MISSING_TOOLS; then
    warn "$message (command '$cmd' not installed; skipped)"
    return 1
  fi

  fail "$message (command '$cmd' not installed)"
  return 1
}

run_check() {
  local message="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    pass "$message"
  else
    fail "$message"
  fi
}

cluster_reachable() {
  kubectl cluster-info >/dev/null 2>&1
}

manifest_has_kind_name() {
  local manifest_file="$1"
  local expected_kind="$2"
  local expected_name="$3"
  awk -v kind="$expected_kind" -v name="$expected_name" '
    /^kind:[[:space:]]*/ {
      current_kind = $2
      in_metadata = 0
      next
    }
    /^metadata:[[:space:]]*$/ {
      in_metadata = 1
      next
    }
    /^[^[:space:]]/ && $0 !~ /^metadata:[[:space:]]*$/ {
      in_metadata = 0
    }
    in_metadata && /^[[:space:]]+name:[[:space:]]*/ {
      current_name = $2
      if (current_kind == kind && current_name == name) {
        found = 1
      }
      in_metadata = 0
    }
    END {
      exit(found ? 0 : 1)
    }
  ' "$manifest_file"
}

section "td-56f4e8 | Helm chart scaffold"
expect_file "helm/smith/Chart.yaml" "Chart metadata exists"
expect_file "helm/smith/values.yaml" "Base values exist"
expect_file "helm/smith/templates/core-deployment.yaml" "Core deployment template exists"
expect_file "helm/smith/templates/api-deployment.yaml" "API deployment template exists"
expect_file "helm/smith/templates/console-deployment.yaml" "Console deployment template exists"
expect_file "helm/smith/templates/api-service.yaml" "API service template exists"
expect_file "helm/smith/templates/console-service.yaml" "Console service template exists"
expect_file "helm/smith/templates/serviceaccount.yaml" "ServiceAccount template exists"
expect_file "helm/smith/templates/rbac.yaml" "RBAC template exists"

expect_rg "readinessProbe:" "helm/smith/templates/core-deployment.yaml" "Core readiness probe defined"
expect_rg "livenessProbe:" "helm/smith/templates/core-deployment.yaml" "Core liveness probe defined"
expect_rg "readinessProbe:" "helm/smith/templates/api-deployment.yaml" "API readiness probe defined"
expect_rg "livenessProbe:" "helm/smith/templates/api-deployment.yaml" "API liveness probe defined"
expect_rg "readinessProbe:" "helm/smith/templates/console-deployment.yaml" "Console readiness probe defined"
expect_rg "livenessProbe:" "helm/smith/templates/console-deployment.yaml" "Console liveness probe defined"

expect_rg "SMITH_API_BASE_URL" "helm/smith/templates/console-deployment.yaml" "Console wired to API service URL"
expect_rg "targetPort: http" "helm/smith/templates/api-service.yaml" "API service targets named HTTP port"
expect_rg "targetPort: http" "helm/smith/templates/console-service.yaml" "Console service targets named HTTP port"

rendered="$(mktemp)"

if check_command helm "Helm is available for render/install validation"; then
  run_check "Chart passes helm lint" helm lint helm/smith
  if helm template smith helm/smith >"$rendered"; then
    pass "Chart renders with default values"

    run_check "Rendered manifests include core deployment" rg -q "name: smith-smith-core" "$rendered"
    run_check "Rendered manifests include api deployment" rg -q "name: smith-smith-api" "$rendered"
    run_check "Rendered manifests include console deployment" rg -q "name: smith-smith-console" "$rendered"
    run_check "Rendered manifests include api service" manifest_has_kind_name "$rendered" "Service" "smith-smith-api"
    run_check "Rendered manifests include console service" manifest_has_kind_name "$rendered" "Service" "smith-smith-console"

    if check_command kubectl "kubectl is available for optional cluster validation"; then
      if cluster_reachable; then
        run_check "Rendered manifests are accepted by kubectl server dry-run" kubectl apply --dry-run=server -f "$rendered"
      else
        warn "Cluster validation skipped (no reachable Kubernetes context)"
      fi
    fi
  else
    fail "Chart renders with default values"
  fi

  run_check "Chart renders with local overlay" helm template smith helm/smith -f helm/smith/values/local.yaml
  run_check "Chart renders with stage overlay" helm template smith helm/smith -f helm/smith/values/stage.yaml
  run_check "Chart renders with prod overlay" helm template smith helm/smith -f helm/smith/values/prod.yaml
fi

rm -f "$rendered"

section "td-eef8f8 | MVP boundary and release gates"
expect_file "docs/mvp-boundary-and-release-gates.md" "MVP boundary doc exists"
expect_rg "## In Scope" "docs/mvp-boundary-and-release-gates.md" "In-scope section documented"
expect_rg "## Out of Scope" "docs/mvp-boundary-and-release-gates.md" "Deferred backlog section documented"
expect_rg "## Required Release Gates" "docs/mvp-boundary-and-release-gates.md" "Release gates section documented"
expect_rg "## MVP Sign-Off Checklist" "docs/mvp-boundary-and-release-gates.md" "Sign-off criteria documented"

expect_rg "etcd" "docs/mvp-boundary-and-release-gates.md" "etcd capability referenced"
expect_rg "Core" "docs/mvp-boundary-and-release-gates.md" "Core capability referenced"
expect_rg "Replica" "docs/mvp-boundary-and-release-gates.md" "Replica capability referenced"
expect_rg "Console" "docs/mvp-boundary-and-release-gates.md" "Console capability referenced"
expect_rg "Helm" "docs/mvp-boundary-and-release-gates.md" "Helm deployment readiness referenced"

expect_rg "Data Integrity" "docs/mvp-boundary-and-release-gates.md" "Consistency/data-integrity gate present"
expect_rg "Traceability|observability|Observability" "docs/mvp-boundary-and-release-gates.md" "Observability/traceability gate present"
expect_rg "Reliability Baseline|resilience|Resilience" "docs/mvp-boundary-and-release-gates.md" "Resilience/reliability gate present"

section "td-39a505 | etcd schema and typed models"
expect_file "docs/etcd-key-schema.md" "etcd schema doc exists"
expect_dir "internal/source/model" "Go model package exists"
expect_file "internal/source/model/keys.go" "Key helper file exists"
expect_file "internal/source/model/types.go" "Types file exists"

expect_rg "PrefixAnomalies" "internal/source/model/keys.go" "Anomalies key prefix defined"
expect_rg "PrefixState" "internal/source/model/keys.go" "State key prefix defined"
expect_rg "PrefixJournal" "internal/source/model/keys.go" "Journal key prefix defined"
expect_rg "type Anomaly struct" "internal/source/model/types.go" "Anomaly struct defined"
expect_rg "type State struct" "internal/source/model/types.go" "State struct defined"
expect_rg "type JournalEntry struct" "internal/source/model/types.go" "JournalEntry struct defined"
expect_rg "type Handoff struct" "internal/source/model/types.go" "Handoff struct defined"

expect_rg "func .*Watch|Watch.*func" "internal/source/model" "Watch helper(s) present"
expect_rg "func .*Get|func .*List|func .*Create|func .*Put|func .*Delete|func .*Update" "internal/source/model" "CRUD helper(s) present"

if rg --files -g '*_test.go' internal/source/model >/dev/null 2>&1; then
  pass "Unit test files exist for model package"

  if check_command go "Go toolchain is available for unit tests"; then
    run_check "Model package unit tests pass" go test ./internal/source/model/...
  fi
else
  fail "Unit test files exist for model package"
fi

section "td-366583 | Core and replica Dockerfiles"
expect_file "docker/core.Dockerfile" "Core Dockerfile exists"
expect_file "docker/replica.Dockerfile" "Replica Dockerfile exists"
expect_file "cmd/smith-core/main.go" "Core entrypoint exists"
expect_file "cmd/smith-replica/main.go" "Replica entrypoint exists"

expect_rg "distroless/static-debian12:nonroot" "docker/core.Dockerfile" "Core runtime image runs as non-root"
expect_rg "distroless/static-debian12:nonroot" "docker/replica.Dockerfile" "Replica runtime image runs as non-root"
expect_rg "go build .*./cmd/smith-core" "docker/core.Dockerfile" "Core Dockerfile builds the smith-core binary"
expect_rg "go build .*./cmd/smith-replica" "docker/replica.Dockerfile" "Replica Dockerfile builds the smith-replica binary"

if check_command go "Go toolchain is available for binary build validation"; then
  run_check "smith-core binary builds" go build -o /tmp/smith-core-bin ./cmd/smith-core
  run_check "smith-replica binary builds" go build -o /tmp/smith-replica-bin ./cmd/smith-replica
fi

if command -v docker >/dev/null 2>&1; then
  pass "Docker is available for image build validation"
  run_check "Core image build succeeds" docker build -f docker/core.Dockerfile -t smith-core:test .
  run_check "Replica image build succeeds" docker build -f docker/replica.Dockerfile -t smith-replica:test .
else
  warn "Docker image build validation skipped (command 'docker' not installed)"
fi

section "td-d1201a | Replica job template Helm integration"
expect_rg "replicaTemplate:" "helm/smith/values.yaml" "Replica template values block exists"
expect_rg "SMITH_REPLICA_TEMPLATE_SERVICE_ACCOUNT" "helm/smith/templates/core-deployment.yaml" "Core deployment receives replica service account template"
expect_rg "SMITH_REPLICA_TEMPLATE_RESOURCES" "helm/smith/templates/core-deployment.yaml" "Core deployment receives replica resource template"
expect_rg "SMITH_REPLICA_TEMPLATE_NODE_SELECTOR" "helm/smith/templates/core-deployment.yaml" "Core deployment receives replica nodeSelector template"
expect_rg "SMITH_REPLICA_TEMPLATE_TOLERATIONS" "helm/smith/templates/core-deployment.yaml" "Core deployment receives replica tolerations template"
expect_rg "SMITH_REPLICA_TEMPLATE_ENV" "helm/smith/templates/core-deployment.yaml" "Core deployment receives replica env template"
expect_rg "\"replicaTemplate\"" "helm/smith/values.schema.json" "Values schema includes replicaTemplate contract"

rendered_replica_template="$(mktemp)"
if check_command helm "Helm is available for replica template render validation"; then
  if helm template smith helm/smith -f helm/smith/values/prod.yaml >"$rendered_replica_template"; then
    pass "Chart renders with prod overlay for replica template wiring"
    run_check "Rendered core deployment includes replica template env wiring" rg -q "SMITH_REPLICA_TEMPLATE_SERVICE_ACCOUNT" "$rendered_replica_template"
    run_check "Rendered core deployment carries prod replica node selector" rg -q "workload-tier.*replica" "$rendered_replica_template"
  else
    fail "Chart renders with prod overlay for replica template wiring"
  fi
fi
rm -f "$rendered_replica_template"

section "td-ece36c | Console container image"
expect_file "docker/console.Dockerfile" "Console Dockerfile exists"
expect_file "console/index.html" "Console static entrypoint exists"
expect_file "console/nginx.conf" "Console nginx config exists"
expect_file "console/runtime-config.template.js" "Console runtime config template exists"
expect_file "console/30-smith-env.sh" "Console runtime env injection script exists"

expect_rg "SMITH_API_BASE_URL" "console/runtime-config.template.js" "Console runtime config references SMITH_API_BASE_URL"
expect_rg "location = /healthz" "console/nginx.conf" "Console liveness endpoint is defined"
expect_rg "location = /readyz" "console/nginx.conf" "Console readiness endpoint is defined"

if command -v docker >/dev/null 2>&1; then
  pass "Docker is available for console image validation"
  run_check "Console image build succeeds" docker build -f docker/console.Dockerfile -t smith-console:test .

  cid=""
  if cid="$(docker run -d --rm -p 13000:3000 -e SMITH_API_BASE_URL=http://smith-api:8080 smith-console:test 2>/dev/null)"; then
    sleep 1
    if docker ps --format '{{.ID}}' | rg -q "^${cid}"; then
      run_check "Console /healthz responds" curl -fsS http://127.0.0.1:13000/healthz
      run_check "Console /readyz responds" curl -fsS http://127.0.0.1:13000/readyz
      run_check "Console runtime config injects API endpoint" sh -c "curl -fsS http://127.0.0.1:13000/runtime-config.js | rg -q 'http://smith-api:8080'"
    else
      warn "Console runtime probe validation skipped (test container exited early)"
    fi
    docker stop "$cid" >/dev/null 2>&1 || true
  else
    warn "Console runtime probe validation skipped (unable to start test container)"
  fi
else
  warn "Console image validation skipped (command 'docker' not installed)"
fi

printf '\nSummary: %d failure(s), %d warning(s).\n' "$failed" "$warned"
if (( failed > 0 )); then
  exit 1
fi
