# Multi-Provider Skill Mount Abstraction (Post-MVP)

## Purpose

Define a provider-agnostic contract for loop skill mounts so Smith can support providers beyond Codex without breaking existing Codex defaults.

## Current Baseline (MVP)

- Canonical loop input: `loop.skills[]`.
- Supported provider: Codex.
- Default mount path when omitted: `/smith/skills/<name>`.

This proposal preserves those defaults and adds a translation layer.

## Canonical Contract (Provider-Neutral)

Canonical skill record (unchanged input semantics):

- `name` (required)
- `source` (required)
- `version` (optional)
- `mount_path` (optional)
- `read_only` (optional, default `true`)

Provider adapters must consume canonical skills and return provider-specific runtime bindings.

## Translation Layer

New core abstraction:

- `SkillMountTranslator` (per provider)
  - input: canonical `[]LoopSkillMount`, provider id, policy
  - output: `ResolvedSkillBinding[]` + audit metadata + warnings/errors

`ResolvedSkillBinding` fields:

- `provider_id`
- `skill_name`
- `resolved_source`
- `resolved_version`
- `resolved_mount_target`
- `read_only`
- `runtime_kind` (`volume`, `workspace_copy`, `remote_artifact`, ...)
- `adapter_metadata` (provider-specific key/value)

## Compatibility Strategy

1. Keep canonical input stable for API/CLI.
2. Codex adapter remains the default implementation.
3. New providers are added by registering a translator; no caller-side schema changes.
4. If a provider cannot satisfy mount semantics, fail with actionable provider-specific error.

## Provider Mapping Expectations

- Codex:
  - `runtime_kind=volume`
  - default mount `/smith/skills/<name>` if `mount_path` omitted
- Provider-X (future, no volume support):
  - `runtime_kind=workspace_copy`
  - mounts become staged files under provider workspace root
- Provider-Y (artifact fetch model):
  - `runtime_kind=remote_artifact`
  - adapter resolves artifact references and injects fetch directives

## Validation Boundaries

Global validation (provider-agnostic):

- name/source required
- duplicate names rejected
- path safety constraints for explicit `mount_path`

Provider validation (adapter-specific):

- unsupported source schemes
- unsupported writable mode
- provider-specific mount target restrictions

## Audit and Traceability

For each loop execution, journal/audit should include:

- canonical requested skills
- translator id/provider id
- resolved bindings and runtime kind per skill
- downgrade/compatibility warnings (if translation changes semantics)

## Rollout Plan

1. Introduce translator interface with Codex implementation only.
2. Migrate core job-generation path to consume `ResolvedSkillBinding`.
3. Add conformance tests shared across providers.
4. Add provider-specific adapters incrementally.

## Non-Goals

- Changing existing `loop.skills` API shape.
- Allowing privileged host-path mounts.
- Implicitly enabling writable mounts for providers without policy allowance.
