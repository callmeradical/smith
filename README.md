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

- `smith-api` (`cmd/smith-api`): HTTP API for loop create/list/get, operator override actions, provider auth lifecycle, and cost reporting.
- `smith-core` (`cmd/smith-core`): watches unresolved loop state in etcd, acquires per-loop locks, transitions loop state, and schedules replica Jobs in Kubernetes.
- `smith-console` (`console/` + Helm deployment): operator UI/runtime assets.
- etcd: authoritative source of truth for anomalies, loop lifecycle state, locks, journal events, handoffs, overrides, and audit records.

### Data Plane

- `smith-replica` (`cmd/smith-replica`): Kubernetes Job worker that executes loop work, appends journal entries, writes handoff output, and finalizes loop state.

### Deployment and Ops Assets

- Helm chart: `helm/smith`
- Dockerfiles: `docker/`
- Core implementation: `internal/source/`
- Supporting docs: `docs/`
