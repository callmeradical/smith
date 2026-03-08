# Smith Deployment Recommendations

## Recommended Production Posture

Smith is optimized for cloud deployment on Kubernetes with autoscaling enabled.

Preferred baseline:
- Managed Kubernetes cluster in cloud environment.
- Cluster autoscaling enabled (node-level scaling).
- Horizontal Pod Autoscaling (HPA) enabled for Smith control-plane services.

## Why

- Loop demand is bursty; autoscaling avoids overprovisioning while preserving throughput.
- Smith executes many parallel replicas; node-level elasticity is required for scale.
- Agent Core and supporting APIs can become CPU/memory bound under high anomaly concurrency.

## Guidance

### Cluster Autoscaling
- Enable cluster/node autoscaler for worker node groups.
- Set min/max node boundaries aligned with expected loop concurrency.
- Reserve headroom for system components to avoid scheduler starvation.
- Use `docs/cluster-autoscaler-prerequisites-runbook.md` as the operational runbook for rollout, failure modes, and troubleshooting.

Recommended node group bounds:
- `local`: min 1, max 3 nodes.
- `stage`: min 3, max 12 nodes.
- `prod`: min 6, max 60 nodes.

Node pool split for production:
- Control plane pool (`smith-core`, `smith-api`, `smith-console`): min 3, max 15.
- Replica pool (`smith-replica` Jobs): min 3, max 45.

### HPA
- Configure HPA for Agent Core and API/WebSocket services.
- Start with CPU and memory targets; add custom metrics over time (queue depth, active anomalies).
- Define scale-up/down stabilization windows to avoid oscillation.

Recommended production targets:
- `core`: min 3 / max 30, CPU 60%, memory 70%, scale-down stabilization 600s.
- `api`: min 3 / max 40, CPU 55%, memory 65%, scale-down stabilization 300s.
- `console`: min 2 / max 10, CPU 65%, memory 75%, scale-down stabilization 300s.

Rollout policy:
- Deploy HPA with conservative max bounds first.
- Observe saturation and queue depth for 3 business days.
- Increase `maxReplicas` in +25% steps when p95 queue delay exceeds SLO.
- Do not change node autoscaler and HPA bounds in the same rollout window.

### Capacity Planning
- Define resource classes for replicas (small/medium/large loops).
- Use namespace quotas and priority classes to protect critical control-plane services.
- Validate behavior under load with scheduled stress tests.

### Reliability
- Pair autoscaling with reconciliation and drift detection.
- Ensure observability dashboards include scaling events, queue depth, and anomaly completion rate.

## Non-Cloud / Local

Local and CI environments (e.g., k3d + vCluster) remain valid for development and verification, but are not a substitute for production autoscaling validation.
