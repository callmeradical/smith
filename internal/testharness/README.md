# Internal Test Harness

Reusable Go test helpers for Smith integration and acceptance suites.

## Packages

- `internal/testharness/fixture`
  - `NewLoopFixtureFromSeed(prefix, seed)`:
    deterministic loop/correlation IDs for repeatable test scenarios.
- `internal/testharness/runtime`
  - `Enabled(envVar)`: boolean gate for env-driven suites.
  - `RequireEnabled(t, envVar)`: skips test unless env var is `true`.
  - `ContextWithTimeout(t, d)`: test-scoped timeout context with cleanup.
- `internal/testharness/assertions`
  - `RequireLoopState(t, got, want)`: loop state assertion.
  - `RequireNonEmpty(t, field, value)`: non-empty string assertion.
  - `RequireJournalMessage(t, entries, contains)`: journal substring assertion.

## Current Usage

- Integration:
  - `internal/source/integration/watch_reconcile_vcluster_test.go`
- Acceptance:
  - `test/acceptance/harness_smoke_test.go`

