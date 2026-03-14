# Docs-to-PRD Lifecycle

This page defines the initial repository contract for turning approved documentation into reviewable PRD inputs for Smith loops.

## Why This Exists

The repository already has:

- current-state implementation and operations docs under `docs/`
- canonical PRD validation for `.agents/tasks/prd.json`
- loop ingress that can execute approved PRD artifacts

What was missing was an explicit planning lifecycle. This contract adds one so documentation changes can become structured AI inputs instead of ad hoc prompts.

## Lifecycle Tree

Top-level implementation and operations docs remain where they are today. The new planning pipeline lives in dedicated subtrees:

```text
docs/
  planning/
    proposed/
    approved/
    archived/
  prds/
    draft/
    approved/
    archived/
```

Semantics:

- `docs/planning/proposed/`: rough or in-review intent documents. These do not trigger PRD generation.
- `docs/planning/approved/`: approved intent documents. Promotion into this state is the automation boundary for generating or updating a PRD.
- `docs/planning/archived/`: withdrawn or superseded planning documents.
- `docs/prds/draft/`: generated or AI-refined PRDs that still need human review.
- `docs/prds/approved/`: PRDs that are ready to become Smith loop input.
- `docs/prds/archived/`: superseded, withdrawn, or historical PRDs.

## Frontmatter Contract

Planning documents under `docs/planning/**` must include frontmatter like:

```yaml
---
id: loop-recovery-improvements
title: Loop Recovery Improvements
status: approved
doc_type: feature
prd_mode: generate
target_branch: main
owner: lars
linked_prd: prd-loop-recovery-improvements-v1
---
```

Required fields for planning docs:

- `id`: stable lowercase kebab-case identifier
- `title`: human-readable name
- `status`: must match the directory name (`proposed`, `approved`, `archived`)
- `doc_type`: one of `feature`, `architecture`, `workflow`, `runbook`, `release-note`, `adr`, `other`
- `prd_mode`: one of `none`, `generate`, `update`
- `target_branch`: branch Smith should eventually target, usually `main`
- `owner`: current responsible editor/reviewer

Rules:

- approved `feature`, `architecture`, and `workflow` docs must use `prd_mode: generate` or `prd_mode: update`
- `prd_mode: update` is only valid for approved docs
- `linked_prd` is optional, but should be populated once a PRD exists

PRD markdown documents under `docs/prds/**` must include frontmatter like:

```yaml
---
id: prd-loop-recovery-improvements-v1
title: PRD - Loop Recovery Improvements
status: draft
doc_type: prd
source_doc: docs/planning/approved/loop-recovery-improvements.md
target_branch: main
owner: lars
---
```

Required fields for PRD docs:

- `id`
- `title`
- `status`: must match the directory name (`draft`, `approved`, `archived`)
- `doc_type: prd`
- `source_doc`: must point back to a planning document
- `target_branch`
- `owner`

## Trigger Rules

PRD generation should be driven by lifecycle transitions, not by arbitrary doc edits.

The initial trigger contract is:

1. A planning document moves from `docs/planning/proposed/` to `docs/planning/approved/`
2. Or a document under `docs/planning/approved/` changes from `status: proposed` to `status: approved`
3. Or an already-approved doc with `prd_mode: update` changes and needs its linked PRD refreshed

The repository now includes a helper to detect those candidates:

```bash
./scripts/docs/find-prd-triggers.py <base-ref> <head-ref>
```

Example:

```bash
./scripts/docs/find-prd-triggers.py origin/main HEAD
```

The script emits a JSON array describing approved planning docs that should create or update PRDs.

## Operational Flow

The intended pipeline is:

1. Author intent in `docs/planning/proposed/`
2. Review and promote it to `docs/planning/approved/`
3. Generate or update a PRD in `docs/prds/draft/`
4. Review and promote that PRD to `docs/prds/approved/`
5. Feed the approved PRD into a Smith loop
6. When the work lands, update the current-state docs under the main `docs/` tree

This keeps future intent, formalized work definitions, and implemented reality separate.

## Validation

Lifecycle docs are now validated by:

- `./scripts/docs/validate-lifecycle-docs.py`
- `./scripts/docs/quality-check.sh`

The validator checks required frontmatter, status-directory alignment, allowed document types, and PRD/source linkage rules.
