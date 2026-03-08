# Local Development and Deployment Workflow (make-first)

## Goal

Provide a consistent, low-friction local workflow for Smith using `make` as the primary command surface.

## Principles

- One-command entrypoints for common developer tasks.
- Deterministic local environment setup.
- Idempotent deploy/teardown commands.
- Clear separation between local dev and CI/release paths.

## Proposed Make Targets

### Environment and Tooling
- `make doctor`
  - Validate required tools (`go`, `node`, `kubectl`, `helm`, `k3d`, `vcluster`, `docker`, `td`, `gh`).
- `make bootstrap`
  - Install/prepare local dependencies and generate local config defaults.

### Local Cluster Lifecycle
- `make cluster-up`
  - Start `k3d` host cluster and provision `vcluster` namespace/profile.
- `make cluster-down`
  - Tear down local cluster resources cleanly.
- `make cluster-reset`
  - Full local reset (destroy + recreate cluster and key state).

### Build and Deploy
- `make build`
  - Build local binaries/images required for local deployment.
- `make deploy-local`
  - Deploy Smith locally via Helm with local values profile.
- `make undeploy-local`
  - Remove local deployment from cluster.

### Dev Runtime and Validation
- `make logs`
  - Tail key service logs for local troubleshooting.
- `make test`
  - Run default local test suite.
- `make test-integration`
  - Run integration suite against local k3d/vcluster env.
- `make test-e2e`
  - Run selected e2e loop scenarios locally.

### Quality and Hygiene
- `make lint`
  - Run lint/format checks.
- `make ci-local`
  - Run local approximation of CI pipeline.

## Local Deployment Profile

- Helm values profile for local run (e.g., `values.local.yaml`) should:
  - reduce resource requests
  - enable local image refs and pull policy
  - configure local endpoints/secrets strategy for development

## Non-Goals

- Replacing CI/release workflows with local-only scripts.
- Supporting all cloud-specific production paths from local make targets.

