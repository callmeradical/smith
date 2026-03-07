# Image Tagging and Versioning Strategy

## Tag Types

- Semver release tag:
  - `vMAJOR.MINOR.PATCH` (example: `v0.1.0`)
  - immutable, promoted for production Helm values
- Commit SHA tag:
  - `sha-<git_sha>` (example: `sha-a1b2c3d`)
  - immutable, used for CI verification and canary deployment
- Branch preview tag:
  - `branch-<sanitized_branch>` (example: `branch-main`)
  - mutable, non-production preview only

## Helm Compatibility Rules

- Chart defaults pin all control-plane images to semver tags.
- Production overlays should use semver or SHA tags only.
- Branch tags are allowed only in local/stage experimentation.
- Mixed-version deployments are supported only within the same minor line unless explicitly tested.

## Version Matrix (MVP)

| Chart Version | core/api/console Default Tag | Compatibility |
| --- | --- | --- |
| `0.1.x` | `v0.1.0` (or newer `v0.1.x`) | fully supported |
| `0.1.x` | `sha-*` built from `0.1.x` branch | supported for CI/canary |
| `0.1.x` | `branch-*` | non-prod only |

## Rollback Guidance

- Rollback targets must use previously published immutable tags (`v*` or `sha-*`).
- Avoid rollback to mutable `branch-*` tags.
- Helm rollback should pair chart revision with image tags that were deployed together in release metadata.

## CI Publish Expectations

- Every merged commit publishes:
  - `sha-<git_sha>`
  - optional `branch-<branch>`
- Release workflow publishes:
  - semver tags (`v*`)
- Signed provenance and digest pinning should be added when CI pipeline task (`td-ffc123`) is implemented.

## Build Targets

- Core image Dockerfile: `docker/core.Dockerfile` (builds `./cmd/smith-core` and exposes `/healthz` + `/readyz` on `:8081`).
- Replica image Dockerfile: `docker/replica.Dockerfile` (builds `./cmd/smith-replica` for Job startup).
- Runtime base for both images: `gcr.io/distroless/static-debian12:nonroot` (UID `65532`).
