# PRD Authoring Workflow

This workflow defines how Smith turns a human-authored PRD into a loop-ready artifact.

## Canonical JSON Contract

Canonical PRDs are JSON documents written to `.agents/tasks/prd.json`. The shared validator in `internal/source/model/prd.go` is the source of truth for both CLI and API behavior.

Required top-level fields:

- `version`: positive integer schema version. The current canonical value is `1`.
- `project`: non-empty project name.
- `overview`: non-empty scope summary.
- `qualityGates`: non-empty list of executable verification commands.
- `stories`: non-empty list of canonical `US-###` stories.

Story rules:

- Story IDs must be sequential and canonical: `US-001`, `US-002`, and so on.
- Story `status` must be one of `open`, `in_progress`, or `done`.
- Each story must include `title`, `description`, and at least one acceptance criterion.
- `dependsOn` may only reference earlier, known story IDs.

Validation semantics:

- `valid=true` means the PRD is accepted for export and ingress.
- `readiness=pass` means there are no blocking errors or warnings.
- `readiness=warn` means execution is allowed but the validator found non-blocking issues such as oversized stories or weak acceptance criteria.
- `readiness=fail` means execution is blocked.
- Diagnostics are machine-readable objects with `code`, `path`, `message`, optional `storyId`, and optional `suggestion`.

## Markdown Authoring Rules

Markdown is an authoring format only. Smith normalizes markdown into canonical JSON before validation, export, or ingress.

Supported document structure:

- `# <Project>` sets the project name.
- `## Overview`, `## Goals`, `## Non-Goals`, `## Success Metrics`, `## Open Questions`, `## Rules`, and `## Quality Gates` map to canonical sections.
- `## Stories` contains ordered story entries.
- `### US-001: Story title` starts a canonical story.
- `#### Status`, `#### Depends On`, and `#### Acceptance Criteria` map to story metadata.
- List items under `Depends On` must reference canonical IDs such as `US-001`.

Authoring guidance:

- Keep story descriptions in prose and acceptance criteria in bullets.
- Include at least one negative-path acceptance criterion when a workflow can fail.
- Keep each story small enough for one Ralph iteration.

## End-To-End Happy Path

The repository ships a valid markdown fixture at `docs/examples/prd-authoring/valid-prd.md`.

Import markdown into canonical JSON:

```bash
smith --prd --from-markdown docs/examples/prd-authoring/valid-prd.md --out .agents/tasks/prd.json
```

Validate the canonical artifact:

```bash
smith --prd validate .agents/tasks/prd.json
```

Expected validation result:

```json
{
  "valid": true,
  "readiness": "pass"
}
```

Render stable markdown back from canonical JSON:

```bash
smith --prd --from-json .agents/tasks/prd.json --to-markdown /tmp/prd-roundtrip.md
```

Ingress the canonical PRD when it is ready for autonomous execution:

```bash
smithctl --output json prd submit --file .agents/tasks/prd.json --source-ref .agents/tasks/prd.json
```

When the response contains loop IDs and `source_ref` values like `.agents/tasks/prd.json#US-001`, the PRD is eligible for autonomous execution.

## Negative Path

The repository also ships an invalid canonical fixture at `docs/examples/prd-authoring/invalid-prd.json`.

Validate the invalid PRD:

```bash
smith --prd validate docs/examples/prd-authoring/invalid-prd.json
```

Expected diagnostics:

```json
{
  "valid": false,
  "errors": [
    {
      "code": "prd_missing_quality_gates",
      "path": "$.qualityGates",
      "message": "at least one quality gate is required"
    },
    {
      "code": "prd_missing_stories",
      "path": "$.stories",
      "message": "at least one story is required"
    }
  ],
  "readiness": "fail"
}
```

Submitting the same file through ingress stays blocked:

```bash
smithctl --output json prd submit --file docs/examples/prd-authoring/invalid-prd.json
```

The API returns the same validation report and no loops are created.

## Verification Hooks

The documented workflow is exercised by:

- `go test ./...`
- `./scripts/test/e2e-prd-authoring.sh`
- `./scripts/validate-acceptance.sh`
- `make ci-local-act`
