#!/usr/bin/env python3
"""
Finale Uebersetzungspruefung fuer ui/lang
Vergleicht alle Sprachen gegen Deutsch (de) und Englisch (en) als Referenz
"""

import json
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

def count_null_values(data):
    """Zaehlt null/None und leere String-Werte"""
    count = 0
    for key, value in data.items():
        if value is None:
            count += 1
        elif isinstance(value, str) and value.strip() == "":
            count += 1
    return count

def check_translations():
    """Hauptprueffunktion"""
    
    # Finde alle JSON-Dateien
    all_files = list(LANG_DIR.rglob("*.json"))
    
    # Gruppiere nach Dateipfad (ohne Sprachcode)
    file_groups = defaultdict(dict)
    
    for filepath in all_files:
        if filepath.name == "meta.json":
            continue
            
        lang = filepath.stem
        rel_dir = filepath.parent.relative_to(LANG_DIR)
        group_key = str(rel_dir) if str(rel_dir) != "." else filepath.stem
        
        file_groups[group_key][lang] = filepath
    
    # Ergebnisse sammeln
    file_results = []
    summary = defaultdict(lambda: defaultdict(int))
    
    for group_key, lang_files in sorted(file_groups.items()):
        # Lade Referenzdateien (de und en)
        ref_data = {}
        for ref_lang in REFERENCE_LANGS:
            if ref_lang in lang_files:
                ref_data[ref_lang] = load_json(lang_files[ref_lang])
        
        if not ref_data:
            continue
            
        # Verwende Deutsch als primäre Referenz
        primary_ref = ref_data.get("de", ref_data.get("en"))
        ref_keys = set(primary_ref.keys())
        
        # Prüfe jede Sprache
        for lang in ALL_LANGS:
            if lang in REFERENCE_LANGS:
                continue
                
            if lang not in lang_files:
                file_results.append((group_key, lang, "DATEI_FEHLT", 0, 0, 0))
                summary["missing_files"][lang] += 1
                continue
            
            # Lade Übersetzung
            trans_data = load_json(lang_files[lang])
            
            if "__ERROR__" in trans_data:
                file_results.append((group_key, lang, "JSON_FEHLER", 0, 0, 0))
                summary["json_errors"][lang] += 1
                continue
            
            trans_keys = set(trans_data.keys())
            
            # Fehlende Schlüssel (in Referenz aber nicht in Übersetzung)
            missing_keys = ref_keys - trans_keys
            
            # Zusätzliche Schlüssel (in Übersetzung aber nicht in Referenz)
            extra_keys = trans_keys - ref_keys
            
            # Null/leere Werte zählen
            null_count = count_null_values(trans_data)
            
            file_results.append((group_key, lang, "OK", len(missing_keys), null_count, len(extra_keys)))
            
            if missing_keys:
                summary["missing_keys"][lang] += len(missing_keys)
            if null_count:
                summary["null_values"][lang] += null_count
            if extra_keys:
                summary["extra_keys"][lang] += len(extra_keys)
    
    # Bericht erstellen
    report = []
    report.append("=" * 80)
    report.append("UEBERSETZUNGSANALYSE - ui/lang")
    report.append("=" * 80)
    report.append("")
    report.append("Referenzsprachen: Deutsch (de) + Englisch (en)")
    report.append("Gepruefte Sprachen: " + ", ".join([l for l in ALL_LANGS if l not in REFERENCE_LANGS]))
    report.append("")
    
    # Zusammenfassung
    total_missing_files = sum(summary["missing_files"].values())
    total_json_errors = sum(summary["json_errors"].values())
    total_missing_keys = sum(summary["missing_keys"].values())
    total_null = sum(summary["null_values"].values())
    total_extra = sum(summary["extra_keys"].values())
    
    report.append("-" * 80)
    report.append("GESAMTSTATISTIK:")
    report.append("-" * 80)
    report.append(f"  Fehlende Dateien:     {total_missing_files}")
    report.append(f"  JSON-Fehler:          {total_json_errors}")
    report.append(f"  Fehlende Schluessel:  {total_missing_keys}")
    report.append(f"  Null/Leere Werte:     {total_null}")
    report.append(f"  Zusaetzliche Keys:    {total_extra}")
    report.append("")
    
    # Pro Sprache
    report.append("-" * 80)
    report.append("PRO SPRACHE:")
    report.append("-" * 80)
    report.append(f"{'Sprache':<10} {'Dateien':>10} {'Fehl.Keys':>12} {'Null/Leer':>12} {'Extra':>12}")
    report.append("-" * 80)
    
    for lang in sorted([l for l in ALL_LANGS if l not in REFERENCE_LANGS]):
        files_with_issues = sum(1 for g, l, s, m, n, e in file_results if l == lang and (m > 0 or n > 0))
        mk = summary["missing_keys"].get(lang, 0)
        nv = summary["null_values"].get(lang, 0)
        ek = summary["extra_keys"].get(lang, 0)
        report.append(f"{lang:<10} {files_with_issues:>10} {mk:>12} {nv:>12} {ek:>12}")
    
    report.append("")
    
    # Dateien mit Problemen
    report.append("-" * 80)
    report.append("DATEIEN MIT PROBLEMEN:")
    report.append("-" * 80)
    
    problem_files = [(g, l, s, m, n, e) for g, l, s, m, n, e in file_results 
                     if s in ("DATEI_FEHLT", "JSON_FEHLER") or m > 0 or n > 0]
    
    if problem_files:
        report.append(f"{'Datei':<50} {'Sprache':<8} {'Status':<12} {'Fehlt':>8} {'Null/Leer':>10}")
        report.append("-" * 80)
        for group_key, lang, status, missing, null_count, extra in sorted(problem_files):
            filename = f"{group_key}/{lang}.json"
            status_str = status if status != "OK" else "INHALT"
            report.append(f"{filename:<50} {lang:<8} {status_str:<12} {missing:>8} {null_count:>10}")
    else:
        report.append("Keine Probleme gefunden!")
    
    report.append("")
    report.append("=" * 80)
    report.append("STATUS: " + ("ALLE UEBERSETZUNGEN SIND VOLLSTAENDIG" if total_null == 0 and total_missing_keys == 0 
                               else "ES WURDEN PROBLEME GEFUNDEN"))
    report.append("=" * 80)
    
    return "\n".join(report), total_null == 0 and total_missing_keys == 0 and total_missing_files == 0

if __name__ == "__main__":
    output, is_complete = check_translations()
    
    # Speichern
    with open("translation_final_report.txt", "w", encoding="utf-8") as f:
        f.write(output)
    
    # Ausgabe
    print(output)
    print(f"\nBericht gespeichert in: translation_final_report.txt")
