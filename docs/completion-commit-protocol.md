# Completion Commit Protocol (Saga)

## Goal

Prevent split-brain terminal outcomes between:
- Git/code state (commit pushed)
- etcd loop state (`synced` transition)

## Protocol Phases

1. `prepared`: completion flow starts and is persisted.
2. `code_committed`: code push succeeded and commit SHA is persisted.
3. `state_committed`: etcd state transition to `synced` succeeded.
4. `compensation_needed`: code commit succeeded but state transition failed.
5. `compensated`: revert/compensation succeeded and loop returned to `unresolved`.

## Execution Rules

- On commit/push failure:
  - loop remains non-terminal (`unresolved`)
  - outcome is retryable
- On state finalize failure after code commit:
  - protocol records `compensation_needed`
  - attempts Git revert compensation
  - if compensation succeeds: state set to `unresolved`, retryable
  - if compensation fails: outcome marked `compensation_required` and escalated

## Crash Safety

- Because every phase is persisted, restart/reconciler logic can continue from the last known phase.
- No terminal `synced` state is written unless both code and state commits succeed.
- If code commit exists without state commit, the protocol explicitly records non-terminal compensation state instead of silently terminating.
