# Smith Requirements Traceability Matrix

Sources:
- Requirements: `docs/requirements-fr-nfr.md`
- Backlog: `td` issues in this repository
- MVP delivery umbrella: `td-395d2a` (distributed autonomous orchestration end-to-end verification)

## Functional Requirements Traceability

| Requirement | Mapped td Issues | Notes |
| --- | --- | --- |
| FR-001 Anomaly persistence in etcd | td-39a505, td-7740c7 | Data model + schema versioning/migration |
| FR-002 Task lifecycle state tracking | td-39a505, td-9908f3, td-e10388 | State schema, watcher transitions, reconciliation |
| FR-003 Append-only journal stream | td-036323, td-113f99, td-90af46 | Journal pipeline, live UI stream, retention follow-up |
| FR-004 Agent Core unresolved watcher | td-9908f3, td-f92499 | Watch handling and single-writer safety |
| FR-005 Replica job orchestration | td-8544d1, td-d1201a | Core job generation + Helm template integration |
| FR-006 Replica context resume via handoff | td-744ecf, td-39a505 | Startup memory transfer + typed handoff schema |
| FR-007 Strict completion consistency protocol | td-c60475, td-e10388, td-c6301e | Saga semantics + reconciliation + Git policy |
| FR-008 Reconciliation for drift/zombies | td-e10388, td-f92499 | Drift repair/escalation + concurrency guardrails |
| FR-009 Console grid + live journal | td-113f99, td-433354 | Console features + latency validation |
| FR-010 Manual operator override | td-59f69d, td-ccc047 | Override UX + RBAC/audit controls |
| FR-011 Loop policy configurability | td-d12fd7, td-9908f3 | UI-defined policy + runtime enforcement |
| FR-012 Deployability via Helm | td-56f4e8, td-a1aebc, td-bcc114, td-c38e55, td-c1d238, td-b7aa01 | Chart, values, secrets, env overlays, ops runbook, image wiring |
| FR-013 Containerized delivery | td-366583, td-ece36c, td-ffc123, td-a096c0, td-b77cdd | Dockerfiles, publish pipeline, versioning, multi-arch |
| FR-014 Agent provider routing and Codex authentication | td-f8e9dd, td-afdaa4, td-5842f1 | Provider abstraction + Codex login/refresh + operator auth UX |
| FR-015 Multi-source loop ingress and smithctl control surface | td-93543b, td-dc2332, td-fa3f21, td-6de678, td-bb3ded, td-c37a53, td-b7248b, td-2b3e2f | Ingress API + GitHub/PRD ingestion + interactive attach + smithctl lifecycle flows |
| FR-016 Configurable loop execution environments | td-2517d0, td-8e7c54, td-65e317, td-c7e14a, td-f93e43, td-fdcb47, td-c155ab | Environment schema + mise/image/Dockerfile/preset execution modes + CLI and e2e coverage |
| FR-017 Skill volume mounts in loop runtime (Codex-first) | td-d93ca9, td-2f6291, td-75f932, td-d1c89c, td-bafce8, td-a2e46e | Skill mount schema + runtime injection + API validation + smithctl support + e2e coverage |

## Non-Functional Requirements Traceability

| Requirement | Mapped td Issues | Notes |
| --- | --- | --- |
| NFR-001 Consistency and correctness | td-c60475, td-f92499, td-e10388, td-59d13e | Completion protocol, lock semantics, reconciliation, failure testing |
| NFR-002 Scalability | td-8544d1, td-f92499, td-56f4e8, td-c38e55 | K8s job scaling, concurrency model, deploy profile support |
| NFR-003 Traceability and auditability | td-036323, td-c6301e, td-ccc047, td-59f69d | Journaling, Git traceability, audited overrides |
| NFR-004 Observability latency | td-113f99, td-433354 | Console streaming + watch-based fanout + latency p95 benchmark harness |
| NFR-005 Resilience and recoverability | td-e0abfb, td-e10388, td-59d13e | Backup/restore, reconciliation recovery, chaos tests |
| NFR-006 Security | td-ccc047, td-bcc114, td-be5578, td-e55c12 | RBAC/audit, PAT secret handling, deferred auth alternatives |
| NFR-007 Cost visibility | td-dd0e4d, td-036323 | Token/cost reporting and underlying event stream |
| NFR-008 Usability and accessibility | td-113f99, td-d12fd7, td-59f69d | Console experience + style/accessibility contract |
| NFR-009 Maintainability and release control | td-eef8f8, td-c1d238, td-59d13e, td-a096c0 | MVP gates, release runbook, test matrix, version policy |
| NFR-010 Credential security and auth reliability | td-afdaa4, td-5842f1, td-ccc047, td-bcc114 | Secure token storage, auth lifecycle events, RBAC and secrets controls |
| NFR-011 Operational ergonomics and automation readiness | td-bb3ded, td-c37a53, td-b7248b, td-2d1f40 | smithctl scriptability and non-UI loop operations |
| NFR-012 Environment reproducibility and supply-chain safety | td-2517d0, td-65e317, td-c7e14a, td-c155ab | Deterministic environment resolution, image/build validation, and auditable runtime metadata |
| NFR-013 Safe and auditable skill mount execution | td-d93ca9, td-75f932, td-2f6291, td-bafce8 | Policy-constrained skill sources/mount paths with consistent audit trail |

## Coverage Gaps

Current mapping indicates every FR/NFR in `docs/requirements-fr-nfr.md` has at least one linked td issue. Quantitative SLO targets remain tracked as open clarifications in requirements doc.
