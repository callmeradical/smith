SHELL := /bin/bash
.DEFAULT_GOAL := help
SMITH_NAMESPACE ?= smith-system
SMITH_RELEASE ?= smith
SMITH_VALUES ?= helm/smith/values/local.yaml
SMITH_LOCAL_VALUES ?= helm/smith/values/local.yaml
SMITH_STAGING_VALUES ?= helm/smith/values/staging.yaml
SMITH_PROD_VALUES ?= helm/smith/values/prod.yaml
SMITH_K3D_CLUSTER_NAME ?= smith-int
SMITH_CORE_IMAGE ?= ghcr.io/smith/core:v0.1.0
SMITH_API_IMAGE ?= ghcr.io/smith/api:v0.1.0
SMITH_REPLICA_IMAGE ?= ghcr.io/smith/replica:v0.1.0
SMITH_CONSOLE_IMAGE ?= ghcr.io/smith/console:v0.1.0
SMITH_TEST_ARTIFACTS_DIR ?= /tmp/smith-test-artifacts
ACT ?= act
SMITH_FIXTURE_DIR ?= /tmp/smith-test-repo
SMITH_K3D_CLUSTER_NAME ?= smith-int
SMITH_LOCAL_CORE_IMAGE ?= smith-core:local
SMITH_LOCAL_API_IMAGE ?= smith-api:local
SMITH_LOCAL_REPLICA_IMAGE ?= smith-replica:local
SMITH_LOCAL_CONSOLE_IMAGE ?= smith-console:local
SMITH_LOCAL_CHAT_IMAGE ?= smith-chat:local
SMITH_MIN_GO_VERSION ?= 1.22.0
SMITH_MIN_KUBECTL_VERSION ?= 1.29.0
SMITH_MIN_HELM_VERSION ?= 3.13.0
.PHONY: help \
	doctor bootstrap \
	cluster cluster-up cluster-down cluster-reset cluster-health \
	build build-local image-build-local image-load-local images-local deploy deploy-local deploy-staging deploy-prod rollout-local undeploy undeploy-local \
	console-build-local console-load-local console-rollout-local console-deploy-local \
	chat-build-local chat-load-local chat-deploy-local \
	test test-unit test-frontend \
	\
	teardown \
	build docs-check ci-local hooks-install hooks-run-pre-commit hooks-run-pre-push
help: ## Show available make targets
	@awk 'BEGIN {FS = ":.*##"; printf "Smith local developer workflow\n\nTargets:\n"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
doctor: ## Validate local prerequisites for make-first workflow
	@set -euo pipefail; \
	missing=0; \
	for cmd in go kubectl helm docker k3d vcluster; do \
	  if ! command -v $$cmd >/dev/null 2>&1; then \
	    echo "missing required command: $$cmd"; \
	    missing=1; \
	  fi; \
	done; \
	if [[ $$missing -ne 0 ]]; then \
	  echo "doctor failed: install missing prerequisites before continuing"; \
	  exit 1; \
	fi; \
	ver_ge() { \
	  local current="$$1"; local required="$$2"; \
	  [[ "$$(printf '%s\n%s\n' "$$required" "$$current" | sort -V | head -n1)" == "$$required" ]]; \
	}; \
	go_v="$$(go version | awk '{print $$3}' | sed 's/^go//')"; \
	if ! ver_ge "$$go_v" "$(SMITH_MIN_GO_VERSION)"; then \
	  echo "doctor failed: go $$go_v is below required $(SMITH_MIN_GO_VERSION)"; \
	  echo "remediation: install newer Go from https://go.dev/dl/"; \
	  exit 1; \
	fi; \
	kubectl_v="$$(kubectl version --client=true -o yaml 2>/dev/null | awk '/gitVersion:/{print $$2; exit}' | sed 's/^v//')"; \
	if [[ -n "$$kubectl_v" ]] && ! ver_ge "$$kubectl_v" "$(SMITH_MIN_KUBECTL_VERSION)"; then \
	  echo "doctor failed: kubectl $$kubectl_v is below required $(SMITH_MIN_KUBECTL_VERSION)"; \
	  echo "remediation: upgrade kubectl via package manager or Kubernetes release binaries"; \
	  exit 1; \
	fi; \
	helm_v="$$(helm version --short 2>/dev/null | sed -E 's/^v([0-9]+\.[0-9]+\.[0-9]+).*/\\1/')"; \
	if [[ -n "$$helm_v" ]] && ! ver_ge "$$helm_v" "$(SMITH_MIN_HELM_VERSION)"; then \
	  echo "doctor failed: helm $$helm_v is below required $(SMITH_MIN_HELM_VERSION)"; \
	  echo "remediation: upgrade helm via package manager or https://helm.sh/docs/intro/install/"; \
	  exit 1; \
	fi; \
	echo "doctor passed: required local tools found"
