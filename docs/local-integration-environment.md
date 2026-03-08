# Local Integration Environment (k3d + vCluster + etcd)

This environment provides a reproducible local target for Smith integration/e2e tests.

## Make-First Workflow (Standard)

Use `make` as the primary local workflow entrypoint:

```bash
make help
```

Target matrix:

| Target | Contract | Prerequisites |
| --- | --- | --- |
| `make doctor` | Fails fast when required local tools are missing. | `go`, `kubectl`, `helm`, `docker`, `k3d`, `vcluster` in `PATH` |
| `make bootstrap` | Installs missing `k3d`/`vcluster` via script helpers. | `brew` or `curl` available |
| `make cluster-up` (`make cluster`) | Provisions local `k3d + vcluster + etcd`. | Doctor checks pass |
| `make deploy` | Installs/upgrades Helm release into namespace (`SMITH_NAMESPACE`, `SMITH_RELEASE`, `SMITH_VALUES`). | Reachable Kubernetes cluster + Helm |
| `make test` (`make test-matrix`) | Runs local non-cluster matrix (fixtures, verification, e2e scripts). | Go toolchain + local repo dependencies |
| `make test-integration` | Runs vCluster-backed integration workflow. | `cluster-up` completed |
| `make teardown` | Removes Helm release and tears down local cluster stack. | None; best-effort cleanup |

Default configurable vars:

- `SMITH_NAMESPACE` (default `smith-system`)
- `SMITH_RELEASE` (default `smith`)
- `SMITH_VALUES` (default `helm/smith/values/local.yaml`)
- `SMITH_TEST_ARTIFACTS_DIR` (default `/tmp/smith-test-artifacts`)
- `SMITH_FIXTURE_DIR` (default `/tmp/smith-test-repo`)

## Prerequisites

Required CLI tools:

- `kubectl`
- `helm`
- `k3d`
- `vcluster`

Install missing tools:

```bash
./scripts/integration/prereqs.sh
```

If `env-up` fails with disk-pressure taints, clean local Docker storage and retry:

```bash
docker system prune -af
```

## Bring Up Environment

```bash
./scripts/integration/env-up.sh
```

Creates:

- k3d host cluster: `smith-int`
- vCluster: `smith-vc` in namespace `smith-vcluster`
- etcd (bitnami chart) in namespace `smith-system`

By default, `env-up` connects to the vCluster context before installing etcd so integration tests run against vCluster APIs.

Default etcd endpoint in-cluster:

`http://smith-etcd.smith-system.svc.cluster.local:2379`

## Tear Down Environment

```bash
./scripts/integration/env-down.sh
```

## Deterministic Test Runs

Use the test matrix harness:

```bash
./scripts/integration/run-tests.sh
```

For non-cluster local validation (default path):

```bash
./scripts/test/run-matrix.sh
```

Run the vCluster watch/reconcile integration target directly:

```bash
./scripts/integration/test-watch-reconcile.sh
```

## Configuration Overrides

Scripts accept environment variable overrides:

- `SMITH_K3D_CLUSTER_NAME`
- `SMITH_K3D_SERVERS`
- `SMITH_K3D_AGENTS`
- `SMITH_K3D_PORT_HTTP`
- `SMITH_K3D_PORT_HTTPS`
- `SMITH_VCLUSTER_NAME`
- `SMITH_VCLUSTER_NAMESPACE`
- `SMITH_ETCD_NAMESPACE`
- `SMITH_ETCD_RELEASE_NAME`
- `SMITH_ETCD_STORAGE_CLASS`
- `SMITH_ETCD_VERSION` (optional chart pin; defaults to latest from repo index)
- `SMITH_ETCD_PERSISTENCE_ENABLED` (default `false` for ephemeral environments)
- `SMITH_ETCD_WAIT_TIMEOUT` (default `8m`)
- `SMITH_ETCD_MODE` (`simple` default, `helm` optional)
- `SMITH_ETCD_IMAGE` (used when `SMITH_ETCD_MODE=simple`)
- `SMITH_VCLUSTER_KUBECONFIG` (path for kubeconfig emitted by `vcluster connect --print`)
