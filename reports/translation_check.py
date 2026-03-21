#!/usr/bin/env python3
"""
Übersetzungsprüfung für AuraGo UI Sprachdateien
Vergleicht alle Sprachdateien mit der englischen Referenz
"""

import json
import os
from pathlib import Path
from typing import Dict, List, Set, Tuple, Optional

# Verzeichnisse, die geprüft werden sollen
TARGET_DIRS = [
    "ui/lang/config/image_generation",
    "ui/lang/config/indexing",
    "ui/lang/config/llm_guardian",
    "ui/lang/config/mcp",
    "ui/lang/config/mcp_server",
    "ui/lang/config/memory_analysis",
    "ui/lang/config/misc",
    "ui/lang/config/netlify",
]

# Sprachdateien (außer en.json, da es die Referenz ist)
LANGUAGES = [
    "cs", "da", "de", "el", "es", "fr", "hi", "it",
    "ja", "nl", "no", "pl", "pt", "sv", "zh"
]

def flatten_json(obj: dict, parent_key: str = "") -> Dict[str, any]:
    """Flacht ein verschachteltes JSON-Objekt zu einem flachen Dictionary ab."""
    items = {}
    for k, v in obj.items():
        new_key = f"{parent_key}.{k}" if parent_key else k
        if isinstance(v, dict):
            items.update(flatten_json(v, new_key))
        else:
            items[new_key] = v
    return items

def load_json_file(filepath: Path) -> Tuple[Optional[dict], Optional[str]]:
    """Lädt eine JSON-Datei und gibt das Dictionary oder einen Fehler zurück."""
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            return json.load(f), None
    except json.JSONDecodeError as e:
        return None, f"JSON-Syntaxfehler: {e}"
    except FileNotFoundError:
        return None, "Datei nicht gefunden"
    except Exception as e:
        return None, f"Fehler: {e}"

def compare_keys(ref_keys: Set[str], trans_keys: Set[str]) -> Tuple[Set[str], Set[str]]:
    """Vergleicht zwei Key-Sets und gibt fehlende und zusätzliche Keys zurück."""
    missing = ref_keys - trans_keys
    extra = trans_keys - ref_keys
    return missing, extra

def check_directory(dir_path: str) -> dict:
    """Prüft alle Sprachdateien in einem Verzeichnis."""
    results = {
        "directory": dir_path,
        "reference": {},
        "translations": {}
    }
    
    base_path = Path(dir_path)
    if not base_path.exists():
        results["error"] = "Verzeichnis nicht gefunden"
        return results
    
    # Lade Referenzdatei (en.json)
    ref_file = base_path / "en.json"
    ref_data, ref_error = load_json_file(ref_file)
    
    if ref_error:
        results["reference"]["error"] = ref_error
        return results
    
    ref_flat = flatten_json(ref_data)
    ref_keys = set(ref_flat.keys())
    results["reference"]["keys_count"] = len(ref_keys)
    results["reference"]["keys"] = sorted(ref_keys)
    
    # Prüfe jede Sprachdatei
    for lang in LANGUAGES:
        trans_file = base_path / f"{lang}.json"
        trans_data, trans_error = load_json_file(trans_file)
        
        lang_result = {
            "file": str(trans_file),
            "exists": trans_file.exists()
        }
        
        if trans_error:
            lang_result["error"] = trans_error
        elif trans_data is not None:
            trans_flat = flatten_json(trans_data)
            trans_keys = set(trans_flat.keys())
            lang_result["keys_count"] = len(trans_keys)
            
            missing, extra = compare_keys(ref_keys, trans_keys)
            
            if missing:
                lang_result["missing_keys"] = sorted(missing)
            if extra:
                lang_result["extra_keys"] = sorted(extra)
            
            if not missing and not extra:
                lang_result["status"] = "OK"
            else:
                lang_result["status"] = "INCOMPLETE"
        
        results["translations"][lang] = lang_result
    
    return results

