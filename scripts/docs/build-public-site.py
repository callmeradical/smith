#!/usr/bin/env python3
from __future__ import annotations

import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
DOCS_ROOT = ROOT / "docs"
CONFIG_PATH = ROOT / "zensical.toml"
PUBLIC_EXCLUDES = {
    "docs/planning",
    "docs/prds",
    "docs/docs-to-prd-lifecycle.md",
}


def should_exclude(path: Path) -> bool:
    rel = path.relative_to(ROOT).as_posix()
    return any(rel == item or rel.startswith(f"{item}/") for item in PUBLIC_EXCLUDES)


def copy_public_docs(target_docs_root: Path) -> None:
    for source in DOCS_ROOT.rglob("*"):
        if should_exclude(source):
            continue
        rel = source.relative_to(DOCS_ROOT)
        target = target_docs_root / rel
        if source.is_dir():
            target.mkdir(parents=True, exist_ok=True)
            continue
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source, target)


def main() -> int:
    with tempfile.TemporaryDirectory(prefix="smith-public-docs-") as temp_dir:
        temp_root = Path(temp_dir)
        temp_docs = temp_root / "docs"
        temp_docs.mkdir(parents=True, exist_ok=True)

        copy_public_docs(temp_docs)
        shutil.copy2(CONFIG_PATH, temp_root / "zensical.toml")

        result = subprocess.run(["zensical", "build"], cwd=temp_root)
        if result.returncode != 0:
            return result.returncode

        site_dir = temp_root / "site"
        target_site = ROOT / "site"
        if target_site.exists():
            shutil.rmtree(target_site)
        shutil.copytree(site_dir, target_site)

    print("Public docs site build completed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
