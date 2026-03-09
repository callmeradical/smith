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
