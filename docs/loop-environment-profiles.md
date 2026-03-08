# Loop Environment Profiles

## Goal

Allow each loop to run in a deterministic, configurable execution environment.

## Supported Environment Sources (MVP + Extensions)

1. `mise` profile
- Loop references a `mise.toml` (or inline toolchain profile).
- Smith provisions matching runtime/tool versions before execution.

2. Container image reference
- Loop specifies a prebuilt image (`registry/repo:tag` or digest).
- Replica runs directly in that image.

3. Dockerfile build
- Loop specifies Dockerfile path/context and optional build args.
- Smith builds image (or resolves cached build) and executes loop in resulting image.

4. Named environment preset
- Admin-defined reusable profiles (e.g., `go-1.24`, `node-22`, `fullstack-default`).
- Preset can expand to mise + image + env vars + resource defaults.

## Precedence Rules

Highest to lowest precedence:
1. Loop explicit container image
2. Loop Dockerfile build spec
3. Loop mise profile
4. Project default preset

## API Shape (proposed)

`loop.environment` object:
- `preset` (string)
- `mise_file` (path)
- `container_image` (string)
- `dockerfile`:
  - `path`
  - `context`
  - `build_args`
- `env_vars` (allowlist)
- `resources` (cpu/memory)

## Validation and Security

- Enforce allowlist/regex for permitted registries/images.
- Validate Dockerfile build context boundaries.
- Disallow privileged runtime settings by default.
- Audit selected environment source in journal and handoff metadata.

## Observability

Record per-loop environment metadata:
- resolved image digest
- mise/tool versions
- build duration/cache hit
- environment resolution errors

## Non-Goals (MVP)

- Arbitrary privileged container runtime configuration.
- Complex multi-stage build orchestration beyond single loop image resolution.

