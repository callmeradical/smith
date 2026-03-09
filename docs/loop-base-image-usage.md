# Loop Base Image Usage in Loop Definitions

## Goal

Give loop authors a single reference for using the base image and the skills
mount contract together.

## Required Contract

- Base image reference must point to the loop base image:
  - local development: `loop-base:local`
  - production-like: registry tag or digest (for example
    `ghcr.io/acme/loop-base:2026.03.09` or
    `ghcr.io/acme/loop-base@sha256:<digest>`)
- Skills root path in the container is fixed:
  - `/home/dev/.codex/skills`
- For loop skill mounts, set `skills[*].mount_path` to a path under that root
  (for example `/home/dev/.codex/skills/commit`).

## Local Development Example

Loop definition payload:

```json
{
  "title": "US-007 local base-image usage",
  "description": "Validate local base image and skills mount path",
  "source_type": "interactive",
  "source_ref": "terminal/us-007-local",
  "environment": {
    "container_image": {
      "ref": "loop-base:local",
      "pull_policy": "IfNotPresent"
    }
  },
  "skills": [
    {
      "name": "commit",
      "source": "local://skills/commit",
      "mount_path": "/home/dev/.codex/skills/commit",
      "read_only": true
    }
  ]
}
```

Equivalent `smithctl` command:

```bash
smithctl loop create \
  --title "US-007 local base-image usage" \
  --description "Validate local base image and skills mount path" \
  --source-type interactive \
  --source-ref terminal/us-007-local \
  --env-image-ref loop-base:local \
  --env-image-pull-policy IfNotPresent \
  --skill name=commit,source=local://skills/commit,mount_path=/home/dev/.codex/skills/commit,read_only=true
```

Start-success smoke example using the same image and skills path:

```bash
mkdir -p tmp-skills/commit
printf 'name: commit\n' > tmp-skills/commit/SKILL.md
docker run --rm \
  -v "$(pwd)/tmp-skills:/home/dev/.codex/skills:ro" \
  loop-base:local \
  sh -lc 'test -f /home/dev/.codex/skills/commit/SKILL.md && codex --version'
```

Expected result: command exits `0`, proving the sample loop definition contract
can start with the base image and mounted skills path.

## Production-Like Example

Use an immutable image reference and keep the same mount contract:

```json
{
  "title": "US-007 prod-like base-image usage",
  "source_type": "prd_task",
  "source_ref": "docs/prd1.md#us-007",
  "environment": {
    "container_image": {
      "ref": "ghcr.io/acme/loop-base@sha256:<digest>",
      "pull_policy": "Always"
    }
  },
  "skills": [
    {
      "name": "commit",
      "source": "local://skills/commit",
      "mount_path": "/home/dev/.codex/skills/commit",
      "read_only": true
    }
  ]
}
```

## Misconfigured Mount Path (Negative Case)

Incorrect loop definition snippet:

```json
{
  "skills": [
    {
      "name": "commit",
      "source": "local://skills/commit",
      "mount_path": "/home/dev/.codex/skillz/commit",
      "read_only": true
    }
  ]
}
```

Expected failure symptoms:

- loop worker starts, but skill files are missing under
  `/home/dev/.codex/skills/...`
- commands that expect `/home/dev/.codex/skills/commit/SKILL.md` fail
- runtime diagnostics show empty or missing entries in
  `/home/dev/.codex/skills`

Repro command for the failure symptom:

```bash
docker run --rm \
  -v "$(pwd)/tmp-skills:/home/dev/.codex/skillz:ro" \
  loop-base:local \
  sh -lc 'test -f /home/dev/.codex/skills/commit/SKILL.md'
```

Expected result: command exits non-zero.

Correction: set the mount target to `/home/dev/.codex/skills` (or a child path
under it, such as `/home/dev/.codex/skills/commit`).

## Bundled Tools and Internal Binaries Verified by Gates

Bundled tools verified by base-image smoke/quality gates:

- `codex`
- `git`
- `curl`
- `jq`
- `make`
- `node`
- `npm`
- `pnpm`
- `python3`
- `pip`
- `rg`
- `bash`

Internal binaries verified by smoke/quality gates:

- `smithctl`
- `smith-verify-completion`

Verification commands:

- `./scripts/check-base-tooling-smoke.sh loop-base:local`
- `./scripts/check-base-internal-binaries-smoke.sh loop-base:local`
- `./scripts/check-base-skills-mount-smoke.sh loop-base:local`
- `./scripts/run-base-quality-gates.sh loop-base:local`
