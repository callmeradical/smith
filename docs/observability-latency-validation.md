# Observability Latency Validation

This document defines and validates the end-to-end latency path for NFR-004:

`journal write in etcd` -> `smith-api stream fanout` -> `console render`.

## Path Definition

1. A loop event is persisted with `JournalEntry.timestamp` at write time.
2. `GET /v1/loops/{id}/journal/stream` emits journal entries over SSE.
3. Console receives `entry` events and appends lines to the terminal panel.
4. Console computes per-entry latency as:
   - `Date.now() - entry.timestamp`
5. Console displays rolling `p95` in the journal header.

## Implementation Notes

- `smith-api` stream now uses etcd watch fanout (`clientv3.Watch`) for journal prefixes.
- Initial catch-up entries are sent from a consistent snapshot revision, then watch starts from `revision+1`.
- Keepalive comments are emitted every 15s for long-lived stream health.
- Legacy 1s polling was removed from stream transport.

## Benchmark Harness

Run local benchmark script:

```bash
make test-observability-latency
```

Or directly:

```bash
SMITH_API_BASE_URL=http://127.0.0.1:8080 \
SMITH_LAT_LOOP_ID=latency-loop \
SMITH_LAT_SAMPLES=40 \
./scripts/integration/measure-observability-latency.sh
```

The script:

- toggles manual overrides (`unresolved` <-> `overwriting`) to generate journal events,
- consumes SSE stream entries for the loop,
- computes p95/p99 from observed `now - entry.timestamp` latency,
- reports pass/warn against the `<100ms p95` target.

## Gap Handling

If measured `p95 >= 100ms`, treat as an SLO miss and apply this remediation order:

1. Verify stream transport is in watch mode (not polling fallback).
2. Check browser and API host clock skew and local CPU saturation.
3. Reduce console refresh contention (`auto-refresh`) during latency tests.
4. Scale API/etcd resources and retest in staging under expected concurrency.
