# Smith PRD Workflow Validation

## Overview

Document and verify the PRD authoring workflow from markdown authoring to loop ingress.

## Goals

- Normalize markdown into canonical JSON.
- Validate readiness before ingress.
- Preserve story order during markdown export.

## Rules

- Canonical JSON remains the source of truth.
- Validation failures block ingress.

## Quality Gates

- go test ./...
- ./scripts/validate-acceptance.sh
- make ci-local-act

## Stories

### US-001: Document canonical PRD workflow

As a maintainer, I want a single documented path from markdown authoring to canonical JSON.

#### Acceptance Criteria

- `smith --prd --from-markdown` writes `.agents/tasks/prd.json`.
- `smith --prd validate` reports `valid=true` and `readiness=pass`.
- `smith --prd --from-json --to-markdown` produces stable markdown output.
- Invalid markdown structure is rejected before canonical JSON is accepted.

### US-002: Block invalid PRDs before ingress

As a maintainer, I want invalid PRDs rejected before autonomous execution starts.

#### Depends On

- US-001

#### Acceptance Criteria

- `smithctl prd submit` accepts a valid canonical PRD and returns loop IDs.
- Invalid PRDs return machine-readable diagnostics.
- No loop is created when readiness validation fails.
