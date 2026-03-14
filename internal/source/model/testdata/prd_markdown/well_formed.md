# Smith PRD Validation

## Overview

Normalize markdown PRDs into canonical JSON for downstream workflows.

## Goals

- Make canonical PRD JSON deterministic.
- Preserve story ordering from markdown.

## Non-Goals

- Building a rich editor.

## Success Metrics

- Imports preserve story IDs and quality gates.

## Rules

- Canonical JSON is the source of truth.

## Quality Gates

- go test ./...
- ./scripts/validate-acceptance.sh

## Stories

### US-001: Define validation contract

As a maintainer, I want validation diagnostics shared across entrypoints.

#### Acceptance Criteria

- Validation returns machine-readable diagnostics.

#### Status

in_progress

### US-002: Normalize markdown PRDs into canonical JSON

As a user, I want markdown normalized into canonical PRD JSON.

#### Depends On

- US-001

#### Acceptance Criteria

- Supported headings map into canonical JSON fields.
- Quality gates are preserved.
