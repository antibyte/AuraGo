#!/usr/bin/env python3
"""
Translation file checker for AuraGo
Checks for missing keys, incomplete translations, and JSON syntax errors
"""

import json
import os
import sys
from collections import defaultdict

# Set UTF-8 encoding
sys.stdout.reconfigure(encoding='utf-8')

# Language codes
LANGUAGES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']

# Sections to check
SECTIONS = ['help', 'setup', 'missions']

class TranslationChecker:
    def __init__(self):
        self.issues = []
        self.files_checked = 0
        self.files_with_problems = set()
        
    def load_json(self, filepath):
        """Load JSON file, return data or None if error"""
        try:
            with open(filepath, 'r', encoding='utf-8') as f:
                return json.load(f)
        except json.JSONDecodeError as e:
            self.issues.append({
                'file': filepath,
                'type': 'JSON_SYNTAX_ERROR',
                'key': None,
                'message': f"JSON syntax error: {e}"
            })
            return None
        except Exception as e:
            self.issues.append({
                'file': filepath,
                'type': 'FILE_ERROR',
                'key': None,
                'message': f"Error reading file: {e}"
            })
            return None
    
    def check_section(self, section):
        """Check all files in a section"""
        base_path = f"ui/lang/{section}"
        
        # Load English reference
        en_file = f"{base_path}/en.json"
        en_data = self.load_json(en_file)
        
        if en_data is None:
            print(f"ERROR: Cannot load English reference for {section}")
            return
            
        self.files_checked += 1
        
        # Check other languages
        for lang in LANGUAGES:
            if lang == 'en':
                continue
                
            lang_file = f"{base_path}/{lang}.json"
            if not os.path.exists(lang_file):
                self.issues.append({
                    'file': lang_file,
                    'type': 'MISSING_FILE',
                    'key': None,
                    'message': f"File does not exist"
                })
                self.files_with_problems.add(lang_file)
                continue
            
            self.files_checked += 1
            lang_data = self.load_json(lang_file)
            
            if lang_data is None:
                self.files_with_problems.add(lang_file)
                continue
            
            # Check for missing keys
            for key in en_data:
                if key not in lang_data:
                    self.issues.append({
                        'file': lang_file,
                        'type': 'MISSING_KEY',
                        'key': key,
                        'reference': en_data[key][:100] if en_data[key] else "(empty)",
                        'message': f"Missing key: {key}"
                    })
                    self.files_with_problems.add(lang_file)
            
            # Check for extra keys (in translation but not in English)
            for key in lang_data:
                if key not in en_data:
                    self.issues.append({
                        'file': lang_file,
                        'type': 'EXTRA_KEY',
                        'key': key,
                        'message': f"Extra key (not in en): {key}"
                    })
                    self.files_with_problems.add(lang_file)
            
            # Check for suspicious translations (very short or same as English)
            for key in en_data:
                if key in lang_data:
                    en_value = en_data[key]
                    lang_value = lang_data[key]
                    
                    # Empty translation
                    if not lang_value or lang_value.strip() == '':
                        self.issues.append({
                            'file': lang_file,
                            'type': 'EMPTY_TRANSLATION',
                            'key': key,
                            'message': f"Empty translation",
                            'reference': en_value[:80]
                        })
                        self.files_with_problems.add(lang_file)
                    
                    # Very short translation (might be incomplete) but not for short English
                    elif len(lang_value) < 10 and len(en_value) > 40:
                        self.issues.append({
                            'file': lang_file,
                            'type': 'SHORT_TRANSLATION',
                            'key': key,
                            'message': f"Very short translation ({len(lang_value)} chars) for long English ({len(en_value)} chars)",
                            'reference': en_value[:80],
                            'translation': lang_value
                        })
                        self.files_with_problems.add(lang_file)
                    
                    # Check for English text in non-English file (long English text that is identical)
                    elif len(en_value) > 30 and lang_value == en_value:
                        self.issues.append({
                            'file': lang_file,
                            'type': 'NOT_TRANSLATED',
                            'key': key,
                            'message': f"Text not translated (same as English)",
                            'reference': en_value[:80]
                        })
                        self.files_with_problems.add(lang_file)
    
    def run(self):
        """Run full check"""
        print("=" * 80)
        print("AURAGO TRANSLATION CHECK")
        print("=" * 80)
        
        for section in SECTIONS:
            print(f"\nChecking section: {section}")
            self.check_section(section)
        
        # Print results
        print("\n" + "=" * 80)
        print("DETAILED ISSUES")
        print("=" * 80)
        
        # Group issues by file
        issues_by_file = defaultdict(list)
        for issue in self.issues:
            issues_by_file[issue['file']].append(issue)
        
        # Print detailed issues
        current_file = None
        for issue in self.issues:
            if issue['file'] != current_file:
                current_file = issue['file']
                print(f"\n{'='*60}")
                print(f"FILE: {current_file}")
                print('='*60)
            
            print(f"\n  [{issue['type']}] {issue['key'] or 'N/A'}")
            print(f"  Message: {issue['message']}")
            if 'reference' in issue:
                ref = issue['reference'].replace('\n', ' ')
                print(f"  English: {ref}")
            if 'translation' in issue:
                trans = issue['translation'].replace('\n', ' ')
                print(f"  Current: {trans}")
        
        # Summary
        print("\n" + "=" * 80)
        print("SUMMARY")
        print("=" * 80)
        print(f"Files checked: {self.files_checked}")
        print(f"Files with problems: {len(self.files_with_problems)}")
        print(f"Total issues found: {len(self.issues)}")
        
        # Count by type
        type_counts = defaultdict(int)
        for issue in self.issues:
            type_counts[issue['type']] += 1
        
        print("\nIssues by type:")
        for issue_type, count in sorted(type_counts.items()):
            print(f"  {issue_type}: {count}")
        
        print("\nFiles with problems:")
        for f in sorted(self.files_with_problems):
            print(f"  - {f}")

if __name__ == '__main__':
    checker = TranslationChecker()
    checker.run()
