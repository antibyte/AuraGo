#!/usr/bin/env python3
"""Generate untranslated_values.json by analyzing current UI files."""
import json
from pathlib import Path
from collections import defaultdict

LANG_DIR = Path("ui/lang")
LANGUAGES = ["cs", "da", "de", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"]

def flatten(obj, prefix=""):
    """Flatten nested JSON into dot-notation keys."""
    items = {}
    if isinstance(obj, dict):
        for k, v in obj.items():
            new_key = f"{prefix}.{k}" if prefix else k
            if isinstance(v, dict):
                items.update(flatten(v, new_key))
            elif isinstance(v, str):
                items[new_key] = v
    return items

# Collect untranslated values
untranslated_values = defaultdict(int)

for section in LANG_DIR.iterdir():
    if not section.is_dir():
        continue
    
    en_path = section / "en.json"
    if not en_path.exists():
        continue
    
    try:
        with open(en_path, "r", encoding="utf-8-sig") as f:
            en_data = json.load(f)
    except Exception as e:
        print(f"Error reading {en_path}: {e}")
        continue
    
    en_flat = flatten(en_data)
    
    for lang in LANGUAGES:
        if lang == "en":
            continue
        
        lang_path = section / f"{lang}.json"
        if not lang_path.exists():
            continue
        
        try:
            with open(lang_path, "r", encoding="utf-8-sig") as f:
                lang_data = json.load(f)
        except Exception as e:
            print(f"Error reading {lang_path}: {e}")
            continue
        
        lang_flat = flatten(lang_data)
        
        # Find keys where English value is unchanged in target language
        for key, en_val in en_flat.items():
            if not isinstance(en_val, str):
                continue
            if key in lang_flat and lang_flat[key] == en_val:
                untranslated_values[en_val] += 1

# Write untranslated_values.json
output = {"values": dict(untranslated_values)}
with open("disposable/untranslated_values.json", "w", encoding="utf-8") as f:
    json.dump(output, f, ensure_ascii=False, indent=2)

print(f"Generated untranslated_values.json with {len(untranslated_values)} unique values")
print(f"Total occurrences: {sum(untranslated_values.values())}")