bootstrap: ## Install missing k3d/vcluster prerequisites
	@set -euo pipefail; \
	./scripts/integration/prereqs.sh; \
	mkdir -p "$$HOME/.smith"; \
	if [[ ! -f "$$HOME/.smith/config.json" ]]; then \
	  printf '%s\n' '{"current_context":"default","contexts":{"default":{"server":"http://127.0.0.1:8080","token":""}}}' > "$$HOME/.smith/config.json"; \
	  echo "bootstrap: created $$HOME/.smith/config.json"; \
	else \
	  echo "bootstrap: preserved existing $$HOME/.smith/config.json"; \
	fi
cluster: cluster-up ## Alias for cluster-up
cluster-up: ## Provision local k3d + vcluster + etcd environment
	./scripts/integration/env-up.sh
cluster-down: ## Delete local k3d + vcluster + etcd environment
	./scripts/integration/env-down.sh
cluster-reset: ## Reset local k3d + vcluster + etcd (down then up)
	@set -euo pipefail; \
	echo "[cluster-reset] tearing down existing local environment"; \
	$(MAKE) --no-print-directory cluster-down; \
	echo "[cluster-reset] bringing local environment back up"; \
	$(MAKE) --no-print-directory cluster-up; \
	echo "[cluster-reset] completed"
cluster-health: ## Verify local cluster/vcluster/etcd readiness with actionable failures
	@set -euo pipefail; \
	echo "[cluster-health] checking Kubernetes API reachability"; \
	if ! kubectl cluster-info >/dev/null 2>&1; then \
	  echo "[cluster-health] ERROR: kubectl cannot reach a cluster context"; \
	  echo "[cluster-health] HINT: run 'make cluster-up' or fix KUBECONFIG"; \
	  exit 1; \
	fi; \
	echo "[cluster-health] checking node readiness"; \
	if ! kubectl wait --for=condition=Ready nodes --all --timeout=120s >/dev/null 2>&1; then \
	  echo "[cluster-health] ERROR: one or more cluster nodes are not Ready"; \
	  echo "[cluster-health] HINT: run 'kubectl get nodes -o wide' and inspect node events"; \
	  exit 1; \
	fi; \
	ETCD_NS="$${SMITH_ETCD_NAMESPACE:-smith-system}"; \
	ETCD_RELEASE="$${SMITH_ETCD_RELEASE_NAME:-smith-etcd}"; \
	ETCD_MODE="$${SMITH_ETCD_MODE:-simple}"; \
	echo "[cluster-health] checking etcd service in namespace $${ETCD_NS}"; \
	if ! kubectl -n "$${ETCD_NS}" get svc "$${ETCD_RELEASE}" >/dev/null 2>&1; then \
	  echo "[cluster-health] ERROR: etcd service '$${ETCD_RELEASE}' not found in namespace '$${ETCD_NS}'"; \
	  echo "[cluster-health] HINT: run 'make cluster-up' and review env-up output"; \
	  exit 1; \
	fi; \
	if [[ "$${ETCD_MODE}" == "helm" ]]; then \
	  echo "[cluster-health] checking etcd statefulset readiness"; \
	  if ! kubectl -n "$${ETCD_NS}" rollout status statefulset/"$${ETCD_RELEASE}" --timeout=120s >/dev/null 2>&1; then \
	    echo "[cluster-health] ERROR: etcd statefulset '$${ETCD_RELEASE}' is not ready"; \
	    echo "[cluster-health] HINT: run 'kubectl -n $${ETCD_NS} get pods -o wide'"; \
	    exit 1; \
	  fi; \
	else \
	  echo "[cluster-health] checking etcd deployment readiness"; \
	  if ! kubectl -n "$${ETCD_NS}" rollout status deployment/"$${ETCD_RELEASE}" --timeout=120s >/dev/null 2>&1; then \
	    echo "[cluster-health] ERROR: etcd deployment '$${ETCD_RELEASE}' is not ready"; \
	    echo "[cluster-health] HINT: run 'kubectl -n $${ETCD_NS} get pods -o wide'"; \
	    exit 1; \
	  fi; \
	fi; \
	echo "[cluster-health] ready"
deploy: ## Deploy Smith with Helm using SMITH_VALUES profile
	helm upgrade --install "$(SMITH_RELEASE)" ./helm/smith \
	  --namespace "$(SMITH_NAMESPACE)" \
	  --create-namespace \
	  -f "$(SMITH_VALUES)"
