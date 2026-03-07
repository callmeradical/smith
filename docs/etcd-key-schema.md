# Smith etcd Key Schema (v1)

## Key Layout

- `/smith/v1/anomalies/{loop_id}`: anomaly definition and policy.
- `/smith/v1/state/{loop_id}`: current lifecycle status and ownership.
- `/smith/v1/journal/{loop_id}/{sequence}`: append-only execution events.
- `/smith/v1/handoffs/{loop_id}/{sequence}`: iterative memory transfer records.
- `/smith/v1/locks/{loop_id}`: lease-backed single-writer lock record.
- `/smith/v1/overrides/{loop_id}/{sequence}`: operator control actions.
- `/smith/v1/audit/{yyyy}/{mm}/{dd}/{event_id}`: global immutable audit events.

## Schema Rules

- Every record carries `schema_version`.
- Sequence keys are left-padded to preserve lexicographic order.
- Journal, handoff, override paths are append-only.
- State transitions are compare-and-swap guarded with etcd revision checks.
- Lock writes must include valid etcd lease IDs and heartbeats.

## Allowed Loop State Transitions

- `unresolved -> overwriting`
- `overwriting -> synced`
- `overwriting -> flatline`
- `overwriting -> unresolved` (retry)
- `unresolved|overwriting -> cancelled` (operator action)

All other transitions are rejected.
