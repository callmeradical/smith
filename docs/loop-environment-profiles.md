# Loop Environment Profiles

## Goal

Allow each loop to run in a deterministic, configurable execution environment. Define the loop environment contract used by ingress and API create flows, including precedence, defaults, and validation.

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

## API Contract

`POST /v1/loops` and ingress-derived loop creation accept:

```json
{
  "title": "Build smithctl loop env support",
  "source_type": "prd_task",
  "source_ref": "docs/prd1.md#cli-task-1",
  "environment": {
    "preset": "standard",
    "mise": {
      "tool_versions_file": ".tool-versions"
    },
    "env": {
      "GOFLAGS": "-mod=readonly"
    }
  }
}
```

Stored anomaly records include normalized `environment` with `resolved_mode`.

## Schema

Top-level fields:

- `preset` (string): environment baseline preset.
  - Allowed: entries from the environment preset catalog API.
  - Built-in presets: `standard`, `secure`, `performance`, `minimal`.
  - Default: project default preset (`SMITH_DEFAULT_ENV_PRESET`, fallback `standard`).
- `mise` (object): runtime/toolchain resolved from mise.
  - Requires `tool_versions_file` or non-empty `tools` map.
- `container_image` (object): direct replica image override.
  - Requires `ref`.
  - `pull_policy`: `Always`, `IfNotPresent`, `Never` (default `IfNotPresent`).
- `dockerfile` (object): build-on-demand image profile.
  - Requires `context_dir` and `dockerfile_path`.
- `env` (map[string]string): explicit environment variables.
- `resolved_mode` (string): computed server-side mode after normalization.

## Precedence and Conflict Rules

Highest to lowest precedence:

1. `dockerfile` (Loop Dockerfile build spec)
2. `container_image` (Loop explicit container image)
3. `mise` (Loop mise profile)
4. `preset` (Project default preset)

Validation rejects ambiguous source configuration:

- only one of `mise`, `container_image`, `dockerfile` may be set.
- if multiple are provided, request is rejected with actionable conflict error that includes precedence order.

`preset` and `env` can be combined with any single source mode.

## Defaults

- Missing `environment` defaults to:

```json
{
  "preset": "standard",
  "resolved_mode": "preset"
}
```

- `container_image.pull_policy` defaults to `IfNotPresent`.

## Error Semantics

Invalid environment payloads are rejected with HTTP `400` and actionable error messages, for example:

- `environment.container_image.ref is required`
- `environment.dockerfile.context_dir is required`
- `environment source conflict: specify only one of mise, container_image, or dockerfile (precedence is dockerfile > container_image > mise > preset)`

## Preset Catalog API

- `GET /v1/environment/presets`: list available presets and `default_preset`
- `POST /v1/environment/presets`: create preset (`{"name":"team-default"}`)
- `GET /v1/environment/presets/{name}`: fetch preset existence/details
- `PUT /v1/environment/presets/{name}`: create/update preset by name

Loop creation validates `environment.preset` against this catalog, and applies
the project default preset when `environment` is omitted.

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

## Dockerfile Build Runtime Flags

- `SMITH_DOCKERFILE_BUILD_ENABLED=true` enables Dockerfile-based loop image builds in `smith-core`.
- `SMITH_DOCKERFILE_IMAGE_REPOSITORY` overrides target image repository for Dockerfile builds.
- Build metadata (`build_tag`, `build_cache_status`, `build_duration_ms`) is journaled for auditability.

## Non-Goals (MVP)

- Arbitrary privileged container runtime configuration.
- Complex multi-stage build orchestration beyond single loop image resolution.
