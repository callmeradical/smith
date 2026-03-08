# Loop Environment Profile

## Goal

Define the loop environment contract used by ingress and API create flows, including precedence, defaults, and validation.

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

Resolution order:

1. `dockerfile`
2. `container_image`
3. `mise`
4. `preset`

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
