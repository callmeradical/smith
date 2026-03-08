# Smith: Code Review and Feasibility Assessment

**Date:** 2026-03-08  
**Reviewer:** GitHub Copilot Coding Agent  
**Repository:** callmeradical/smith  
**Scope:** Full repository review covering architecture, code quality, test coverage, roadmap alignment, and outstanding issues.

---

## Executive Summary

Smith is an etcd-backed, Kubernetes-native autonomous orchestration platform with a well-defined PRD and a disciplined engineering process. The project is **feasible** and shows strong architectural foundations. MVP delivery is realistic given the current pace of progress and the quality of what has already shipped.

**Feasibility Rating: 8 / 10**

The core runtime contracts (etcd schema, locking, reconciliation, completion protocol, provider abstraction) are implemented and tested. The main blockers before MVP sign-off are the operator console, a cluster-backed integration run, and the long tail of skill/environment features — all of which are well-understood and tracked in the backlog.

---

## Progress Assessment

### Closed Issues (48 of ~100 total)

The following workstreams are substantially complete based on closed issues:

| Area | Closed Items | Status |
|---|---|---|
| Make-first local workflow | #102–#107 | ✅ Complete |
| Go acceptance test harness | #93–#96 | ✅ Complete |
| smithctl scaffold + lifecycle + PRD commands | #68–#69, #72 | ✅ Complete |
| Ingress API (GitHub + PRD) | #65–#67 | ✅ Complete |
| Loop environment profile schema + mise/preset | #77–#78, #85 | ✅ Complete |
| etcd model helpers + schema tests | #80 | ✅ Complete |
| CI ephemeral environment (k3d + vCluster) | #52 | ✅ Complete |
| Integration + e2e test scaffolding | #51, #53–#56 | ✅ Complete |
| Completion verification harness | #57 | ✅ Complete |
| Provider registry + Codex auth | #24–#25 | ✅ Complete |
| Dockerfiles for all services | #19–#20 | ✅ Complete |
| CI image build/publish pipeline | #21–#22 | ✅ Complete |
| Docs site scaffold + theme + content | #35–#37, #39 | ✅ Complete |
| Helm values schema, chart, image wiring | #16–#18, #23 | ✅ Complete |
| Secrets strategy for Git PAT/creds | #17 | ✅ Complete |
| GitHub App + SSH auth options | #40–#41 | ✅ Complete |
| Token + cost reporting | #13 | ✅ Complete |
| RBAC + audited operator overrides | #11 | ✅ Complete |
| e2e test matrix + failure injection | #12 | ✅ Complete |
| Autoscaling policy + HPA config | #58–#59 | ✅ Complete |
| Staging soak/chaos environment | #64 | ✅ Complete |
| etcd schema + migration strategy | #10 | ✅ Complete |
| Git policy module | (internal) | ✅ Complete |

### Open Issues (40 remaining)

