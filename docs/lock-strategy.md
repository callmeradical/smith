# Single-Writer Lease Lock Strategy

## Objective

Guarantee at most one active mutating Replica/Core holder per loop ID.

## Lock Semantics

- Lock key: `/smith/v1/locks/{loop_id}`
- Ownership: `holder` field identifies active writer.
- Freshness: lock is valid while `heartbeat_at + lease_timeout > now`.
- Concurrency guard: acquire/renew/release operations use compare-and-swap revision checks.

## Acquisition Rules

- No existing lock: acquire succeeds.
- Existing lock owned by same holder: treated as renewal.
- Existing lock owned by another holder and not expired: acquire rejected (`ErrLockHeld`).
- Existing lock expired: new holder may steal lock via CAS update.

## Renewal and Release

- Only current holder can renew lock heartbeat.
- Only current holder can release lock.
- CAS mismatch on update/delete signals lock loss (`ErrLockLost`) and requires caller to stop mutating work.

## Split-Brain Prevention

- Every mutating step must verify lease ownership before acting.
- Failed CAS on lock write/read-modify-write is treated as ownership loss.
- Terminal transitions should be blocked when ownership is lost.
