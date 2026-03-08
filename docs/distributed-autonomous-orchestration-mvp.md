# Smith MVP: Distributed Autonomous Orchestration

## Intent

Deliver Smith as an etcd-backed, Kubernetes-native autonomous orchestration platform where:
- etcd is the authoritative state machine.
- Agent Core drives execution by watching unresolved loop state.
- Replica workers execute in Kubernetes Jobs.
- Operator Console and CLI provide control, visibility, and audited intervention.

## Philosophy

Smith does not use persona-specific agents.

- Execution units are intentionally homogeneous and omnicapable.
- Anthropomorphic "agent personalities" are treated as constraints that reduce scheduling flexibility and system-level throughput.
- Scale comes from replicating the same worker contract many times, not from assigning different identities or personas.
- Orchestration is built from neutral primitives: state transitions, leases/locks, jobs, journals, and handoffs.
- The implementation is influenced by Ralph, `marcus/sidecar`, `marcus/td`, and similar projects, with an explicit goal of operating beyond single-machine constraints.

Smith also intentionally moves beyond a single-machine local file-system model. etcd + Kubernetes provide a distributed control substrate so loops can run across broad compute while preserving deterministic state, auditability, and resumability.

## Architecture

### Control Plane Components

- `smith-core` (Deployment)
  - Watches etcd loop state keys.
  - Acquires per-loop lock before scheduling work.
  - Creates and monitors Replica Jobs.
  - Reconciles etcd state with Kubernetes runtime.
- `smith-api` (Deployment)
  - Exposes ingress and control APIs (`/v1/loops`, `/v1/ingress/*`, `/v1/control/*`).
  - Performs authn/authz and emits audited operator actions.
- `smith-console` (Deployment)
  - UI for active loops, journal stream, and override workflows.
- `etcd` (external or in-cluster)
  - Stores anomalies, lifecycle state, locks, journal, handoff, and audit records.

### Data Plane Components

- `smith-replica` (Job)
  - Pulls loop context and prior handoff.
  - Executes autonomous coding cycle.
  - Streams journal entries.
  - Writes completion signal and handoff.

## etcd as Source of Truth

All lifecycle decisions read/write etcd keys. Kubernetes is treated as runtime state that must reconcile back to etcd.

Required key families:
- `/smith/v1/anomalies/{loop_id}`
- `/smith/v1/state/{loop_id}`
- `/smith/v1/journal/{loop_id}/{seq}`
- `/smith/v1/handoffs/{loop_id}/{seq}`
- `/smith/v1/locks/{loop_id}`
- `/smith/v1/overrides/{loop_id}/{seq}`
- `/smith/v1/audit/{yyyy}/{mm}/{dd}/{event_id}`

## Core Loop Contract

1. Ingress writes anomaly + initial state (`unresolved`).
2. Core watcher receives unresolved event.
3. Core acquires lock (`/locks/{loop_id}` lease-backed).
4. Core transitions state to `overwriting` and creates Replica Job.
5. Replica writes append-only journal events and heartbeats.
6. On success, Replica writes completion payload + handoff; Core finalizes to `synced`.
7. On failure/timeout, Core applies retry policy or transitions to `flatline` with reason.
8. Reconciler continuously repairs drift between etcd and Job/Pod reality.

## Concurrency and Safety

- Single-writer enforced with etcd compare-and-swap on lock and state revision.
- Duplicate watch events are tolerated through idempotent transition checks.
- Completion uses revision-checked state transitions to prevent split-brain terminal states.
- Reconciler marks zombie jobs stale and can terminate them under policy.

## Operator Control and Audit

Operator capabilities:
- View loop matrix and live journal.
- Pause/resume/cancel loops.
- Eject running Replica.
- Override loop state with required reason.

Each operator action produces immutable audit entries containing:
- actor
- timestamp
- action
- target loop
- reason
- correlation ID

## Kubernetes-Native Deployment

Helm chart requirements:
- Deploy Core, API, Console Deployments.
- Configure RBAC and service accounts.
- Configure image repository/tag/pull policy via values.
- Configure etcd endpoint/TLS and auth secrets.
- Support local/stage/prod values overlays.

## Operator Evidence Queries

- `GET /v1/loops/{id}/trace?limit=500` returns state, anomaly, journal, handoffs, overrides, and loop-scoped audit records.
- `GET /v1/loops/{id}/handoffs` returns append-only handoff chain for replica resumability analysis.
- `GET /v1/loops/{id}/overrides` returns append-only operator override history.
- `GET /v1/audit?loop_id={id}` returns immutable audit records scoped to a loop (auth required).

## MVP Exit Criteria

- Core watcher processes unresolved loops end-to-end.
- Per-loop lock prevents concurrent mutating replicas.
- Journal + handoff trace exists for every loop execution.
- Operator actions are authenticated, authorized, and audited.
- Reconciliation prevents persistent zombie/drift states.

## Delivery Verification (2026-03-08)

Task record:
- `td-395d2a` - Smith MVP: distributed autonomous orchestration

Validation gates executed:
- `go test ./...` passed (API, Core, Replica, reconcile, e2e, acceptance packages).
- `helm template smith ./helm/smith -f helm/smith/values/local.yaml` passed (322 rendered lines).
- `./scripts/validate-acceptance.sh` passed (0 failures; 2 warnings for optional runtime checks skipped without active cluster/runtime session).

Operator control surface verified:
- `POST /v1/control/override` (state override with audit + journal event)
- `POST /v1/loops/{id}/control/attach` (interactive attach with audit + journal event)
- `POST /v1/loops/{id}/control/detach` (interactive detach with audit + journal event)
- `POST /v1/loops/{id}/control/command` (interactive command intent with audit + journal event)

Traceability evidence queries verified:
- `GET /v1/loops/{id}/trace`
- `GET /v1/loops/{id}/journal`
- `GET /v1/loops/{id}/handoffs`
- `GET /v1/loops/{id}/overrides`
- `GET /v1/audit?loop_id={id}`
