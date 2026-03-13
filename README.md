# Smith: A Distributed Runtime for Autonomous Software Development

**Turning product requirements into running software through autonomous development loops.**

Smith is a distributed runtime designed to execute autonomous development loops across a cluster of machines. By combining Product Requirement Documents (PRDs), GitHub Issues, and automated validation workflows, Smith enables software systems to continuously build themselves.

Instead of assigning tasks manually to engineers, Smith distributes autonomous "Ralph loops" across infrastructure. Each loop reads project requirements, evaluates the current repository state, implements tasks, validates changes, and contributes improvements back to the codebase.

The result is a new model of software development where repositories become active environments capable of continuously progressing toward their stated goals.

## The Problem

Modern software development is constrained by manual coordination. Even with modern tooling, teams still rely on humans to interpret PRDs, translate requirements into issues, implement code, validate results, and track progress. While automation has improved build pipelines and deployment, the development process itself remains largely manual.

At the same time, AI coding systems have emerged that can generate code, but they typically operate in isolated sessions without long-term memory or structured execution models. This creates a gap between AI capability and production software workflows.

## The Smith Approach

Smith bridges this gap by treating software development as a **continuous autonomous process**. Instead of orchestrating discrete workflows, Smith distributes persistent development loops across infrastructure. Each loop performs the following cycle:

1.  **Read** the project PRD
2.  **Evaluate** GitHub issues and repository state
3.  **Identify** the next unfinished task
4.  **Implement** the change
5.  **Run validation** (tests, builds, linting)
6.  **Commit** improvements
7.  **Repeat** until all requirements are satisfied

These loops run continuously across a cluster—typically Kubernetes—allowing development activity to scale horizontally.

## Key Concepts

### Ralph Loops

At the heart of Smith is the **Ralph Loop**, a structured feedback loop designed for reliable autonomous development. A Ralph loop externalizes state into the repository itself. Instead of relying on fragile model memory, it stores progress in Git commits, PRD documents, task tracking files, and automated tests.

Each iteration starts with fresh model context but reconstructs state from the repository, enabling long-running autonomous progress over thousands of iterations.

### Choreography, Not Orchestration

Traditional workflow engines rely on centralized orchestration. Smith takes a different approach: **Choreography**.

There is no central controller dictating every step. Instead:
- **PRDs** define goals.
- **GitHub issues** define tasks.
- **Tests** define correctness.
- **Agents** react to repository state.

The repository becomes the shared coordination layer where autonomous loops cooperate. This allows Smith to support large numbers of development loops working simultaneously across multiple repositories.

## Purpose

Smith coordinates these autonomous execution loops as a state machine stored in etcd. It is designed to:

- accept operator ingress requests (direct, GitHub issues, PRD tasks),
- convert each request into a deterministic loop lifecycle (`unresolved -> running -> synced|flatline|cancelled`),
- enforce safe concurrency with per-loop locks and revision-checked state transitions,
- run loop workers as Kubernetes Jobs and preserve execution evidence (journal, handoff, override, audit).

In this repository, the focus is the MVP control plane, deployment assets, and verification/test harnesses.

## Philosophy

Smith intentionally does not personify agents.

- **Homogeneous Workers:** Agents are modeled as homogeneous and omnicapable execution units, not distinct personalities. Anthropomorphizing agents is treated as an implementation constraint that reduces operational flexibility and performance.
- **Uniform Replication:** The target model is many equivalent workers, same contract, same capabilities, horizontally scalable.
- **Role-Neutral Primitives:** The system design favors state, locks, jobs, and handoffs over persona-specific behavior.
- **Distributed Substrate:** Smith moves beyond a single-machine file-system model by using etcd + Kubernetes as the control substrate, so execution can scale across distributed compute while preserving deterministic state and traceability.

