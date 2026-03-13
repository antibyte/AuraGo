#!/usr/bin/env python3
"""
Qualitätsprüfung für Übersetzungen
Erkennt:
1. Null/leere Werte
2. Falsche Sprache (z.B. Englisch statt Zielsprache)
3. Übersetzungen die gleich dem Quelltext sind (unübersetzt)
"""

import json
import sys
from pathlib import Path
from collections import defaultdict

sys.stdout.reconfigure(encoding='utf-8')

LANG_DIR = Path("ui/lang")
REFERENCE_LANGS = ["de", "en"]
ALL_LANGS = ["cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"]

def load_json(filepath):
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            return json.load(f)
    except Exception as e:
        return {"__ERROR__": str(e)}

def is_latin_script(text):
    """Prüft ob Text hauptsächlich lateinische Buchstaben enthält"""
    if not text:
        return False
    latin_chars = sum(1 for c in text if ('a' <= c.lower() <= 'z'))
    total_letters = sum(1 for c in text if c.isalpha())
    if total_letters == 0:
        return True  # Keine Buchstaben, trotzdem als Latin betrachten
    return latin_chars / total_letters > 0.8

def detect_script_type(text):
    """
    Erkennt den Schrifttyp:
    - latin: Lateinische Buchstaben
    - cyrillic: Kyrillisch
    - cjk: Chinesisch/Japanisch/Koreanisch
    - arabic: Arabisch/Hebräisch
    - greek: Griechisch
    - devanagari: Hindi/Sanskrit
    """
    if not text:
        return "unknown"
    
    # Zähle Zeichen pro Schriftsystem
    cjk = sum(1 for c in text if '\u4e00' <= c <= '\u9fff' or '\u3040' <= c <= '\u30ff')
    greek = sum(1 for c in text if '\u0370' <= c <= '\u03ff' or '\u1f00' <= c <= '\u1fff')
    cyrillic = sum(1 for c in text if '\u0400' <= c <= '\u04ff')
    devanagari = sum(1 for c in text if '\u0900' <= c <= '\u097f')
    arabic = sum(1 for c in text if '\u0600' <= c <= '\u06ff' or '\u0590' <= c <= '\u05ff')
    
    total_chars = sum(1 for c in text if c.isalpha())
    if total_chars == 0:
        return "neutral"  # Zahlen, Symbole etc.
    
    if cjk / total_chars > 0.5:
        return "cjk"
    if greek / total_chars > 0.5:
        return "greek"
    if cyrillic / total_chars > 0.5:
        return "cyrillic"
    if devanagari / total_chars > 0.5:
        return "devanagari"
    if arabic / total_chars > 0.5:
        return "arabic"
    
    return "latin"

def get_expected_script(lang):
    """Gibt das erwartete Schriftsystem für eine Sprache zurück"""
    script_map = {
        "cs": "latin",
        "da": "latin",
        "de": "latin",
        "el": "greek",
        "en": "latin",
        "es": "latin",
        "fr": "latin",
        "hi": "devanagari",
        "it": "latin",
        "ja": "cjk",
        "nl": "latin",
        "no": "latin",
        "pl": "latin",
        "pt": "latin",
        "sv": "latin",
        "zh": "cjk",
    }
    return script_map.get(lang, "latin")

def is_english_text(text):
    """Heuristik um zu erkennen ob ein Text Englisch ist"""
    if not text or len(text) < 10:
        return False
    
    text_lower = text.lower()
    
    # Häufige englische Wörter
    en_indicators = [" the ", " and ", " is ", " to ", " of ", " a ", " in ", 
                     " for ", " with ", " you ", " that ", " it ", " on ", " are ",
                     "this", "from", "have", "not", "be", "or", "as", "at"]
    
    matches = sum(1 for word in en_indicators if word in text_lower)
    
    # Wenn mehr als 2 typisch englische Wörter gefunden
    return matches >= 3

