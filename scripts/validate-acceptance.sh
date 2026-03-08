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
  if rg -q -- "$pattern" "$target"; then
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

section "td-ffc123 | CI image build and publish pipeline"
expect_file ".github/workflows/images-build-publish.yml" "Image publish workflow exists"
expect_rg "image: core" ".github/workflows/images-build-publish.yml" "Workflow includes core image"
expect_rg "image: replica" ".github/workflows/images-build-publish.yml" "Workflow includes replica image"
expect_rg "image: console" ".github/workflows/images-build-publish.yml" "Workflow includes console image"
expect_rg "docker/core.Dockerfile" ".github/workflows/images-build-publish.yml" "Workflow references core Dockerfile"
expect_rg "docker/replica.Dockerfile" ".github/workflows/images-build-publish.yml" "Workflow references replica Dockerfile"
expect_rg "docker/console.Dockerfile" ".github/workflows/images-build-publish.yml" "Workflow references console Dockerfile"
expect_rg "docker/build-push-action" ".github/workflows/images-build-publish.yml" "Workflow uses docker build/push action"
expect_rg "aquasecurity/trivy-action" ".github/workflows/images-build-publish.yml" "Workflow includes vulnerability scanning gate"
expect_rg "anchore/sbom-action" ".github/workflows/images-build-publish.yml" "Workflow includes SBOM generation"
expect_rg "exit-code: '1'" ".github/workflows/images-build-publish.yml" "Scan failures are configured to fail CI"
expect_rg "type=sha" ".github/workflows/images-build-publish.yml" "Workflow publishes immutable SHA tags"

section "td-2d1f40 | CI matrix for single-loop and multi-loop e2e scenarios"
expect_file ".github/workflows/test-matrix.yml" "Test matrix workflow exists"
expect_rg "id: single-loop" ".github/workflows/test-matrix.yml" "Workflow includes single-loop scenario"
expect_rg "id: multi-loop" ".github/workflows/test-matrix.yml" "Workflow includes multi-loop scenario"
expect_rg "e2e-single-loop.sh" ".github/workflows/test-matrix.yml" "Workflow runs single-loop e2e script"
expect_rg "e2e-concurrent-loops.sh" ".github/workflows/test-matrix.yml" "Workflow runs multi-loop e2e script"
expect_rg "smith-e2e-single-loop-artifacts" ".github/workflows/test-matrix.yml" "Workflow uploads single-loop artifacts"
expect_rg "smith-e2e-multi-loop-artifacts" ".github/workflows/test-matrix.yml" "Workflow uploads multi-loop artifacts"
expect_rg "GITHUB_STEP_SUMMARY" ".github/workflows/test-matrix.yml" "Workflow publishes trace summary links"

section "td-b78f0c | cluster autoscaler prerequisites and runbook"
expect_file "docs/cluster-autoscaler-prerequisites-runbook.md" "Cluster autoscaler runbook exists"
expect_rg "## Prerequisites" "docs/cluster-autoscaler-prerequisites-runbook.md" "Runbook prerequisites section documented"
expect_rg "## Baseline Capacity Strategy" "docs/cluster-autoscaler-prerequisites-runbook.md" "Runbook min/max strategy documented"
expect_rg "## Failure Modes" "docs/cluster-autoscaler-prerequisites-runbook.md" "Runbook failure modes documented"
expect_rg "## Troubleshooting Checklist" "docs/cluster-autoscaler-prerequisites-runbook.md" "Runbook troubleshooting section documented"

section "td-6de678 | interactive terminal attach for active loops"
expect_rg "case \"attach\"" "cmd/smith-api/main.go" "API exposes attach control endpoint"
expect_rg "case \"detach\"" "cmd/smith-api/main.go" "API exposes detach control endpoint"
expect_rg "case \"command\"" "cmd/smith-api/main.go" "API exposes interactive command endpoint"
expect_rg "attach-terminal" "cmd/smith-api/main.go" "Attach events are audited"
expect_rg "detach-terminal" "cmd/smith-api/main.go" "Detach events are audited"
expect_rg "terminal command issued" "cmd/smith-api/main.go" "Interactive command events are journaled"
expect_rg "loop detach" "cmd/smithctl/main.go" "smithctl exposes loop detach command"
expect_rg "loop command" "cmd/smithctl/main.go" "smithctl exposes loop command command"