The platform direction is informed by [Ralph](https://github.com/snarktank/ralph), [marcus/sidecar](https://github.com/marcus/sidecar), [marcus/td](https://github.com/marcus/td), and related projects, but is engineered to scale beyond a single developer machine.

## Architecture Summary

Smith is split into control-plane and data-plane components.

### Control Plane

- `smith-api` (`cmd/smith-api`): HTTP API for loop create/list/get, GitHub + PRD ingress, operator override actions, provider auth lifecycle, and cost reporting.
- `smith-core` (`cmd/smith-core`): watches unresolved loop state in etcd, acquires per-loop locks, transitions loop state, and schedules replica Jobs in Kubernetes.
- `smithctl` (`cmd/smithctl`): kubectl-style operator CLI for `loop` and `prd` resources with context/config support and scriptable JSON output.
- `smith` (`cmd/smith`): PRD launcher CLI (`smith --prd`) for interactive PRD generation before build loops.
- `smith-console` (`console/` + Helm deployment): operator UI/runtime assets.
- etcd: authoritative source of truth for anomalies, loop lifecycle state, locks, journal events, handoffs, overrides, and audit records.

### Data Plane

- `smith-replica` (`cmd/smith-replica`): Kubernetes Job worker that executes loop work, appends journal entries, writes handoff output, and finalizes loop state.

### Deployment and Ops Assets

- Helm chart: `helm/smith`
- Dockerfiles: `docker/`
- Core implementation: `internal/source/`
- Supporting docs: `docs/`
- Make-first local workflow: `make help` (doctor/bootstrap/cluster/deploy/test/teardown)

## Key API Endpoints

Implemented today:
- `POST /v1/loops` single/batch direct loop creation.
- `POST /v1/loops` supports environment profiles (`preset`, `mise`, `container_image`, `dockerfile`) with server-side validation/defaulting.
- `POST /v1/ingress/github/issues` ingest one or more GitHub issues into loop specs.
- `POST /v1/ingress/prd` ingest markdown/json PRD inputs into loop specs.
- `GET /v1/loops/{id}` and `GET /v1/loops/{id}/journal` for state and traceability.
- `GET /v1/loops/{id}/runtime` to resolve namespace/pod/container attachability for console terminal control.
- `POST /v1/loops/{id}/control/attach`, `/command`, and `/detach` for authenticated operator interactive terminal control.
- `POST /v1/control/override` for operator state overrides with reason/audit trail.
- `POST /v1/auth/codex/connect/start|complete`, `GET /v1/auth/codex/status`, and `POST /v1/auth/codex/disconnect` for provider auth lifecycle.
- `GET /v1/reporting/cost?loop_id={id}` for loop token/cost aggregation from journal metadata.

Aspirational (planned, not implemented yet):
- `GET /v1/loops/{id}/handoffs`, `GET /v1/loops/{id}/overrides`, and `GET /v1/loops/{id}/trace` for end-to-end execution evidence.
- `GET /v1/audit?loop_id={id}` for immutable operator/auth action audit records.

Terminal control API contracts, required auth/RBAC permissions, and troubleshooting are documented in:
- [`docs/loop-ingress-and-cli.md`](docs/loop-ingress-and-cli.md)

## Local Git Hooks

Install repo-managed hooks:

```bash
make hooks-install
```

Hook behavior:
- `pre-commit`: quick checks (`go test ./cmd/...`)
- `pre-push`: full gate (`make build` + `make test`)

Temporarily bypass hooks if needed:

```bash
SKIP_GIT_HOOKS=1 git commit -m "..."
SKIP_GIT_HOOKS=1 git push
```

## Frontend Playwright Tests

Install frontend test dependencies:

```bash
npm install
```

Run Playwright tests for the console UI:

```bash
npm run test:frontend
# or
make test-frontend
```

Artifacts are written under `output/playwright/` (HTML report + failure artifacts).

Run tests against a deployed, port-forwarded console UI:

```bash
kubectl -n smith-system port-forward svc/smith-smith-console 3000:3000
npm run test:frontend:live
```

## Technology Stack and Thanks

See the dedicated documentation page for:
- the current technology stack (Kubernetes, Helm, vCluster, etcd, Go, Docker, and related tooling),
- acknowledgments and inspiration credits, including [marcus/td](https://github.com/marcus/td), [marcus/sidecar](https://github.com/marcus/sidecar), and [Ralph](https://github.com/snarktank/ralph).

Reference: [docs/technology-stack-and-thanks.md](docs/technology-stack-and-thanks.md)

## PRD-First Loop Workflow

Generate a PRD JSON (interactive agent session):

```bash
smith --prd "Build issue-driven loop execution with terminal attach support" --out .agents/tasks/prd.json
```

If a PRD already exists at `.agents/tasks/prd.json`, replica issue/prompt workflows skip PRD generation and move straight to iterative build.
