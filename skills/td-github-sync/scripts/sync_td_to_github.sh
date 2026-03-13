#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<USAGE
Usage:
  sync_td_to_github.sh --repo <owner/repo> [--epic <td-id> | --ids <td-id,td-id,...>] [--include-epic]

Options:
  --repo          GitHub repository (required), e.g. callmeradical/smith
  --epic          td epic id to sync all child issues from
  --ids           Comma-separated td issue IDs to sync
  --include-epic  Include the epic itself when using --epic

Behavior:
  - Creates missing GitHub issues with title format: [td-xxxxxx] <td title>
  - Leaves existing GitHub issues unchanged (idempotent)
USAGE
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

repo=""
epic=""
ids_csv=""
include_epic="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) repo="${2:-}"; shift 2 ;;
    --epic) epic="${2:-}"; shift 2 ;;
    --ids) ids_csv="${2:-}"; shift 2 ;;
    --include-epic) include_epic="true"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage; exit 1 ;;
  esac
done

[[ -n "$repo" ]] || { echo "--repo is required" >&2; usage; exit 1; }
[[ -z "$epic" || -z "$ids_csv" ]] || { echo "use either --epic or --ids, not both" >&2; exit 1; }
[[ -n "$epic" || -n "$ids_csv" ]] || { echo "one of --epic or --ids is required" >&2; exit 1; }

require_cmd td
require_cmd gh
require_cmd jq

gh auth status -h github.com >/dev/null 2>&1 || {
  echo "gh is not authenticated for github.com" >&2
  exit 1
}

if [[ -n "$epic" ]]; then
  issues_json=$(td list --epic "$epic" --json)
  if [[ "$include_epic" == "true" ]]; then
    epic_json=$(td show "$epic" --json)
    issues_json=$(jq -s '.[0] + [.[1]]' <(echo "$issues_json") <(echo "$epic_json"))
  fi
else
  IFS=',' read -r -a id_array <<<"$ids_csv"
  issues_json=$(for id in "${id_array[@]}"; do td show "$id" --json; done | jq -s .)
fi

existing=$(gh issue list -R "$repo" --state all --limit 500 --json number,title,url)

jq -c '.[]' <<<"$issues_json" | while IFS= read -r item; do
  id=$(jq -r '.id' <<<"$item")
  title=$(jq -r '.title' <<<"$item")
  type=$(jq -r '.type' <<<"$item")
  priority=$(jq -r '.priority' <<<"$item")
  parent=$(jq -r '.parent_id // ""' <<<"$item")
  desc=$(jq -r '.description // ""' <<<"$item")
  acc=$(jq -r '.acceptance // ""' <<<"$item")

  found=$(jq -r --arg id "$id" '.[] | select(.title | contains("["+$id+"]")) | .url' <<<"$existing" | head -n1)
  if [[ -n "$found" ]]; then
    echo "EXISTS  ${id} ${found}"
    continue
  fi

  gh_title="[${id}] ${title}"
  body=$(cat <<BODY
Synced from Task Director.

- TD ID: ${id}
- Type: ${type}
- Priority: ${priority}
- Parent: ${parent:-none}

## Description
${desc}

## Acceptance Criteria
${acc}
BODY
)

  url=$(gh issue create -R "$repo" --title "$gh_title" --body "$body")
  echo "CREATED ${id} ${url}"
done

echo "DONE sync run complete"
