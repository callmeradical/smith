# Smith Requirements Matrix (FR/NFR)

Source PRD: `docs/prd1.md`

## Functional Requirements (FR)

### FR-001: Anomaly persistence in etcd
- Description: Persist anomaly/task definitions in etcd under `/smith/anomalies/{id}`.
- Inputs: New anomaly metadata and identifiers.
- Outputs: Durable anomaly object retrievable by ID.
- Acceptance:
  - Create/read/update operations succeed for valid anomaly IDs.
  - Object can be fetched by worker/core components.
  - Invalid payloads are rejected with clear errors.

### FR-002: Task lifecycle state tracking
- Description: Persist lifecycle state under `/smith/state/{id}` with at least `unresolved`, `running`, `synced`.
- Acceptance:
  - State transitions are persisted and watchable.
  - Invalid transition attempts are rejected or flagged.

### FR-003: Append-only journal stream
- Description: Record agent actions and tool output under `/smith/journal/{id}/{timestamp}`.
- Acceptance:
  - Journal entries are append-only and time-ordered per anomaly.
  - Operator consumers can stream/read entries in near-real-time.

### FR-004: Agent Core unresolved watcher
- Description: Controller watches `/smith/state/` and reacts to `unresolved` anomalies.
- Acceptance:
  - New/updated `unresolved` anomalies are detected idempotently.
  - Duplicate watch events do not trigger duplicate active execution.

### FR-005: Replica job orchestration
- Description: Agent Core creates and manages Kubernetes Jobs for replicas.
- Acceptance:
  - Job specs include anomaly context (e.g., STORY_ID) and Git context.
  - Job lifecycle events feed back into state machine handling.

### FR-006: Replica context resume via handoff
- Description: Replica loads anomaly metadata + latest handoff on startup.
- Acceptance:
  - Startup blocks until required context load succeeds/fails deterministically.
  - Missing prior handoff is handled safely for first-run anomalies.

### FR-007: Completion protocol with strict consistency semantics
- Description: Completion flow ensures code update + etcd state transition are never terminally ambiguous.
- Acceptance:
  - Saga/compensation or equivalent protocol implemented.
  - Crash points are recoverable; terminal ambiguity is prevented.

### FR-008: Reconciliation loop for drift/zombies
- Description: Continuously reconcile etcd task state with K8s runtime state.
- Acceptance:
  - Mismatches are auto-repaired or escalated to Flatline.
  - Reconciliation outcomes are observable and auditable.

### FR-009: Operator Console grid + live journal
- Description: UI provides active replica grid and live journal stream per anomaly.
- Acceptance:
  - Selecting an anomaly attaches a live stream.
  - Grid reflects active workloads and key health signals.

### FR-010: Manual operator override
- Description: Operators can eject a replica and edit anomaly state.
- Acceptance:
  - Override actions require confirmation and are audited.
  - Override effects propagate correctly to controller/runtime.

### FR-011: Loop policy configurability
- Description: Retry/backoff/termination policy is configurable when defining a loop.
- Acceptance:
  - Policy configuration is validated and persisted.
  - Agent Core behavior reflects configured policy.

### FR-012: Deployability via Helm
- Description: Smith deploys via Helm with values-driven configuration.
- Acceptance:
  - Core chart installs core services successfully.
  - Environment overlays (local/stage/prod) are supported.
  - Secret integration and image references are configurable.

### FR-013: Containerized delivery
- Description: Core, replica, and console are shipped as container images.
- Acceptance:
  - CI builds/tags/scans/publishes images.
  - Helm values reference versioned image tags.

### FR-014: Agent provider routing and Codex authentication
- Description: Smith can select an agent provider per loop and authenticate to Codex via a Codex CLI-style login flow.
- Acceptance:
  - Loop definition supports `provider_id` and `model` with validation.
  - Codex login flow supports operator connect/reconnect and token refresh lifecycle.
  - Provider calls fail safely with actionable auth status when credentials are missing/expired.

### FR-015: Multi-source loop ingress and smithctl control surface
- Description: Smith supports loop creation from GitHub issues, PRDs, and direct interactive control, with `smithctl` as primary operator CLI.
- Acceptance:
  - API supports single and batch loop creation with source metadata and idempotency.
  - Ingestion from GitHub issues and PRDs creates traceable loop specs.
  - `smithctl` supports loop create/get/logs/attach/cancel and PRD create/submit flows.

