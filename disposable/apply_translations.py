#!/usr/bin/env python3
"""Apply translations from translations.json to all language files."""
import json, os
from pathlib import Path
from collections import defaultdict

LANG_DIR = Path("ui/lang")
NON_EN_LANGS = ["cs","da","de","el","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh"]

def read_json(path):
    try:
        with open(path, "r", encoding="utf-8-sig") as f:
            return json.load(f), None
    except Exception as e:
        return None, str(e)

def write_json(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
        f.write("\n")

def apply_translations():
    trans_path = Path("disposable/translations.json")
    if not trans_path.exists():
        print("ERROR: disposable/translations.json not found")
        return

    with open(trans_path, "r", encoding="utf-8") as f:
        TRANSLATIONS = json.load(f)

    print(f"Loaded {len(TRANSLATIONS)} translation entries")

    stats = defaultdict(lambda: {"fixed": 0, "errors": 0})
    dirs = []
    for root, _, files in os.walk(LANG_DIR):
        if "en.json" in files:
            dirs.append(Path(root))

    for directory in sorted(dirs):
        en_path = directory / "en.json"
        en_data, err = read_json(en_path)
        if err:
            continue

        for lang in NON_EN_LANGS:
            lang_path = directory / f"{lang}.json"
            if not lang_path.exists():
                continue

            lang_data, err = read_json(lang_path)
            if err:
                stats[lang]["errors"] += 1
                continue

            modified = False
            for key, en_value in en_data.items():
                if key not in lang_data:
                    continue
                lang_value = str(lang_data[key]).strip()
                en_value_str = str(en_value).strip()
                if lang_value.lower() != en_value_str.lower():
                    continue
                if not en_value_str:
                    continue

                if en_value_str in TRANSLATIONS:
                    trans = TRANSLATIONS[en_value_str].get(lang)
                    if trans:
                        lang_data[key] = trans
                        modified = True
                        stats[lang]["fixed"] += 1

            if modified:
                write_json(lang_path, lang_data)

    print("\n=== Translation Fix Summary ===")
    total_fixed = 0
    for lang in NON_EN_LANGS:
        s = stats[lang]
        if s["fixed"] > 0 or s["errors"] > 0:
            print(f"  {lang}: {s['fixed']} fixed, {s['errors']} errors")
            total_fixed += s["fixed"]
    print(f"\nTotal keys fixed: {total_fixed}")

if __name__ == "__main__":
    apply_translations()
