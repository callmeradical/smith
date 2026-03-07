# Smith Loop Ingress and Control Surface

## Goal

Define how loops are created and controlled for single and multi-loop execution.

## Ingress Modes

### 1. GitHub Issue Ingress
- Operators can select one or more GitHub issues as loop sources.
- Smith maps each issue into an anomaly payload with source metadata.
- Batch submission supports running multiple loops in parallel.

### 2. PRD Ingress
- Operators can submit PRD documents (markdown/json) to create one or many loops.
- PRD parser extracts tasks/scopes and emits loop specs.
- Generated anomalies retain traceability to source PRD and section IDs.

### 3. Direct Interactive Ingress
- Operator starts a loop and attaches an interactive terminal session.
- Session supports live command/control, journal view, and manual interventions.
- Session events are fully journaled.

## Control Plane API (Proposed)

- `POST /v1/loops` create a single loop
- `POST /v1/loops/batch` create multiple loops atomically by request
- `GET /v1/loops/{id}` loop status/details
- `GET /v1/loops/{id}/journal/stream` live journal stream
- `POST /v1/loops/{id}/control/attach` attach interactive terminal session
- `POST /v1/ingress/github/issues` ingest one or more GitHub issues
- `POST /v1/ingress/prd` ingest PRD and emit loop specs

## smithctl (kubectl-style UX)

CLI should be resource-oriented and scriptable.

Example command surface:
- `smith loop create -f loop.yaml`
- `smith loop create --from-github 123`
- `smith loop create --from-prd docs/prd1.md`
- `smith loop create --batch issues.yaml`
- `smith loop get <id>`
- `smith loop logs <id> --follow`
- `smith loop attach <id>`
- `smith loop cancel <id>`
- `smith prd create <name> --template <tpl>`
- `smith prd submit <file>`

## MVP Decisions

- MVP supports all three ingress modes (GitHub issues, PRDs, direct interactive).
- `smithctl` is the primary operator path for automation and terminal workflows.
- Operator Console remains the visual control/monitoring layer.

## Security and Audit

- All ingress actions require authenticated identity and are RBAC-checked.
- Every loop creation/update/control action is journaled with actor + source metadata.
- Interactive terminal sessions record attach/detach/control events.

## Non-Goals (MVP)

- Free-form plugin ingress from arbitrary third-party trackers.
- Full bidirectional sync engines for every external system.


## Implemented API Surface (Current)

### Single Loop Create

`POST /v1/loops`

Payload (single):

```json
{
  "idempotency_key": "issue-123",
  "title": "Fix flaky lock renewal",
  "description": "Investigate and patch lock renewal race",
  "source_type": "github_issue",
  "source_ref": "org/repo#123",
  "provider_id": "codex",
  "model": "gpt-5-codex",
  "metadata": {
    "priority": "p0"
  }
}
```

### Batch Loop Create

`POST /v1/loops`

Payload (batch):

```json
{
  "loops": [
    {
      "idempotency_key": "issue-124",
      "title": "Task A",
      "description": "...",
      "source_type": "github_issue",
      "source_ref": "org/repo#124"
    },
    {
      "idempotency_key": "issue-125",
      "title": "Task B",
      "description": "...",
      "source_type": "github_issue",
      "source_ref": "org/repo#125"
    }
  ]
}
```

### Idempotency and Response Semantics

- `idempotency_key` is persisted in anomaly metadata.
- Loop ID is derived deterministically from idempotency key/source when `loop_id` is not supplied.
- Repeated submissions return existing loop with `created=false`.
- Batch responses return per-item status in `results[]`.
