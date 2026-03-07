# Go-Native Acceptance Test Harness Strategy

## Objective

Replace bash-centric acceptance orchestration with a Go-native testing stack that is deterministic, composable, and CI-friendly while preserving current Smith MVP coverage.

## Design Principles

- Prefer `go test` as the primary entrypoint for all layers.
- Keep behavior checks close to production packages for unit/integration fidelity.
- Isolate external dependencies behind fixture/runtime adapters.
- Emit machine-readable artifacts (JSON) for CI triage.
- Preserve ability to run fast local subsets.

## Standard Test Stack

Core frameworks:

- `testing` (stdlib): base runner, subtests, parallelization, benchmarks.
- `testify` (`assert`/`require`): integration/system assertion ergonomics.
- `godog` (BDD): acceptance workflows for operator-facing loop scenarios.

Supporting runtime utilities:

- `httptest`: API integration tests.
- `testcontainers-go`: ephemeral etcd and optional helper services.
- `client-go` fake clients for unit-level Kubernetes behavior.
- optional real-cluster gate via existing k3d/vCluster workflow for high-fidelity system runs.

## Package Layout

New Go harness packages (incremental rollout):

- `internal/testharness/fixture`
  - deterministic fixture repo creation and cleanup
  - branch/outcome seed data loaders
- `internal/testharness/runtime`
  - etcd lifecycle helpers (containerized/local endpoint)
  - API/core process launchers and readiness checks
- `internal/testharness/assertions`
  - reusable state, journal, handoff, audit assertions
- `internal/testharness/artifacts`
  - JSON reports and failure snapshots

Test suites:

- unit: `internal/source/**/_test.go`
- integration: `internal/source/integration/...`
- acceptance (BDD + table-driven): `test/acceptance/...`
- system/high-fidelity (cluster-backed): `test/system/...`

## Ownership Boundaries

Unit tests own:

- pure business logic, state transitions, retry math, schema validation.
- no real etcd or Kubernetes API dependencies.

Integration tests own:

- cross-package behavior with real etcd and process boundaries.
- API/store/core interactions and serialization contracts.

Acceptance tests own:

- operator-visible workflows (ingress, lifecycle, override, environment profile selection).
- deterministic end-state assertions and traceability chains.

System tests own:

- real Kubernetes reconciliation behavior, drift/zombie corrections, concurrency safety under cluster conditions.

CI gate ownership:

- required on every PR: unit + integration + acceptance smoke.
- optional/periodic: full system suite against ephemeral cluster.

## Migration Plan (from Bash)

Phase 0: Baseline mapping

- Map existing scripts in `scripts/test/*` and `scripts/integration/*` to target Go suites.
- Preserve scripts as wrappers until parity is met.

Phase 1: Harness foundation

- Create `internal/testharness/{fixture,runtime,assertions,artifacts}`.
- Port fixture provisioning and completion verification helpers first.

Phase 2: Acceptance parity

- Port `e2e-single-loop.sh` and `e2e-concurrent-loops.sh` into `test/acceptance` Go tests.
- Keep current shell scripts delegating to `go test` commands.

Phase 3: Failure injection + environment coverage

- Port failure injection scenarios and ingress/environment modes into Go suites.
- Standardize report artifacts as JSON per test run.

Phase 4: CI and retirement

- Update CI to use `go test`-based acceptance targets.
- Retire bash implementations once parity matrix is green for two consecutive releases.

## Command Surface

Target command model:

- `go test ./internal/source/...`
- `go test ./test/acceptance/... -run TestSmoke`
- `go test ./test/system/... -run TestCluster -count=1`

Wrapper compatibility (temporary):

- `scripts/test/run-matrix.sh` becomes a thin invoker for the Go suites.
- Existing shell commands remain documented until full retirement.

## Approval Criteria

This strategy is complete when:

- all new acceptance coverage is authored in Go,
- boundaries (unit/integration/acceptance/system) are enforced by package layout,
- CI uses Go-native targets as the primary gate,
- legacy bash paths are wrapper-only or removed.
