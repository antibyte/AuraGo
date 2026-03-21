#!/usr/bin/env python3
"""
AuraGo UI Translation Automation

Translates all UI text from German (de.json) master files into all supported
languages using Google Translate via the deep-translator library (free, no API key).

Usage:
    python scripts/translate_ui.py                  # Smart mode (only suspicious/missing)
    python scripts/translate_ui.py --force           # Re-translate everything
    python scripts/translate_ui.py --dry-run         # Analyze only, no writes
    python scripts/translate_ui.py --validate-only   # Validate existing translations
    python scripts/translate_ui.py --lang fr         # Only one language
    python scripts/translate_ui.py --dir missions    # Only one directory
    python scripts/translate_ui.py --verbose         # Detailed output
"""

import argparse
import json
import os
import re
import sys
import time
from collections import OrderedDict
from datetime import datetime, timezone

try:
    from deep_translator import GoogleTranslator
except ImportError:
    print("ERROR: deep-translator not installed. Run: pip install deep-translator")
    sys.exit(1)

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

LANG_DIR = os.path.join("ui", "lang")
SOURCE_LANG = "de"
FALLBACK_LANG = "en"

ALL_LANGS = ["cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"]
TARGET_LANGS = [l for l in ALL_LANGS if l not in (SOURCE_LANG, FALLBACK_LANG)]

# Map our lang codes to Google Translate codes
GOOGLE_LANG_MAP = {
    "cs": "cs", "da": "da", "el": "el", "es": "es", "fr": "fr",
    "hi": "hi", "it": "it", "ja": "ja", "nl": "nl", "no": "no",
    "pl": "pl", "pt": "pt", "sv": "sv", "zh": "zh-CN",
    "en": "en",
}

# Words/names that should never be translated
KEEP_WORDS = {
    "AuraGo", "Fritz!Box", "FritzBox", "Docker", "Proxmox", "Home Assistant",
    "Telegram", "Discord", "MQTT", "SSH", "API", "SSL", "TLS", "HTTPS", "HTTP",
    "cron", "webhook", "WebDAV", "SSE", "REST", "JSON", "YAML", "SQL", "SQLite",
    "MeshCentral", "Tailscale", "Let's Encrypt", "Ansible", "Rocket.Chat",
    "NAS", "NAS-Server", "CPU", "RAM", "GPU", "IP", "DNS", "URL", "URI",
    "Python", "Go", "JavaScript",
}

BATCH_SIZE = 50
SLEEP_BETWEEN_BATCHES = 1.0
SLEEP_ON_RATE_LIMIT = 30.0
MAX_RETRIES = 3

# ---------------------------------------------------------------------------
# Emoji & Placeholder Handling
# ---------------------------------------------------------------------------

# Matches common emoji sequences at the start of a string
EMOJI_RE = re.compile(
    r"^(["
    r"\U0001F300-\U0001FAFF"  # Misc symbols, emoticons, etc.
    r"\u2600-\u27BF"          # Misc symbols
    r"\u2B50\u2B55"           # Stars, circles
    r"\u23F0-\u23FA"          # Symbols
    r"\u2700-\u27BF"          # Dingbats
    r"\uFE00-\uFE0F"         # Variation selectors
    r"\u200D"                 # Zero-width joiner
    r"\u20E3"                 # Enclosing keycap
    r"\U0001F1E0-\U0001F1FF" # Flags
    r"]+\s*)"
)

# Matches placeholders like {0}, {name}, %s, %d and HTML tags
PLACEHOLDER_RE = re.compile(r"\{[^}]*\}|%[sd]|<[^>]+>")


def extract_special(text):
    """Extract leading emojis and placeholders from text before translation."""
    prefix_emoji = ""
    match = EMOJI_RE.match(text)
    if match:
        prefix_emoji = match.group(1)
        text = text[len(prefix_emoji):]

    placeholders = []
    counter = [0]

    def save_ph(m):
        placeholders.append(m.group(0))
        counter[0] += 1
        return f" XLPH{counter[0]}X "

    text = PLACEHOLDER_RE.sub(save_ph, text)
    return text, {"emoji": prefix_emoji, "placeholders": placeholders}


