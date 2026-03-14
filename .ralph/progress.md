# Progress Log
Started: Mon Mar  9 04:32:36 EDT 2026

## Codebase Patterns
- (add reusable patterns here)

---
## [2026-03-09 04:41:52 EDT] - US-001: Resolve runtime pod/container for a loop
Thread: ses_e20a4f
Run: 20260309-043236-60668 (iteration 1)
Run log: /Users/lars/Dev/smith.terminal-support/.ralph/runs/run-20260309-043236-60668-iter-1.log
Run summary: /Users/lars/Dev/smith.terminal-support/.ralph/runs/run-20260309-043236-60668-iter-1.md
- Guardrails reviewed: yes
- No-commit run: false
- Commit: 2ccd8bb feat(api): add loop runtime resolution endpoint
- Post-commit status: `.ralph/progress.md` modified (pending progress commit)
- Verification:
  - Command: make build -> PASS
  - Command: go test ./cmd/smith-api/... -> PASS
  - Command: go test ./internal/source/... -> PASS
  - Command: go test ./... -> PASS
  - Command: npm run test:frontend -> PASS
  - Command: make test-matrix -> PASS
- Files changed:
  - cmd/smith-api/main.go
  - cmd/smith-api/main_test.go
  - helm/smith/templates/rbac.yaml
  - .ralph/progress.md
- What was implemented
  - Added runtime resolution for loop IDs using active loop state plus `worker_job_name` to discover runtime pods in Kubernetes.
  - Exposed `GET /v1/loops/{id}/runtime` returning namespace/pod/container/pod phase with `attachable` and `reason`.
  - Implemented negative reasons including exact `loop not active` and `runtime pod not found`, plus non-running/container-missing reasons.
  - Added unit tests for running, pending, terminal, and missing-runtime scenarios and route parsing for `/runtime`.
  - Updated API RBAC to allow pod `get`/`list` for runtime lookup.
- **Learnings for future iterations:**
  - Patterns discovered
  - `StateRecord.WorkerJobName` is the stable key for mapping loop state to runtime pod discovery via `job-name` label.
  - Gotchas encountered
  - API runtime namespace must default to the same namespace used by the auth store deployment env; relying on `POD_NAMESPACE` alone is not sufficient.
  - Useful context
  - Pre-commit hooks in this repo run `go test ./cmd/...` and can pull dependencies on first invocation.
---
## [2026-03-09 04:53:07 EDT] - US-002: Attach and detach terminal sessions against runtime target
Thread: ses_89ea7b
Run: 20260309-043236-60668 (iteration 2)
Run log: /Users/lars/Dev/smith.terminal-support/.ralph/runs/run-20260309-043236-60668-iter-2.log
Run summary: /Users/lars/Dev/smith.terminal-support/.ralph/runs/run-20260309-043236-60668-iter-2.md
- Guardrails reviewed: yes
- No-commit run: false
- Commit: 242c58f feat(api): bind terminal attach to runtime target
- Post-commit status: .ralph/runs/run-20260309-043236-60668-iter-2.log modified by post-commit hooks
- Verification:
  - Command: go test ./cmd/smith-api/... -> PASS
  - Command: make build -> PASS
  - Command: go test ./internal/source/... -> PASS
  - Command: go test ./... -> PASS
  - Command: npm run test:frontend -> PASS
  - Command: make test-matrix -> PASS
- Files changed:
  - cmd/smith-api/main.go
  - cmd/smith-api/main_test.go
  - .ralph/activity.log
  - .ralph/progress.md
  - .ralph/runs/run-20260309-043236-60668-iter-2.log
- What was implemented
  - Updated `POST /v1/loops/{id}/control/attach` to require resolved runtime target attachability before session attach, returning HTTP 409 when runtime pod is not Running.
  - Reworked terminal session tracking to persist actor session metadata (terminal source, status, runtime target reference, runtime identity fields, actor attach count).
  - Updated attach/detach audit and journal metadata to include actor, terminal source, and runtime target identity.
  - Updated `POST /v1/loops/{id}/control/detach` to detach only the specified attached actor while preserving other actor attachments.
  - Added API tests covering: runtime non-running attach conflict with no session creation, actor attach count increments, detach behavior, and attach/detach metadata assertions.
