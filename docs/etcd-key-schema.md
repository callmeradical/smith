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

## Versioning Contract

- Current write schema: `v1`.
- Backward-readable schema: `v1alpha1` (legacy) and `v1`.
- Forward compatibility: unknown versions (for example `v2`) are rejected by readers until explicit support is added.
- Write policy: components only write `v1` records once upgraded (single-write version).
- Missing `schema_version` is treated as legacy `v1alpha1` for migration safety during bootstrap/upgrade of older records.

## Migration Strategy

- Migration model: dual-read, single-write.
- During rolling upgrades, controllers accept both `v1alpha1` and `v1` state payloads.
- Legacy `v1alpha1` state payloads are migrated in-memory to `v1` shape (`status` field mapped to `state`) before business logic runs.
- On next successful mutation, migrated records are persisted back as `v1`, progressively draining legacy state without global downtime.
- In-flight anomalies continue processing because state decoding is version-aware before transition logic is applied.
- Any unsupported future version is surfaced as an explicit schema error and should be routed to operator intervention path.

## Allowed Loop State Transitions

- `unresolved -> running`
- `running -> synced`
- `running -> flatline`
- `running -> unresolved` (retry)
- `unresolved|running -> cancelled` (operator action)

All other transitions are rejected.