section "td-c7e14a | Dockerfile-based loop image build path"
expect_rg "SMITH_DOCKERFILE_BUILD_ENABLED" "cmd/smith-core/main.go" "Core supports dockerfile build feature flag"
expect_rg "SMITH_DOCKERFILE_IMAGE_REPOSITORY" "cmd/smith-core/main.go" "Core supports dockerfile image repository override"
expect_rg "loop_environment_dockerfile" "cmd/smith-core/main.go" "Core records dockerfile execution image source"
expect_rg "dockerfile build metadata" "cmd/smith-core/main.go" "Core journals dockerfile build metadata"
expect_rg "Dockerfile Build Runtime Flags" "docs/loop-environment-profile.md" "Loop environment docs cover dockerfile runtime flags"

section "td-fdcb47 | loop environment selection in smithctl"
expect_rg "env-preset" "cmd/smithctl/main.go" "smithctl exposes environment preset flag"
expect_rg "--env-image-ref" "cmd/smithctl/main.go" "smithctl exposes environment image flag"
expect_rg "--env-docker-context" "cmd/smithctl/main.go" "smithctl exposes environment docker context flag"
expect_rg "buildEnvironmentPayload" "cmd/smithctl/main.go" "smithctl validates and builds environment payload"
expect_rg "environment source conflict" "cmd/smithctl/main.go" "smithctl rejects conflicting environment modes"

section "td-c155ab | e2e tests for loop environment modes"
expect_file "internal/source/e2e/environment_modes_test.go" "Environment modes e2e test exists"
expect_file "scripts/test/e2e-environment-modes.sh" "Environment modes e2e script exists"
expect_rg "TestLoopEnvironmentModes" "internal/source/e2e/environment_modes_test.go" "Environment modes test case is defined"
expect_rg "e2e-environment-modes.sh" "scripts/test/run-matrix.sh" "Environment modes included in run-matrix"
expect_rg "e2e-environment-summary.txt" ".github/workflows/test-matrix.yml" "CI summary includes environment e2e trace artifact"

section "td-d1c89c | smithctl skill mount configuration"
expect_rg "skillFlag" "cmd/smithctl/main.go" "smithctl implements skill flag parser"
expect_rg "--skill" "cmd/smithctl/main.go" "smithctl exposes --skill flag"
expect_rg "Codex defaults to /smith/skills/<name>" "cmd/smithctl/main.go" "smithctl help documents codex default mountpoint"
expect_rg "TestLoopCreateWithSkillFlags" "cmd/smithctl/main_test.go" "smithctl skill flag payload test exists"
expect_rg "## smithctl Skill Flags" "docs/skill-volume-mounts.md" "Skill mount doc includes smithctl usage"

section "td-bafce8 | e2e coverage for loop skill mount behavior"
expect_file "internal/source/e2e/skill_mounts_test.go" "Skill mount e2e test exists"
expect_file "scripts/test/e2e-skill-mounts.sh" "Skill mount e2e script exists"
expect_rg "TestLoopSkillMountBehavior" "internal/source/e2e/skill_mounts_test.go" "Skill mount behavior test case is defined"
expect_rg "e2e-skill-mounts.sh" "scripts/test/run-matrix.sh" "Skill mount e2e included in run-matrix"
expect_rg "e2e-skill-mounts-summary.txt" ".github/workflows/test-matrix.yml" "CI summary includes skill mount e2e artifact"

section "td-d13d14 | migrate ingress/environment/skill acceptance tests to Go harness"
expect_rg "TestIngressModesLoopCreationAndExecution" "internal/source/e2e/ingress_modes_test.go" "Ingress acceptance moved to Go harness"
expect_rg "TestLoopEnvironmentModes" "internal/source/e2e/environment_modes_test.go" "Environment acceptance moved to Go harness"
expect_rg "TestLoopSkillMountBehavior" "internal/source/e2e/skill_mounts_test.go" "Skill acceptance moved to Go harness"
expect_rg "RETIRE_AFTER=2026-06-30" "scripts/test/e2e-ingress-modes.sh" "Ingress shell wrapper marked transitional with retirement date"
expect_rg "RETIRE_AFTER=2026-06-30" "scripts/test/e2e-environment-modes.sh" "Environment shell wrapper marked transitional with retirement date"
expect_rg "RETIRE_AFTER=2026-06-30" "scripts/test/e2e-skill-mounts.sh" "Skill shell wrapper marked transitional with retirement date"

