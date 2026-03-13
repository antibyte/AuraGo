#!/usr/bin/env python3
"""
Ubersetzungsprufung fur ui/lang
Vergleicht alle Sprachen gegen Deutsch (de) und Englisch (en) als Referenz
"""

import json
import os
from pathlib import Path
from collections import defaultdict

LANG_DIR = Path("ui/lang")
REFERENCE_LANGS = ["de", "en"]
ALL_LANGS = ["cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"]

def load_json(filepath):
    """Lade JSON-Datei"""
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            return json.load(f)
    except Exception as e:
        return {"__ERROR__": str(e)}

def get_all_keys(obj, prefix=""):
    """Rekursiv alle Schlussel extrahieren"""
    keys = set()
    if isinstance(obj, dict):
        for key, value in obj.items():
            full_key = f"{prefix}.{key}" if prefix else key
            keys.add(full_key)
            if isinstance(value, dict):
                keys.update(get_all_keys(value, full_key))
    return keys

def get_value_at_path(obj, path):
    """Wert anhand Pfad holen"""
    parts = path.split('.')
    current = obj
    for part in parts:
        if isinstance(current, dict) and part in current:
            current = current[part]
        else:
            return None
    return current

def is_truly_empty(value):
    """
    Prueft auf wirklich leere Werte:
    - None/null
    - Leerer String ""
    - String mit nur Leerzeichen "   "
    """
    if value is None:
        return True
    if isinstance(value, str):
        if value.strip() == "":
            return True
    return False

