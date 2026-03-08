# Replica Loop Git History Policy

## Branch Model

- Per-loop, per-attempt working branches:
  - `smith/loop/{loop_id}/a{attempt}`
- Branch names are sanitized from loop IDs and deterministic.
- Branches are deleted after successful merge (`delete branch on merge = true`).

## Checkpoint Strategy

- Checkpoint commits are allowed during active execution for crash recovery.
- Checkpoint commit format:
  - `chore(loop-checkpoint): <summary> [<correlation_id>]`
- Checkpoint commits are not preserved in final history when squash/rewrite is enabled.

## Final Commit Shape

- Final commit follows conventional format:
  - `feat(loop): <summary>`
- Required trailers:
  - `Loop-ID: <loop_id>`
  - `Correlation-ID: <correlation_id>`
- This preserves traceability without exposing noisy incremental checkpoints.

## Merge Method

- Default merge method: `squash`.
- Allowed methods: `squash`, `rebase`, `merge`.
- MVP default keeps one clean final commit per successful loop attempt.

## Failure and Cleanup

- If loop fails before finalization, checkpoints may remain on the attempt branch for forensic replay.
- Completion saga is responsible for pushing final commit and synchronizing etcd state.
- Reconciliation and branch cleanup policy should remove stale attempt branches once state reaches terminal outcomes.
