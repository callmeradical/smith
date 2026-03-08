# Smith End-to-End Test Matrix and Failure-Injection Suite

## Scope

This matrix defines the minimum runnable suite for reliability gating across unit, integration, e2e, and failure injection.

## Matrix

| ID | Layer | Scenario | Command | Expected Result |
| --- | --- | --- | --- | --- |
| M-001 | Unit | Core unresolved watcher idempotency/retry | `go test ./internal/source/core/...` | No duplicate execution intent per revision; retry semantics stable |
| M-002 | Unit | Lease lock single-writer and stale lock takeover | `go test ./internal/source/locking/...` | Lock safety invariants hold |
| M-003 | Unit | Completion saga crash points | `go test ./internal/source/completion/...` | Commit failure, sync failure, compensation failure handled deterministically |
| M-004 | Unit | Reconcile zombie/drift behavior | `go test ./internal/source/reconcile/...` | Drift corrected/escalated per policy |
| M-005 | Fixture | Deterministic loop repository fixture | `./scripts/fixtures/provision-smith-test-repo.sh && ./scripts/fixtures/verify-smith-test-repo.sh` | Fixture is reproducible with expected branches/outcomes |
| M-005a | Verification | Completion verification harness report | `./scripts/test/verify-completion.sh /tmp/smith-test-repo single-loop-success` | Machine-readable report validates commit metadata, expected files/diff footprint, and optional handoff/phase consistency |
| M-006 | Integration (optional in CI) | etcd + core watch/reconcile loop in vCluster | `./scripts/integration/test-watch-reconcile.sh` | State watch semantics, duplicate-event handling, lifecycle transitions, and zombie-job drift correction validated against real etcd + Kubernetes APIs |
| M-007 | E2E | Single loop unresolved -> synced | `./scripts/test/e2e-single-loop.sh` | Single-loop completion report includes commit/handoff integrity; optional cluster mode runs lifecycle integration path |
| M-008 | E2E | Concurrent loop safety (no split-brain writer) | `./scripts/test/e2e-concurrent-loops.sh` | N concurrent loop workers preserve per-loop isolation and completion verification for concurrent fixture branches |
| M-009 | E2E | Ingress mode coverage (GitHub, PRD, interactive) | `./scripts/test/e2e-ingress-modes.sh` | `smithctl` ingress commands create loops and each created loop resolves to synced state with source traceability (`source_type`, `source_ref`) |
| M-010 | E2E | Environment mode coverage (preset, mise, image, dockerfile) | `./scripts/test/e2e-environment-modes.sh` | `smithctl loop create` environment flags map to deterministic resolved mode metadata |
| M-011 | E2E | Skill mount behavior (explicit/default/failure) | `./scripts/test/e2e-skill-mounts.sh` | Skill mounts respect explicit/default paths, fail on invalid sources, and emit journal metadata |

## Failure-Injection Cases

Completion saga crash-point validation (covered by `internal/source/completion/protocol_test.go`):

- `F-001`: Commit/push failure -> loop remains retryable (`OutcomeRetryable`).
- `F-002`: State finalize failure after commit -> compensation (revert) succeeds -> retryable.
- `F-003`: State finalize failure + compensation failure -> `OutcomeCompensationRequired` with ambiguous-terminal guard.

Reconcile failure behavior (covered by `internal/source/reconcile/loop_test.go`):

- `F-004`: Missing runtime while `overwriting` -> unresolved retry or flatline when stale.
- `F-005`: Failed runtime with attempt exhaustion -> flatline escalation.
- `F-006`: Zombie runtime after terminal etcd state -> runtime deletion.

## Replayable Artifacts

`./scripts/test/run-matrix.sh` emits artifacts under `SMITH_TEST_ARTIFACTS_DIR` (default `/tmp/smith-test-artifacts`):

- `summary.txt`
- `fixture-git-log.txt`
- `fixture-branches.txt`
- `expected-outcomes.json`
- `completion-report-*.json`
- `e2e-ingress-summary.txt`

These are intended for post-failure replay and debugging in CI.

## Flaky Rate Control

Use repeated integration execution to measure and gate flakiness:

```bash
SMITH_IT_FLAKE_RUNS=3 ./scripts/integration/flake-check.sh
```

Current policy: any failure in repeated runs fails the gate.

## CI Ephemeral Environment

Workflow: `.github/workflows/ephemeral-integration-env.yml`

This workflow provisions an ephemeral `k3d + vCluster + etcd` stack per run, executes integration tests, uploads diagnostics/artifacts, and always tears down the environment.

## CI Loop Scenario Matrix

Workflow: `.github/workflows/test-matrix.yml`

- Runs dedicated PR e2e scenarios for:
  - single-loop completion (`scripts/test/e2e-single-loop.sh`)
  - multi-loop concurrency safety (`scripts/test/e2e-concurrent-loops.sh`)
  - ingress modes (`scripts/test/e2e-ingress-modes.sh`)
  - environment modes (`scripts/test/e2e-environment-modes.sh`)
  - skill mount modes (`scripts/test/e2e-skill-mounts.sh`)
- Uploads per-scenario artifacts and publishes evidence file names in `GITHUB_STEP_SUMMARY`.
