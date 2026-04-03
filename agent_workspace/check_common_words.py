import json
from pathlib import Path

COMMON_WORDS = {
    'Save': ['fr', 'es', 'it', 'pl', 'cs', 'nl', 'sv', 'no', 'da', 'hi', 'zh', 'ja', 'el', 'pt'],
    'Cancel': ['fr', 'es', 'it', 'pl', 'cs', 'nl', 'sv', 'no', 'da', 'hi', 'zh', 'ja', 'el', 'pt'],
    'Delete': ['fr', 'es', 'it', 'pl', 'cs', 'nl', 'sv', 'no', 'da', 'hi', 'zh', 'ja', 'el', 'pt'],
    'Loading': ['fr', 'es', 'it', 'pl', 'cs', 'nl', 'sv', 'no', 'da', 'hi', 'zh', 'ja', 'el', 'pt'],
    'Error': ['fr', 'es', 'it', 'pl', 'cs', 'nl', 'sv', 'no', 'da', 'hi', 'zh', 'ja', 'el', 'pt'],
    'Enabled': ['fr', 'es', 'it', 'pl', 'cs', 'nl', 'sv', 'no', 'da', 'hi', 'zh', 'ja', 'el', 'pt'],
    'Disabled': ['fr', 'es', 'it', 'pl', 'cs', 'nl', 'sv', 'no', 'da', 'hi', 'zh', 'ja', 'el', 'pt'],
}

files_to_check = []
for lang in ['fr', 'es', 'it', 'pl', 'cs', 'nl', 'sv', 'no', 'da', 'pt', 'el']:
    files_to_check.append(Path(f'ui/lang/dashboard/{lang}.json'))
    files_to_check.append(Path(f'ui/lang/config/{lang}.json'))
    for subdir in Path('ui/lang/config').iterdir():
        if subdir.is_dir():
            p = subdir / f'{lang}.json'
            if p.exists():
                files_to_check.append(p)

issues = []
for path in files_to_check:
    if not path.exists():
        continue
    try:
        with open(path, 'r', encoding='utf-8-sig') as f:
            data = json.load(f)
    except:
        continue
    lang = path.stem
    for k, v in data.items():
        if not isinstance(v, str):
            continue
        for word, langs in COMMON_WORDS.items():
            if lang in langs and v.strip() == word:
                issues.append(f'{path.relative_to("ui/lang")}: {k} = "{v}"')

print(f'Found {len(issues)} untranslated common words')
for issue in issues[:50]:
    print(issue)