def restore_special(text, metadata):
    """Restore emojis and placeholders after translation."""
    for i, ph in enumerate(metadata["placeholders"], 1):
        # Google Translate sometimes changes spacing/case around markers
        patterns = [
            f"XLPH{i}X",
            f"xlph{i}x",
            f"Xlph{i}x",
            f"XLPH {i} X",
            f"xlph {i} x",
            f"Xlph {i} X",
            f"XLPH{i} X",
            f"XLPH {i}X",
        ]
        for pat in patterns:
            if pat in text:
                text = text.replace(pat, ph, 1)
                break
        else:
            # Fallback: regex for any mangled form
            mangled_re = re.compile(rf"[Xx][Ll][Pp][Hh]\s*{i}\s*[Xx]", re.IGNORECASE)
            text = mangled_re.sub(ph, text, count=1)

    # Clean up extra spaces around restored placeholders
    text = re.sub(r"  +", " ", text).strip()
    return metadata["emoji"] + text


def protect_keep_words(text):
    """Replace keep-words with markers to prevent translation."""
    replacements = {}
    for i, word in enumerate(sorted(KEEP_WORDS, key=len, reverse=True)):
        marker = f"XKWP{i}X"
        if word in text:
            text = text.replace(word, marker)
            replacements[marker] = word
    return text, replacements


def restore_keep_words(text, replacements):
    """Restore protected words after translation."""
    for marker, word in replacements.items():
        # Handle potential spacing/case changes
        patterns = [marker, marker.lower(), marker.capitalize()]
        for pat in patterns:
            if pat in text:
                text = text.replace(pat, word)
                break
        else:
            text = re.sub(re.escape(marker), word, text, flags=re.IGNORECASE)
    return text


# ---------------------------------------------------------------------------
# Translation Engine
# ---------------------------------------------------------------------------

def translate_batch_safe(texts, target_lang, verbose=False):
    """Translate a list of texts from German to target language with safety measures."""
    if not texts:
        return []

    google_lang = GOOGLE_LANG_MAP.get(target_lang, target_lang)
    translator = GoogleTranslator(source="de", target=google_lang)
    results = []

    for i in range(0, len(texts), BATCH_SIZE):
        batch = texts[i:i + BATCH_SIZE]

        # Pre-process: extract emojis, placeholders, protect words
        processed = []
        metadata_list = []
        kw_replacements_list = []
        skip_indices = set()  # Indices where no real text remains to translate

        for idx, text in enumerate(batch):
            cleaned, meta = extract_special(text)
            cleaned, kw_reps = protect_keep_words(cleaned)

            # Check if there's any real text left to translate
            test_text = cleaned
            for marker in kw_reps:
                test_text = test_text.replace(marker, "")
            test_text = re.sub(r"XLPH\d+X", "", test_text).strip(" /-:.,()[]|&•")

            if len(test_text) <= 1:
                # Nothing meaningful to translate — keep original
                skip_indices.add(idx)

            processed.append(cleaned)
            metadata_list.append(meta)
            kw_replacements_list.append(kw_reps)

        # Build list of texts that actually need translation
        to_translate = [processed[idx] for idx in range(len(batch)) if idx not in skip_indices]

        if to_translate:
            # Translate with retry
            translated_batch = None
            for attempt in range(MAX_RETRIES):
                try:
                    translated_batch = translator.translate_batch(to_translate)
                    break
                except Exception as e:
                    err_str = str(e).lower()
                    if "429" in err_str or "rate" in err_str or "too many" in err_str:
                        if verbose:
                            print(f"  Rate limited, waiting {SLEEP_ON_RATE_LIMIT}s...")
                        time.sleep(SLEEP_ON_RATE_LIMIT)
                    else:
                        wait = 2 ** attempt
                        if verbose:
                            print(f"  Error: {e}, retrying in {wait}s...")
                        time.sleep(wait)

            if translated_batch is None:
                print(f"  WARNING: Translation failed for batch {i}-{i+len(batch)}, keeping originals")
                results.extend(batch)
                continue
        else:
            translated_batch = []

        # Merge translated results back with skipped items
        trans_iter = iter(translated_batch)
        for idx in range(len(batch)):
            if idx in skip_indices:
                # Restore from original (just put back emoji/placeholders/keep-words)
                restored = restore_keep_words(processed[idx], kw_replacements_list[idx])
                restored = restore_special(restored, metadata_list[idx])
                results.append(restored)
            else:
                trans = next(trans_iter, None)
                if trans is None:
                    trans = batch[idx]
                trans = restore_keep_words(trans, kw_replacements_list[idx])
                trans = restore_special(trans, metadata_list[idx])
                results.append(trans)

        if i + BATCH_SIZE < len(texts):
            time.sleep(SLEEP_BETWEEN_BATCHES)

    return results


