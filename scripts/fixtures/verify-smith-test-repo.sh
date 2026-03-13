#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="${1:-/tmp/smith-test-repo}"

if [[ ! -d "$REPO_DIR/.git" ]]; then
  echo "fixture repo not found: $REPO_DIR" >&2
  exit 1
fi

required_branches=(
  main
  scenario/single-loop-success
  scenario/concurrent-safe-a
  scenario/concurrent-safe-b
  scenario/merge-conflict
)

for branch in "${required_branches[@]}"; do
  git -C "$REPO_DIR" rev-parse --verify "$branch" >/dev/null
  echo "branch present: $branch"
done

git -C "$REPO_DIR" show scenario/single-loop-success:service/handler.txt | grep -q "handler=v2"
git -C "$REPO_DIR" show scenario/concurrent-safe-a:service/feature-a.txt | grep -q "feature=a"
git -C "$REPO_DIR" show scenario/concurrent-safe-b:service/feature-b.txt | grep -q "feature=b"
git -C "$REPO_DIR" show scenario/merge-conflict:service/handler.txt | grep -q "handler=conflict-path"

echo "fixture verification passed"