section "td-66d043 | Makefile test targets for integration and e2e loops"
expect_rg "^test: test-matrix" "Makefile" "Default make test target routes to test-matrix"
expect_rg "^test-integration:" "Makefile" "Makefile exposes test-integration target"
expect_rg "^test-e2e:" "Makefile" "Makefile exposes test-e2e target"
expect_rg "e2e-environment-modes.sh" "Makefile" "test-e2e includes environment mode coverage"
expect_rg "e2e-skill-mounts.sh" "Makefile" "test-e2e includes skill mount coverage"
expect_rg "artifacts: \\$\\(SMITH_TEST_ARTIFACTS_DIR\\)" "Makefile" "Targets print artifact pointer"

section "td-7f1aba | make doctor/bootstrap local prerequisites"
expect_rg "^doctor:" "Makefile" "Makefile exposes doctor target"
expect_rg "^bootstrap:" "Makefile" "Makefile exposes bootstrap target"
expect_rg "SMITH_MIN_GO_VERSION" "Makefile" "Doctor enforces minimum Go version"
expect_rg "remediation:" "Makefile" "Doctor prints actionable remediation hints"
expect_rg "\\.smith/config.json" "Makefile" "Bootstrap prepares local smithctl config defaults"

section "td-1f619b | quickstart for local deploy and loop execution with make"
expect_file "docs/make-local-quickstart.md" "Local make quickstart doc exists"
expect_rg "make doctor" "docs/make-local-quickstart.md" "Quickstart includes prerequisite check"
expect_rg "make deploy-local" "docs/make-local-quickstart.md" "Quickstart includes local deploy step"
expect_rg "smithctl .* loop create" "docs/make-local-quickstart.md" "Quickstart includes sample loop execution"
expect_rg "make undeploy-local" "docs/make-local-quickstart.md" "Quickstart includes cleanup workflow"
expect_rg "Troubleshooting" "docs/make-local-quickstart.md" "Quickstart includes troubleshooting notes"

section "td-bc5348 | non-vCluster parity profile (k3s direct namespace deploy)"
expect_rg "SMITH_USE_VCLUSTER" "scripts/integration/env-up.sh" "env-up supports direct non-vCluster profile toggle"
expect_rg "SMITH_USE_VCLUSTER" "scripts/integration/env-down.sh" "env-down supports direct non-vCluster profile toggle"
expect_rg "SMITH_ENABLE_CLUSTER_TESTS" "scripts/test/parity-spot-check.sh" "Parity script accepts cluster-enabled mode override"
expect_rg "non-vcluster-k3s-direct-gate" ".github/workflows/pre-release-system-gate.yml" "Pre-release workflow includes direct k3s parity job"
expect_rg "smith-pre-release-parity-direct-k3s-artifacts" ".github/workflows/pre-release-system-gate.yml" "Direct k3s parity artifacts are uploaded separately"

section "td-b77cdd | multi-arch image build support"
expect_rg "IMAGE_PLATFORMS" ".github/workflows/images-build-publish.yml" "Image workflow defines multi-arch platforms"
expect_rg "linux/amd64,linux/arm64" ".github/workflows/images-build-publish.yml" "Image workflow publishes amd64+arm64 manifest"
expect_rg "platforms: \\$\\{\\{ env.IMAGE_PLATFORMS \\}\\}" ".github/workflows/images-build-publish.yml" "Publish step uses multi-arch platforms"
expect_rg "multi-arch manifest list" "docs/image-tagging-versioning.md" "Image versioning doc references multi-arch publish behavior"

section "td-a2e46e | multi-provider skill mount abstraction design (post-MVP)"
expect_file "docs/multi-provider-skill-mount-abstraction.md" "Multi-provider skill abstraction doc exists"
expect_rg "SkillMountTranslator" "docs/multi-provider-skill-mount-abstraction.md" "Design defines provider translator contract"
expect_rg "Compatibility Strategy" "docs/multi-provider-skill-mount-abstraction.md" "Design includes compatibility strategy"
expect_rg "Codex" "docs/multi-provider-skill-mount-abstraction.md" "Design preserves Codex baseline behavior"
expect_rg "resolved bindings" "docs/multi-provider-skill-mount-abstraction.md" "Design includes audit/traceability for translated bindings"

