# Pre-Release System Gate

Smith promotion is blocked unless both system-gate profiles pass:

- `vcluster` profile: full k3d + vCluster + etcd system run.
- `parity` profile: non-vCluster parity run.
- `parity-direct-k3s` profile (workflow job): k3d direct namespace deploy with `SMITH_USE_VCLUSTER=false`.

Workflow: `.github/workflows/pre-release-system-gate.yml`

## Gate Command

```bash
./scripts/release/pre-release-system-gate.sh <vcluster|parity>
```

## What Gets Validated

For both profiles, the gate requires:

- matrix/e2e suite pass (`scripts/test/run-matrix.sh` via profile wrapper);
- recovery path pass (`scripts/test/failure-injection.sh`);
- override path pass (`go test ./cmd/smithctl -run TestLoopCancelBatchPostsOverride -count=1`).

For `vcluster`, the gate also requires watch/reconcile integration against live vCluster APIs:

- `scripts/integration/test-watch-reconcile.sh`

For `parity-direct-k3s`, the gate runs the parity profile with `SMITH_ENABLE_CLUSTER_TESTS=true`
after provisioning k3d + etcd without vCluster to expose non-vCluster behavior differences.

## Artifact Output

Gate artifacts are written under `SMITH_TEST_ARTIFACTS_DIR` and include:

- `summary.txt`
- `recovery-override-validation.txt`
- matrix and fixture evidence files emitted by `run-matrix.sh`

Workflow artifacts are uploaded separately per job:

- `smith-pre-release-vcluster-artifacts`
- `smith-pre-release-parity-artifacts`
- `smith-pre-release-parity-direct-k3s-artifacts`

This separation provides clear diff signals when failures are profile-specific.

## Failure Semantics

Any failed command exits non-zero and fails the gate job. In GitHub Actions, this blocks pre-release promotion until fixed or explicitly re-run after remediation.
