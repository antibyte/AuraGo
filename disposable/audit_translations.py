#!/usr/bin/env python3
"""
Vollständiger Audit der Übersetzungsdateien in ui/lang/
Vergleicht alle Sprachen gegen en.json als Referenz.
"""

import json
import os
from pathlib import Path
from collections import defaultdict

LANG_DIR = Path("ui/lang")
REPORT_PATH = Path("reports/translation_audit_report.md")
LANGUAGES = ["cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"]
NON_EN_LANGS = [l for l in LANGUAGES if l != "en"]

def flatten(obj, prefix=""):
    """Flattet ein verschachteltes JSON-Dict zu dot-notation Keys."""
    items = {}
    if isinstance(obj, dict):
        for k, v in obj.items():
            new_key = f"{prefix}.{k}" if prefix else k
            if isinstance(v, dict):
                items.update(flatten(v, new_key))
            else:
                items[new_key] = v
    return items

def collect_dirs():
    """Sammelt alle Verzeichnisse, die mindestens en.json enthalten."""
    dirs = []
    for root, _, files in os.walk(LANG_DIR):
        if "en.json" in files:
            dirs.append(Path(root))
    return sorted(dirs)

def analyze_dir(directory: Path):
    """Analysiert ein einzelnes Übersetzungsverzeichnis."""
    en_path = directory / "en.json"
    try:
        with open(en_path, "r", encoding="utf-8") as f:
            en_data = flatten(json.load(f))
    except Exception as e:
        return None, f"Fehler beim Lesen von {en_path}: {e}"

    results = {}
    for lang in NON_EN_LANGS:
        lang_path = directory / f"{lang}.json"
        if not lang_path.exists():
            results[lang] = {
                "exists": False,
                "missing_keys": list(en_data.keys()),
                "untranslated_keys": [],
                "extra_keys": [],
                "total_keys": 0,
                "coverage_pct": 0.0,
            }
            continue

        try:
            with open(lang_path, "r", encoding="utf-8") as f:
                lang_data = flatten(json.load(f))
        except Exception as e:
            results[lang] = {
                "exists": True,
                "error": str(e),
                "missing_keys": [],
                "untranslated_keys": [],
                "extra_keys": [],
                "total_keys": 0,
                "coverage_pct": 0.0,
            }
            continue

        en_keys = set(en_data.keys())
        lang_keys = set(lang_data.keys())

        missing = sorted(en_keys - lang_keys)
        extra = sorted(lang_keys - en_keys)

        # Unübersetzte = Wert identisch mit en.json (case-insensitive, strip)
        untranslated = []
        for k in en_keys & lang_keys:
            en_val = str(en_data[k]).strip()
            lang_val = str(lang_data[k]).strip()
            if en_val.lower() == lang_val.lower() and en_val:
                untranslated.append(k)

        total = len(en_keys)
        coverage = round(((total - len(missing)) / total) * 100, 1) if total else 100.0

        results[lang] = {
            "exists": True,
            "missing_keys": missing,
            "untranslated_keys": untranslated,
            "extra_keys": extra,
            "total_keys": len(lang_keys),
            "coverage_pct": coverage,
        }

    return en_data, results

