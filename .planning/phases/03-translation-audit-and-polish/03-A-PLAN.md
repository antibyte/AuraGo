---
phase: 03-translation-audit-and-polish
plan: A
type: execute
wave: 1
depends_on: []
files_modified:
  - ui/lang/setup/de.json
  - ui/lang/setup/en.json
autonomous: true
requirements:
  - I18N-01
  - I18N-04

must_haves:
  truths:
    - "Every translation key used in HTML files exists in all 15 language JSON files"
    - "No missing keys across any language file"
  artifacts:
    - path: "ui/lang/setup/de.json"
      provides: "German translations"
    - path: "ui/lang/setup/en.json"
      provides: "English reference translations"
  key_links:
    - from: "ui/setup.html"
      to: "ui/lang/setup/*.json"
      via: "data-i18n attributes"
    - from: "ui/config.html"
      to: "ui/lang/config/*.json"
      via: "I18N global"
---

<objective>
Build a complete audit of all translation keys across all 15 language files, identifying missing keys in each language.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@ui/lang/setup/de.json
@ui/lang/setup/en.json
@ui/setup.html
@ui/config.html

<!-- Key types from shared.js -->
interface I18N {
  [key: string]: string;
}
function t(k: string, p?: Record<string, string>): string;
</context>

<tasks>

<task type="auto">
  <name>Task 1: Extract all translation keys from English reference</name>
  <files>ui/lang/setup/en.json</files>
  <action>
Extract ALL unique translation keys from the English JSON files (setup and config sections).

Use this command to get all keys:
```bash
grep -h '"' ui/lang/setup/en.json ui/lang/config/en.json 2>/dev/null | sed 's/.*"\([^"]*\)":.*/\1/' | sort -u
```

The English file is the canonical reference - all other languages must have these keys.
  </action>
  <verify>
<python_script>
import json
import os
from pathlib import Path

lang_dir = Path("ui/lang")
en_keys = set()

# Extract from setup/en.json and config/en.json
for section in ["setup", "config"]:
    en_file = lang_dir / section / "en.json"
    if en_file.exists():
        with open(en_file) as f:
            data = json.load(f)
            en_keys.update(data.keys())

print(f"English reference keys: {len(en_keys)}")
for k in sorted(en_keys)[:20]:
    print(f"  {k}")
</python_script>
  </verify>
  <done>List of all English translation keys available</done>
</task>

<task type="auto">
  <name>Task 2: Compare all 15 language files for missing keys</name>
  <files>ui/lang/setup/de.json</files>
  <action>
Compare each of the 15 language files against the English reference to find missing keys.

Languages to check: cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh

For each language file, identify which keys from the English reference are MISSING.

Per D-20: Systematic key audit - compare all keys in HTML files against all 15 language JSON files.
  </action>
  <verify>
<python_script>
import json
from pathlib import Path

lang_dir = Path("ui/lang")
languages = ["cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"]

# Get reference keys from en.json
def get_keys(section):
    en_file = lang_dir / section / "en.json"
    if en_file.exists():
        with open(en_file) as f:
            return set(json.load(f).keys())
    return set()

setup_keys = get_keys("setup")
config_keys = get_keys("config")
all_en_keys = setup_keys | config_keys

print(f"Total English reference keys: {len(all_en_keys)}")
print(f"  Setup: {len(setup_keys)}, Config: {len(config_keys)}")
print()

for lang in languages:
    missing = []
    for section in ["setup", "config"]:
        lang_file = lang_dir / section / f"{lang}.json"
        if lang_file.exists():
            with open(lang_file) as f:
                data = json.load(f)
                lang_keys = set(data.keys())
                section_missing = all_en_keys - lang_keys
                if section_missing:
                    missing.append(f"{section}: {len(section_missing)} missing")
        else:
            missing.append(f"{section}: FILE MISSING")
    
    status = "OK" if not missing else f"MISSING: {', '.join(missing)}"
    print(f"{lang.upper()}: {status}")
</python_script>
  </verify>
  <done>Report of missing keys per language file generated</done>
</task>

<task type="auto">
  <name>Task 3: Identify data-i18n usage in HTML files</name>
  <files>ui/setup.html</files>
  <action>
Extract all unique data-i18n keys used in HTML files and verify they exist in the language files.

This ensures no keys are used in HTML that don't exist in the translation files.

Per D-22: Build complete key list from HTML data-i18n attributes and t() calls.
  </action>
  <verify>
<python_script>
import re
from pathlib import Path

html_files = list(Path("ui").glob("*.html"))
pattern = re.compile(r'data-i18n[_-]?(?:attr|placeholder|title|aria-label|ph)?=["\']([^"\']+)["\']')

all_keys = set()
for html_file in html_files:
    content = html_file.read_text(encoding='utf-8')
    matches = pattern.findall(content)
    all_keys.update(matches)

print(f"Unique data-i18n keys used in HTML: {len(all_keys)}")
for k in sorted(all_keys)[:30]:
    print(f"  {k}")
</python_script>
  </verify>
  <done>Complete list of data-i18n keys used in HTML files</done>
</task>

</tasks>

<verification>
- [ ] Python script ran successfully and produced key counts
- [ ] Missing keys identified for each language
- [ ] Report shows which languages need additions
</verification>

<success_criteria>
- Complete list of all English translation keys (reference)
- Per-language missing key report
- HTML data-i18n keys cross-referenced against language files
</success_criteria>

<output>
After completion, create `.planning/phases/03-translation-audit-and-polish/03-A-SUMMARY.md`
</output>