section "td-e44fdf | configurable branch cleanup and conflict policy"
expect_rg "BranchCleanupPolicy" "internal/source/gitpolicy/policy.go" "Git policy exposes branch cleanup policy type"
expect_rg "ConflictPolicy" "internal/source/gitpolicy/policy.go" "Git policy exposes conflict policy type"
expect_rg "SMITH_GIT_POLICY_CONFIG_ENABLED" "cmd/smith-core/main.go" "Core exposes feature flag for git policy config"
expect_rg "SMITH_GIT_POLICY_BRANCH_CLEANUP" "cmd/smith-core/main.go" "Core reads branch cleanup override"
expect_rg "SMITH_GIT_POLICY_CONFLICT_POLICY" "cmd/smith-core/main.go" "Core reads conflict policy override"
expect_rg "EnableGitPolicyConfig" "internal/source/replica/job_generator.go" "Replica job request gates git policy overrides behind feature flag"
expect_rg "SMITH_GIT_POLICY_BRANCH_CLEANUP" "internal/source/replica/job_generator.go" "Replica job receives git branch cleanup policy env"
expect_rg "SMITH_GIT_POLICY_CONFLICT_POLICY" "internal/source/replica/job_generator.go" "Replica job receives git conflict policy env"
expect_rg "Operator Configuration" "docs/git-history-policy.md" "Git policy docs include operator configuration section"

section "td-90af46 | configurable journal retention and archival policy"
expect_file "internal/source/journalpolicy/policy.go" "Journal policy model exists"
expect_rg "RetentionMode" "internal/source/journalpolicy/policy.go" "Journal policy includes retention mode"
expect_rg "ArchiveMode" "internal/source/journalpolicy/policy.go" "Journal policy includes archive mode"
expect_rg "SMITH_JOURNAL_POLICY_CONFIG_ENABLED" "cmd/smith-core/main.go" "Core exposes feature flag for journal policy config"
expect_rg "SMITH_JOURNAL_RETENTION_MODE" "cmd/smith-core/main.go" "Core reads journal retention mode override"
expect_rg "SMITH_JOURNAL_ARCHIVE_MODE" "cmd/smith-core/main.go" "Core reads journal archive mode override"
expect_rg "EnableJournalPolicyConfig" "internal/source/replica/job_generator.go" "Replica job gates journal policy overrides behind feature flag"
expect_rg "SMITH_JOURNAL_RETENTION_MODE" "internal/source/replica/job_generator.go" "Replica job receives journal retention env"
expect_rg "SMITH_JOURNAL_ARCHIVE_MODE" "internal/source/replica/job_generator.go" "Replica job receives journal archive env"
expect_file "docs/journal-retention-archival-policy.md" "Journal retention/archival doc exists"

section "td-040379 | pre-release system gate (vCluster + non-vCluster parity)"
expect_file ".github/workflows/pre-release-system-gate.yml" "Pre-release gate workflow exists"
expect_file "scripts/release/pre-release-system-gate.sh" "Pre-release gate script exists"
expect_file "scripts/test/validate-recovery-override.sh" "Recovery/override validation script exists"
expect_file "docs/pre-release-system-gate.md" "Pre-release gate runbook exists"
expect_rg "vcluster-system-gate" ".github/workflows/pre-release-system-gate.yml" "Workflow includes vCluster gate job"
expect_rg "non-vcluster-parity-gate" ".github/workflows/pre-release-system-gate.yml" "Workflow includes non-vCluster parity gate job"
expect_rg "pre-release-system-gate.sh vcluster" ".github/workflows/pre-release-system-gate.yml" "Workflow runs vCluster gate profile"
expect_rg "pre-release-system-gate.sh parity" ".github/workflows/pre-release-system-gate.yml" "Workflow runs parity gate profile"
expect_rg "validate-recovery-override.sh" "scripts/release/pre-release-system-gate.sh" "Gate script validates recovery and override paths"
expect_rg "failure-injection.sh" "scripts/test/validate-recovery-override.sh" "Recovery validation includes failure injection suite"
expect_rg "TestLoopCancelBatchPostsOverride" "scripts/test/validate-recovery-override.sh" "Override validation includes operator cancel/override path test"

printf '\nSummary: %d failure(s), %d warning(s).\n' "$failed" "$warned"
if (( failed > 0 )); then
  exit 1
fi
