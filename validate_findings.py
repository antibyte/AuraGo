#!/usr/bin/env python3
"""
Validierung der Übersetzungsprüfung - korrigierte Version
"""

import json
import sys
from pathlib import Path
from collections import defaultdict

sys.stdout.reconfigure(encoding='utf-8')

LANG_DIR = Path("ui/lang")
REFERENCE_LANGS = ["de", "en"]
ALL_LANGS = ["cs", "da", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"]

def load_json(filepath):
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            return json.load(f)
    except Exception as e:
        return {"__ERROR__": str(e)}

def is_mostly_latin(text):
    """Prüft ob Text hauptsächlich lateinische Buchstaben enthält (ohne Platzhalter)"""
    if not text:
        return False
    # Entferne Platzhalter {{...}}
    cleaned = text
    while '{{' in cleaned and '}}' in cleaned:
        start = cleaned.find('{{')
        end = cleaned.find('}}', start) + 2
        if end > start:
            cleaned = cleaned[:start] + cleaned[end:]
        else:
            break
    
    latin_chars = sum(1 for c in cleaned if 'a' <= c.lower() <= 'z')
    total_letters = sum(1 for c in cleaned if c.isalpha())
    if total_letters == 0:
        return False
    return latin_chars / total_letters > 0.8

def has_cyrillic(text):
    """Prüft auf kyrillische Buchstaben"""
    return any('\u0400' <= c <= '\u04ff' for c in text)

def has_greek(text):
    """Prüft auf griechische Buchstaben"""
    return any('\u0370' <= c <= '\u03ff' or '\u1f00' <= c <= '\u1fff' for c in text)

def has_cjk(text):
    """Prüft auf CJK (Chinesisch/Japanisch/Koreanisch)"""
    return any('\u4e00' <= c <= '\u9fff' or '\u3040' <= c <= '\u30ff' for c in text)

def has_devanagari(text):
    """Prüft auf Devanagari (Hindi)"""
    return any('\u0900' <= c <= '\u097f' for c in text)

def is_english_text(text):
    """Erkennt echte englische Sätze (nicht nur einzelne Worte)"""
    if not text or len(text) < 15:
        return False
    
    text_lower = text.lower()
    # Platzhalter entfernen
    import re
    text_clean = re.sub(r'\{\{[^}]+\}\}', '', text_lower)
    text_clean = re.sub(r'[^\w\s]', '', text_clean)
    
    # Häufige englische Wörter (mehr als 3 nötig)
    en_indicators = ["the", "and", "is", "to", "of", "a", "in", "for", "with", 
                     "you", "that", "it", "on", "are", "this", "from", "have", "not"]
    words = set(text_clean.split())
    matches = sum(1 for word in en_indicators if word in words)
    
    # Mindestens 3 verschiedene englische Wörter + hauptsächlich lateinisch
    return matches >= 3 and is_mostly_latin(text)

def validate_files():
    print("=" * 80)
    print("VALIDIERUNG DER ÜBERSETZUNGSANALYSE")
    print("=" * 80)
    print()
    
    # Beispiele prüfen
    test_cases = [
        ("ui/lang/chat/el.json", "el", "griechisch"),
        ("ui/lang/chat/hi.json", "hi", "hindi (devanagari)"),
        ("ui/lang/chat/ja.json", "ja", "japanisch"),
        ("ui/lang/chat/zh.json", "zh", "chinesisch"),
    ]
    
    for filepath_str, lang, expected_script in test_cases:
        filepath = Path(filepath_str)
        if not filepath.exists():
            continue
            
        data = load_json(filepath)
        print(f"\n📁 {filepath.name} ({lang} - erwartet: {expected_script})")
        print("-" * 60)
        
        issues_found = []
        
        for key, value in data.items():
            if not isinstance(value, str) or not value.strip():
                continue
            
            text = value.strip()
            
            # Prüfe auf tschechische Wörter (spezifisch)
            cz_words = ["agent", "je", "aktivni", "pripojen", "odpojen", "chyba", 
                       "nacitani", "vymazat", "soubor", "upravit", "premysli", "vystup"]
            text_lower = text.lower()
            
            found_cz = [w for w in cz_words if w in text_lower]
            
            if found_cz:
                issues_found.append((key, text[:50], "TSCHECHISCH", found_cz))
                continue
            
            # Prüfe auf echte englische Sätze
            if lang != "en" and is_english_text(text):
                issues_found.append((key, text[:50], "ENGLISCH", []))
                continue
        
        if issues_found:
            print(f"  Gefundene Probleme: {len(issues_found)}")
            for key, text, problem_type, details in issues_found[:5]:
                detail_str = f" ({', '.join(details)})" if details else ""
                print(f"    [{problem_type}]{detail_str}")
                print(f"      {key}")
                print(f"      → {text}")
        else:
            print("  ✅ Keine offensichtlichen Probleme gefunden")
    
    print()
    print("=" * 80)
    print("KORREKTUR DER BEFUNDE:")
    print("=" * 80)
    print()
    print("BASIEREND AUF MANUELLER PRÜFUNG:")
    print()
    print("✅ GRIECHISCH (el):")
    print("   - Tatsächlich gemischt: Enthält GRIECHISCHE und TSCHECHISCHE Wörter")
    print("   - z.B.: 'Ο agent ειναι ενεργος' (griechisch) + 'Rozpocet' (tschechisch)")
    print("   - Bewertung: TEILWEISE KORREKT - Tschechische Einmischung bestätigt")
    print()
    print("✅ HINDI (hi):")
    print("   - Tatsächlich gemischt: Enthält HINDI (Devanagari) und TSCHECHISCHE Wörter")
    print("   - z.B.: 'नमस्ते! मैं AuraGo हूं' (hindi) + 'Agent je aktivni' (tschechisch)")
    print("   - Bewertung: TEILWEISE KORREKT - Tschechische Einmischung bestätigt")
    print()
    print("❌ JAPANISCH (ja) - FALSE POSITIVE:")
    print("   - Datei ist KORREKT auf Japanisch")
    print("   - Mein Skript hat Platzhalter {{cost}} als 'falsches Schriftsystem' erkannt")
    print("   - Bewertung: FALSCHER ALARM")
    print()
    print("❌ CHINESISCH (zh) - FALSE POSITIVE:")
    print("   - Datei ist KORREKT auf Chinesisch (außer 'playful' in Zeile 50)")
    print("   - Mein Skript hat Platzhalter {{...}} als 'falsches Schriftsystem' erkannt")
    print("   - Bewertung: FALSCHER ALARM (außer einem englischen Wort)")
    print()
    print("=" * 80)

if __name__ == "__main__":
    validate_files()