def generate_report(results_list: List[dict]) -> str:
    """Generiert einen detaillierten Bericht."""
    report_lines = []
    report_lines.append("=" * 80)
    report_lines.append("ÜBERSETZUNGSPRÜFUNG - DETAILLIERTER BERICHT")
    report_lines.append("=" * 80)
    report_lines.append("")
    
    total_issues = 0
    
    for result in results_list:
        dir_name = result["directory"]
        report_lines.append("-" * 80)
        report_lines.append(f"VERZEICHNIS: {dir_name}")
        report_lines.append("-" * 80)
        
        if "error" in result:
            report_lines.append(f"FEHLER: {result['error']}")
            report_lines.append("")
            continue
        
        ref_info = result["reference"]
        if "error" in ref_info:
            report_lines.append(f"REFERENZFEHLER: {ref_info['error']}")
            report_lines.append("")
            continue
        
        ref_keys_count = ref_info.get("keys_count", 0)
        report_lines.append(f"Referenz (en.json): {ref_keys_count} Keys")
        report_lines.append("")
        
        translations = result["translations"]
        dir_issues = 0
        
        for lang in LANGUAGES:
            trans_info = translations.get(lang, {})
            
            if not trans_info.get("exists", False):
                report_lines.append(f"  [{lang}] FEHLER: Datei nicht gefunden")
                dir_issues += 1
                continue
            
            if "error" in trans_info:
                report_lines.append(f"  [{lang}] FEHLER: {trans_info['error']}")
                dir_issues += 1
                continue
            
            status = trans_info.get("status", "UNKNOWN")
            keys_count = trans_info.get("keys_count", 0)
            
            if status == "OK":
                report_lines.append(f"  [{lang}] OK ({keys_count} Keys)")
            else:
                missing = trans_info.get("missing_keys", [])
                extra = trans_info.get("extra_keys", [])
                
                issues = []
                if missing:
                    issues.append(f"{len(missing)} fehlende Keys")
                if extra:
                    issues.append(f"{len(extra)} zusätzliche Keys")
                
                report_lines.append(f"  [{lang}] INCOMPLETE ({keys_count} Keys) - {', '.join(issues)}")
                dir_issues += 1
                
                if missing:
                    report_lines.append(f"    Fehlende Keys:")
                    for key in missing:
                        report_lines.append(f"      - {key}")
                
                if extra:
                    report_lines.append(f"    Zusätzliche Keys:")
                    for key in extra:
                        report_lines.append(f"      + {key}")
        
        report_lines.append("")
        report_lines.append(f"Zusammenfassung: {dir_issues} Probleme gefunden")
        report_lines.append("")
        total_issues += dir_issues
    
    report_lines.append("=" * 80)
    report_lines.append(f"GESAMTZAHL DER PROBLEME: {total_issues}")
    report_lines.append("=" * 80)
    
    return "\n".join(report_lines)

def generate_summary(results_list: List[dict]) -> str:
    """Generiert eine kurze Zusammenfassung."""
    summary_lines = []
    summary_lines.append("\n" + "=" * 80)
    summary_lines.append("ZUSAMMENFASSUNG")
    summary_lines.append("=" * 80)
    
    total_files = 0
    total_ok = 0
    total_incomplete = 0
    total_errors = 0
    
    for result in results_list:
        dir_name = result["directory"]
        translations = result.get("translations", {})
        
        for lang in LANGUAGES:
            trans_info = translations.get(lang, {})
            total_files += 1
            
            if not trans_info.get("exists", False) or "error" in trans_info:
                total_errors += 1
            elif trans_info.get("status") == "OK":
                total_ok += 1
            else:
                total_incomplete += 1
    
    summary_lines.append(f"Geprüfte Dateien:     {total_files}")
    summary_lines.append(f"Vollständig (OK):     {total_ok}")
    summary_lines.append(f"Unvollständig:        {total_incomplete}")
    summary_lines.append(f"Fehler:               {total_errors}")
    summary_lines.append("=" * 80)
    
    return "\n".join(summary_lines)

def main():
    """Hauptfunktion."""
    results_list = []
    
    print("Starte Übersetzungsprüfung...")
    print()
    
    for dir_path in TARGET_DIRS:
        print(f"Prüfe: {dir_path}")
        result = check_directory(dir_path)
        results_list.append(result)
    
    # Generiere Bericht
    report = generate_report(results_list)
    summary = generate_summary(results_list)
    
    # Speichere Bericht
    report_path = Path("reports/translation_report.txt")
    report_path.parent.mkdir(exist_ok=True)
    
    with open(report_path, 'w', encoding='utf-8') as f:
        f.write(report)
        f.write(summary)
    
    print()
    print(report)
    print(summary)
    print()
    print(f"Bericht gespeichert unter: {report_path}")

if __name__ == "__main__":
    main()
