import os
import re

def find_markdown_files(directory):
    md_files = []
    for root, _, files in os.walk(directory):
        for file in files:
            if file.endswith('.md'):
                md_files.append(os.path.join(root, file))
    return md_files

def check_links(md_files):
    broken_links = []
    link_pattern = re.compile(r'\[([^\]]+)\]\(([^)]+)\)')
    for file in md_files:
        with open(file, 'r', encoding='utf-8') as f:
            content = f.read()
            links = link_pattern.findall(content)
            for text, link in links:
                if link.startswith('http'):
                    continue
                if link.startswith('#'):
                    continue
                # Remove fragment
                link_path = link.split('#')[0]
                if not link_path:
                    continue

                # Resolve relative path
                dir_path = os.path.dirname(file)
                target_path = os.path.normpath(os.path.join(dir_path, link_path))

                if not os.path.exists(target_path):
                    broken_links.append((file, text, link, target_path))
    return broken_links

md_files = find_markdown_files('.')
broken = check_links(md_files)
for file, text, link, target in broken:
    print(f"Broken link in {file}: [{text}]({link}) -> {target}")
