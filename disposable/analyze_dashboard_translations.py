#!/usr/bin/env python3
"""Analyze dashboard translation files for missing and untranslated strings."""

import json
import os
from pathlib import Path

LANG_DIR = Path("ui/lang/dashboard")
EN_FILE = LANG_DIR / "en.json"

# Load reference English file
with open(EN_FILE, "r", encoding="utf-8") as f:
    en_data = json.load(f)

en_keys = set(en_data.keys())
print(f"Reference (en.json): {len(en_keys)} keys\n")

# Languages to analyze
languages = ["cs", "da", "de", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"]

for lang in languages:
    file_path = LANG_DIR / f"{lang}.json"
    if not file_path.exists():
        print(f"[{lang}] FILE NOT FOUND")
        continue
    
    with open(file_path, "r", encoding="utf-8") as f:
        data = json.load(f)
    
    # Check structure
    is_nested = "dashboard" in data and isinstance(data.get("dashboard"), dict)
    is_flat = all(k.startswith("dashboard.") for k in data.keys())
    
    # Flatten if nested
    if is_nested:
        flat_data = {}
        for cat_key, cat_value in data.get("dashboard", {}).items():
            if isinstance(cat_value, dict):
                for sub_key, sub_value in cat_value.items():
                    flat_data[f"dashboard.{cat_key}_{sub_key}"] = sub_value
            else:
                flat_data[f"dashboard.{cat_key}"] = cat_value
        data = flat_data
    
    # Find keys
    file_keys = set(data.keys())
    missing_keys = en_keys - file_keys
    untranslated_keys = []
    
    for key in file_keys & en_keys:
        if data[key] == en_data[key]:
            untranslated_keys.append(key)
    
    # Report
    structure = "NESTED (needs conversion)" if is_nested else ("FLAT (correct)" if is_flat else "UNKNOWN")
    print(f"[{lang}] Structure: {structure}")
    print(f"  Keys in file: {len(file_keys)} (expected: {len(en_keys)})")
    
    if missing_keys:
        print(f"  MISSING keys ({len(missing_keys)}):")
        for k in sorted(missing_keys)[:10]:
            print(f"    - {k}")
        if len(missing_keys) > 10:
            print(f"    ... and {len(missing_keys) - 10} more")
    
    if untranslated_keys:
        print(f"  UNTRANSLATED keys ({len(untranslated_keys)}):")
        for k in sorted(untranslated_keys)[:10]:
            print(f"    - {k}: \"{data[k]}\"")
        if len(untranslated_keys) > 10:
            print(f"    ... and {len(untranslated_keys) - 10} more")
    
    if not missing_keys and not untranslated_keys and (is_flat or not is_nested):
        print(f"  ✓ Complete and translated correctly!")
    
    print()
