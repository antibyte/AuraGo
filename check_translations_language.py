#!/usr/bin/env python3
"""
Erweiterte Uebersetzungspruefung mit Spracherkennung
Prueft ob die Ubersetzungen tatsaechlich in der Zielsprache sind
"""

import json
import re
from pathlib import Path
from collections import defaultdict

try:
    from langdetect import detect, DetectorFactory
    from langdetect.lang_detect_exception import LangDetectException
    LANGDETECT_AVAILABLE = True
    DetectorFactory.seed = 0  # Reproduzierbare Ergebnisse
except ImportError:
    LANGDETECT_AVAILABLE = False
    print("WARNUNG: langdetect nicht installiert. Verwende einfache Heuristik.")

LANG_DIR = Path("ui/lang")
REFERENCE_LANGS = ["de", "en"]
ALL_LANGS = ["cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"]

# Mapping von unseren Sprachcodes zu langdetect codes
LANG_MAPPING = {
    "cs": "cs",    # Czech
    "da": "da",    # Danish
    "de": "de",    # German
    "el": "el",    # Greek
    "en": "en",    # English
    "es": "es",    # Spanish
    "fr": "fr",    # French
    "hi": "hi",    # Hindi
    "it": "it",    # Italian
    "ja": "ja",    # Japanese
    "nl": "nl",    # Dutch
    "no": "no",    # Norwegian
    "pl": "pl",    # Polish
    "pt": "pt",    # Portuguese
    "sv": "sv",    # Swedish
    "zh": "zh-cn", # Chinese
}

# Typische Wörter/Muster für jede Sprache (Fallback wenn langdetect nicht verfügbar)
LANGUAGE_PATTERNS = {
    "cs": ["ř", "ě", "š", "č", "ž", "ý", "á", "í", "é", "ů", "ú", "ó", "ď", "ť", "ň"],
    "da": ["æ", "ø", "å", "og", "det", "er", "til", "af", "på", "som"],
    "de": ["ä", "ö", "ü", "ß", "und", "der", "die", "das", "ist", "zu", "für", "mit", "auf", "nicht"],
    "el": ["α", "β", "γ", "δ", "ε", "ζ", "η", "θ", "ι", "κ", "λ", "μ", "ν", "ξ", "ο", "π", "ρ", "σ", "τ", "υ", "φ", "χ", "ψ", "ω"],
    "en": ["the", "and", "is", "to", "of", "a", "in", "for", "on", "with", "not"],
    "es": ["ñ", "á", "é", "í", "ó", "ú", "ü", "el", "la", "de", "y", "en", "que", "un", "es"],
    "fr": ["ç", "é", "è", "ê", "à", "ù", "le", "la", "de", "et", "est", "un", "pour", "dans"],
    "hi": ["अ", "आ", "इ", "ई", "उ", "ऊ", "ए", "ऐ", "ओ", "औ", "क", "ख", "ग", "घ"],
    "it": ["à", "è", "é", "ì", "ò", "ù", "il", "di", "e", "la", "che", "è", "per", "un"],
    "ja": ["あ", "い", "う", "え", "お", "か", "き", "く", "け", "こ", "漢", "平", "片"],
    "nl": ["ij", "de", "het", "van", "en", "een", "te", "dat", "voor", "op", "met"],
    "no": ["æ", "ø", "å", "og", "er", "det", "til", "av", "på", "som", "for"],
    "pl": ["ą", "ć", "ę", "ł", "ń", "ó", "ś", "ź", "ż", "i", "w", "z", "nie", "na", "do", "się"],
    "pt": ["ã", "õ", "ç", "á", "é", "í", "ó", "ú", "de", "a", "o", "que", "e", "do", "da"],
    "sv": ["å", "ä", "ö", "och", "är", "det", "till", "av", "på", "som", "för", "inte"],
    "zh": ["的", "一", "是", "不", "了", "人", "我", "在", "有", "他", "这", "个", "们", "中", "来", "上"],
}

