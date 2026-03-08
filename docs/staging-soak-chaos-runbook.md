# Staging Soak/Chaos Runbook

## Purpose

Run optional long-duration staging validation beyond PR CI scope using `k3d + vCluster + etcd`.

Goals:

- validate sustained watch/reconcile behavior under repeated iterations,
- exercise failure-injection paths in the same environment,
- verify parity with non-vCluster profile.

## Workflow

Scheduled workflow: `.github/workflows/staging-soak-chaos.yml`

- `staging-vcluster-soak-chaos` job:
  - provisions `smith-staging` k3d cluster + vCluster + etcd,
  - executes repeated soak/chaos loop via `scripts/integration/staging-soak-chaos.sh`,
  - uploads artifacts,
  - tears down environment.
- `parity-spot-check` job:
  - runs non-vCluster matrix parity via `scripts/test/parity-spot-check.sh`.

Default schedule: Mondays at 09:00 UTC.

## Manual Execution

Provision and run locally:

```bash
./scripts/integration/env-up.sh
SMITH_SOAK_ITERATIONS=3 SMITH_SOAK_INTERVAL_SECONDS=60 ./scripts/integration/staging-soak-chaos.sh
./scripts/integration/env-down.sh
```

Run parity spot-check without vCluster:

```bash
./scripts/test/parity-spot-check.sh
```

## Key Parameters

- `SMITH_SOAK_ITERATIONS` (default `3`)
- `SMITH_SOAK_INTERVAL_SECONDS` (default `60`)
- `SMITH_TEST_ARTIFACTS_DIR` (default `/tmp/smith-staging-artifacts` for soak)

## Artifacts

Soak artifacts include per-iteration logs for:

- watch/reconcile integration test,
- failure-injection suite,
- matrix run output.

Parity artifacts include matrix summary and fixture verification output.

## Escalation Triggers

Escalate when any of the following occur:

- repeated watch/reconcile failures across iterations,
- failure-injection tests regress,
- parity spot-check diverges from expected non-cluster behavior.
