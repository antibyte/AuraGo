#!/usr/bin/env python3
"""Validate all knowledge i18n JSON files."""
import json
import os

LANG_DIR = os.path.join(os.path.dirname(__file__), '..', 'ui', 'lang', 'knowledge')
LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']

errors = []
ref_keys = None

for lang in LANGS:
    filepath = os.path.join(LANG_DIR, f"{lang}.json")
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            data = json.load(f)
        keys = set(data.keys())
        if ref_keys is None:
            ref_keys = keys
            print(f"{lang}.json: {len(data)} keys (reference)")
        else:
            extra = keys - ref_keys
            missing = ref_keys - keys
            if extra:
                for k in sorted(extra):
                    errors.append(f"{lang}.json: extra key '{k}'")
            if missing:
                for k in sorted(missing):
                    errors.append(f"{lang}.json: missing key '{k}'")
            status = "OK" if not extra and not missing else "MISMATCH"
            print(f"{lang}.json: {len(data)} keys - {status}")
    except json.JSONDecodeError as e:
        errors.append(f"{lang}.json: JSON parse error: {e}")
        print(f"{lang}.json: PARSE ERROR")

print()
if errors:
    print(f"VALIDATION FAILED - {len(errors)} error(s):")
    for e in errors:
        print(f"  - {e}")
else:
    print("All validations passed! All 16 language files have consistent keys.")