def load_json(filepath):
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            return json.load(f)
    except Exception as e:
        return {"__ERROR__": str(e)}

def detect_language_simple(text, expected_lang):
    """
    Einfache Spracherkennung basierend auf typischen Zeichen/Wörtern
    Gibt einen Score zurück (0-100) wie wahrscheinlich die Sprache korrekt ist
    """
    if not text or not isinstance(text, str):
        return None
    
    text_lower = text.lower()
    
    # Japanisch und Chinesisch haben eindeutige Zeichen
    if expected_lang == "ja":
        ja_chars = sum(1 for c in text if '\u3040' <= c <= '\u309f' or '\u30a0' <= c <= '\u30ff' or '\u4e00' <= c <= '\u9faf')
        return 100 if ja_chars > 0 else 0
    
    if expected_lang == "zh":
        zh_chars = sum(1 for c in text if '\u4e00' <= c <= '\u9fff')
        return 100 if zh_chars > 0 else 0
    
    if expected_lang == "hi":
        hi_chars = sum(1 for c in text if '\u0900' <= c <= '\u097f')
        return 100 if hi_chars > 0 else 0
    
    if expected_lang == "el":
        el_chars = sum(1 for c in text if '\u0370' <= c <= '\u03ff' or '\u1f00' <= c <= '\u1fff')
        return 100 if el_chars > 0 else 0
    
    # Für andere Sprachen: prüfe auf typische Zeichen
    patterns = LANGUAGE_PATTERNS.get(expected_lang, [])
    if not patterns:
        return 50  # Unbekannte Sprache
    
    # Zähle Treffer
    matches = 0
    for pattern in patterns:
        if pattern in text_lower:
            matches += 1
    
    # Berechne Score (prozentual)
    score = min(100, (matches / len(patterns)) * 200)  # 200 als Faktor, damit nicht zu streng
    
    # Prüfe auf offensichtlich falsche Sprachen
    wrong_language = False
    if expected_lang not in ["en"]:
        # Prüfe ob hauptsächlich englisch
        en_patterns = LANGUAGE_PATTERNS["en"]
        en_matches = sum(1 for p in en_patterns if p in text_lower)
        if en_matches >= 3 and matches < 2:
            wrong_language = True
    
    if wrong_language:
        return 0
    
    return score

def detect_language_langdetect(text, expected_lang):
    """Verwendet langdetect Bibliothek"""
    if not text or not isinstance(text, str):
        return None
    
    # Text muss lang genug sein
    if len(text) < 10:
        return 50  # Zu kurz für zuverlässige Erkennung
    
    try:
        detected = detect(text)
        expected_mapped = LANG_MAPPING.get(expected_lang, expected_lang)
        
        if detected == expected_mapped:
            return 100
        
        # Ähnliche Sprachen prüfen
        similar = {
            "da": ["no", "sv"],
            "no": ["da", "sv"],
            "sv": ["da", "no"],
            "pt": ["es"],
            "es": ["pt"],
        }
        
        if detected in similar.get(expected_lang, []):
            return 70  # Ähnliche Sprache
        
        return 0
    except LangDetectException:
        return 50

def analyze_file(filepath, expected_lang):
    """Analysiert eine Datei und gibt Probleme zurück"""
    data = load_json(filepath)
    
    if "__ERROR__" in data:
        return [("JSON_FEHLER", data["__ERROR__"])]
    
    problems = []
    sample_checked = 0
    
    for key, value in data.items():
        if value is None or not isinstance(value, str) or not value.strip():
            continue
        
        # Nur einen Teil der Strings prüfen (Stichprobe)
        if sample_checked >= 20:
            break
        
        # Überspringe sehr kurze Texte und technische Platzhalter
        if len(value) < 15 or value.startswith("{{") or value.startswith("http"):
            continue
        
        # Sprache erkennen
        if LANGDETECT_AVAILABLE:
            score = detect_language_langdetect(value, expected_lang)
        else:
            score = detect_language_simple(value, expected_lang)
        
        if score is not None and score < 30:
            problems.append(("FALSCHE_SPRACHE", key, value[:50], score))
            sample_checked += 1
        elif score is not None and score < 60:
            problems.append(("VERDACHTIG", key, value[:50], score))
            sample_checked += 1
    
    return problems

