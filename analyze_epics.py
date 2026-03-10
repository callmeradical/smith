
import json
import re
import subprocess

def get_issues():
    res = subprocess.run(["gh", "issue", "list", "--state", "open", "--json", "number,title,body"], capture_output=True, text=True)
    return json.loads(res.stdout)

def analyze_epic(issue):
    body = issue['body']
    # Look for [x] and [ ] patterns
    done = re.findall(r'\[x\]', body, re.IGNORECASE)
    undone = re.findall(r'\[ \]', body)
    
    total = len(done) + len(undone)
    if total == 0:
        return None
    
    percent = (len(done) / total) * 100
    return {
        'number': issue['number'],
        'title': issue['title'],
        'done': len(done),
        'undone': len(undone),
        'total': total,
        'percent': percent,
        'undone_items': re.findall(r'\[ \]\s*(.*)', body)
    }

issues = get_issues()
epics = []
for issue in issues:
    analysis = analyze_epic(issue)
    if analysis:
        epics.append(analysis)

# Sort by percentage descending
epics.sort(key=lambda x: x['percent'], reverse=True)

for epic in epics:
    print(f"Issue #{epic['number']}: {epic['title']}")
    print(f"  Progress: {epic['done']}/{epic['total']} ({epic['percent']:.1f}%)")
    if epic['undone_items']:
        print(f"  Undone items:")
        for item in epic['undone_items']:
            print(f"    - {item}")
    print("-" * 40)