# ---------------------------------------------------------------------------
# Analysis
# ---------------------------------------------------------------------------

def is_universal_value(value):
    """Check if a value is inherently language-independent (format strings, abbreviations, tech terms)."""
    stripped = value.strip()
    # Pure format strings: "${{cost}} / ${{limit}}", "{{count}} tok", etc.
    without_placeholders = PLACEHOLDER_RE.sub("", stripped)
    without_placeholders = re.sub(r"\$?\{\{[^}]*\}\}", "", without_placeholders)
    remaining = without_placeholders.strip(" /$-:|•()[]0123456789.,")
    if len(remaining) <= 3:
        return True
    # Very short values (1-2 chars) like "OK", "→", etc.
    if len(stripped) <= 2:
        return True
    # Check if value is entirely composed of keep-words, punctuation, and spaces
    test = stripped
    for word in sorted(KEEP_WORDS, key=len, reverse=True):
        test = test.replace(word, "")
    test = test.strip(" /-:.,()[]|&•")
    if len(test) <= 2:
        return True
    return False


def should_retranslate(value, de_value, en_value, lang):
    """Determine if a value needs retranslation."""
    if not value or not value.strip():
        return True, "empty"
    # Skip universal values that are legitimately the same across languages
    if is_universal_value(de_value):
        return False, "ok"
    if value == de_value and lang != SOURCE_LANG:
        return True, "same_as_de"
    if value == en_value and lang != FALLBACK_LANG:
        return True, "same_as_en"
    return False, "ok"


def check_placeholders(original, translated):
    """Check that placeholders are preserved."""
    orig_ph = set(PLACEHOLDER_RE.findall(original))
    trans_ph = set(PLACEHOLDER_RE.findall(translated))
    return orig_ph == trans_ph


# ---------------------------------------------------------------------------
# File I/O
# ---------------------------------------------------------------------------

def load_json(filepath):
    """Load a JSON file."""
    if not os.path.exists(filepath):
        return {}
    with open(filepath, "r", encoding="utf-8") as f:
        return json.load(f)


def save_json(filepath, data):
    """Save data as sorted JSON with consistent formatting."""
    ordered = OrderedDict(sorted(data.items()))
    content = json.dumps(ordered, ensure_ascii=False, indent=2)
    # Ensure file ends with newline
    if not content.endswith("\n"):
        content += "\n"
    with open(filepath, "w", encoding="utf-8", newline="\n") as f:
        f.write(content)


# ---------------------------------------------------------------------------
# Main Processing
# ---------------------------------------------------------------------------

def get_directories():
    """Get all translation subdirectories."""
    if not os.path.isdir(LANG_DIR):
        print(f"ERROR: {LANG_DIR} not found. Run from project root.")
        sys.exit(1)
    return sorted([
        d for d in os.listdir(LANG_DIR)
        if os.path.isdir(os.path.join(LANG_DIR, d))
    ])


