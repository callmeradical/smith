.PHONY: build test test-bdd docs-check ci-local hooks-install hooks-run-pre-commit hooks-run-pre-push

build:
	go build ./...

test:
	go test ./...

test-bdd:
	go test ./test/acceptance -run TestFeatures -count=1

docs-check:
	./scripts/docs/quality-check.sh

ci-local: build test test-bdd docs-check

hooks-install:
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit .githooks/pre-push
	@echo "Installed git hooks from .githooks"

hooks-run-pre-commit:
	@echo "[pre-commit] running quick checks..."
	go test ./cmd/...

hooks-run-pre-push:
	@echo "[pre-push] running build and full tests..."
	$(MAKE) build
	$(MAKE) test
