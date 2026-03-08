#!/usr/bin/env bash
set -euo pipefail

TARGET_DIR="${1:-/tmp/smith-test-repo}"
REPO_DIR="$(cd "$(dirname "$TARGET_DIR")" && pwd)/$(basename "$TARGET_DIR")"

rm -rf "$REPO_DIR"
mkdir -p "$REPO_DIR"
cd "$REPO_DIR"

git init -b main >/dev/null
git config user.name "smith-fixture"
git config user.email "smith-fixture@example.com"

mkdir -p service tasks
echo "handler=v1" > service/handler.txt
echo "fixtures baseline" > README.md
git add .
git commit -m "fixture: baseline" >/dev/null

# Scenario: single loop success
git checkout -b scenario/single-loop-success >/dev/null
echo "task: update handler to v2" > tasks/single-loop.md
echo "handler=v2" > service/handler.txt
git add .
git commit -m "scenario: single loop success" >/dev/null

# Scenario: concurrent branch A
git checkout main >/dev/null
git checkout -b scenario/concurrent-safe-a >/dev/null
mkdir -p tasks
echo "task: concurrent A" > tasks/concurrent-a.md
echo "feature=a" > service/feature-a.txt
git add .
git commit -m "scenario: concurrent branch a" >/dev/null

# Scenario: concurrent branch B
git checkout main >/dev/null
git checkout -b scenario/concurrent-safe-b >/dev/null
mkdir -p tasks
echo "task: concurrent B" > tasks/concurrent-b.md
echo "feature=b" > service/feature-b.txt
git add .
git commit -m "scenario: concurrent branch b" >/dev/null

# Scenario: deterministic merge conflict on same file/line as single-loop-success
git checkout main >/dev/null
git checkout -b scenario/merge-conflict >/dev/null
mkdir -p tasks
echo "task: merge conflict" > tasks/merge-conflict.md
echo "handler=conflict-path" > service/handler.txt
git add .
git commit -m "scenario: merge conflict" >/dev/null

git checkout main >/dev/null

echo "provisioned fixture repo: $REPO_DIR"
git branch --list
