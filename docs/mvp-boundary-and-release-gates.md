# Smith MVP Boundary and Release Gates

## In Scope (MVP)

- etcd-backed loop state, journal, handoff, and lock records.
- Kubernetes-native execution model (Core + Replica Jobs).
- Operator API + Console with live journal and audited overrides.
- Helm-based deployment for control-plane services.
- Single provider path (Codex) with secure credential references.
- `smithctl` lifecycle control for create/get/logs/attach/cancel.

## Out of Scope (Post-MVP)

- Full multi-provider dynamic arbitration.
- Advanced autoscaling policies tuned by custom metrics.
- External secret-manager integrations beyond Kubernetes Secrets.
- Arbitrary third-party ingress plugin framework.

## Required Release Gates

### Gate 1: Data Integrity
- etcd schema version `v1` documented and implemented.
- State transitions validated (`unresolved -> overwriting -> synced|flatline|cancelled`).
- Append-only journal and handoff records verified.

### Gate 2: Orchestration Correctness
- Core watcher launches Jobs for unresolved loops.
- Lock semantics prevent concurrent mutating workers for same loop.
- Reconciler identifies and resolves orphaned/zombie execution.

### Gate 3: Operator Safety
- RBAC enforced for state overrides and destructive controls.
- All overrides and control actions produce audit entries.
- Console and CLI expose intervention paths with confirmation.

### Gate 4: Traceability
- Every loop run has correlated anomaly/state/journal/handoff chain.
- Correlation IDs preserved across API, Core, Replica, and audit logs.
- Evidence query documented for operator and incident workflows.

### Gate 5: Deployability
- Helm chart installs cleanly to a target namespace.
- Configurable image tags and pull policies are wired.
- etcd endpoint/auth configuration validated at startup.

### Gate 6: Reliability Baseline
- Failure injection covers worker crash, API restart, and temporary etcd outage.
- No ambiguous terminal state after crash/restart scenarios.
- Recovery runbook validated for restore and reconcile.

## MVP Sign-Off Checklist

- [ ] FR-001 through FR-012 verified in an integration environment.
- [ ] NFR-001, NFR-003, NFR-005, NFR-006, NFR-009 validated.
- [ ] Manual operator override tested and audited.
- [ ] Helm upgrade + rollback dry run completed.
- [ ] Release notes include known limitations and deferred backlog links.