- **Learnings for future iterations:**
  - Patterns discovered
  - Handler-level dependency hooks (`getStateFn`, `appendAuditFn`, `appendJournalFn`) enable focused HTTP behavior tests without full etcd wiring.
  - Gotchas encountered
  - Pre-commit hooks can mutate iteration run logs after commit; check `git status --porcelain` immediately and include follow-up commit if needed.
  - Useful context
  - Runtime resolution helper already provides phase/reason fidelity, so attach conflict handling can reuse it directly for consistent API errors.
---
## [2026-03-09 05:05:28 EDT] - US-003: Execute line commands inside active loop container
Thread: 
Run: 20260309-043236-60668 (iteration 3)
Run log: /Users/lars/Dev/smith.terminal-support/.ralph/runs/run-20260309-043236-60668-iter-3.log
Run summary: /Users/lars/Dev/smith.terminal-support/.ralph/runs/run-20260309-043236-60668-iter-3.md
- Guardrails reviewed: yes
- No-commit run: false
- Commit: 8d75f5f feat(api): execute commands in loop container
- Post-commit status: `.ralph/runs/run-20260309-043236-60668-iter-3.log` modified after commit hooks
- Verification:
  - Command: make build -> PASS
  - Command: go test ./cmd/smith-api/... -> PASS
  - Command: go test ./internal/source/... -> PASS
  - Command: go test ./... -> PASS
  - Command: npm run test:frontend -> PASS
  - Command: make test-matrix -> PASS
- Files changed:
  - cmd/smith-api/main.go
  - cmd/smith-api/main_test.go
  - go.mod
  - go.sum
  - .ralph/activity.log
  - .ralph/progress.md
- What was implemented
  - Implemented Kubernetes pod exec integration in `smith-api` using `k8s.io/client-go/tools/remotecommand` and shell wrapping with `/bin/sh -lc`.
  - Updated `POST /v1/loops/{id}/control/command` to execute against the actor's attached runtime target, capture stdout/stderr, parse exit status, and return `delivered=true` with result metadata.
  - Added journal streaming for command lifecycle (`started`, output lines, `completed`) so outputs like `echo hello` are persisted in loop journal entries.
  - Added command max-size validation (`terminalCommandMaxSize`) returning HTTP 400 plus `terminal-command-rejected` audit entries tagged with `result=rejected`.
  - Added tests for successful command execution and output journaling, attach-required 409 behavior with zero exec calls, and oversize-command rejection with rejected audit metadata.
- **Learnings for future iterations:**
  - Patterns discovered
  - Runtime-aware terminal execution can reuse attach session metadata (namespace/pod/container) to avoid re-resolving pod targets for every command.
  - Gotchas encountered
  - Importing `remotecommand` required additional transitive go.sum entries (`gorilla/websocket`, `spdystream`, `go-flowrate`) before tests would run.
  - Useful context
  - Pre-commit hooks run `go test ./cmd/...` and may continue appending to iteration run logs after commits; check and commit log tails explicitly.
---
## [2026-03-09 05:29:53 EDT] - US-004: Add interactive command controls to pod detail UI
Thread: 
Run: 20260309-043236-60668 (iteration 4)
Run log: /Users/lars/Dev/smith.terminal-support/.ralph/runs/run-20260309-043236-60668-iter-4.log
Run summary: /Users/lars/Dev/smith.terminal-support/.ralph/runs/run-20260309-043236-60668-iter-4.md
- Guardrails reviewed: yes
- No-commit run: false
- Commit: a2db662 feat(console): add pod detail terminal controls
- Post-commit status: clean
- Verification:
  - Command: make build -> PASS
  - Command: go test ./cmd/smith-api/... -> PASS
  - Command: go test ./internal/source/... -> PASS
  - Command: go test ./... -> PASS
  - Command: npm run test:frontend -> PASS
  - Command: make test-matrix -> PASS
  - Command: dev-browser scripted pod-view verification -> PASS
- Files changed:
  - console/index.html
  - test/playwright/console.spec.js
  - .agents/tasks/prd-console-terminal-attach.json
  - .ralph/activity.log
  - .ralph/errors.log
  - .ralph/runs/run-20260309-043236-60668-iter-3.log
  - .ralph/runs/run-20260309-043236-60668-iter-3.md
  - .ralph/runs/run-20260309-043236-60668-iter-4.log
  - .ralph/.tmp/prompt-20260309-043236-60668-4.md
  - .ralph/.tmp/story-20260309-043236-60668-4.json
  - .ralph/.tmp/story-20260309-043236-60668-4.md
  - .ralph/progress.md