def check_translations():
    """Hauptpruffunktion"""
    
    all_files = list(LANG_DIR.rglob("*.json"))
    file_groups = defaultdict(dict)
    
    for filepath in all_files:
        if filepath.name == "meta.json":
            continue
        lang = filepath.stem
        rel_dir = filepath.parent.relative_to(LANG_DIR)
        group_key = str(rel_dir) if str(rel_dir) != "." else filepath.stem
        file_groups[group_key][lang] = filepath
    
    # Ergebnisse
    file_results = []  # (pfad, sprache, status, fehlende, leere, extra)
    summary = defaultdict(lambda: defaultdict(int))
    empty_keys_details = []  # Details zu leeren Werten
    
    for group_key, lang_files in sorted(file_groups.items()):
        ref_data = {}
        for ref_lang in REFERENCE_LANGS:
            if ref_lang in lang_files:
                ref_data[ref_lang] = load_json(lang_files[ref_lang])
        
        if not ref_data:
            continue
            
        primary_ref = ref_data.get("de", ref_data.get("en"))
        ref_keys = get_all_keys(primary_ref)
        
        for lang in ALL_LANGS:
            if lang in REFERENCE_LANGS:
                continue
                
            filepath = lang_files.get(lang)
            if not filepath:
                file_results.append((group_key, lang, "DATEI_FEHLT", 0, 0, 0))
                summary["missing_files"][lang] += 1
                continue
            
            trans_data = load_json(filepath)
            
            if "__ERROR__" in trans_data:
                file_results.append((group_key, lang, "JSON_FEHLER", 0, 0, 0))
                summary["json_errors"][lang] += 1
                continue
            
            trans_keys = get_all_keys(trans_data)
            
            missing_keys = ref_keys - trans_keys
            extra_keys = trans_keys - ref_keys
            
            empty_count = 0
            for key in ref_keys & trans_keys:
                value = get_value_at_path(trans_data, key)
                if is_truly_empty(value):
                    empty_count += 1
                    empty_keys_details.append((str(filepath), lang, key))
            
            file_results.append((group_key, lang, "OK", len(missing_keys), empty_count, len(extra_keys)))
            
            if missing_keys:
                summary["missing_keys"][lang] += len(missing_keys)
            if empty_count:
                summary["empty_values"][lang] += empty_count
            if extra_keys:
                summary["extra_keys"][lang] += len(extra_keys)
    
    # Bericht erstellen
    report = []
    report.append("=" * 80)
    report.append("UBERSETZUNGSPRUFUNG - ZUSAMMENFASSUNG")
    report.append("=" * 80)
    report.append(f"Referenz: Deutsch + Englisch")
    report.append(f"Geprueft: {', '.join([l for l in ALL_LANGS if l not in REFERENCE_LANGS])}")
    report.append("")
    
    # Probleme zaehlen
    total_missing_files = sum(summary["missing_files"].values())
    total_json_errors = sum(summary["json_errors"].values())
    total_missing_keys = sum(summary["missing_keys"].values())
    total_empty = sum(summary["empty_values"].values())
    total_extra = sum(summary["extra_keys"].values())
    
    report.append("-" * 80)
    report.append("GESAMTSTATISTIK:")
    report.append("-" * 80)
    report.append(f"  Fehlende Dateien:     {total_missing_files}")
    report.append(f"  JSON-Fehler:          {total_json_errors}")
    report.append(f"  Fehlende Schluessel:  {total_missing_keys}")
    report.append(f"  Leere Werte:          {total_empty}")
    report.append(f"  Zusaetzliche Keys:    {total_extra}")
    report.append("")
    
    # Pro Sprache
    report.append("-" * 80)
    report.append("PRO SPRACHE:")
    report.append("-" * 80)
    report.append(f"{'Sprache':<10} {'Fehl.Datei':>12} {'JSON-Fehler':>12} {'Fehl.Keys':>12} {'Leer':>12} {'Extra':>12}")
    report.append("-" * 80)
    
    for lang in sorted([l for l in ALL_LANGS if l not in REFERENCE_LANGS]):
        mf = summary["missing_files"].get(lang, 0)
        je = summary["json_errors"].get(lang, 0)
        mk = summary["missing_keys"].get(lang, 0)
        ev = summary["empty_values"].get(lang, 0)
        ek = summary["extra_keys"].get(lang, 0)
        report.append(f"{lang:<10} {mf:>12} {je:>12} {mk:>12} {ev:>12} {ek:>12}")
    
    report.append("")
    
    # Dateien mit Problemen
    report.append("-" * 80)
    report.append("DATEIEN MIT PROBLEMEN:")
    report.append("-" * 80)
    
    problem_files = []
    for group_key, lang, status, missing, empty, extra in file_results:
        if status == "DATEI_FEHLT":
            problem_files.append((group_key, lang, "DATEI_FEHLT", 0, 0, 0))
        elif status == "JSON_FEHLER":
            problem_files.append((group_key, lang, "JSON_FEHLER", 0, 0, 0))
        elif missing > 0 or empty > 0:
            problem_files.append((group_key, lang, "INHALT", missing, empty, extra))
    
    if problem_files:
        report.append(f"{'Datei':<50} {'Sprache':<8} {'Status':<15} {'Fehlt':>8} {'Leer':>8}")
        report.append("-" * 80)
        for group_key, lang, status, missing, empty, extra in sorted(problem_files):
            filename = f"{group_key}/{lang}.json"
            report.append(f"{filename:<50} {lang:<8} {status:<15} {missing:>8} {empty:>8}")
    else:
        report.append("Keine Probleme gefunden!")
    
    # Details zu leeren Werten (nur wenn es nicht zu viele sind)
    if total_empty > 0 and total_empty <= 100:
        report.append("")
        report.append("-" * 80)
        report.append("DETAILS: LEERE WERTE")
        report.append("-" * 80)
        for filepath, lang, key in sorted(empty_keys_details)[:50]:
            report.append(f"  {filepath}: {key}")
        if len(empty_keys_details) > 50:
            report.append(f"  ... und {len(empty_keys_details) - 50} weitere")
    
    report.append("")
    report.append("=" * 80)
    report.append("LEGENDE:")
    report.append("  DATEI_FEHLT  - Ubersetzungsdatei existiert nicht")
    report.append("  JSON_FEHLER  - Datei enthaelt ungueltiges JSON")
    report.append("  INHALT       - Fehlende Schluessel oder leere Werte")
    report.append("=" * 80)
    
    return "\n".join(report), total_empty == 0 and total_missing_keys == 0 and total_missing_files == 0 and total_json_errors == 0

if __name__ == "__main__":
    output, is_complete = check_translations()
    # In Datei schreiben
    with open("translation_report.txt", "w", encoding="utf-8") as f:
        f.write(output)
    # Und auf Konsole ausgeben
    print(output)
    print(f"\nBericht gespeichert in: translation_report.txt")
    if is_complete:
        print("\n[OK] Alle Ubersetzungen sind vollstandig!")
    else:
        print("\n[WARNUNG] Es wurden Probleme gefunden!")
