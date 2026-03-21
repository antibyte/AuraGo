#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Übersetzungsprüfung für AuraGo
Vergleicht alle Sprachdateien mit der englischen Referenz
"""

import json
import os

# Referenz-Keys aus en.json laden
def load_json(path):
    try:
        with open(path, 'r', encoding='utf-8') as f:
            return json.load(f)
    except json.JSONDecodeError as e:
        return {'_error': f'JSON Syntaxfehler: {str(e)}'}
    except Exception as e:
        return {'_error': str(e)}

# Invasion Referenz
invasion_en = load_json('ui/lang/invasion/en.json')
invasion_keys = set(invasion_en.keys())

# Cheatsheets Referenz
cheatsheets_en = load_json('ui/lang/cheatsheets/en.json')
cheatsheets_keys = set(cheatsheets_en.keys())

languages = ['cs', 'da', 'de', 'el', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']

all_problems = []
files_checked = 0
files_with_problems = set()

print('=' * 80)
print('DETAILLIERTE UEBERSETZUNGSANALYSE')
print('=' * 80)

# Invasion Analyse
print('\n' + '=' * 80)
print('INVASION UEBERSETZUNGEN')
print('=' * 80)

for lang in languages:
    path = f'ui/lang/invasion/{lang}.json'
    files_checked += 1
    data = load_json(path)
    
    if '_error' in data:
        problem = {
            'file': path,
            'type': 'JSON_SYNTAX',
            'details': data['_error']
        }
        all_problems.append(problem)
        files_with_problems.add(path)
        print(f'\n[{lang}] JSON FEHLER: {data["_error"]}')
        continue
    
    current_keys = set(data.keys())
    missing = invasion_keys - current_keys
    extra = current_keys - invasion_keys
    
    if missing or extra:
        files_with_problems.add(path)
        print(f'\n[{lang}] Key-Probleme in {path}:')
        if missing:
            for key in sorted(missing):
                problem = {
                    'file': path,
                    'type': 'MISSING_KEY',
                    'key': key,
                    'details': f'Fehlender Key: {key}'
                }
                all_problems.append(problem)
                print(f'  - FEHLEND: {key}')
        if extra:
            for key in sorted(extra):
                problem = {
                    'file': path,
                    'type': 'EXTRA_KEY',
                    'key': key,
                    'details': f'Zusätzlicher Key: {key}'
                }
                all_problems.append(problem)
                print(f'  - ZUSAETZLICH: {key}')

# Cheatsheets Analyse
print('\n' + '=' * 80)
print('CHEATSHEETS UEBERSETZUNGEN')
print('=' * 80)

for lang in languages:
    path = f'ui/lang/cheatsheets/{lang}.json'
    files_checked += 1
    data = load_json(path)
    
    if '_error' in data:
        problem = {
            'file': path,
            'type': 'JSON_SYNTAX',
            'details': data['_error']
        }
        all_problems.append(problem)
        files_with_problems.add(path)
        print(f'\n[{lang}] JSON FEHLER: {data["_error"]}')
        continue
    
    current_keys = set(data.keys())
    missing = cheatsheets_keys - current_keys
    extra = current_keys - cheatsheets_keys
    
    if missing or extra:
        files_with_problems.add(path)
        print(f'\n[{lang}] Key-Probleme in {path}:')
        if missing:
            for key in sorted(missing):
                problem = {
                    'file': path,
                    'type': 'MISSING_KEY',
                    'key': key,
                    'details': f'Fehlender Key: {key}'
                }
                all_problems.append(problem)
                print(f'  - FEHLEND: {key}')
        if extra:
            for key in sorted(extra):
                problem = {
                    'file': path,
                    'type': 'EXTRA_KEY',
                    'key': key,
                    'details': f'Zusätzlicher Key: {key}'
                }
                all_problems.append(problem)
                print(f'  - ZUSAETZLICH: {key}')

# Zusammenfassung
print('\n' + '=' * 80)
print('ZUSAMMENFASSUNG')
print('=' * 80)
print(f'Gepruefte Dateien: {files_checked}')
print(f'Gefundene Probleme: {len(all_problems)}')
print(f'Dateien mit Problemen: {len(files_with_problems)}')

if files_with_problems:
    print('\nProblematische Dateien:')
    for f in sorted(files_with_problems):
        print(f'  - {f}')

if all_problems:
    print('\nAlle Probleme:')
    for p in all_problems:
        print(f"  [{p['type']}] {p['file']}: {p.get('key', p.get('details', ''))}")