- What was implemented
  - Added explicit pod detail terminal controls (attach, detach, command input, run button), runtime target summary, and terminal status indicator.
  - Implemented pod-view UI state machine (`idle`, `attaching`, `attached`, `executing`, `detaching`, `error`) with control locking and status messaging.
  - Wired runtime resolution calls to `/v1/loops/{id}/runtime` and surfaced non-attachable reason text for inactive loops.
  - Added command execution behavior for Enter and Run button; controls lock while executing; command input clears only after successful execution.
  - Added Playwright coverage for success and failure control paths, including disabled controls and runtime reason on non-active loops.
  - Completed required browser verification via `dev-browser` with screenshot: `/Users/lars/.codex/skills/dev-browser/tmp/us004-pod-view-controls.png`.
- **Learnings for future iterations:**
  - Patterns discovered
  - Pod-view terminal UX can stay deterministic by deriving enable/disable rules from one centralized sync function.
  - Gotchas encountered
  - Mocked journal events must use strictly increasing sequence numbers or the UI will drop entries as out-of-order.
  - Useful context
  - The frontend Playwright suite already uses an extensible API mock helper, so adding control endpoint behaviors there keeps scenario tests stable.
---
## [2026-03-09 05:43:52 EDT] - US-005: Harden security and operational limits for web terminal control
Thread: codex_43878
Run: 20260309-043236-60668 (iteration 5)
Run log: /Users/lars/Dev/smith.terminal-support/.ralph/runs/run-20260309-043236-60668-iter-5.log
Run summary: /Users/lars/Dev/smith.terminal-support/.ralph/runs/run-20260309-043236-60668-iter-5.md
- Guardrails reviewed: yes
- No-commit run: false
- Commit: e348d88 security(api): harden terminal control limits
- Post-commit status: `.ralph/runs/run-20260309-043236-60668-iter-5.log` modified by post-commit hooks
- Verification:
  - Command: make build -> PASS
  - Command: go test ./cmd/smith-api/... -> PASS
  - Command: go test ./internal/source/... -> PASS
  - Command: go test ./... -> PASS
  - Command: npm run test:frontend -> PASS
  - Command: make test-matrix -> PASS
- Files changed:
  - cmd/smith-api/main.go
  - cmd/smith-api/main_test.go
  - .agents/tasks/prd-console-terminal-attach.json
  - .ralph/activity.log
  - .ralph/progress.md
  - .ralph/errors.log
  - .ralph/runs/run-20260309-043236-60668-iter-4.log
  - .ralph/runs/run-20260309-043236-60668-iter-4.md
  - .ralph/runs/run-20260309-043236-60668-iter-5.log
  - .ralph/.tmp/prompt-20260309-043236-60668-5.md
  - .ralph/.tmp/story-20260309-043236-60668-5.json
  - .ralph/.tmp/story-20260309-043236-60668-5.md
- What was implemented
  - Enforced early auth rejection in attach/command/detach handlers with explicit API error codes and rejected audit records before runtime/state resolution continues.
  - Added per-session command rate limiting (`5` commands per `10s` window) with HTTP 429 throttling, `Retry-After` header, and throttle metadata in terminal command audit records.
  - Standardized terminal control audit metadata to include `request_status=accepted|rejected` and `rejection_reason`/`error_code` for blocked paths.
  - Kept max command length enforcement and upgraded oversized-command responses to include explicit API code metadata.
  - Added smith-api tests for unauthorized attach/detach/command behavior, no-runtime/no-exec guard behavior on unauthorized requests, accepted/rejected audit tagging, explicit error codes, and per-session throttling behavior.
- **Learnings for future iterations:**
  - Patterns discovered
  - Centralizing accepted/rejected metadata helpers keeps audit semantics consistent across attach/detach/command handlers.
  - Gotchas encountered
  - Rejected-path audit writes can trigger nil-store panics in focused handler tests unless `appendAudit`/`appendJournal` safely no-op when store backing is absent.
  - Useful context
  - Pre-commit hooks run `go test ./cmd/...` and can append to iteration run logs after commit, requiring a follow-up commit to restore clean status.