def check_translations():
    """Hauptprüffunktion"""
    
    all_files = list(LANG_DIR.rglob("*.json"))
    file_groups = defaultdict(dict)
    
    for filepath in all_files:
        if filepath.name == "meta.json":
            continue
        lang = filepath.stem
        rel_dir = filepath.parent.relative_to(LANG_DIR)
        group_key = str(rel_dir) if str(rel_dir) != "." else filepath.stem
        file_groups[group_key][lang] = filepath
    
    all_issues = []
    stats = defaultdict(lambda: {
        "files": 0, "null_values": 0, "wrong_script": 0, 
        "english_in_non_en": 0, "untranslated": 0
    })
    
    for group_key, lang_files in sorted(file_groups.items()):
        # Lade Referenz (Englisch bevorzugt für Vergleich)
        ref_data = {}
        for ref_lang in REFERENCE_LANGS:
            if ref_lang in lang_files:
                ref_data[ref_lang] = load_json(lang_files[ref_lang])
        
        if not ref_data:
            continue
        
        en_data = ref_data.get("en", {})
        de_data = ref_data.get("de", {})
        
        for lang in ALL_LANGS:
            if lang in REFERENCE_LANGS:
                continue
            
            if lang not in lang_files:
                continue
            
            filepath = lang_files[lang]
            trans_data = load_json(filepath)
            
            if "__ERROR__" in trans_data:
                continue
            
            stats[lang]["files"] += 1
            expected_script = get_expected_script(lang)
            
            for key, value in trans_data.items():
                if value is None:
                    all_issues.append((filepath, lang, "NULL_WERT", key, ""))
                    stats[lang]["null_values"] += 1
                    continue
                
                if not isinstance(value, str):
                    continue
                
                text = value.strip()
                if not text:
                    all_issues.append((filepath, lang, "LEERER_WERT", key, ""))
                    stats[lang]["null_values"] += 1
                    continue
                
                # Prüfe Schriftsystem (nur für nicht-Latin-Sprachen)
                actual_script = detect_script_type(text)
                if expected_script != "latin" and actual_script != expected_script and actual_script != "neutral":
                    if len(text) > 10:
                        all_issues.append((filepath, lang, "FALSCHES_SCHRIFTSYSTEM", key, text[:50]))
                        stats[lang]["wrong_script"] += 1
                        continue
                
                # Prüfe auf Englisch in nicht-englischen Dateien
                # (nur für Texte die lang genug sind und nicht nur Platzhalter)
                if len(text) > 20 and not text.startswith("{{"):
                    if is_english_text(text):
                        # Vergleiche mit englischer Referenz
                        en_value = en_data.get(key, "")
                        if text.lower() == en_value.lower():
                            all_issues.append((filepath, lang, "UNUEBERSETZT", key, text[:50]))
                            stats[lang]["untranslated"] += 1
                        elif lang not in ["en"]:
                            all_issues.append((filepath, lang, "ENGLISCH", key, text[:50]))
                            stats[lang]["english_in_non_en"] += 1
    
    # Bericht erstellen
    report_lines = []
    report_lines.append("=" * 80)
    report_lines.append("UEBERSETZUNGSQUALITAETSPRUEFUNG")
    report_lines.append("=" * 80)
    report_lines.append("")
    report_lines.append("Referenz: Deutsch (de) + Englisch (en)")
    report_lines.append("Geprüfte Sprachen: " + ", ".join([l for l in ALL_LANGS if l not in REFERENCE_LANGS]))
    report_lines.append("")
    
    # Zusammenfassung
    total_issues = len(all_issues)
    
    report_lines.append("-" * 80)
    report_lines.append("GESAMTSTATISTIK:")
    report_lines.append("-" * 80)
    report_lines.append(f"  Gefundene Probleme: {total_issues}")
    report_lines.append("")
    
    # Pro Sprache
    report_lines.append("-" * 80)
    report_lines.append("DETAILS PRO SPRACHE:")
    report_lines.append("-" * 80)
    report_lines.append(f"{'Sprache':<8} {'Dateien':>8} {'Null/Leer':>10} {'Falsches Script':>15} {'Englisch':>10} {'Unübersetzt':>12}")
    report_lines.append("-" * 80)
    
    for lang in sorted([l for l in ALL_LANGS if l not in REFERENCE_LANGS]):
        s = stats[lang]
        total = s["null_values"] + s["wrong_script"] + s["english_in_non_en"] + s["untranslated"]
        report_lines.append(f"{lang:<8} {s['files']:>8} {s['null_values']:>10} {s['wrong_script']:>15} {s['english_in_non_en']:>10} {s['untranslated']:>12}")
    
    report_lines.append("")
    
    # Detaillierte Probleme
    if all_issues:
        report_lines.append("-" * 80)
        report_lines.append("DETAILLIERTE PROBLEME (erste 50):")
        report_lines.append("-" * 80)
        report_lines.append("")
        
        for i, (filepath, lang, ptype, key, text) in enumerate(all_issues[:50]):
            rel_path = str(filepath.relative_to(LANG_DIR))
            report_lines.append(f"[{ptype}] {rel_path}")
            report_lines.append(f"  Sprache: {lang}")
            report_lines.append(f"  Key: {key}")
            if text:
                report_lines.append(f"  Text: {text}")
            report_lines.append("")
        
        if len(all_issues) > 50:
            report_lines.append(f"... und {len(all_issues) - 50} weitere Probleme")
            report_lines.append("")
    
    report_lines.append("=" * 80)
    report_lines.append("LEGENDE:")
    report_lines.append("  NULL_WERT           - Wert ist null/None")
    report_lines.append("  LEERER_WERT         - Wert ist leerer String")
    report_lines.append("  FALSCHE_SCHRIFTSYSTEM - Falsches Schriftsystem (z.B. Latin statt CJK)")
    report_lines.append("  ENGLISCH            - Text scheint Englisch zu sein (in nicht-EN Datei)")
    report_lines.append("  UNUEBERSETZT        - Text ist identisch mit englischer Referenz")
    report_lines.append("=" * 80)
    
    report_text = "\n".join(report_lines)
    
    # Speichern
    with open("translation_quality_report.txt", "w", encoding="utf-8") as f:
        f.write(report_text)
    
    return report_text, total_issues

if __name__ == "__main__":
    output, issues = check_translations()
    print(output)
    print(f"\nBericht gespeichert in: translation_quality_report.txt")
    
    if issues == 0:
        print("\n[OK] Keine Probleme gefunden!")
    else:
        print(f"\n[WARNUNG] {issues} Probleme gefunden!")