deploy-local: ## Deploy Smith via Helm using local values profile
	$(MAKE) --no-print-directory images-local
	helm upgrade --install "$(SMITH_RELEASE)" ./helm/smith \
	  --namespace "$(SMITH_NAMESPACE)" \
	  --create-namespace \
	  --set global.rolloutId="$(shell date +%s)" \
	  -f "$(SMITH_LOCAL_VALUES)"
	$(MAKE) --no-print-directory rollout-local
console-build-local: ## Build only the console local image
	docker build -f docker/console.Dockerfile -t "$(SMITH_LOCAL_CONSOLE_IMAGE)" .
console-load-local: ## Load only the console image into k3d
	k3d image import "$(SMITH_LOCAL_CONSOLE_IMAGE)" -c "$(SMITH_K3D_CLUSTER_NAME)"
console-rollout-local: ## Restart only the console deployment
	kubectl rollout restart deployment/$(SMITH_RELEASE)-smith-console -n $(SMITH_NAMESPACE)
	kubectl rollout status deployment/$(SMITH_RELEASE)-smith-console -n $(SMITH_NAMESPACE)
console-deploy-local: console-build-local console-load-local console-rollout-local ## Build, load, and restart only the console
api-build-local: ## Build only the smith-api local image
	docker build -f docker/api.Dockerfile -t "$(SMITH_LOCAL_API_IMAGE)" .
api-load-local: ## Load only the smith-api image into k3d
	k3d image import "$(SMITH_LOCAL_API_IMAGE)" -c "$(SMITH_K3D_CLUSTER_NAME)"
api-rollout-local: ## Restart only the smith-api deployment
	kubectl rollout restart deployment/$(SMITH_RELEASE)-smith-api -n $(SMITH_NAMESPACE)
	kubectl rollout status deployment/$(SMITH_RELEASE)-smith-api -n $(SMITH_NAMESPACE)
api-deploy-local: api-build-local api-load-local api-rollout-local ## Build, load, and restart only the smith-api
replica-build-local: ## Build only the smith-replica local image
	docker build -f docker/replica.Dockerfile -t "$(SMITH_LOCAL_REPLICA_IMAGE)" .
replica-load-local: ## Load only the smith-replica image into k3d
	k3d image import "$(SMITH_LOCAL_REPLICA_IMAGE)" -c "$(SMITH_K3D_CLUSTER_NAME)"
replica-deploy-local: replica-build-local replica-load-local ## Build and load only the smith-replica (no rollout needed as it runs as Jobs)
chat-build-local: ## Build only the smith-chat local image
	docker build -f docker/chat.Dockerfile -t "$(SMITH_LOCAL_CHAT_IMAGE)" .
chat-load-local: ## Load only the smith-chat image into k3d
	k3d image import "$(SMITH_LOCAL_CHAT_IMAGE)" -c "$(SMITH_K3D_CLUSTER_NAME)"
chat-deploy-local: chat-build-local chat-load-local ## Build and load only the smith-chat
console-api-deploy-local: console-build-local console-load-local api-build-local api-load-local rollout-local ## Build, load, and restart console + api
rollout-local: ## Force restart local deployments to pick up new images
	kubectl rollout restart deployment/$(SMITH_RELEASE)-smith-api -n $(SMITH_NAMESPACE)
	kubectl rollout restart deployment/$(SMITH_RELEASE)-smith-console -n $(SMITH_NAMESPACE)
	kubectl rollout restart deployment/$(SMITH_RELEASE)-smith-core -n $(SMITH_NAMESPACE)
	-kubectl rollout restart deployment/$(SMITH_RELEASE)-smith-chat -n $(SMITH_NAMESPACE)
	kubectl rollout status deployment/$(SMITH_RELEASE)-smith-api -n $(SMITH_NAMESPACE)
	kubectl rollout status deployment/$(SMITH_RELEASE)-smith-console -n $(SMITH_NAMESPACE)
	kubectl rollout status deployment/$(SMITH_RELEASE)-smith-core -n $(SMITH_NAMESPACE)
deploy-staging: ## Deploy Smith via Helm using staging values profile
	helm upgrade --install "$(SMITH_RELEASE)" ./helm/smith \
	  --namespace "$(SMITH_NAMESPACE)" \
	  --create-namespace \
	  -f "$(SMITH_STAGING_VALUES)"
deploy-prod: ## Deploy Smith via Helm using production values profile
	helm upgrade --install "$(SMITH_RELEASE)" ./helm/smith \
	  --namespace "$(SMITH_NAMESPACE)" \
	  --create-namespace \
	  -f "$(SMITH_PROD_VALUES)"