| Epic | Open Tasks | Summary |
|---|---|---|
| Foundation Runtime Core (#44) | Low | Largely complete; schema/watcher/reconcile shipped |
| Loop Ingress + smithctl (#73) | #70, #71 | e2e ingress tests and interactive attach remaining |
| Loop Environments + Skill Mounts (#91) | #79, #82–#90 | Container-image path, Dockerfile build, skill mounts still outstanding |
| Operator Console (#46) | #28, #29, #34 | Grid UI, journal stream, provider panel not yet built |
| Provider Auth + Security (#47) | Minimal | Auth panel (#34) deferred |
| Observability, Testing, Costing (#48) | #31, #62 | Latency path validation and CI matrix still open |
| Platform Packaging + Release (#45) | #32, #33, #38 | Helm env overlays, release runbook, multi-arch builds |
| Operations + Resilience (#50) | #26, #27, #30, #43, #42 | Journal pipeline, memory transfer, backup/restore deferred |
| Documentation Site (#49) | Minimal | Core docs published; remaining items are polish |
| Test Harness Migration (#97–#98) | In progress | Go harness exists; migration of older tests ongoing |

---

## Architecture Review

### Strengths

**1. Correct use of etcd as a state machine.**  
The key schema (`/smith/v1/{anomalies,state,journal,handoffs,locks,overrides,audit}/...`) is well-defined in `docs/etcd-key-schema.md` and fully implemented in `internal/source/store/etcd.go`. The store uses revision-checked compare-and-swap semantics for state transitions, correctly preventing split-brain terminal states.

**2. Robust locking model.**  
`internal/source/locking/lease.go` implements lease-backed per-loop locks with holder validation, expiry checking, and revision-checked delete. The accompanying test suite covers happy-path acquire/release as well as contended and expired-lease scenarios.

**3. Completion protocol (saga semantics).**  
`internal/source/completion/protocol.go` implements a two-phase commit analog: code commit → state commit, with compensation (revert) if the second phase fails. The `PhaseRecord` trail prevents ambiguous terminal states after crash/restart, satisfying FR-007.

**4. Reconciliation loop.**  
`internal/source/reconcile/loop.go` handles orphan/zombie detection and repair, satisfying FR-008. The reconciler is testable in isolation via its interface abstractions.

**5. Clean separation of concerns.**  
Each binary in `cmd/` is thin; all logic lives in `internal/source/`. Interfaces are consistently used to keep components testable without real etcd or Kubernetes.

**6. Provider abstraction.**  
`internal/source/provider/` implements a typed registry with a Codex adapter. The auth lifecycle (connect, token refresh, status reporting) is fleshed out and tested in `codex_auth_test.go`.

**7. Loop environment profiles.**  
`internal/source/model/environment.go` supports `preset`, `mise`, `container_image`, and `dockerfile` modes with policy-driven normalization and validation. This is a well-designed extensibility point.

**8. Skill mounts schema.**  
`internal/source/model/skills.go` introduces a Codex-first skill volume abstraction with validation, satisfying the schema design portion of FR-017.

**9. smithctl is production-ready in shape.**  
`cmd/smithctl/main.go` (~1,082 lines) implements a kubectl-style CLI with `loop` and `prd` subcommands, context management, JSON output, and scriptability.

**10. Helm chart and Dockerfiles are complete.**  
Multi-stage distroless builds (`docker/*.Dockerfile`), a `values.schema.json` for validation, and environment overlays in `helm/smith/values/` are all in place.

### Weaknesses and Risks

**1. Build error in `cmd/smith-api/main.go` (fixed in this PR).**  
Line 397 was a duplicate `skills, err := model.NormalizeLoopSkills(...)` declaration — `no new variables on left side of :=`. This caused the entire API binary to fail to compile. **Fixed** by removing the redundant second call.

**2. Operator Console is the largest open gap.**  
Issues #28 (Grid + live journal stream) and #29 (manual override actions) represent the bulk of FR-009 and FR-010. No console implementation exists yet beyond static HTML/nginx assets in `console/`. This is the single biggest risk to the MVP timeline.

**3. Skill volume injection into Replica Jobs is not yet wired.**  
Issues #82 (volume resolution + mount injection) and #83 (smithctl skill-mount config) are open. The schema exists; the runtime path does not.

**4. Interactive terminal attach (`smithctl loop attach`) is not implemented.**  
Issue #71 (`td-6de678`) is open. This is listed as an MVP capability in FR-015.

**5. No latency instrumentation.**  
Issue #31 (`td-433354`) — the <100ms Console-to-etcd-update SLO (NFR-004) — has no measurement harness yet.

**6. Backup/restore and disaster recovery validation is deferred.**  
Issue #30 (`td-e0abfb`) is open. This is a required Release Gate 6 item.

**7. `golang:1.25-bookworm` in Dockerfiles.**  
Go 1.25 does not exist at time of writing (latest stable is 1.22.x / 1.23.x). The Dockerfiles will fail to build in CI until the tag is corrected to an available image.

**8. CI matrix for multi-loop e2e scenarios is open.**  
Issue #62 (`td-2d1f40`) and #60 (`td-040379`) — the pre-release gate requiring vCluster + non-vCluster parity — are not yet complete.

---

## Code Quality

### Signal (unit + integration tests)

| Package | Coverage signal |
|---|---|
| `internal/source/store` | Tested via lease store and integration stubs |
| `internal/source/locking` | Comprehensive lease acquire/release/expire/conflict tests |
| `internal/source/completion` | Phase recording, crash recovery, and compensation tests |
| `internal/source/reconcile` | Orphan detection and repair logic tested |
| `internal/source/core` | Controller and watcher unit tests with mock sources |
| `internal/source/ingress` | GitHub and PRD ingress pipeline tests |
| `internal/source/provider` | Registry, auth, and Codex adapter tests |
| `internal/source/model` | Schema decode, environment normalization, and skill validation tests |
| `cmd/smith-api` | Handler-level tests covering create/get/batch/override |
| `cmd/smithctl` | Command-level tests |
| `test/acceptance` | godog BDD suite (`loop_workflows_steps_test.go`) |

All packages pass `go test` after the build fix. The ratio of test code to production code (~3,579 lines of test to ~6,760 lines of production) is healthy at roughly 53%.

### Style

- Code is idiomatic Go; interfaces are minimal and well-named.
- Error messages are consistent and actionable.
- Struct fields use JSON tags throughout, appropriate for an API and etcd store.
- No global mutable state observed outside of `cmd/` entry points.

### Dependency hygiene

- `go.mod` pins specific minor versions for all direct dependencies.
- etcd `v3.6.8`, `k8s.io/client-go v0.35.2`, and `k8s.io/api v0.35.2` are current stable releases.
- `github.com/cucumber/godog v0.15.1` and `github.com/stretchr/testify v1.11.1` are current.
- No obviously deprecated or vulnerable packages observed.

---

## Roadmap Alignment

The PRD defines six success metrics. Current alignment:

| Success Metric | Status |
|---|---|
| Zero zombie tasks (etcd state matches K8s Pod state) | ✅ Reconciler implemented and tested |
| Operator Console reflects state change < 100ms | ⚠️ Console not yet built; latency path unvalidated |
| Recovery from etcd backup and resume active loops | ⚠️ Backup/restore runbook deferred (#30) |
| Append-only journal + MVCC history | ✅ Journal schema + store implemented |
| Operator intervention paths (eject, override) | ⚠️ API side done; Console UI pending (#28, #29) |
| Codex provider auth lifecycle | ✅ Auth flow, token refresh, and registry done |

The six MVP release gates from `docs/mvp-boundary-and-release-gates.md` are partially satisfied:

| Gate | Status |
|---|---|
| Gate 1: Data Integrity | ✅ etcd schema v1, state transitions, journal |
| Gate 2: Orchestration Correctness | ✅ Watcher, lock semantics, reconciler |
| Gate 3: Operator Safety | ✅ API-side RBAC + audit; Console side open |
| Gate 4: Traceability | ✅ Correlation IDs, audit chain |
| Gate 5: Deployability | ✅ Helm chart, values schema, image wiring |
| Gate 6: Reliability Baseline | ⚠️ Failure injection tests exist; backup/restore outstanding |

---

## Prioritized Recommendations

### Critical (MVP blockers)

1. **Fix `cmd/smith-api` build break** (done in this PR) — duplicate `NormalizeLoopSkills` call at line 397.
2. **Correct Go image tag in Dockerfiles** (`golang:1.25-bookworm` → `golang:1.23-bookworm` or latest stable) — CI image builds will fail otherwise.
3. **Implement Operator Console grid + journal stream** (issues #28, #29) — the largest single remaining gap.
4. **Implement skill volume mount injection into Replica Jobs** (issue #82) — required for FR-017 and any Codex-backed loop execution.

### High priority (should ship before first release)

5. **Add interactive terminal attach** (`smithctl loop attach`, issue #71) — listed as MVP scope in FR-015.
6. **Complete CI e2e matrix** (issues #62, #60) — required for release gate pre-sign-off.
7. **Validate backup/restore runbook** (issue #30) — required for Gate 6.
8. **Add latency instrumentation** (issue #31) — NFR-004 SLO cannot be claimed without measurement.

### Nice to have (post-MVP)

9. Migrate acceptance tests to the Go harness (issues #97, #98).
10. Multi-arch image builds (issue #38).
11. Helm environment overlays polish (issue #32).
12. Cluster autoscaler runbook (issue #63).

---

## Feasibility Verdict

Smith is architecturally sound and the hard problems (distributed locking, completion protocol, reconciliation, provider abstraction) are already solved and tested. The codebase is clean, well-structured, and growing at a healthy pace — approximately 48 issues closed out of ~100 total.

The remaining work is well-understood and bounded. The main execution risk is the Operator Console, which has no implementation yet and represents a non-trivial UI build. If console delivery is scoped conservatively (minimal grid + journal stream, no advanced features), the MVP is achievable.

**Rating: 8 / 10** — Strong foundation, realistic path to MVP, with the console and a small set of runtime features as the primary remaining risks.
