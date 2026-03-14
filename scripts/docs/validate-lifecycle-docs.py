#!/usr/bin/env python3
from __future__ import annotations

import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
DOCS_ROOT = ROOT / "docs"

KIND_CONFIG = {
    "planning": {
        "base": DOCS_ROOT / "planning",
        "statuses": {"proposed", "approved", "archived"},
        "required": {"id", "title", "status", "doc_type", "prd_mode", "target_branch", "owner"},
        "doc_types": {"feature", "architecture", "workflow", "runbook", "release-note", "adr", "other"},
        "prd_modes": {"none", "generate", "update"},
        "link_field": "linked_prd",
    },
    "prds": {
        "base": DOCS_ROOT / "prds",
        "statuses": {"draft", "approved", "archived"},
        "required": {"id", "title", "status", "doc_type", "source_doc", "target_branch", "owner"},
        "doc_types": {"prd"},
        "prd_modes": None,
        "link_field": "source_doc",
    },
}

ID_PATTERN = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")


def parse_frontmatter(path: Path) -> tuple[dict[str, object], list[str]]:
    text = path.read_text(encoding="utf-8")
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return {}, [f"{path}: missing frontmatter block"]

    data: dict[str, object] = {}
    idx = 1
    while idx < len(lines):
        line = lines[idx]
        if line.strip() == "---":
            return data, []
        if not line.strip():
            idx += 1
            continue
        if line.startswith("  - ") or line.startswith("\t- "):
            return {}, [f"{path}: invalid top-level list item in frontmatter"]
        if ":" not in line:
            return {}, [f"{path}: invalid frontmatter line '{line}'"]
        key, raw_value = line.split(":", 1)
        key = key.strip()
        value = raw_value.strip()
        if not key:
            return {}, [f"{path}: invalid empty frontmatter key"]
        if not value:
            items: list[str] = []
            idx += 1
            while idx < len(lines):
                nested = lines[idx]
                if nested.strip() == "---":
                    data[key] = items
                    return data, []
                if not nested.strip():
                    idx += 1
                    continue
                if nested.startswith("  - "):
                    items.append(nested[4:].strip())
                    idx += 1
                    continue
                break
            data[key] = items
            continue
        if value.startswith("[") and value.endswith("]"):
            inner = value[1:-1].strip()
            data[key] = [] if not inner else [item.strip().strip("'\"") for item in inner.split(",")]
        elif value.lower() in {"true", "false"}:
            data[key] = value.lower() == "true"
        else:
            data[key] = value.strip("'\"")
        idx += 1

    return {}, [f"{path}: missing closing frontmatter delimiter"]


def validate_file(kind: str, path: Path) -> list[str]:
    cfg = KIND_CONFIG[kind]
    frontmatter, parse_errors = parse_frontmatter(path)
    if parse_errors:
        return parse_errors

    errors: list[str] = []
    rel = path.relative_to(cfg["base"])
    parts = rel.parts
    if len(parts) < 2:
        errors.append(f"{path}: expected file inside status subdirectory")
        return errors

    status_dir = parts[0]
    if status_dir not in cfg["statuses"]:
        errors.append(f"{path}: unknown lifecycle status directory '{status_dir}'")
        return errors

    missing = sorted(field for field in cfg["required"] if not frontmatter.get(field))
    for field in missing:
        errors.append(f"{path}: missing required frontmatter field '{field}'")

    status = str(frontmatter.get("status", ""))
    if status and status != status_dir:
        errors.append(f"{path}: frontmatter status '{status}' does not match directory '{status_dir}'")

    doc_id = str(frontmatter.get("id", ""))
    if doc_id and not ID_PATTERN.match(doc_id):
        errors.append(f"{path}: id must be lowercase kebab-case")

    title = str(frontmatter.get("title", ""))
    if title and len(title) < 8:
        errors.append(f"{path}: title should be descriptive (minimum 8 characters)")

    doc_type = str(frontmatter.get("doc_type", ""))
    if doc_type and doc_type not in cfg["doc_types"]:
        errors.append(f"{path}: unsupported doc_type '{doc_type}'")

    owner = str(frontmatter.get("owner", ""))
    if owner and not owner.strip():
        errors.append(f"{path}: owner must be non-empty")

    target_branch = str(frontmatter.get("target_branch", ""))
    if target_branch and "/" in target_branch and target_branch.startswith("refs/"):
        errors.append(f"{path}: target_branch should be a branch name, not a full ref")

    if kind == "planning":
        prd_mode = str(frontmatter.get("prd_mode", ""))
        if prd_mode and prd_mode not in cfg["prd_modes"]:
            errors.append(f"{path}: unsupported prd_mode '{prd_mode}'")
        if status == "approved" and doc_type in {"feature", "architecture", "workflow"} and prd_mode == "none":
            errors.append(
                f"{path}: approved {doc_type} docs must declare prd_mode 'generate' or 'update'"
            )
        if status != "approved" and prd_mode == "update":
            errors.append(f"{path}: prd_mode 'update' is only valid for approved planning docs")

    if kind == "prds":
        if str(frontmatter.get("doc_type", "")) != "prd":
            errors.append(f"{path}: PRD documents must use doc_type 'prd'")
        source_doc = str(frontmatter.get("source_doc", ""))
        if source_doc and not source_doc.startswith("docs/planning/"):
            errors.append(f"{path}: source_doc must point to docs/planning/")

    return errors


def find_lifecycle_files() -> list[tuple[str, Path]]:
    files: list[tuple[str, Path]] = []
    for kind, cfg in KIND_CONFIG.items():
        if not cfg["base"].exists():
            continue
        for path in sorted(cfg["base"].rglob("*.md")):
            files.append((kind, path))
    return files


def main() -> int:
    errors: list[str] = []
    files = find_lifecycle_files()
    for kind, path in files:
        errors.extend(validate_file(kind, path))

    if errors:
        print("Lifecycle documentation validation failed:")
        for error in errors:
            print(f"  - {error}")
        return 1

    print(f"Lifecycle documentation validation passed for {len(files)} file(s).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
