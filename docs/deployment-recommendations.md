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

### HPA
- Configure HPA for Agent Core and API/WebSocket services.
- Start with CPU and memory targets; add custom metrics over time (queue depth, active anomalies).
- Define scale-up/down stabilization windows to avoid oscillation.

### Capacity Planning
- Define resource classes for replicas (small/medium/large loops).
- Use namespace quotas and priority classes to protect critical control-plane services.
- Validate behavior under load with scheduled stress tests.

### Reliability
- Pair autoscaling with reconciliation and drift detection.
- Ensure observability dashboards include scaling events, queue depth, and anomaly completion rate.

## Non-Cloud / Local

Local and CI environments (e.g., k3d + vCluster) remain valid for development and verification, but are not a substitute for production autoscaling validation.