undeploy: ## Remove Helm release from cluster
	-helm uninstall "$(SMITH_RELEASE)" -n "$(SMITH_NAMESPACE)"
undeploy-local: ## Remove local Helm deployment
	-helm uninstall "$(SMITH_RELEASE)" -n "$(SMITH_NAMESPACE)"

test: test-unit test-frontend test-acceptance ## Run default local test workflow (non-cluster matrix)

test-unit: ## Run full Go test suite
	go test ./...

test-bdd: ## Run godog-based BDD acceptance suite
	go test ./test/acceptance -run TestFeatures -count=1

test-acceptance-smoke: ## Run acceptance smoke suite with JSON artifact output
	@set -euo pipefail; \
	mkdir -p "$(SMITH_TEST_ARTIFACTS_DIR)"; \
	go test ./test/acceptance -run TestHarnessSmoke -count=1 -json | tee "$(SMITH_TEST_ARTIFACTS_DIR)/acceptance-smoke.jsonl"

test-acceptance-bdd: ## Run acceptance BDD suite with JSON artifact output
	@set -euo pipefail; \
	mkdir -p "$(SMITH_TEST_ARTIFACTS_DIR)"; \
	go test ./test/acceptance -run TestFeatures -count=1 -json | tee "$(SMITH_TEST_ARTIFACTS_DIR)/acceptance-bdd.jsonl"

test-acceptance: test-acceptance-smoke test-acceptance-bdd ## Run all Go-native acceptance harness suites

test-frontend: ## Run Playwright frontend/component tests for console
	@if [ ! -d node_modules ]; then npm install; fi
	@if [ ! -d frontend/node_modules ]; then cd frontend && npm install; fi
	@if [ ! -d frontend/.svelte-kit ]; then cd frontend && npx svelte-kit sync; fi
	cd frontend && npm run build
	npm run test:frontend

trivy-scan-local: ## Run local vulnerability scans on all Smith images
	@echo "[trivy] scanning images for critical vulnerabilities..."
	@vulnerabilities=0; \
	for img in core api replica console chat; do \
	  echo "Scanning smith-$$img:local..."; \
	  if ! trivy image --severity CRITICAL --exit-code 1 "smith-$$img:local" > /tmp/trivy-$$img.log 2>&1; then \
	    cat /tmp/trivy-$$img.log; \
	    vulnerabilities=$$((vulnerabilities + 1)); \
	  else \
	    cat /tmp/trivy-$$img.log; \
	  fi; \
	done; \
	if [ $$vulnerabilities -gt 0 ]; then \
	  echo "[trivy] Found CRITICAL vulnerabilities in $$vulnerabilities images. Fails build."; \
	  exit 1; \
	fi

docs-check: ## Run docs quality checks
	./scripts/docs/quality-check.sh

ci-local: ## Run local CI-equivalent checks
	$(MAKE) build
	$(MAKE) test-unit
	$(MAKE) test-acceptance
	$(MAKE) docs-check

ci-local-act: ## Run core CI jobs locally using 'act' (requires 'act' and Docker)
	@if ! command -v "$(ACT)" >/dev/null 2>&1; then \
		echo "ERROR: 'act' is not installed."; \
		echo "HINT: install it with 'brew install act' or from https://github.com/nektos/act"; \
		exit 1; \
	fi
	@echo "[act] running CI jobs..."
	$(ACT) -j lint-and-check
	$(ACT) -j go-unit-tests
	$(ACT) -j node-unit-tests
	$(ACT) -j playwright-tests
	$(ACT) -j acceptance-tests
	$(ACT) -j test-matrix

hooks-run-pre-commit: ## Run the local pre-commit hook workload via act
	@if ! command -v "$(ACT)" >/dev/null 2>&1; then \
		echo "ERROR: 'act' is not installed."; \
		echo "HINT: install it with 'brew install act' or from https://github.com/nektos/act"; \
		exit 1; \
	fi
	@echo "[act] running pre-commit hook workload..."
	$(ACT) -j lint-and-check

hooks-run-pre-push: ## Run the local pre-push hook workload via act
	@if ! command -v "$(ACT)" >/dev/null 2>&1; then \
		echo "ERROR: 'act' is not installed."; \
		echo "HINT: install it with 'brew install act' or from https://github.com/nektos/act"; \
		exit 1; \
	fi
	@echo "[act] running pre-push hook workload..."
	$(ACT) -j go-unit-tests
	$(ACT) -j node-unit-tests
	$(ACT) -j playwright-tests

hooks-install: ## Install repository git hooks from .githooks
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit .githooks/pre-push
	@echo "Installed git hooks from .githooks"
