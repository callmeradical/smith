SHELL := /bin/bash
.DEFAULT_GOAL := help

SMITH_NAMESPACE ?= smith-system
SMITH_RELEASE ?= smith
SMITH_VALUES ?= helm/smith/values/local.yaml
SMITH_TEST_ARTIFACTS_DIR ?= /tmp/smith-test-artifacts
SMITH_FIXTURE_DIR ?= /tmp/smith-test-repo

.PHONY: help \
	doctor bootstrap \
	cluster cluster-up cluster-down \
	deploy undeploy \
	test test-unit test-matrix test-integration test-e2e test-bdd \
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
	echo "doctor passed: required local tools found"

bootstrap: ## Install missing k3d/vcluster prerequisites
	./scripts/integration/prereqs.sh

cluster: cluster-up ## Alias for cluster-up

cluster-up: ## Provision local k3d + vcluster + etcd environment
	./scripts/integration/env-up.sh

cluster-down: ## Delete local k3d + vcluster + etcd environment
	./scripts/integration/env-down.sh

deploy: ## Deploy Smith locally with Helm (namespace/release/values configurable)
	helm upgrade --install "$(SMITH_RELEASE)" ./helm/smith \
	  --namespace "$(SMITH_NAMESPACE)" \
	  --create-namespace \
	  -f "$(SMITH_VALUES)"

undeploy: ## Remove Helm release from local cluster
	-helm uninstall "$(SMITH_RELEASE)" -n "$(SMITH_NAMESPACE)"

test: test-matrix ## Run default local test workflow (non-cluster matrix)

test-unit: ## Run full Go test suite
	go test ./...

test-matrix: ## Run script-based local matrix (fixture + e2e + verification)
	SMITH_TEST_ARTIFACTS_DIR="$(SMITH_TEST_ARTIFACTS_DIR)" \
	SMITH_FIXTURE_DIR="$(SMITH_FIXTURE_DIR)" \
	./scripts/test/run-matrix.sh

test-integration: ## Run vcluster-backed integration workflow
	./scripts/integration/run-tests.sh

test-e2e: ## Run local e2e scripts directly
	./scripts/test/e2e-single-loop.sh
	./scripts/test/e2e-concurrent-loops.sh

test-bdd: ## Run godog-based BDD acceptance suite
	go test ./test/acceptance -run TestFeatures -count=1

teardown: undeploy cluster-down ## Teardown local deploy and cluster environment

build: ## Build all Go binaries
	go build ./...

docs-check: ## Run docs quality checks
	./scripts/docs/quality-check.sh

ci-local: ## Run local CI-equivalent checks
	$(MAKE) build
	$(MAKE) test-unit
	$(MAKE) test-bdd
	$(MAKE) docs-check

hooks-install: ## Install repository git hooks from .githooks
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit .githooks/pre-push
	@echo "Installed git hooks from .githooks"

hooks-run-pre-commit: ## Run pre-commit checks manually
	@echo "[pre-commit] running quick checks..."
	go test ./cmd/...

hooks-run-pre-push: ## Run pre-push checks manually
	@echo "[pre-push] running build and full tests..."
	$(MAKE) build
	$(MAKE) test-unit
