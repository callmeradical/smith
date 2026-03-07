# Reconciliation Loop for Drift and Zombie Prevention

## Inputs

- etcd lifecycle state per loop (`unresolved`, `overwriting`, terminal states)
- Kubernetes runtime status per loop Job (`pending`, `running`, `succeeded`, `failed`, `missing`)

## Repair/Escalation Rules

- `unresolved` + runtime active (`pending|running`): auto-correct state to `overwriting`.
- `overwriting` + runtime `missing`:
  - stale loop: escalate to `flatline`
  - non-stale loop: return to `unresolved` for retry
- `overwriting` + runtime `failed`:
  - attempts remaining: return to `unresolved`
  - max attempts reached: escalate to `flatline`
- terminal state (`synced|flatline|cancelled`) + runtime active: delete zombie Job.

## Metrics

- `smith_reconcile_runs_total`
- `smith_reconcile_drift_detected_total`
- `smith_reconcile_drift_corrected_total`
- `smith_reconcile_drift_escalated_total`

These metrics are emitted on every reconcile evaluation and on each drift outcome path.
