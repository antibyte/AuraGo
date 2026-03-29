#!/usr/bin/env python3
"""Analyze AuraGo translation/i18n systems for inconsistencies."""

import json
import os
from pathlib import Path
from collections import defaultdict

REPO_ROOT = Path("c:/Users/andre/Documents/repo/AuraGo")
I18N_PATH = REPO_ROOT / "ui" / "i18n.json"
LANG_DIR = REPO_ROOT / "ui" / "lang"
REPORT_PATH = REPO_ROOT / "reports" / "i18n_analysis_report.md"

LANGUAGES = ['de', 'en', 'cs', 'da', 'el', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']
SECTIONS_OF_INTEREST = ['chat', 'dashboard', 'config', 'common', 'setup', 'knowledge', 'skills', 'invasion']

def flatten_dict(d, parent_key=''):
    """Flatten nested dict into dot-notation keys."""
    items = []
    for k, v in d.items():
        new_key = f"{parent_key}.{k}" if parent_key else k
        if isinstance(v, dict):
            items.extend(flatten_dict(v, new_key))
        else:
            items.append(new_key)
    return items

def load_i18n():
    with open(I18N_PATH, encoding='utf-8') as f:
        return json.load(f)

def analyze_i18n_json(data):
    print("## i18n.json Analysis\n")
    
    # Key counts per language
    lang_keys = {}
    for lang in LANGUAGES:
        if lang in data:
            keys = set(flatten_dict(data[lang]))
            lang_keys[lang] = keys
            print(f"- `{lang}`: {len(keys)} keys")
    
    # Compare against English
    en_keys = lang_keys.get('en', set())
    print(f"\n### Missing keys in non-English languages (vs en with {len(en_keys)} keys)\n")
    missing_by_lang = {}
    for lang in LANGUAGES:
        if lang == 'en':
            continue
        if lang not in lang_keys:
            missing_by_lang[lang] = "LANGUAGE MISSING ENTIRELY"
            print(f"- `{lang}`: **LANGUAGE MISSING**")
            continue
        missing = en_keys - lang_keys[lang]
        extra = lang_keys[lang] - en_keys
        missing_by_lang[lang] = missing
        print(f"- `{lang}`: missing {len(missing)} keys, extra {len(extra)} keys")
        if missing and len(missing) <= 20:
            for k in sorted(missing):
                print(f"  - `{k}`")
        elif missing:
            print(f"  - (first 10: {', '.join(sorted(missing)[:10])})")
    
    return lang_keys, missing_by_lang

def analyze_lang_dir():
    print("\n## ui/lang/ Analysis\n")
    
    # Discover all sections and files
    sections = sorted([d.name for d in LANG_DIR.iterdir() if d.is_dir()])
    print(f"Sections found: {len(sections)}")
    for s in sections:
        print(f"  - {s}")
    
    # For each section, count files per language
    section_lang_counts = defaultdict(dict)
    section_lang_keys = defaultdict(lambda: defaultdict(set))
    
    for section in sections:
        section_path = LANG_DIR / section
        # Recursively find all JSON files
        json_files = list(section_path.rglob("*.json"))
        
        # Group by language (filename without .json)
        lang_files = defaultdict(list)
        for jf in json_files:
            lang = jf.stem
            lang_files[lang].append(jf)
        
        for lang in LANGUAGES:
            files = lang_files.get(lang, [])
            section_lang_counts[section][lang] = len(files)
            
            # Load and flatten keys
            all_keys = set()
            for f in files:
                try:
                    with open(f, encoding='utf-8') as fh:
                        data = json.load(fh)
                    # Keys are relative to section/file
                    rel_path = f.relative_to(LANG_DIR / section).with_suffix('')
                    prefix = str(rel_path).replace(os.sep, '.').replace('\\', '.')
                    if isinstance(data, dict):
                        for k in flatten_dict(data):
                            all_keys.add(f"{prefix}.{k}")
                    else:
                        all_keys.add(prefix)
                except Exception as e:
                    print(f"ERROR reading {f}: {e}")
            section_lang_keys[section][lang] = all_keys
    
    # Check missing files per section
    print("\n### Missing lang/ files by section\n")
    for section in sections:
        missing_langs = [l for l in LANGUAGES if section_lang_counts[section].get(l, 0) == 0]
        if missing_langs:
            print(f"- `{section}`: missing {len(missing_langs)} languages: {', '.join(missing_langs)}")
        else:
            print(f"- `{section}`: all 16 languages present ({sum(section_lang_counts[section].values())} total files)")
    
    # Key counts for sections of interest
    print("\n### Key counts for sections of interest\n")
    for section in SECTIONS_OF_INTEREST:
        if section not in section_lang_keys:
            print(f"- `{section}`: SECTION NOT FOUND")
            continue
        en_count = len(section_lang_keys[section].get('en', set()))
        print(f"\n**{section}** (en: {en_count} keys)")
        for lang in LANGUAGES:
            count = len(section_lang_keys[section].get(lang, set()))
            missing = en_count - count if lang != 'en' else 0
            print(f"  - `{lang}`: {count} keys" + (f" (missing {missing} vs en)" if lang != 'en' and missing > 0 else ""))
    
    return section_lang_keys, section_lang_counts

def compare_systems(i18n_data, section_lang_keys):
    print("\n## Cross-System Comparison\n")
    
    # Flatten i18n.json keys with section prefix
    i18n_by_section = defaultdict(lambda: defaultdict(set))
    for lang in LANGUAGES:
        if lang not in i18n_data:
            continue
        for key in flatten_dict(i18n_data[lang]):
            top = key.split('.')[0]
            i18n_by_section[top][lang].add(key)
    
    print("### Section overlap between i18n.json and ui/lang/\n")
    for section in SECTIONS_OF_INTEREST:
        i18n_en_keys = i18n_by_section.get(section, {}).get('en', set())
        lang_en_keys = section_lang_keys.get(section, {}).get('en', set())
        
        # Try to normalize keys for comparison
        # i18n.json keys: section.subkey...
        # lang keys: section.lang_file.subkey...  (but we stripped section prefix above)
        # Actually lang keys are like file.subkey, let's prepend section
        lang_en_keys_prefixed = {f"{section}.{k}" for k in lang_en_keys}
        
        overlap = i18n_en_keys & lang_en_keys_prefixed
        only_i18n = i18n_en_keys - lang_en_keys_prefixed
        only_lang = lang_en_keys_prefixed - i18n_en_keys
        
        print(f"\n**{section}**:")
        print(f"  - i18n.json en keys: {len(i18n_en_keys)}")
        print(f"  - ui/lang/ en keys: {len(lang_en_keys_prefixed)}")
        print(f"  - Overlap: {len(overlap)}")
        print(f"  - Only in i18n.json: {len(only_i18n)}")
        print(f"  - Only in ui/lang/: {len(only_lang)}")
        if only_i18n and len(only_i18n) <= 10:
            for k in sorted(only_i18n)[:10]:
                print(f"    - {k}")
        if only_lang and len(only_lang) <= 10:
            for k in sorted(only_lang)[:10]:
                print(f"    - {k}")

def find_untranslated(section_lang_keys):
    print("\n## Untranslated Strings Check\n")
    
    # Sample: check de, es, fr, ja, zh in a few sections
    sample_langs = ['de', 'es', 'fr', 'ja', 'zh']
    sample_sections = ['common', 'dashboard', 'chat']
    
    found_any = False
    for section in sample_sections:
        print(f"\n### {section}\n")
        en_data = load_lang_section_files(section, 'en')
        for lang in sample_langs:
            lang_data = load_lang_section_files(section, lang)
            untranslated = []
            for file_path, en_dict in en_data.items():
                lang_dict = lang_data.get(file_path, {})
                untranslated.extend(find_untranslated_in_dict(en_dict, lang_dict, file_path))
            if untranslated:
                found_any = True
                print(f"- `{lang}`: {len(untranslated)} potential untranslated strings")
                for item in untranslated[:10]:
                    print(f"  - `{item['file']}` → `{item['key']}`: `{item['value']}`")
            else:
                print(f"- `{lang}`: no obvious untranslated strings found")
    
    if not found_any:
        print("No obvious untranslated strings detected in sampled sections.")

def load_lang_section_files(section, lang):
    """Load all JSON files for a section+lang into a dict keyed by relative path."""
    result = {}
    section_path = LANG_DIR / section
    if not section_path.exists():
        return result
    for jf in section_path.rglob(f"{lang}.json"):
        rel = str(jf.relative_to(section_path))
        try:
            with open(jf, encoding='utf-8') as f:
                result[rel] = json.load(f)
        except Exception as e:
            print(f"ERROR reading {jf}: {e}")
    return result

def find_untranslated_in_dict(en_dict, lang_dict, file_path, prefix=''):
    """Recursively find values in lang_dict that match en_dict exactly."""
    untranslated = []
    if not isinstance(en_dict, dict):
        return untranslated
    for k, v in en_dict.items():
        lang_v = lang_dict.get(k) if isinstance(lang_dict, dict) else None
        key_path = f"{prefix}.{k}" if prefix else k
        if isinstance(v, dict) and isinstance(lang_v, dict):
            untranslated.extend(find_untranslated_in_dict(v, lang_v, file_path, key_path))
        elif isinstance(v, str) and isinstance(lang_v, str):
            # Heuristic: exact match with English, and value is not empty/special
            if v.strip() == lang_v.strip() and v.strip() and not v.startswith('http') and len(v) > 3:
                # Exclude common technical terms or placeholders
                if v not in ['OK', 'ID', 'URL', 'API', 'AI', 'LLM', 'SSH', 'CPU', 'RAM', 'GPU', 'SSD', 'HDD', 'NAS', 'VPN', 'DNS', 'DHCP', 'IP', 'TLS', 'SSL', 'HTTP', 'HTTPS', 'JSON', 'YAML', 'XML', 'HTML', 'CSS', 'JS', 'SQL', 'CSV', 'PDF', 'PNG', 'JPG', 'JPEG', 'GIF', 'SVG', 'MP3', 'MP4', 'WEBM', 'OGG', 'TrueNAS', 'AuraGo']:
                    untranslated.append({'file': file_path, 'key': key_path, 'value': v})
    return untranslated

def main():
    os.makedirs(REPORT_PATH.parent, exist_ok=True)
    
    i18n_data = load_i18n()
    lang_keys, missing_by_lang = analyze_i18n_json(i18n_data)
    section_lang_keys, section_lang_counts = analyze_lang_dir()
    compare_systems(i18n_data, section_lang_keys)
    find_untranslated(section_lang_keys)
    
    print("\n\nAnalysis complete. See report for details.")

if __name__ == '__main__':
    main()
