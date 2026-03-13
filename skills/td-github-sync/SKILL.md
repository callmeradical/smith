---
name: td-github-sync
description: Sync Task Director (td) issues to GitHub issues with stable [td-id] title mapping and idempotent create-only behavior. Use when users ask to mirror td stories/epics into GitHub, audit td-vs-GitHub issue parity, or bulk-create missing GitHub issues from td.
---

# TD GitHub Sync

## Overview
Use this skill to mirror `td` work items into GitHub issues while preserving traceability via title prefix format `[td-xxxxxx]`.

The sync is create-only and idempotent: existing GitHub issues are detected and left unchanged.

## Workflow
1. Verify auth and repo target.
2. Choose source scope:
- Epic sync: all child issues in an epic (optionally include the epic itself).
- Explicit sync: a provided list of td issue IDs.
3. Create missing GitHub issues with standardized body fields.
4. Return mapping of `td-id -> GitHub issue URL`.

## Commands
Epic sync including epic record:
```bash
skills/td-github-sync/scripts/sync_td_to_github.sh \
  --repo callmeradical/smith \
  --epic td-f6f789 \
  --include-epic
```

Explicit issue sync:
```bash
skills/td-github-sync/scripts/sync_td_to_github.sh \
  --repo callmeradical/smith \
  --ids td-76bc71,td-98b5f7,td-2558d6
```

## Output Contract
- `CREATED <td-id> <github-url>` for newly created issues.
- `EXISTS  <td-id> <github-url>` for already-synced issues.
- `DONE sync run complete` summary line.

## Constraints
- Requires `td`, `gh`, and `jq`.
- Requires `gh auth status -h github.com` to pass.
- Assumes GitHub issue titles use `[td-id]` prefix for parity lookup.
