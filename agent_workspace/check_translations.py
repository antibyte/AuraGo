import json
import os
from pathlib import Path
from collections import defaultdict

LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']
BASE = Path('ui/lang')

def load_json(path):
    try:
        with open(path, 'r', encoding='utf-8') as f:
            return json.load(f)
    except Exception as e:
        return {"__ERROR__": str(e)}

issues = []

def check_dir(dir_path):
    subdirs = [d for d in dir_path.iterdir() if d.is_dir()]
    if not subdirs:
        # files directly in dir
        check_files_in_dir(dir_path)
    else:
        for subdir in subdirs:
            check_files_in_dir(subdir)

def check_files_in_dir(folder):
    en_path = folder / 'en.json'
    if not en_path.exists():
        return
    en_data = load_json(en_path)
    if '__ERROR__' in en_data:
        issues.append(f"ERROR loading {en_path}: {en_data['__ERROR__']}")
        return
    en_keys = set(en_data.keys())
    for lang in LANGS:
        if lang == 'en':
            continue
        lang_path = folder / f'{lang}.json'
        if not lang_path.exists():
            issues.append(f"MISSING FILE: {lang_path}")
            continue
        lang_data = load_json(lang_path)
        if '__ERROR__' in lang_data:
            issues.append(f"ERROR loading {lang_path}: {lang_data['__ERROR__']}")
            continue
        lang_keys = set(lang_data.keys())
        missing = en_keys - lang_keys
        extra = lang_keys - en_keys
        for k in sorted(missing):
            issues.append(f"MISSING KEY: {lang_path.relative_to('ui/lang')}: {k}")
        for k in sorted(extra):
            issues.append(f"EXTRA KEY: {lang_path.relative_to('ui/lang')}: {k}")
        # Check for empty or same-as-english values
        for k in en_keys & lang_keys:
            val = lang_data[k]
            if isinstance(val, str):
                if val.strip() == '':
                    issues.append(f"EMPTY VALUE: {lang_path.relative_to('ui/lang')}: {k}")
                elif val == en_data[k] and lang not in ['en']:
                    # only flag if it's exactly the same and not a proper noun
                    if not any(x in k for x in ['integration_', 'badge_']):
                        issues.append(f"SAME AS EN: {lang_path.relative_to('ui/lang')}: {k} = {val[:50]}")

# Check config and dashboard
check_dir(Path('ui/lang/config'))
check_dir(Path('ui/lang/dashboard'))

# Print summary
print(f"Total issues found: {len(issues)}")
print()

# Group by file
by_file = defaultdict(list)
for issue in issues:
    parts = issue.split(': ', 1)
    if len(parts) == 2:
        by_file[parts[0]].append(parts[1])
    else:
        by_file['OTHER'].append(issue)

for cat in sorted(by_file.keys()):
    items = by_file[cat]
    print(f"\n=== {cat}: {len(items)} ===")
    for item in items[:20]:
        print(f"  {item}")
    if len(items) > 20:
        print(f"  ... and {len(items) - 20} more")
