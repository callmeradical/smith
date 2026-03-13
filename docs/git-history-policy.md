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

## Operator Configuration (Feature-Gated)

- Default behavior remains:
  - `branch_cleanup = on_merge`
  - `conflict_policy = manual_review`
  - `delete_branch_on_merge = true`
- Override support is behind `SMITH_GIT_POLICY_CONFIG_ENABLED=true` on `smith-core`.
- Supported overrides:
  - `SMITH_GIT_POLICY_BRANCH_CLEANUP`: `on_merge` or `never`
  - `SMITH_GIT_POLICY_CONFLICT_POLICY`: `manual_review` or `fail_fast`
  - `SMITH_GIT_POLICY_DELETE_BRANCH_ON_MERGE`: `true` or `false`
- Validation rejects incompatible combinations (for example `branch_cleanup=on_merge` with `delete_branch_on_merge=false`).