def process_directory(dir_name, langs, force=False, dry_run=False, verbose=False):
    """Process one translation directory. Returns stats dict."""
    de_path = os.path.join(LANG_DIR, dir_name, f"{SOURCE_LANG}.json")
    en_path = os.path.join(LANG_DIR, dir_name, f"{FALLBACK_LANG}.json")

    de = load_json(de_path)
    en = load_json(en_path)

    if not de:
        if verbose:
            print(f"  [{dir_name}] Skipping: no de.json")
        return {}

    stats = {}

    for lang in langs:
        lang_path = os.path.join(LANG_DIR, dir_name, f"{lang}.json")
        existing = load_json(lang_path)

        to_translate_keys = []
        to_translate_values = []
        reasons = {}

        for key, de_value in de.items():
            en_value = en.get(key, "")
            current = existing.get(key, "")

            if force:
                to_translate_keys.append(key)
                to_translate_values.append(de_value)
                reasons[key] = "force"
            else:
                needs_it, reason = should_retranslate(current, de_value, en_value, lang)
                if needs_it:
                    to_translate_keys.append(key)
                    to_translate_values.append(de_value)
                    reasons[key] = reason

        # Also add any keys missing from the target file
        for key in de:
            if key not in existing and key not in to_translate_keys:
                to_translate_keys.append(key)
                to_translate_values.append(de[key])
                reasons[key] = "missing"

        changed = len(to_translate_keys)
        kept = len(de) - changed

        stats[lang] = {
            "changed": changed,
            "kept": kept,
            "total": len(de),
            "reasons": dict(sorted(
                {r: list(reasons.values()).count(r) for r in set(reasons.values())}.items()
            )) if reasons else {},
        }

        if changed > 0 and not dry_run:
            if verbose:
                print(f"  [{dir_name}/{lang}] Translating {changed} keys...")
            translated = translate_batch_safe(to_translate_values, lang, verbose)

            for key, trans in zip(to_translate_keys, translated):
                existing[key] = trans

            # Ensure all DE keys are present
            for key in de:
                if key not in existing:
                    existing[key] = de[key]

            save_json(lang_path, existing)

        elif changed > 0 and dry_run:
            if verbose:
                for key in to_translate_keys[:5]:
                    print(f"    {key}: [{reasons[key]}] \"{existing.get(key, '')}\"")
                if changed > 5:
                    print(f"    ... and {changed - 5} more")

    return stats