### FR-016: Configurable loop execution environments
- Description: Smith allows per-loop execution environment selection using `mise`, container image references, Dockerfile build specs, or named presets.
- Acceptance:
  - Environment schema supports precedence rules and validation for conflicting inputs.
  - Loops can run with deterministic tool/runtime resolution using selected environment mode.
  - Resolved environment metadata (image/tool versions) is recorded for traceability.

### FR-017: Skill volume mounts in loop runtime (Codex-first)
- Description: Smith supports loop-defined skill mounts as runtime volumes, with Codex default mountpoint behavior in MVP.
- Acceptance:
  - Loop schema supports skill source/version/mount path/read-only settings.
  - Replica job generation injects resolved skill mounts and fails fast on invalid/missing skills.
  - Resolved skill mount metadata is recorded in journal/handoff for traceability.

## Non-Functional Requirements (NFR)

### NFR-001: Consistency and correctness
- Requirement: etcd must be source-of-truth and avoid split-brain terminal states.
- Metric:
  - Zero ambiguous terminal completion states in failure-injection tests.
  - No concurrent mutating replicas per anomaly when single-writer lock is enabled.

### NFR-002: Scalability
- Requirement: Support parallel anomaly execution via Kubernetes.
- Metric:
  - Horizontal replica scaling supported without controller correctness regressions.

### NFR-003: Traceability and auditability
- Requirement: All significant actions are journaled and recoverable for audit.
- Metric:
  - Full action trail exists for each anomaly lifecycle.
  - Operator overrides include actor/time/reason.

### NFR-004: Observability latency
- Requirement: Console reflects state updates quickly.
- Metric:
  - Target p95 end-to-end propagation latency: <100ms from etcd update to console display (or measured gap documented with remediation plan).

### NFR-005: Resilience and recoverability
- Requirement: System can recover from etcd backup and resume active loops.
- Metric:
  - Restore drill demonstrates matrix recovery and loop continuation.
  - RTO/RPO values documented.

### NFR-006: Security
- Requirement: RBAC protects destructive actions; sensitive credentials handled securely.
- Metric:
  - Server-side authorization checks enforced for overrides.
  - No plaintext secret leakage in logs/config output.

### NFR-007: Performance efficiency and cost visibility
- Requirement: Runtime consumption is measurable and reportable.
- Metric:
  - Token and cost reports available by anomaly/time range.
  - Cost calculation basis is versioned/documented.

### NFR-008: Usability and accessibility
- Requirement: Operator UI works across desktop/mobile and supports accessibility baselines.
- Metric:
  - Responsive layouts for common breakpoints.
  - Keyboard focus and reduced-motion behavior implemented.

### NFR-009: Maintainability and release control
- Requirement: Explicit MVP scope and release gates are documented and enforced.
- Metric:
  - MVP checklist and deferred backlog tracked.
  - Upgrade/rollback runbook exists for Helm releases.

### NFR-010: Credential security and auth reliability
- Requirement: Provider credentials are stored and used securely with resilient refresh handling.
- Metric:
  - No raw provider tokens are persisted in etcd or logs.
  - Auth refresh success/failure events are auditable with alerting for persistent failures.

### NFR-011: Operational ergonomics and automation readiness
- Requirement: Loop control interfaces are scriptable and consistent for both interactive and non-interactive operations.
- Metric:
  - `smithctl` commands provide machine-parseable output for CI/automation workflows.
  - Core loop lifecycle actions are executable without UI dependency.

### NFR-012: Environment reproducibility and supply-chain safety
- Requirement: Loop environments must be reproducible and constrained by policy.
- Metric:
  - Environment resolution is deterministic across reruns for the same loop spec.
  - Registry/build-context validation prevents disallowed image sources and unsafe build paths.

### NFR-013: Safe and auditable skill mount execution
- Requirement: Skill mounts must be policy-constrained, read-only by default, and fully auditable.
- Metric:
  - Invalid mount sources/paths are rejected by validation policy.
  - Skill mount selection and resolved versions are consistently logged for every loop execution.

## Open Clarifications
- Define concrete throughput targets (e.g., max concurrent anomalies per cluster size).
- Define exact p95/p99 latency measurement methodology and sampling window.
- Define SLOs for reconciliation recovery time after drift detection.
