import json, pathlib

with open('ui/i18n.json', encoding='utf-8') as f:
    data = json.load(f)

pathlib.Path('ui/lang').mkdir(exist_ok=True)

for lang, content in data.items():
    fname = 'ui/lang/meta.json' if lang == '_meta' else f'ui/lang/{lang}.json'
    with open(fname, 'w', encoding='utf-8') as f:
        json.dump(content, f, ensure_ascii=False, indent=2)
    count = len(content) if isinstance(content, dict) else '?'
    print(f"Written {fname} ({count} keys)")