def main():
    dirs = collect_dirs()
    report_lines = []
    report_lines.append("# Übersetzungs-Audit Report – AuraGo UI")
    report_lines.append("")
    report_lines.append(f"**Erstellt:** 2026-04-10")
    report_lines.append(f"**Referenzsprache:** English (en)")
    report_lines.append(f"**Geprüfte Sprachen:** {', '.join(NON_EN_LANGS)}")
    report_lines.append(f"**Geprüfte Verzeichnisse:** {len(dirs)}")
    report_lines.append("")

    summary_by_lang = defaultdict(lambda: {"missing": 0, "untranslated": 0, "files": 0, "total_files": 0})
    problematic_files = []

    for directory in dirs:
        rel_dir = directory.relative_to(LANG_DIR)
        en_data, results = analyze_dir(directory)
        if results is None:
            report_lines.append(f"## {rel_dir}")
            report_lines.append(f"_Fehler: {en_data}_")
            report_lines.append("")
            continue

        has_issues = False
        section_lines = []
        section_lines.append(f"## `{rel_dir}`")
        section_lines.append("")

        for lang in NON_EN_LANGS:
            r = results[lang]
            summary_by_lang[lang]["total_files"] += 1
            if not r["exists"]:
                summary_by_lang[lang]["missing"] += len(r["missing_keys"])
                summary_by_lang[lang]["files"] += 1
                has_issues = True
                section_lines.append(f"### {lang}")
                section_lines.append(f"- **Datei fehlt vollständig!** Alle {len(r['missing_keys'])} Keys fehlen.")
                section_lines.append("")
                continue

            if r.get("error"):
                summary_by_lang[lang]["files"] += 1
                has_issues = True
                section_lines.append(f"### {lang}")
                section_lines.append(f"- **Parse-Fehler:** {r['error']}")
                section_lines.append("")
                continue

            issues = []
            if r["missing_keys"]:
                issues.append(f"{len(r['missing_keys'])} fehlende Keys")
            if r["untranslated_keys"]:
                issues.append(f"{len(r['untranslated_keys'])} unübersetzte Keys")
            if r["extra_keys"]:
                issues.append(f"{len(r['extra_keys'])} überzählige Keys")

            summary_by_lang[lang]["missing"] += len(r["missing_keys"])
            summary_by_lang[lang]["untranslated"] += len(r["untranslated_keys"])

            if issues:
                summary_by_lang[lang]["files"] += 1
                has_issues = True
                section_lines.append(f"### {lang} ({r['coverage_pct']}% Abdeckung)")
                if r["missing_keys"]:
                    section_lines.append(f"**Fehlende Keys ({len(r['missing_keys'])}):**")
                    for k in r["missing_keys"]:
                        section_lines.append(f"- `{k}`")
                    section_lines.append("")
                if r["untranslated_keys"]:
                    section_lines.append(f"**Unübersetzte Keys ({len(r['untranslated_keys'])}):**")
                    for k in r["untranslated_keys"][:20]:  # Limit für Lesbarkeit
                        section_lines.append(f"- `{k}` = `{en_data.get(k, 'N/A')}`")
                    if len(r["untranslated_keys"]) > 20:
                        section_lines.append(f"- ... und {len(r['untranslated_keys']) - 20} weitere")
                    section_lines.append("")
                if r["extra_keys"]:
                    section_lines.append(f"**Überzählige Keys ({len(r['extra_keys'])}):**")
                    for k in r["extra_keys"]:
                        section_lines.append(f"- `{k}`")
                    section_lines.append("")

        if has_issues:
            problematic_files.append(str(rel_dir))
            report_lines.extend(section_lines)

    # Zusammenfassung einfügen (vor den Details)
    summary_lines = []
    summary_lines.append("## Zusammenfassung")
    summary_lines.append("")
    summary_lines.append("| Sprache | Fehlende Keys | Unübersetzte Keys | Betroffene Dateien | Abdeckung (geschätzt) |")
    summary_lines.append("|---------|--------------:|------------------:|-------------------:|-----------------------|")
    for lang in NON_EN_LANGS:
        s = summary_by_lang[lang]
        total = s["total_files"]
        files = s["files"]
        pct = f"{round((1 - files/total)*100, 1)}%" if total else "N/A"
        summary_lines.append(f"| `{lang}` | {s['missing']} | {s['untranslated']} | {files}/{total} | {pct} |")
    summary_lines.append("")
    summary_lines.append(f"**Dateien mit Problemen:** {len(problematic_files)}/{len(dirs)}")
    summary_lines.append("")

    final_report = report_lines[:5] + summary_lines + report_lines[5:]

    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    with open(REPORT_PATH, "w", encoding="utf-8") as f:
        f.write("\n".join(final_report))

    print(f"Bericht geschrieben nach: {REPORT_PATH}")
    print(f"Verzeichnisse geprüft: {len(dirs)}")
    print(f"Dateien mit Problemen: {len(problematic_files)}")

if __name__ == "__main__":
    main()
