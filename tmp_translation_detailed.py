#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Detaillierte Übersetzungsprüfung für AuraGo
Prüft auf nicht übersetzte Werte und falsche Übersetzungen
"""

import json
import re

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

# Cheatsheets Referenz
cheatsheets_en = load_json('ui/lang/cheatsheets/en.json')

languages = ['cs', 'da', 'de', 'el', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']

# Bekannte nicht übersetzte Patterns (auf Niederländisch belassen)
dutch_patterns = [
    'Toegangstype', 'Toewijzen', 'Overerven', 'Bevestigen', 'Beschrijving',
    'Bewerken', 'Poort', 'Notities', 'Provider', 'Aangepast', 'Direct',
    'Doel', 'Architectuur', 'Verbinding', 'Gebruikersnaam', 'Wachtwoord',
    'Productie', 'Server', 'Optionele', 'Latijn', 'Latijn om', 'te behouden',
    'verstuurd', 'wordt permanent', 'verwijderd', 'uit de vault'
]

# Bekannte falsche Übersetzungen
check_specific_errors = {
    'cheatsheets.content': {
        'es': ('rebaja', 'Contenido (Markdown)'),
        'it': ('ribasso', 'Contenuto (Markdown)'),
        'pl': ('przecena', 'Treść (Markdown)'),
        'zh': ('降价', '内容（Markdown）'),
    },
    'cheatsheets.save': {
        'da': ('Spare', 'Gem'),
        'el': ('Εκτός', 'Αποθήκευση'),
        'hi': ('बचाना', 'सहेजें'),
        'nl': ('Redden', 'Opslaan'),
        'no': ('Spare', 'Lagre'),
        'pl': ('Ratować', 'Zapisz'),
        'sv': ('Spara', 'Spara'),  # Schwedisch ist korrekt
        'zh': ('节省', '保存'),
    },
    'invasion.cancel': {
        'cs': ('Zrusit', 'Zrušit'),  # Fehlendes Akut
        'da': ('Zrusit', 'Annuller'),
        'el': ('Ακυρωση', 'Ακύρωση'),  # Fehlender Akzent
        'hi': ('Zrusit', 'रद्द करें'),
        'no': ('Zrusit', 'Avbryt'),
        'sv': ('Zrusit', 'Avbryt'),
    },
    'invasion.delete': {
        'cs': ('Smazat', 'Smazat'),  # Korrekt
        'da': ('Smazat', 'Slet'),
        'el': ('Διαγραφη', 'Διαγραφή'),  # Fehlender Akzent
        'hi': ('Smazat', 'हटाएं'),
        'no': ('Smazat', 'Slett'),
        'sv': ('Smazat', 'Ta bort'),
    },
    'invasion.edit': {
        'cs': ('Upravit', 'Upravit'),  # Korrekt
        'da': ('Upravit', 'Rediger'),
        'el': ('Επεξεργασια', 'Επεξεργασία'),  # Fehlender Akzent
        'hi': ('Upravit', 'संपादित करें'),
        'no': ('Upravit', 'Rediger'),
        'sv': ('Upravit', 'Redigera'),
    },
    'invasion.save': {
        'cs': ('Ulozit', 'Uložit'),  # Fehlendes Akut
        'da': ('Ulozit', 'Gem'),
        'el': ('Αποθηκευση', 'Αποθήκευση'),  # Fehlender Akzent
        'hi': ('Ulozit', 'सहेजें'),
        'no': ('Ulozit', 'Lagre'),
        'sv': ('Ulozit', 'Spara'),
    },
    'cheatsheets.name': {
        'cs': ('jméno', 'Název'),  # Klein geschrieben
        'da': ('navn', 'Navn'),  # Klein geschrieben
        'el': ('όνομα', 'Όνομα'),  # Klein geschrieben
        'es': ('nombre', 'Nombre'),  # Klein geschrieben
        'fr': ('nom', 'Nom'),  # Klein geschrieben
        'it': ('nome', 'Nome'),  # Klein geschrieben
        'nl': ('naam', 'Naam'),  # Klein geschrieben
        'no': ('navn', 'Navn'),  # Klein geschrieben
        'pl': ('nazwa', 'Nazwa'),  # Klein geschrieben
        'pt': ('nome', 'Nome'),  # Klein geschrieben
        'sv': ('namn', 'Namn'),  # Klein geschrieben
    }
}

all_problems = []
files_checked = 0

print('=' * 100)
print('DETAILLIERTE UEBERSETZUNGSPRUEFUNG')
print('=' * 100)

# Prüfe Invasion
print('\n' + '=' * 100)
print('INVASION - NICHT UEBERSETZTE WERTE')
print('=' * 100)

for lang in languages:
    path = f'ui/lang/invasion/{lang}.json'
    files_checked += 1
    data = load_json(path)
    
    if '_error' in data:
        all_problems.append({
            'file': path,
            'type': 'JSON_SYNTAX',
            'key': '-',
            'current': '-',
            'recommended': f'JSON Fehler: {data["_error"]}'
        })
        continue
    
    for key, value in data.items():
        if key == '_error':
            continue
            
        en_value = invasion_en.get(key, '')
        
        # Prüfe auf nicht übersetzte niederländische Werte
        for dutch in dutch_patterns:
            if dutch in value and dutch not in en_value:
                # Prüfe ob es sich um einen technischen Begriff handelt
                if key in ['invasion.access_docker', 'invasion.access_ssh', 'invasion.base_url']:
                    continue
                all_problems.append({
                    'file': path,
                    'type': 'NOT_TRANSLATED',
                    'key': key,
                    'current': value,
                    'recommended': f'[UEBERSETZUNG ERFORDERLICH]'
                })
                break
        
        # Prüfe auf spezifische bekannte Fehler
        if key in check_specific_errors and lang in check_specific_errors[key]:
            expected_wrong, expected_correct = check_specific_errors[key][lang]
            if value == expected_wrong:
                all_problems.append({
                    'file': path,
                    'type': 'WRONG_TRANSLATION',
                    'key': key,
                    'current': value,
                    'recommended': expected_correct
                })

# Prüfe Cheatsheets
print('\n' + '=' * 100)
print('CHEATSHEETS - PROBLEME')
print('=' * 100)

for lang in languages:
    path = f'ui/lang/cheatsheets/{lang}.json'
    files_checked += 1
    data = load_json(path)
    
    if '_error' in data:
        all_problems.append({
            'file': path,
            'type': 'JSON_SYNTAX',
            'key': '-',
            'current': '-',
            'recommended': f'JSON Fehler: {data["_error"]}'
        })
        continue
    
    for key, value in data.items():
        if key == '_error':
            continue
            
        en_value = cheatsheets_en.get(key, '')
        
        # Prüfe auf nicht übersetzte niederländische Werte
        for dutch in dutch_patterns:
            if dutch in value and dutch not in en_value:
                all_problems.append({
                    'file': path,
                    'type': 'NOT_TRANSLATED',
                    'key': key,
                    'current': value,
                    'recommended': f'[UEBERSETZUNG ERFORDERLICH]'
                })
                break
        
        # Prüfe auf spezifische bekannte Fehler
        if key in check_specific_errors and lang in check_specific_errors[key]:
            expected_wrong, expected_correct = check_specific_errors[key][lang]
            if value == expected_wrong:
                all_problems.append({
                    'file': path,
                    'type': 'WRONG_TRANSLATION',
                    'key': key,
                    'current': value,
                    'recommended': expected_correct
                })

# Ausgabe aller Probleme
print('\n' + '=' * 100)
print('DETAILLIERTE PROBLEMLISTE')
print('=' * 100)

for problem in all_problems:
    print(f"\nDatei: {problem['file']}")
    print(f"Typ: {problem['type']}")
    print(f"Key: {problem['key']}")
    print(f"Aktueller Wert: {problem['current']}")
    print(f"Empfohlene Korrektur: {problem['recommended']}")

# Zusammenfassung
print('\n' + '=' * 100)
print('ZUSAMMENFASSUNG')
print('=' * 100)
print(f'Gepruefte Dateien: {files_checked}')
print(f'Gefundene Probleme: {len(all_problems)}')

if all_problems:
    files_with_problems = set(p['file'] for p in all_problems)
    print(f'Dateien mit Problemen: {len(files_with_problems)}')
    print('\nProblematische Dateien:')
    for f in sorted(files_with_problems):
        count = sum(1 for p in all_problems if p['file'] == f)
        print(f'  - {f} ({count} Probleme)')
else:
    print('Keine Probleme gefunden!')
