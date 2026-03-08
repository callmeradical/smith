# Smith

Smith is an etcd-backed, Kubernetes-native autonomous orchestration platform.

## Purpose

Smith coordinates autonomous execution loops as a state machine stored in etcd. It is designed to:

- accept loop ingress requests from operator-facing APIs,
- drive loop execution safely with lock-based concurrency,
- run loop workers as Kubernetes Jobs,
- maintain journal, handoff, and audit trails for every loop.

In this repository, the focus is the MVP control plane, deployment assets, and verification/test harnesses.

## Architecture Summary

Smith is split into control-plane and data-plane components.

### Control Plane

- `smith-api` (`cmd/smith-api`): HTTP API for loop create/list/get, GitHub + PRD ingress, operator override actions, provider auth lifecycle, and cost reporting.
- `smith-core` (`cmd/smith-core`): watches unresolved loop state in etcd, acquires per-loop locks, transitions loop state, and schedules replica Jobs in Kubernetes.
- `smithctl` (`cmd/smithctl`): kubectl-style operator CLI for `loop` and `prd` resources with context/config support and scriptable JSON output.
- `smith-console` (`console/` + Helm deployment): operator UI/runtime assets.
- etcd: authoritative source of truth for anomalies, loop lifecycle state, locks, journal events, handoffs, overrides, and audit records.

### Data Plane

- `smith-replica` (`cmd/smith-replica`): Kubernetes Job worker that executes loop work, appends journal entries, writes handoff output, and finalizes loop state.

### Deployment and Ops Assets

- Helm chart: `helm/smith`
- Dockerfiles: `docker/`
- Core implementation: `internal/source/`
- Supporting docs: `docs/`
- Make-first local workflow: `make help` (doctor/bootstrap/cluster/deploy/test/teardown)

## Key API Endpoints

- `POST /v1/loops` single/batch direct loop creation.
- `POST /v1/loops` supports environment profiles (`preset`, `mise`, `container_image`, `dockerfile`) with server-side validation/defaulting.
- `POST /v1/ingress/github/issues` ingest one or more GitHub issues into loop specs.
- `POST /v1/ingress/prd` ingest markdown/json PRD inputs into loop specs.
- `GET /v1/loops/{id}` and `GET /v1/loops/{id}/journal` for state and traceability.
- `GET /v1/loops/{id}/handoffs`, `GET /v1/loops/{id}/overrides`, and `GET /v1/loops/{id}/trace` for end-to-end execution evidence.
- `POST /v1/control/override` for operator state overrides with reason/audit trail.
- `GET /v1/audit?loop_id={id}` for immutable operator/auth action audit records.

## Local Git Hooks

Install repo-managed hooks:

```bash
make hooks-install
```

Hook behavior:
- `pre-commit`: quick checks (`go test ./cmd/...`)
- `pre-push`: full gate (`make build` + `make test`)

Temporarily bypass hooks if needed:

```bash
SKIP_GIT_HOOKS=1 git commit -m "..."
SKIP_GIT_HOOKS=1 git push
```
