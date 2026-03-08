# Backup/Restore Disaster Recovery Runbook

## Goal

Validate that Smith matrix data in etcd can be recovered from snapshot and that active loop context continuity is preserved after simulated outage.

This runbook captures:

- snapshot workflow
- outage simulation
- restore validation
- RTO/RPO metrics

## Prerequisites

- `kubectl` access to the Smith cluster
- etcd pod running in `smith-system` (or override `SMITH_ETCD_NAMESPACE`)
- etcd image includes `etcdctl`, `etcdutl`, and `etcd`

## Automated Drill

Use the DR drill script:

```bash
chmod +x scripts/integration/dr-restore-drill.sh
scripts/integration/dr-restore-drill.sh
```

Useful overrides:

```bash
SMITH_ETCD_NAMESPACE=smith-system \
SMITH_ETCD_RELEASE_NAME=smith-etcd \
SMITH_DR_ARTIFACTS_DIR=/tmp/smith-dr \
SMITH_DR_TIMEOUT=180s \
scripts/integration/dr-restore-drill.sh
```

Dry-run preview:

```bash
SMITH_DR_DRY_RUN=true scripts/integration/dr-restore-drill.sh
```

## What the Drill Validates

1. Writes a canary loop anomaly/state into etcd (`overwriting` state).
2. Takes etcd snapshot (`snapshot save`) and copies snapshot artifact.
3. Simulates outage by deleting the etcd pod and waiting for replacement readiness.
4. Performs restore rehearsal from snapshot in an isolated data dir inside etcd pod.
5. Boots temporary restore etcd instance on alternate local ports.
6. Confirms canary loop key exists in restored dataset.
7. Emits JSON report with RTO/RPO.

## Artifacts

Default output directory:

- `/tmp/smith-dr/snapshot-<timestamp>.db`
- `/tmp/smith-dr/dr-report.json`

`dr-report.json` fields include:

- `canary_loop_id`
- `etcd_pod_before` / `etcd_pod_after`
- `snapshot_*`, `outage_*`, `restore_validation_*` timestamps
- `rto_seconds`
- `rpo_seconds`

## RTO / RPO Definitions

- `RTO`: `outage_recovered_at - outage_started_at`
- `RPO`: `outage_started_at - snapshot_completed_at`

Target thresholds should be defined per environment and compared against recorded report values.

## Recovery Continuity Expectation

The drill passes continuity when the restored snapshot contains the canary loop state key and payload, demonstrating matrix context survives outage + restore path.

## Failure Handling

If drill fails:

1. Capture pod logs:
   - `kubectl -n smith-system logs <etcd-pod> --all-containers --tail=300`
2. Capture describe output:
   - `kubectl -n smith-system describe pod <etcd-pod>`
3. Preserve artifacts in `SMITH_DR_ARTIFACTS_DIR`.
4. Open incident and include:
   - failure step
   - `dr-report.json` (if generated)
   - etcd pod diagnostics
