#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

echo "Validating lifecycle documentation metadata"
./scripts/docs/validate-lifecycle-docs.py

python3 - <<'PY'
from pathlib import Path
import re
import sys

root = Path('docs')
pattern = re.compile(r'\[[^\]]+\]\(([^)]+)\)')
errors = []

for markdown_file in root.rglob('*.md'):
    text = markdown_file.read_text(encoding='utf-8')
    for target in pattern.findall(text):
        value = target.strip()
        if not value:
            continue
        if '://' in value or value.startswith('#') or value.startswith('mailto:'):
            continue
        local_path = value.split('#', 1)[0]
        resolved = (markdown_file.parent / local_path).resolve()
        if not resolved.exists():
            errors.append((str(markdown_file), target))

if errors:
    print('Broken markdown links detected:')
    for source, target in errors:
        print(f'  {source} -> {target}')
    sys.exit(1)

print('No broken local markdown links found.')
PY

echo "Running source docs build validation"
zensical build >/tmp/smith-docs-build.log

echo "Running public docs build validation"
./scripts/docs/build-public-site.py >/tmp/smith-public-docs-build.log

echo "Docs quality checks passed"
