#!/usr/bin/env python3
"""
Deep Audit Script for Cross-Language Contamination in Translation Files

This script audits all translation files under ui/lang/ to detect:
1. Cross-language contamination (e.g., Czech text in Greek files)
2. Untranslated entries (same as English)
3. Wrong language text in files

Usage: python deep_audit_translations.py
"""

import json
import os
import re
from collections import defaultdict
from pathlib import Path
from typing import Dict, List, Set, Tuple

# Language codes
LANGUAGES = ['cs', 'da', 'de', 'el', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']

# Special character patterns for language detection
LANGUAGE_PATTERNS = {
    'cs': re.compile(r'[řěščžňťďáíéúůý]', re.IGNORECASE),  # Czech
    'el': re.compile(r'[αβγδεζηθικλμνξοπρστυφχψωΑΒΓΔΕΖΗΘΙΚΛΜΝΞΟΠΡΣΤΥΦΧΨΩ]', re.IGNORECASE),  # Greek
    'de': re.compile(r'[ßüöäÜÖÄ]'),  # German (special chars)
    'ja': re.compile(r'[\u3040-\u309F\u30A0-\u30FF]'),  # Japanese (Hiragana/Katakana)
    'zh': re.compile(r'[\u4E00-\u9FFF]'),  # Chinese
    'hi': re.compile(r'[\u0900-\u097F]'),  # Hindi/Devanagari
}

# Language names for reporting
LANGUAGE_NAMES = {
    'cs': 'Czech',
    'da': 'Danish',
    'de': 'German',
    'el': 'Greek',
    'es': 'Spanish',
    'fr': 'French',
    'hi': 'Hindi',
    'it': 'Italian',
    'ja': 'Japanese',
    'nl': 'Dutch',
    'no': 'Norwegian',
    'pl': 'Polish',
    'pt': 'Portuguese',
    'sv': 'Swedish',
    'zh': 'Chinese',
    'en': 'English',
}

# Get the project root directory (parent of disposable/)
SCRIPT_DIR = Path(__file__).parent.resolve()
PROJECT_ROOT = SCRIPT_DIR.parent
BASE_DIR = PROJECT_ROOT / 'ui' / 'lang'
RESULTS_FILE = SCRIPT_DIR / 'audit_results.txt'


def detect_language(text: str) -> List[str]:
    """Detect which languages the text contains based on special characters."""
    if not text or not isinstance(text, str):
        return []
    
    detected = []
    for lang, pattern in LANGUAGE_PATTERNS.items():
        if pattern.search(text):
            detected.append(lang)
    
    return detected


def is_likely_german(text: str) -> bool:
    """Check if text is likely German based on common patterns."""
    german_patterns = [
        r'\bder\b', r'\bdie\b', r'\bdas\b', r'\bund\b', r'\bist\b',
        r'\bmit\b', r'\bfür\b', r'\bnicht\b', r'\bein\b', r'\beine\b',
        r'\bauf\b', r'\bzu\b', r'\bim\b', r'\bvon\b', r'\ban\b',
        r'ße\b', r'üb\w', r'öb\w', r'äß\w'
    ]
    for pattern in german_patterns:
        if re.search(pattern, text, re.IGNORECASE):
            return True
    return False


def audit_directory(dir_path: Path) -> List[Dict]:
    """Audit all translation files in a directory."""
    issues = []
    
    if not dir_path.is_dir():
        return issues
    
    # Find en.json as reference
    en_file = dir_path / 'en.json'
    if not en_file.exists():
        print(f"  [SKIP] No en.json found in {dir_path.name}")
        return issues
    
    try:
        with open(en_file, 'r', encoding='utf-8') as f:
            en_data = json.load(f)
    except json.JSONDecodeError as e:
        print(f"  [ERROR] Invalid JSON in {en_file}: {e}")
        return issues
    
    # Check each language file
    for lang in LANGUAGES:
        lang_file = dir_path / f'{lang}.json'
        if not lang_file.exists():
            continue
        
        try:
            with open(lang_file, 'r', encoding='utf-8') as f:
                lang_data = json.load(f)
        except json.JSONDecodeError as e:
            print(f"  [ERROR] Invalid JSON in {lang_file}: {e}")
            continue
        
        # Compare each key
        for key, en_value in en_data.items():
            if key not in lang_data:
                issues.append({
                    'dir': dir_path.name,
                    'file': f'{lang}.json',
                    'key': key,
                    'issue': 'missing',
                    'expected': str(en_value),
                    'actual': None,
                    'detected_langs': [],
                    'expected_lang': lang,
                })
                continue
            
            lang_value = lang_data[key]
            
            # Skip if value is None or empty
            if not lang_value:
                continue
            
            # Check if it's identical to English (untranslated)
            if str(lang_value).strip() == str(en_value).strip():
                issues.append({
                    'dir': dir_path.name,
                    'file': f'{lang}.json',
                    'key': key,
                    'issue': 'untranslated',
                    'expected': str(en_value),
                    'actual': str(lang_value),
                    'detected_langs': [],
                    'expected_lang': lang,
                })
                continue
            
            # Detect languages in the value
            detected = detect_language(str(lang_value))
            
            # Check for cross-language contamination
            if detected and lang not in detected:
                # Text contains characters from other languages
                # Check if it's contamination (e.g., Czech in Greek file)
                detected_names = [LANGUAGE_NAMES.get(d, d) for d in detected]
                issues.append({
                    'dir': dir_path.name,
                    'file': f'{lang}.json',
                    'key': key,
                    'issue': 'contamination',
                    'expected': str(en_value),
                    'actual': str(lang_value),
                    'detected_langs': detected,
                    'detected_lang_names': detected_names,
                    'expected_lang': lang,
                    'expected_lang_name': LANGUAGE_NAMES.get(lang, lang),
                })
            
            # Special check for German
            elif lang != 'de' and is_likely_german(str(lang_value)):
                issues.append({
                    'dir': dir_path.name,
                    'file': f'{lang}.json',
                    'key': key,
                    'issue': 'possible_german',
                    'expected': str(en_value),
                    'actual': str(lang_value),
                    'detected_langs': ['de'],
                    'detected_lang_names': ['German'],
                    'expected_lang': lang,
                    'expected_lang_name': LANGUAGE_NAMES.get(lang, lang),
                })
    
    return issues


def main():
    """Main audit function."""
    print("=" * 80)
    print("Deep Translation Audit - Cross-Language Contamination Detection")
    print("=" * 80)
    print()
    
    all_issues = []
    all_dirs = []
    
    # Collect all directories to audit
    for item in BASE_DIR.iterdir():
        if item.is_dir() and item.name != 'meta':
            all_dirs.append(item)
    
    all_dirs.sort(key=lambda x: x.name)
    
    print(f"Found {len(all_dirs)} directories to audit:")
    for d in all_dirs:
        print(f"  - {d.name}")
    print()
    
    # Audit each directory
    for dir_path in all_dirs:
        print(f"Auditing: {dir_path.name}/")
        
        # Check if it's a config subdirectory (has its own structure)
        config_subdirs = [p for p in dir_path.iterdir() if p.is_dir()] if dir_path.is_dir() else []
        
        if config_subdirs:
            # Config directory with subdirs - audit each subdir
            for subdir in sorted(config_subdirs):
                print(f"  -> {subdir.name}/")
                issues = audit_directory(subdir)
                all_issues.extend(issues)
                if issues:
                    print(f"      Found {len(issues)} issues")
        else:
            # Normal directory with language files
            issues = audit_directory(dir_path)
            all_issues.extend(issues)
            if issues:
                print(f"  Found {len(issues)} issues")
        
        print()
    
    # Group issues by type
    contamination_issues = [i for i in all_issues if i['issue'] == 'contamination']
    german_issues = [i for i in all_issues if i['issue'] == 'possible_german']
    untranslated_issues = [i for i in all_issues if i['issue'] == 'untranslated']
    missing_issues = [i for i in all_issues if i['issue'] == 'missing']
    
    # Write results to file
    with open(RESULTS_FILE, 'w', encoding='utf-8') as f:
        f.write("=" * 80 + "\n")
        f.write("CROSS-LANGUAGE CONTAMINATION AUDIT RESULTS\n")
        f.write("=" * 80 + "\n\n")
        
        f.write(f"Total issues found: {len(all_issues)}\n")
        f.write(f"  - Cross-language contamination: {len(contamination_issues)}\n")
        f.write(f"  - Possible German text: {len(german_issues)}\n")
        f.write(f"  - Untranslated (same as English): {len(untranslated_issues)}\n")
        f.write(f"  - Missing entries: {len(missing_issues)}\n\n")
        
        # Write contamination issues
        if contamination_issues:
            f.write("=" * 80 + "\n")
            f.write("CROSS-LANGUAGE CONTAMINATION (Actual wrong language text)\n")
            f.write("=" * 80 + "\n\n")
            
            for issue in contamination_issues:
                f.write(f"Directory: {issue['dir']}\n")
                f.write(f"File: {issue['file']}\n")
                f.write(f"Key: {issue['key']}\n")
                f.write(f"Expected Language: {issue['expected_lang_name']} ({issue['expected_lang']})\n")
                f.write(f"Detected Language(s): {', '.join(issue['detected_lang_names'])} ({', '.join(issue['detected_langs'])})\n")
                f.write(f"Expected Value: {issue['expected']}\n")
                f.write(f"Actual Value: {issue['actual']}\n")
                f.write("-" * 40 + "\n\n")
        
        # Write German issues
        if german_issues:
            f.write("=" * 80 + "\n")
            f.write("POSSIBLE GERMAN TEXT (Pattern-based detection)\n")
            f.write("=" * 80 + "\n\n")
            
            for issue in german_issues:
                f.write(f"Directory: {issue['dir']}\n")
                f.write(f"File: {issue['file']}\n")
                f.write(f"Key: {issue['key']}\n")
                f.write(f"Expected Language: {issue['expected_lang_name']} ({issue['expected_lang']})\n")
                f.write(f"Detected Language: German\n")
                f.write(f"Expected Value: {issue['expected']}\n")
                f.write(f"Actual Value: {issue['actual']}\n")
                f.write("-" * 40 + "\n\n")
        
        # Write untranslated issues
        if untranslated_issues:
            f.write("=" * 80 + "\n")
            f.write("UNTRANSLATED ENTRIES (Same as English)\n")
            f.write("=" * 80 + "\n\n")
            
            # Group by file
            by_file = defaultdict(list)
            for issue in untranslated_issues:
                by_file[issue['file']].append(issue)
            
            for file, issues in sorted(by_file.items()):
                f.write(f"\nFile: {file} ({len(issues)} untranslated entries)\n")
                for issue in issues[:10]:  # Show first 10
                    f.write(f"  Key: {issue['key']}\n")
                    f.write(f"    Value: {issue['actual'][:80]}{'...' if len(str(issue['actual'])) > 80 else ''}\n")
                if len(issues) > 10:
                    f.write(f"  ... and {len(issues) - 10} more\n")
        
        # Write missing issues
        if missing_issues:
            f.write("\n" + "=" * 80 + "\n")
            f.write("MISSING ENTRIES\n")
            f.write("=" * 80 + "\n\n")
            
            # Group by file
            by_file = defaultdict(list)
            for issue in missing_issues:
                by_file[issue['file']].append(issue)
            
            for file, issues in sorted(by_file.items()):
                f.write(f"\nFile: {file} ({len(issues)} missing entries)\n")
                for issue in issues[:10]:  # Show first 10
                    f.write(f"  Key: {issue['key']}\n")
                if len(issues) > 10:
                    f.write(f"  ... and {len(issues) - 10} more\n")
    
    # Print summary to console
    print("=" * 80)
    print("SUMMARY")
    print("=" * 80)
    print()
    print(f"Total issues found: {len(all_issues)}")
    print(f"  - Cross-language contamination: {len(contamination_issues)}")
    print(f"  - Possible German text: {len(german_issues)}")
    print(f"  - Untranslated (same as English): {len(untranslated_issues)}")
    print(f"  - Missing entries: {len(missing_issues)}")
    print()
    print(f"Results written to: {RESULTS_FILE}")
    
    # Print contamination details
    if contamination_issues:
        print()
        print("=" * 80)
        print("CROSS-LANGUAGE CONTAMINATION DETAILS")
        print("=" * 80)
        for issue in contamination_issues:
            print(f"\n[{issue['dir']}/{issue['file']}] Key: {issue['key']}")
            print(f"  Expected: {issue['expected_lang_name']} ({issue['expected_lang']})")
            print(f"  Detected: {', '.join(issue['detected_lang_names'])} ({', '.join(issue['detected_langs'])})")
            print(f"  Value: {issue['actual'][:100]}{'...' if len(issue['actual']) > 100 else ''}")
    
    return all_issues


if __name__ == '__main__':
    main()
