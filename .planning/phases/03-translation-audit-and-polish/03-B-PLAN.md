---
phase: 03-translation-audit-and-polish
plan: B
type: execute
wave: 1
depends_on: []
files_modified:
  - ui/lang/setup/de.json
  - ui/lang/config/de.json
autonomous: true
requirements:
  - I18N-02
  - I18N-03

must_haves:
  truths:
    - "All German translation entries contain only German text (no English)"
    - "All key references point to the correct keys"
  artifacts:
    - path: "ui/lang/setup/de.json"
      provides: "German setup translations"
    - path: "ui/lang/config/de.json"
      provides: "German config translations"
  key_links:
    - from: "ui/setup.html"
      to: "ui/lang/setup/de.json"
      via: "data-i18n attributes"
---

<objective>
Fix mixed-language entries and incorrect key references in the German translation files (and check other affected languages).
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@ui/lang/setup/de.json
@ui/lang/setup/en.json
@ui/lang/config/de.json

<!-- Known issues from research: -->
<!-- setup.header_subtitle: "Quick Setup" (English in German) -->
<!-- setup.step0_language_label: "Language / Sprache" (mixed German/English) -->
<!-- setup.language_custom references wrong key -->
</context>

<tasks>

<task type="auto">
  <name>Task 1: Fix German mixed-language in setup/de.json</name>
  <files>ui/lang/setup/de.json</files>
  <action>
Fix the following known mixed-language entries in ui/lang/setup/de.json:

1. `setup.header_subtitle`: Change "Quick Setup" to "Schnellkonfiguration" (proper German)
2. `setup.step0_language_label`: Change "Language / Sprache" to "Sprache" (pure German)
3. Check all other values for any remaining English strings embedded in German

Per D-23: All language files should contain only the target language - no English strings in German files.
Per D-21: German first - fix the most-used language first.

Use Read tool first to see the full file, then Edit tool to fix specific entries.
  </action>
  <verify>
<python_script>
import json

with open("ui/lang/setup/de.json", encoding="utf-8") as f:
    de = json.load(f)

issues = []

# Check known issues
if de.get("setup.header_subtitle") == "Quick Setup":
    issues.append("setup.header_subtitle is still English: 'Quick Setup'")
if "Language / Sprache" in de.get("setup.step0_language_label", ""):
    issues.append("setup.step0_language_label is mixed: 'Language / Sprache'")

# Check for common English words that should be German
english_words = ["Quick", "Setup", "Next", "Back", "Skip", "Save", "Language"]
for key, value in de.items():
    if isinstance(value, str):
        for word in english_words:
            if word in value and not value.startswith(word):
                issues.append(f"{key} contains English word '{word}': '{value}'")

if issues:
    print("ISSUES FOUND:")
    for i in issues:
        print(f"  - {i}")
else:
    print("OK: No mixed-language entries found")
</python_script>
  </verify>
  <done>
- setup.header_subtitle = "Schnellkonfiguration"
- setup.step0_language_label = "Sprache"
- No other mixed-language entries in setup/de.json
  </done>
</task>

<task type="auto">
  <name>Task 2: Fix setup.language_custom key reference</name>
  <files>ui/lang/setup/de.json</files>
  <action>
Check and fix incorrect key references in ui/lang/setup/de.json.

Per D-25: Fix setup.language_custom key reference (was pointing to wrong key).

In German, the key `setup.language_custom` should reference the correct translation key for custom language option.

Also verify:
- `setup.step0_provider_custom` value is correct German
- Any other keys that might reference wrong values
  </action>
  <verify>
<python_script>
import json

with open("ui/lang/setup/de.json", encoding="utf-8") as f:
    de = json.load(f)

# Check provider values
providers = [
    "setup.step0_provider_anthropic",
    "setup.step0_provider_google", 
    "setup.step0_provider_custom",
    "setup.step0_provider_ollama",
    "setup.step0_provider_openai",
    "setup.step0_provider_openrouter"
]

for key in providers:
    value = de.get(key, "MISSING")
    print(f"{key}: {value}")
</python_script>
  </verify>
  <done>All provider keys have correct German translations</done>
</task>

<task type="auto">
  <name>Task 3: Check other languages for similar issues</name>
  <files>ui/lang/setup/es.json</files>
  <action>
After fixing German, check other languages for similar mixed-language issues.

Focus on:
- Spanish (es) - common mixed-language issues
- French (fr)
- Dutch (nl)
- Any other languages that might have English embedded

Per D-24: Create script to detect mixed-language entries (string contains English words in non-English context).
  </action>
  <verify>
<python_script>
import json
from pathlib import Path

lang_dir = Path("ui/lang/setup")
languages = ["cs", "da", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"]

issues_found = []

for lang in languages:
    file_path = lang_dir / f"{lang}.json"
    if not file_path.exists():
        print(f"{lang.upper()}: FILE MISSING")
        continue
    
    with open(file_path, encoding="utf-8") as f:
        data = json.load(f)
    
    # Check for common English words in values
    english_words = ["Quick", "Setup", "Next", "Back", "Skip", "Save", "Language", "Apply"]
    lang_issues = []
    
    for key, value in data.items():
        if isinstance(value, str):
            for word in english_words:
                # Check if English word appears at start or surrounded by non-letter chars
                if f" {word} " in f" {value} " or value.startswith(word + " ") or f" {word}," in f" {value}":
                    lang_issues.append(f"{key}='{value}'")
    
    if lang_issues:
        issues_found.append(f"{lang.upper()}: {len(lang_issues)} issues")
        for i in lang_issues[:3]:
            issues_found.append(f"  - {i}")

if issues_found:
    print("ISSUES FOUND:")
    for i in issues_found:
        print(i)
else:
    print("OK: No obvious mixed-language entries found")
</python_script>
  </verify>
  <done>Report of mixed-language issues in other language files</done>
</task>

</tasks>

<verification>
- [ ] German setup file has no English strings
- [ ] All key references are correct
- [ ] Other languages checked for similar issues
</verification>

<success_criteria>
- setup/de.json contains only German text
- No mixed-language entries like "Language / Sprache"
- setup.language_custom references correct key
</success_criteria>

<output>
After completion, create `.planning/phases/03-translation-audit-and-polish/03-B-SUMMARY.md`
</output>
