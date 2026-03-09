# Smith

Smith is an etcd-backed, Kubernetes-native autonomous orchestration platform.

## Purpose

Smith coordinates autonomous execution loops as a state machine stored in etcd. It is designed to:

- accept operator ingress requests (direct, GitHub issues, PRD tasks),
- convert each request into a deterministic loop lifecycle (`unresolved -> overwriting -> synced|flatline|cancelled`),
- enforce safe concurrency with per-loop locks and revision-checked state transitions,
- run loop workers as Kubernetes Jobs and preserve execution evidence (journal, handoff, override, audit).

In this repository, the focus is the MVP control plane, deployment assets, and verification/test harnesses.

## Philosophy

Smith intentionally does not personify agents.

- Agents are modeled as homogeneous and omnicapable execution units, not distinct personalities.
- Anthropomorphizing agents is treated as an implementation constraint that reduces operational flexibility and performance.
- The target model is uniform replication: many equivalent workers, same contract, same capabilities, horizontally scalable.
- The system design favors role-neutral orchestration primitives (state, locks, jobs, handoffs) over persona-specific behavior.
- The platform direction is informed by [Ralph](https://github.com/snarktank/ralph), [marcus/sidecar](https://github.com/marcus/sidecar), [marcus/td](https://github.com/marcus/td), and related projects, but is engineered to scale beyond a single developer machine.

Smith also moves beyond a single-machine file-system model by using etcd + Kubernetes as the control substrate, so execution can scale across distributed compute while preserving deterministic state and traceability.

## Architecture Summary

Smith is split into control-plane and data-plane components.

### Control Plane

- `smith-api` (`cmd/smith-api`): HTTP API for loop create/list/get, GitHub + PRD ingress, operator override actions, provider auth lifecycle, and cost reporting.
- `smith-core` (`cmd/smith-core`): watches unresolved loop state in etcd, acquires per-loop locks, transitions loop state, and schedules replica Jobs in Kubernetes.
- `smithctl` (`cmd/smithctl`): kubectl-style operator CLI for `loop` and `prd` resources with context/config support and scriptable JSON output.
- `smith-console` (`console/` + Helm deployment): operator UI/runtime assets.
- etcd: authoritative source of truth for anomalies, loop lifecycle state, locks, journal events, handoffs, overrides, and audit records.

### Data Plane

- `smith-replica` (`cmd/smith-replica`): Kubernetes Job worker that executes loop work, appends journal entries, writes handoff output, and finalizes loop state.

### Deployment and Ops Assets

- Helm chart: `helm/smith`
- Dockerfiles: `docker/`
- Loop base container: repo-root `Dockerfile` (`alpine:3.21`)
- Core implementation: `internal/source/`
- Supporting docs: `docs/`
- Make-first local workflow: `make help` (doctor/bootstrap/cluster/deploy/test/teardown)

## Loop Base Container Baseline

The canonical loop base container skeleton is defined in the repository root
`Dockerfile` and currently uses `alpine:3.21`.

Why Alpine for the baseline:

- small runtime footprint for faster image distribution,
- simple package/runtime surface for reproducible loop environments,
- clear security patch cadence with explicit minor tag pinning.

Bundled developer tooling in the base image includes:
`bash`, `git`, `curl`, `jq`, `ca-certificates`, `make`, `python3`, `pip`,
`ripgrep`, `node`, `npm`, `pnpm`, and `codex`.

pnpm support is installed via explicit global npm install
(`npm install --global pnpm@latest`) in the Dockerfile.

Tooling smoke test:

```bash
./scripts/check-base-tooling-smoke.sh loop-base:local
```

Negative-case check (example):

```bash
REQUIRED_TOOLS="git definitely-missing" ./scripts/check-base-tooling-smoke.sh loop-base:local
```

### Internal Binary Bundle Contract

The loop base image uses a dedicated Docker builder stage to compile internal
Go binaries, then copies artifacts into `/usr/local/bin` in the runtime image.
`/usr/local/bin` is on `PATH` for the runtime user.

Builder inputs and expected artifacts are declared by
`docker/base-internal-binaries.txt`:

- Input package: `./cmd/<binary-name>`
- Builder artifact: `/out/<binary-name>`
- Runtime artifact: `/usr/local/bin/<binary-name>`

To add a new binary, append it to `docker/base-internal-binaries.txt` and
ensure `./cmd/<binary-name>` supports `--version` for smoke verification.

Internal binary smoke test:

```bash
./scripts/check-base-internal-binaries-smoke.sh loop-base:local
```

Negative-case check (example):

```bash
BINARY_LIST_FILE=/tmp/does-not-exist ./scripts/check-base-internal-binaries-smoke.sh loop-base:local
```

### Skills Volume Mount Contract

The runtime image always creates `/home/dev/.codex/skills` and grants access to
the runtime user (`dev`). The container startup command does not write to this
path, so a mounted skills volume is not overwritten at startup.

Loop-definition examples for local and production-like contexts are documented
in [`docs/loop-base-image-usage.md`](docs/loop-base-image-usage.md).

Skills mount smoke test (mounted + unmounted scenarios):

```bash
./scripts/check-base-skills-mount-smoke.sh loop-base:local
```

Example host mount:

```bash
mkdir -p tmp-skills
echo "example-skill" > tmp-skills/example.txt
docker run --rm -v "$(pwd)/tmp-skills:/home/dev/.codex/skills" loop-base:local sh -lc 'ls -la /home/dev/.codex/skills'
```

### Reproducible Quality Gates

Use the fail-fast quality-gate runner to enforce the top-level gates in a
single deterministic sequence:

```bash
./scripts/run-base-quality-gates.sh loop-base:local
```

The script runs these commands in order and exits non-zero on first failure:

1. `docker build -t loop-base:local .`
2. `docker run --rm loop-base:local sh -lc 'codex --version && git --version && node --version && python3 --version && rg --version'`
3. `docker run --rm -v $(pwd)/tmp-skills:/home/dev/.codex/skills loop-base:local sh -lc 'test -d /home/dev/.codex/skills && ls -la /home/dev/.codex/skills'`
4. `hadolint Dockerfile`
5. `shellcheck scripts/*.sh`
6. `trivy image --severity CRITICAL --exit-code 1 loop-base:local`
7. `syft packages loop-base:local -o spdx-json > artifacts/sbom-loop-base.spdx.json`

The SBOM artifact is written to
`artifacts/sbom-loop-base.spdx.json` to preserve scan evidence.

Required local tooling setup for lint and security gates:

```bash
brew install hadolint shellcheck trivy syft
```

Verify tool installation:

```bash
hadolint --version
shellcheck --version
trivy --version
syft version
```

Negative-case security check (expected trivy gate failure):

```bash
./scripts/check-trivy-critical-negative.sh knqyf263/vuln-image:1.2.3
```

This check confirms that `trivy image --severity CRITICAL --exit-code 1 ...`
returns a non-zero exit code when the image contains known critical findings.

## Key API Endpoints

Implemented today:
- `POST /v1/loops` single/batch direct loop creation.
- `POST /v1/loops` supports environment profiles (`preset`, `mise`, `container_image`, `dockerfile`) with server-side validation/defaulting.
- `POST /v1/ingress/github/issues` ingest one or more GitHub issues into loop specs.
- `POST /v1/ingress/prd` ingest markdown/json PRD inputs into loop specs.
- `GET /v1/loops/{id}` and `GET /v1/loops/{id}/journal` for state and traceability.
- `POST /v1/control/override` for operator state overrides with reason/audit trail.
- `POST /v1/auth/codex/connect/start|complete`, `GET /v1/auth/codex/status`, and `POST /v1/auth/codex/disconnect` for provider auth lifecycle.
- `GET /v1/reporting/cost?loop_id={id}` for loop token/cost aggregation from journal metadata.

Aspirational (planned, not implemented yet):
- `GET /v1/loops/{id}/handoffs`, `GET /v1/loops/{id}/overrides`, and `GET /v1/loops/{id}/trace` for end-to-end execution evidence.
- `POST /v1/loops/{id}/control/attach`, `/detach`, and `/command` for authenticated operator interactive control actions.
- `GET /v1/audit?loop_id={id}` for immutable operator/auth action audit records.

## Local Git Hooks

Install repo-managed hooks:

```bash
make hooks-install
```

Hook behavior:
- `pre-commit`: quick checks (`go test ./cmd/...`)
- `pre-push`: full gate (`make build` + `make test`)

Temporarily bypass hooks if needed:

```bash
SKIP_GIT_HOOKS=1 git commit -m "..."
SKIP_GIT_HOOKS=1 git push
```

## Frontend Playwright Tests

Install frontend test dependencies:

```bash
npm install
```

Run Playwright tests for the console UI:

```bash
npm run test:frontend
# or
make test-frontend
```

Artifacts are written under `output/playwright/` (HTML report + failure artifacts).

Run tests against a deployed, port-forwarded console UI:

```bash
kubectl -n smith-system port-forward svc/smith-smith-console 3000:3000
npm run test:frontend:live
```

## Technology Stack and Thanks

See the dedicated documentation page for:
- the current technology stack (Kubernetes, Helm, vCluster, etcd, Go, Docker, and related tooling),
- acknowledgments and inspiration credits, including [marcus/td](https://github.com/marcus/td), [marcus/sidecar](https://github.com/marcus/sidecar), and [Ralph](https://github.com/snarktank/ralph).

Reference: [docs/technology-stack-and-thanks.md](docs/technology-stack-and-thanks.md)
