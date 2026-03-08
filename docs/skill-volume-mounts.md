# Skill Volume Mounts for Loop Runtime

## Goal

Allow loops to mount skill bundles at runtime so agents can access curated instructions/assets/scripts during execution.

## MVP Scope

- Provider support: Codex only.
- Skills can be mounted either:
  - from loop definition (`loop.skills`), or
  - from runtime policy/default presets.
- Default Codex mount path is used unless explicitly overridden.

## Proposed Loop Schema

`loop.skills` array entries:
- `name` (required): logical skill identifier
- `source` (required): source reference (registry/path/artifact ID)
- `mount_path` (optional): provider-specific mount target
- `read_only` (default `true`)
- `version` (optional): pinned skill version/hash

## Codex Default Behavior

- If `mount_path` omitted, use Codex default mountpoint.
- Mount skills as read-only volumes by default.
- Journal resolved skill list and versions in loop metadata/handoff.

## Runtime Integration

- Agent Core resolves requested skills before Replica job creation.
- K8s Job spec includes volume + volumeMount entries for resolved skills.
- Missing or invalid skills fail fast with actionable error state.

## Validation and Security

- Restrict allowed skill sources via allowlist policy.
- Enforce read-only mount by default; writable mounts require explicit policy.
- Prevent path traversal and unsafe mount targets.
- Record mount decisions in audit/journal.

## Non-Goals (MVP)

- Multi-agent dynamic mount translation beyond Codex default.
- Arbitrary privileged volume mounts.

