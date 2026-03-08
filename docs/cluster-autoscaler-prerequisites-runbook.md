# Cluster Autoscaler Prerequisites and Runbook

This runbook defines provider-agnostic requirements for safely operating Smith with a cloud cluster autoscaler.

## Scope

Applies to Kubernetes clusters where:

- Smith control plane (`smith-core`, `smith-api`, `smith-console`) runs as Deployments with HPA.
- Smith replica workload runs as bursty Kubernetes Jobs.
- Node elasticity is provided by a cluster autoscaler implementation.

## Prerequisites

- Cluster autoscaler installed and healthy in-system namespace.
- At least one dedicated node pool for Smith replica burst workloads.
- Node pool labels/taints match Smith scheduling policy (`nodeSelector`/`tolerations` in Helm values).
- Resource requests/limits are set for control-plane components and replica Jobs.
- HPA enabled for control-plane services (`core.autoscaling.enabled`, `api.autoscaling.enabled`, `console.autoscaling.enabled`).
- PodDisruptionBudget and priority classes configured per production policy.
- Metrics pipeline (metrics-server + autoscaler metrics source) is available.

## Baseline Capacity Strategy

Use separate pools:

- Control-plane pool:
  - min nodes >= quorum-safe deployment count (recommend 3).
  - low churn, higher availability class.
- Replica pool:
  - min nodes >= steady-state job concurrency floor.
  - max nodes sized to peak concurrent loops and SLA target.

Sizing formula (starting point):

- `replica_min_nodes = ceil(steady_state_concurrency * replica_request_cpu / node_allocatable_cpu)`
- `replica_max_nodes = ceil(peak_concurrency * replica_request_cpu / node_allocatable_cpu) + surge_buffer`

Use a `surge_buffer` of at least `1` node for bin-packing fragmentation and startup jitter.

## Safety Constraints

- Do not set `maxNodesTotal` lower than required for control-plane minimum replicas + active replica Jobs.
- Keep HPA scale-down stabilization windows conservative (`300s` to `600s`) to avoid fight with node scale-down.
- Avoid simultaneous changes to:
  - HPA min/max
  - cluster autoscaler min/max nodes
  - replica resource requests
- Drain protection:
  - do not evict etcd and Smith core critical pods during autoscaler churn windows.

## Release Change Procedure

1. Verify baseline:
   - `kubectl get hpa -n smith-system`
   - `kubectl get nodes -L nodepool`
   - autoscaler health and event stream are clean.
2. Apply control-plane HPA change first (if needed).
3. Observe for one scale cycle (at least 15 minutes).
4. Apply cluster autoscaler node pool bound changes.
5. Run parity checks:
   - single-loop e2e
   - multi-loop e2e
   - override/recovery validation
6. Record observed scale metrics and update runbook notes.

## Failure Modes

### Pods Pending During Burst

Symptoms:

- Replica Jobs stay `Pending` with insufficient CPU/memory events.

Likely causes:

- Replica node pool max too low.
- Pod requests too high for node shape.
- Node selector/taint mismatch.

Actions:

1. Inspect scheduling events: `kubectl describe pod <pod>`.
2. Increase replica pool max nodes or adjust pod requests.
3. Validate selector/toleration wiring in Helm values.

### HPA Thrash + Node Churn

Symptoms:

- Frequent up/down scaling every few minutes.

Likely causes:

- Aggressive HPA behavior settings.
- Cluster autoscaler scale-down too fast.

Actions:

1. Increase HPA scale-down stabilization window.
2. Increase autoscaler scale-down delay/unneeded time.
3. Re-check request baselines for noisy components.

### Control Plane Starvation

Symptoms:

- API/Core latency spikes while replicas scale.

Likely causes:

- Shared node pool contention.
- Missing priority segregation.

Actions:

1. Isolate control plane and replica pools.
2. Raise control-plane pod priority.
3. Set minimum control-plane node floor to 3.

## Troubleshooting Checklist

- `kubectl get pods -A --field-selector=status.phase=Pending`
- `kubectl get hpa -n smith-system -o wide`
- `kubectl top pods -n smith-system`
- `kubectl top nodes`
- `kubectl get events -A --sort-by=.lastTimestamp | tail -n 200`
- autoscaler logs show scale-up decisions for pending Smith pods
- no continuous `FailedScheduling` loop for replica namespace

## Operational Guardrails

- Run pre-release system gate before promotion:
  - `.github/workflows/pre-release-system-gate.yml`
- Keep replica node pool max changes under 25% per rollout unless incident response is active.
- Revert strategy: restore previous node bounds first, then rollback HPA changes if instability persists.

## Related Docs

- `docs/deployment-recommendations.md`
- `docs/helm-upgrade-rollback-runbook.md`
- `docs/test-matrix-and-failure-injection.md`
- `docs/pre-release-system-gate.md`
