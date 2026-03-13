import os
import re

def find_markdown_files(directory):
    md_files = []
    for root, _, files in os.walk(directory):
        for file in files:
            if file.endswith('.md'):
                md_files.append(os.path.basename(file))
    return md_files

with open('docs/index.md', 'r') as f:
    content = f.read()

md_files = find_markdown_files('docs')
for md_file in md_files:
    if md_file == 'index.md': continue
    if md_file not in content:
        print(f"Missing from docs/index.md: {md_file}")