def validate_directory(dir_name, langs, verbose=False):
    """Validate translations in a directory. Returns issues list."""
    de_path = os.path.join(LANG_DIR, dir_name, f"{SOURCE_LANG}.json")
    en_path = os.path.join(LANG_DIR, dir_name, f"{FALLBACK_LANG}.json")

    de = load_json(de_path)
    en = load_json(en_path)
    issues = []

    for lang in langs:
        lang_path = os.path.join(LANG_DIR, dir_name, f"{lang}.json")
        if not os.path.exists(lang_path):
            issues.append({"dir": dir_name, "lang": lang, "type": "file_missing"})
            continue

        data = load_json(lang_path)

        # Check missing keys
        for key in de:
            if key not in data:
                issues.append({"dir": dir_name, "lang": lang, "key": key, "type": "missing_key"})

        # Check empty values
        for key, val in data.items():
            if not val or not str(val).strip():
                issues.append({"dir": dir_name, "lang": lang, "key": key, "type": "empty_value"})

        # Check same-as-DE
        same_de = sum(1 for k in data if k in de and data[k] == de[k])
        if same_de > 5:
            issues.append({
                "dir": dir_name, "lang": lang, "type": "many_same_as_de",
                "count": same_de, "total": len(data)
            })

        # Check same-as-EN
        same_en = sum(1 for k in data if k in en and data[k] == en[k])
        if same_en > len(data) * 0.3:
            issues.append({
                "dir": dir_name, "lang": lang, "type": "many_same_as_en",
                "count": same_en, "total": len(data)
            })

        # Check placeholder consistency
        for key in data:
            if key in de:
                if not check_placeholders(de[key], data[key]):
                    issues.append({
                        "dir": dir_name, "lang": lang, "key": key,
                        "type": "placeholder_mismatch"
                    })

    return issues


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(description="AuraGo UI Translation Automation")
    parser.add_argument("--force", action="store_true", help="Re-translate everything (except DE)")
    parser.add_argument("--dry-run", action="store_true", help="Analyze only, don't write files")
    parser.add_argument("--validate-only", action="store_true", help="Only validate existing translations")
    parser.add_argument("--lang", type=str, help="Process only this language (e.g. 'fr')")
    parser.add_argument("--dir", type=str, help="Process only this directory (e.g. 'missions')")
    parser.add_argument("--include-en", action="store_true", help="Also re-translate English (with --force)")
    parser.add_argument("--verbose", "-v", action="store_true", help="Verbose output")
    args = parser.parse_args()

    print("=== AuraGo Translation Automation ===")
    print(f"Source: {SOURCE_LANG}.json (German)")

    directories = get_directories()
    if args.dir:
        if args.dir not in directories:
            print(f"ERROR: Directory '{args.dir}' not found in {LANG_DIR}/")
            sys.exit(1)
        directories = [args.dir]

    langs = TARGET_LANGS[:]
    if args.include_en:
        langs.append(FALLBACK_LANG)
    if args.lang:
        if args.lang not in ALL_LANGS:
            print(f"ERROR: Unknown language '{args.lang}'")
            sys.exit(1)
        if args.lang in (SOURCE_LANG,):
            print(f"ERROR: Cannot translate source language '{SOURCE_LANG}'")
            sys.exit(1)
        langs = [args.lang]

    print(f"Target: {len(langs)} language(s): {', '.join(langs)}")
    print(f"Dirs:   {len(directories)} ({', '.join(directories)})")

    if args.validate_only:
        print(f"Mode:   Validate only")
    elif args.dry_run:
        print(f"Mode:   Dry run (no writes)")
    elif args.force:
        print(f"Mode:   Force (re-translate all)")
    else:
        print(f"Mode:   Smart (only suspicious/missing)")
    print()

    # --- Validate Only ---
    if args.validate_only:
        all_issues = []
        for d in directories:
            issues = validate_directory(d, langs, args.verbose)
            all_issues.extend(issues)

        if all_issues:
            print(f"Found {len(all_issues)} issue(s):\n")
            by_type = {}
            for issue in all_issues:
                t = issue["type"]
                by_type.setdefault(t, []).append(issue)

            for itype, items in sorted(by_type.items()):
                print(f"  {itype}: {len(items)}")
                if args.verbose:
                    for item in items[:10]:
                        desc = f"    {item['dir']}/{item.get('lang', '?')}"
                        if 'key' in item:
                            desc += f" key={item['key']}"
                        if 'count' in item:
                            desc += f" ({item['count']}/{item['total']})"
                        print(desc)
                    if len(items) > 10:
                        print(f"    ... and {len(items) - 10} more")
                print()
        else:
            print("All translations look good!")
        return

    # --- Translate ---
    all_stats = {}
    total_changed = 0
    total_kept = 0
    start_time = time.time()

    for d in directories:
        stats = process_directory(d, langs, force=args.force, dry_run=args.dry_run, verbose=args.verbose)
        all_stats[d] = stats

        # Print summary line per directory
        parts = []
        for lang in langs:
            if lang in stats:
                c = stats[lang]["changed"]
                total_changed += c
                total_kept += stats[lang]["kept"]
                if c > 0:
                    parts.append(f"{lang}:{c}")
        if parts:
            print(f"  [{d:15s}] {', '.join(parts)}")
        elif args.verbose:
            print(f"  [{d:15s}] all OK")

    elapsed = time.time() - start_time
    print()
    print("=== Summary ===")
    print(f"  {total_changed:,} translations {'would be ' if args.dry_run else ''}changed")
    print(f"  {total_kept:,} translations kept")
    print(f"  Time: {elapsed:.1f}s")

    if args.dry_run:
        print("\n  (Dry run — no files were modified)")

    # --- Save Report ---
    report = {
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "mode": "force" if args.force else ("dry_run" if args.dry_run else "smart"),
        "stats": {
            "translations_changed": total_changed,
            "translations_kept": total_kept,
            "languages": len(langs),
            "directories": len(directories),
        },
        "details": {},
    }
    for d, dstats in all_stats.items():
        report["details"][d] = {}
        for lang, lstats in dstats.items():
            report["details"][d][lang] = {
                "changed": lstats["changed"],
                "kept": lstats["kept"],
                "reasons": lstats.get("reasons", {}),
            }

    report_path = os.path.join("scripts", "translation_report.json")
    with open(report_path, "w", encoding="utf-8") as f:
        json.dump(report, f, ensure_ascii=False, indent=2)
    print(f"\n  Report saved: {report_path}")

    # --- Post-validation ---
    if not args.dry_run and total_changed > 0:
        print("\n  Running post-validation...")
        all_issues = []
        for d in directories:
            issues = validate_directory(d, langs)
            all_issues.extend(issues)

        ph_issues = [i for i in all_issues if i["type"] == "placeholder_mismatch"]
        empty_issues = [i for i in all_issues if i["type"] == "empty_value"]
        missing_issues = [i for i in all_issues if i["type"] == "missing_key"]

        if ph_issues:
            print(f"  ⚠ {len(ph_issues)} placeholder mismatches")
        if empty_issues:
            print(f"  ⚠ {len(empty_issues)} empty values")
        if missing_issues:
            print(f"  ⚠ {len(missing_issues)} missing keys")
        if not ph_issues and not empty_issues and not missing_issues:
            print("  ✓ All translations validated OK")


if __name__ == "__main__":
    main()