def main():
    print("=" * 80)
    print("UEBERSETZUNGSQUALITAETSPRUEFUNG")
    print("=" * 80)
    print()
    print(f"Spracherkennung: {'langdetect (praezise)' if LANGDETECT_AVAILABLE else 'einfache Heuristik (geschätzt)'}")
    print()
    
    all_files = list(LANG_DIR.rglob("*.json"))
    file_groups = defaultdict(dict)
    
    for filepath in all_files:
        if filepath.name == "meta.json":
            continue
        lang = filepath.stem
        rel_dir = filepath.parent.relative_to(LANG_DIR)
        group_key = str(rel_dir) if str(rel_dir) != "." else filepath.stem
        file_groups[group_key][lang] = filepath
    
    all_problems = []
    stats = defaultdict(lambda: {"checked": 0, "wrong": 0, "suspicious": 0})
    
    for group_key, lang_files in sorted(file_groups.items()):
        for lang in ALL_LANGS:
            if lang in REFERENCE_LANGS:
                continue
            
            if lang not in lang_files:
                continue
            
            filepath = lang_files[lang]
            problems = analyze_file(filepath, lang)
            
            for problem in problems:
                all_problems.append((filepath, lang, problem))
                
                if problem[0] == "FALSCHE_SPRACHE":
                    stats[lang]["wrong"] += 1
                elif problem[0] == "VERDACHTIG":
                    stats[lang]["suspicious"] += 1
                stats[lang]["checked"] += 1
    
    # Bericht
    print("-" * 80)
    print("ZUSAMMENFASSUNG PRO SPRACHE:")
    print("-" * 80)
    print(f"{'Sprache':<10} {'Geprüft':>10} {'Falsch':>10} {'Verdächtig':>12}")
    print("-" * 80)
    
    for lang in sorted([l for l in ALL_LANGS if l not in REFERENCE_LANGS]):
        s = stats[lang]
        print(f"{lang:<10} {s['checked']:>10} {s['wrong']:>10} {s['suspicious']:>12}")
    
    print()
    
    # Detaillierte Probleme
    if all_problems:
        print("-" * 80)
        print("DETAILLIERTE PROBLEME:")
        print("-" * 80)
        
        for filepath, lang, problem in all_problems[:50]:  # Max 50 anzeigen
            ptype, key, text, score = problem
            rel_path = str(filepath.relative_to(LANG_DIR))
            print(f"[{ptype}] {rel_path}")
            print(f"  Key:  {key}")
            print(f"  Text: {text}")
            print(f"  Score: {score}%")
            print()
        
        if len(all_problems) > 50:
            print(f"... und {len(all_problems) - 50} weitere Probleme")
    else:
        print("Keine offensichtlichen Sprachprobleme gefunden!")
    
    print()
    print("=" * 80)
    
    # Speichern
    with open("translation_language_report.txt", "w", encoding="utf-8") as f:
        f.write("UEBERSETZUNGSQUALITAETSPRUEFUNG\n")
        f.write("=" * 80 + "\n\n")
        f.write(f"Spracherkennung: {'langdetect' if LANGDETECT_AVAILABLE else 'einfache Heuristik'}\n\n")
        
        for filepath, lang, problem in all_problems:
            ptype, key, text, score = problem
            rel_path = str(filepath.relative_to(LANG_DIR))
            f.write(f"[{ptype}] {rel_path} ({lang})\n")
            f.write(f"  Key:  {key}\n")
            f.write(f"  Text: {text}\n")
            f.write(f"  Score: {score}%\n\n")
    
    print(f"Bericht gespeichert in: translation_language_report.txt")

if __name__ == "__main__":
    main()