---
## [2026-03-09 05:57:23 EDT] - US-006: Verification coverage and operator documentation
Thread: codex_60138
Run: 20260309-043236-60668 (iteration 6)
Run log: /Users/lars/Dev/smith.terminal-support/.ralph/runs/run-20260309-043236-60668-iter-6.log
Run summary: /Users/lars/Dev/smith.terminal-support/.ralph/runs/run-20260309-043236-60668-iter-6.md
- Guardrails reviewed: yes
- No-commit run: false
- Commit: 3adf22a test(terminal-control): expand verification coverage
- Post-commit status: `.ralph/runs/run-20260309-043236-60668-iter-6.log`
- Verification:
  - Command: make build -> PASS
  - Command: go test ./cmd/smith-api/... -> PASS
  - Command: go test ./internal/source/... -> PASS
  - Command: go test ./... -> PASS
  - Command: npm run test:frontend -> PASS
  - Command: make test-matrix -> PASS
- Files changed:
  - README.md
  - cmd/smith-api/main_test.go
  - cmd/smith-api/pod_exec_runner_test.go
  - docs/loop-ingress-and-cli.md
  - test/playwright/console.spec.js
- What was implemented
  - Expanded API verification coverage for runtime resolution fallback behavior, attach/detach lifecycle rejection handling, command payload validation, and command execution result handling for non-zero exits and execution transport errors.
  - Added simulated Kubernetes exec-flow tests for `kubePodExecRunner.Execute` covering success, non-zero exit status mapping, and stream failures with output capture.
  - Updated Playwright pod detail terminal coverage to explicitly run `echo ok` and verify terminal output visibility before detach.
  - Updated operator-facing documentation with terminal API contracts, auth/RBAC permissions, and troubleshooting for missing/not-running pods and not-attached command failures.
- **Learnings for future iterations:**
  - Patterns discovered
    - `kubePodExecRunner` can be tested without a live cluster by injecting a fake REST client and remote executor.
  - Gotchas encountered
    - Long-running hooks and run logging can mutate `.ralph/runs/*` after a commit; always re-check status and finalize with a cleanup commit.
  - Useful context
    - Terminal command API returns structured error codes for validation/auth/rate-limit failures but attach runtime conflicts currently return text error messages.
---
## [2026-03-14 01:03:56 EDT] - US-001: Define canonical PRD validation and diagnostic contracts
Thread: ses_b56b86
Run: 20260314-005446-82373 (iteration 1)
Run log: /Users/lars/Dev/smith-prd-validation/.ralph/runs/run-20260314-005446-82373-iter-1.log
Run summary: /Users/lars/Dev/smith-prd-validation/.ralph/runs/run-20260314-005446-82373-iter-1.md
- Guardrails reviewed: yes
- No-commit run: false
- Commit: 3d0c98f feat(validation): add canonical PRD diagnostics
- Post-commit status: `clean`
- Verification:
  - Command: make build -> PASS
  - Command: go test ./internal/source/model/... -> PASS
  - Command: go test ./... -> PASS
  - Command: ./scripts/validate-acceptance.sh -> FAIL
  - Command: make ci-local-act -> FAIL
- Files changed:
  - .agents/tasks/prd-prd-generation-validation.json
  - .ralph/.tmp/prompt-20260314-005446-82373-1.md
  - .ralph/.tmp/story-20260314-005446-82373-1.json
  - .ralph/.tmp/story-20260314-005446-82373-1.md
  - .ralph/activity.log
  - .ralph/progress.md
  - .ralph/runs/run-20260314-005446-82373-iter-1.log
  - internal/source/model/prd.go
  - internal/source/model/prd_test.go
- What was implemented
  - Added shared PRD validation report and diagnostic types with stable codes, JSON-style paths, optional story references, readiness, and suggested fixes.
  - Expanded PRD validation to enforce canonical top-level fields, sequential `US-###` story IDs, duplicate detection, dependency reference checks, quality gate presence, and canonical story statuses.
  - Added `ValidatePRDJSON` so malformed JSON returns a machine-readable blocking diagnostic instead of only a raw parse error.
  - Added unit coverage for valid PRDs, malformed JSON, duplicate IDs, unknown dependencies, missing project and quality gates, and invalid statuses.
- **Learnings for future iterations:**
  - Patterns discovered
  - `internal/source/model` is the right shared package for PRD contracts because ingress and future CLI/API flows can consume it without pulling in higher-level workflow code.
  - Gotchas encountered
  - Repo-level verification currently includes unrelated baseline failures in `./scripts/validate-acceptance.sh`, and `act` can fail or mutate run logs during hooks, so `git status --porcelain` must be checked again after each commit.
  - Useful context
  - Existing canonical PRD task files in this repo currently use `open`, `in_progress`, and `done` statuses; the validator now treats that set as canonical.
---
