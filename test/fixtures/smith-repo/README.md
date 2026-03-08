# Smith Test Repository Fixture

Deterministic fixture repository for loop execution and completion verification tests.

## Scenarios

- `scenario/single-loop-success`: simple change expected to complete cleanly.
- `scenario/concurrent-safe-a`: branch A for concurrent loop safety validation.
- `scenario/concurrent-safe-b`: branch B for concurrent loop safety validation.
- `scenario/merge-conflict`: deterministic merge-conflict branch for failure/reconciliation tests.

Expected outcomes are declared in `spec/expected-outcomes.json`.
The verification harness (`scripts/test/verify-completion.sh`) enforces commit subject prefixes and expected file presence per scenario.

## Usage

Provision non-interactively:

```bash
./test/fixtures/smith-repo/scripts/provision.sh /tmp/smith-test-repo
```

Reset to baseline:

```bash
./test/fixtures/smith-repo/scripts/reset.sh /tmp/smith-test-repo
```
