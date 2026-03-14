#!/usr/bin/env python3
from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def git(*args: str) -> str:
    result = subprocess.run(
        ["git", *args],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    )
    return result.stdout


def parse_frontmatter(blob: str) -> dict[str, str]:
    lines = blob.splitlines()
    if not lines or lines[0].strip() != "---":
        return {}
    data: dict[str, str] = {}
    idx = 1
    while idx < len(lines):
        line = lines[idx]
        if line.strip() == "---":
            return data
        if ":" in line:
            key, value = line.split(":", 1)
            data[key.strip()] = value.strip().strip("'\"")
        idx += 1
    return {}


def show(ref: str, path: str) -> str:
    try:
        return git("show", f"{ref}:{path}")
    except subprocess.CalledProcessError:
        return ""


def list_changes(base: str, head: str) -> list[tuple[str, str | None, str]]:
    output = git("diff", "--name-status", "--find-renames", base, head, "--", "docs/planning")
    changes: list[tuple[str, str | None, str]] = []
    for line in output.splitlines():
        if not line.strip():
            continue
        parts = line.split("\t")
        status = parts[0]
        if status.startswith("R"):
            changes.append((status, parts[1], parts[2]))
        elif status.startswith(("A", "M", "C")):
            changes.append((status, None, parts[1]))
    return changes


def trigger_reason(status_code: str, old_path: str | None, new_path: str, base: str, head: str) -> str | None:
    new_meta = parse_frontmatter(show(head, new_path))
    if not new_meta:
        return None
    if "/approved/" not in new_path:
        return None
    if new_meta.get("status") != "approved":
        return None
    if new_meta.get("prd_mode") not in {"generate", "update"}:
        return None

    if status_code.startswith("R") and old_path and "/proposed/" in old_path and "/approved/" in new_path:
        return "promotion"
    if status_code.startswith("A"):
        return "new-approved-doc"

    old_blob = show(base, old_path or new_path)
    old_meta = parse_frontmatter(old_blob)
    old_status = old_meta.get("status")

    if old_status != "approved":
        return "status-change"
    if new_meta.get("prd_mode") == "update":
        return "approved-doc-updated"
    return None


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: find-prd-triggers.py <base-ref> <head-ref>", file=sys.stderr)
        return 2

    base, head = sys.argv[1], sys.argv[2]
    events = []
    for status_code, old_path, new_path in list_changes(base, head):
        reason = trigger_reason(status_code, old_path, new_path, base, head)
        if not reason:
            continue
        meta = parse_frontmatter(show(head, new_path))
        events.append(
            {
                "reason": reason,
                "doc_path": new_path,
                "doc_id": meta.get("id", ""),
                "title": meta.get("title", ""),
                "prd_mode": meta.get("prd_mode", ""),
                "target_branch": meta.get("target_branch", ""),
                "linked_prd": meta.get("linked_prd", ""),
                "source_path": old_path or new_path,
            }
        )

    print(json.dumps(events, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
