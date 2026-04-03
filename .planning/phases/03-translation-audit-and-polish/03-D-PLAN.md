---
phase: 03-translation-audit-and-polish
plan: D
type: execute
wave: 1
depends_on: []
files_modified:
  - ui/lang/setup/de.json
  - ui/lang/setup/es.json
  - ui/lang/setup/fr.json
  - ui/lang/setup/zh.json
autonomous: true
requirements:
  - I18N-04

must_haves:
  truths:
    - "Placeholder lengths are consistent across languages"
    - "UI text does not overflow due to long translations"
    - "Tone and length are consistent across translations"
  artifacts:
    - path: "ui/lang/setup/de.json"
      provides: "German setup translations"
    - path: "ui/lang/setup/en.json"
      provides: "English reference"
  key_links:
    - from: "ui/setup.html"
      to: "ui/lang/setup/de.json"
      via: "data-i18n attributes"
---

<objective>
Verify translation consistency across all 15 languages - placeholder lengths, UI overflow issues, and tone/length consistency.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@ui/lang/setup/de.json
@ui/lang/setup/en.json
@ui/lang/setup/es.json
@ui/lang/setup/fr.json
@ui/lang/setup/zh.json
@ui/setup.html

Per D-30: All 15 languages should have consistent key structure.
Per D-31: Placeholder lengths should be similar across languages (no UI overflow from long translations).
</context>

<tasks>

<task type="auto">
  <name>Task 1: Check placeholder lengths across languages</name>
  <files>ui/lang/setup/de.json</files>
  <action>
Compare placeholder text lengths between English and German (and other languages) for similar keys.

Focus on:
- setup.step0_model_placeholder: "z.B. gpt-4o, claude-sonnet-4-20250514" vs English
- Any placeholders that might be much longer/shorter

Languages to compare: en, de, es, fr, zh (the most different in character length)

This detects cases where a translation is significantly longer than the English reference, which could cause UI overflow.
  </action>
  <verify>
<python_script>
import json
from pathlib import Path

lang_dir = Path("ui/lang/setup")
languages = ["en", "de", "es", "fr", "zh"]

def get_placeholders(lang):
    file_path = lang_dir / f"{lang}.json"
    if not file_path.exists():
        return {}
    with open(file_path, encoding="utf-8") as f:
        data = json.load(f)
    
    placeholders = {}
    for key, value in data.items():
        if isinstance(value, str) and "placeholder" in key.lower():
            placeholders[key] = len(value)
    return placeholders

print("Placeholder lengths comparison:")
print(f"{'Key':<50} {'EN':>6} {'DE':>6} {'ES':>6} {'FR':>6} {'ZH':>6}")
print("-" * 80)

# Get all placeholder keys
all_placeholders = set()
for lang in languages:
    all_placeholders.update(get_placeholders(lang).keys())

for key in sorted(all_placeholders):
    lengths = [get_placeholders(lang).get(key, 0) for lang in languages]
    if max(lengths) > 0:
        marker = " *" if max(lengths) / min(l for l in lengths if l > 0) > 1.5 else ""
        print(f"{key:<50} {lengths[0]:>6} {lengths[1]:>6} {lengths[2]:>6} {lengths[3]:>6} {lengths[4]:>6}{marker}")
</python_script>
  </verify>
  <done>Placeholder length report showing any outliers</done>
</task>

<task type="auto">
  <name>Task 2: Identify long translations that may cause UI overflow</name>
  <files>ui/lang/setup/de.json</files>
  <action>
Identify translations that are significantly longer than their English counterparts, as these may cause UI overflow issues.

This check is especially important for:
- Button labels (limited width)
- Form labels
- Modal text
- Toast messages

Flag any translations that are more than 30% longer than English, as these likely need shortening or UI adjustment.
  </action>
  <verify>
<python_script>
import json
from pathlib import Path

lang_dir = Path("ui/lang/setup")

with open(lang_dir / "en.json", encoding="utf-8") as f:
    en = json.load(f)

long_translations = []

for lang in ["de", "es", "fr", "zh", "ja", "nl", "pt", "pl", "it", "cs", "sv", "no", "da", "el", "hi"]:
    file_path = lang_dir / f"{lang}.json"
    if not file_path.exists():
        continue
    
    with open(file_path, encoding="utf-8") as f:
        data = json.load(f)
    
    for key, en_value in en.items():
        if key not in data:
            continue
        
        lang_value = data[key]
        if isinstance(en_value, str) and isinstance(lang_value, str):
            en_len = len(en_value)
            lang_len = len(lang_value)
            
            # Skip very short strings
            if en_len < 10:
                continue
            
            ratio = lang_len / en_len
            if ratio > 1.3:
                long_translations.append({
                    "lang": lang,
                    "key": key,
                    "en": en_value[:50] + "..." if len(en_value) > 50 else en_value,
                    "lang_val": lang_value[:50] + "..." if len(lang_value) > 50 else lang_value,
                    "ratio": round(ratio, 2)
                })

print(f"Found {len(long_translations)} potentially problematic translations:")
for t in sorted(long_translations, key=lambda x: -x["ratio"])[:15]:
    print(f"  {t['lang'].upper()}: {t['key']} (ratio: {t['ratio']})")
    print(f"    EN: {t['en']}")
    print(f"    {t['lang'].upper()}: {t['lang_val']}")
</python_script>
  </verify>
  <done>List of translations that may need shortening or UI adjustment</done>
</task>

<task type="auto">
  <name>Task 3: Verify tone and length consistency in setup translations</name>
  <files>ui/lang/setup/de.json</files>
  <action>
Spot-check the tone and length consistency of German translations against English.

German is chosen because:
1. It has the most known mixed-language issues (fixed in Plan B)
2. German words tend to be longer than English

Check:
1. Are button labels concise in German? (German tends to compound words)
2. Are there any informal/casual tone mismatches?
3. Do headings, body text, and buttons have appropriate length ratios?

This is a qualitative check - use Read tool to examine specific entries.
  </action>
  <verify>
<python_script>
import json

with open("ui/lang/setup/de.json", encoding="utf-8") as f:
    de = json.load(f)

with open("ui/lang/setup/en.json", encoding="utf-8") as f:
    en = json.load(f)

# Compare some key pairs
key_pairs = [
    ("setup.step0_title", "setup.step0_description"),
    ("setup.step1_title", "setup.step1_description"),
    ("setup.nav_next", "setup.nav_back"),
    ("setup.skip_button", "setup.skip_button_title"),
]

print("Tone/Length comparison (EN vs DE):")
print("-" * 60)
for en_key, de_key in key_pairs:
    en_val = en.get(en_key, "MISSING")
    de_val = de.get(de_key, "MISSING")
    print(f"{en_key}:")
    print(f"  EN: {en_val[:60]}...")
    print(f"  DE: {de_val[:60]}...")
</python_script>
  </verify>
  <done>Spot-check report on tone/length consistency</done>
</task>

</tasks>

<verification>
- [ ] Placeholder lengths compared across all major languages
- [ ] Long translations identified
- [ ] Tone/length spot-check completed
</verification>

<success_criteria>
- No translations are more than 30% longer than English (likely to cause overflow)
- Placeholder text is reasonably consistent in length
- Tone is appropriate for the context (formal for settings, clear for buttons)
</success_criteria>

<output>
After completion, create `.planning/phases/03-translation-audit-and-polish/03-D-SUMMARY.md`
</output>
