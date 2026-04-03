import json
from pathlib import Path
from difflib import get_close_matches

LANGS = ['cs', 'da', 'de', 'el', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']

def load_json(path):
    try:
        with open(path, 'r', encoding='utf-8-sig') as f:
            return json.load(f)
    except Exception as e:
        return None

# Load English reference
en_data = load_json(Path('ui/lang/dashboard/en.json'))
en_keys = set(en_data.keys())

print("=== DASHBOARD KEY ANALYSIS ===\n")

for lang in LANGS:
    path = Path(f'ui/lang/dashboard/{lang}.json')
    data = load_json(path)
    if data is None:
        print(f"ERROR loading {path}")
        continue
    
    lang_keys = set(data.keys())
    missing = en_keys - lang_keys
    extra = lang_keys - en_keys
    
    # Try to find typos: extra keys that are very close to missing keys
    typos = []
    for ek in extra:
        matches = get_close_matches(ek, missing, n=1, cutoff=0.85)
        if matches:
            typos.append((ek, matches[0]))
    
    if missing or extra or typos:
        print(f"--- {lang}.json ---")
        if missing:
            for k in sorted(missing):
                print(f"  MISSING: {k}")
        if extra:
            for k in sorted(extra):
                # check if it's a typo
                is_typo = any(t[0] == k for t in typos)
                if not is_typo:
                    print(f"  EXTRA: {k}")
        if typos:
            for wrong, correct in typos:
                print(f"  TYPO: '{wrong}' should be '{correct}'")
        print()

print("\n=== CONFIG SUBDIRECTORIES KEY ANALYSIS ===\n")
config_base = Path('ui/lang/config')
for subdir in sorted(config_base.iterdir()):
    if not subdir.is_dir():
        continue
    en_path = subdir / 'en.json'
    if not en_path.exists():
        continue
    en_data = load_json(en_path)
    if en_data is None:
        continue
    en_keys = set(en_data.keys())
    
    for lang in LANGS:
        path = subdir / f'{lang}.json'
        if not path.exists():
            continue
        data = load_json(path)
        if data is None:
            print(f"ERROR: {path}")
            continue
        lang_keys = set(data.keys())
        missing = en_keys - lang_keys
        extra = lang_keys - en_keys
        
        typos = []
        for ek in extra:
            matches = get_close_matches(ek, missing, n=1, cutoff=0.85)
            if matches:
                typos.append((ek, matches[0]))
        
        if missing or extra or typos:
            print(f"--- {subdir.name}/{lang}.json ---")
            if missing:
                for k in sorted(missing)[:10]:
                    print(f"  MISSING: {k}")
                if len(missing) > 10:
                    print(f"  ... and {len(missing)-10} more missing")
            if extra:
                for k in sorted(extra)[:10]:
                    is_typo = any(t[0] == k for t in typos)
                    if not is_typo:
                        print(f"  EXTRA: {k}")
                if len(extra) > 10:
                    print(f"  ... and {len(extra)-10} more extra")
            if typos:
                for wrong, correct in typos:
                    print(f"  TYPO: '{wrong}' should be '{correct}'")
            print()
