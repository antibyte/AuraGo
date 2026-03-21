#!/usr/bin/env python3
import json
import os

def load_json(filepath):
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            return json.load(f), None
    except json.JSONDecodeError as e:
        return None, str(e)
    except Exception as e:
        return None, str(e)

languages = ['cs', 'da', 'de', 'el', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']
folders = ['dashboard', 'gallery', 'media']

problems = []
total_files_checked = 0
problematic_files = set()

for folder in folders:
    # Load English reference
    en_path = f'ui/lang/{folder}/en.json'
    en_data, en_error = load_json(en_path)
    
    if en_error:
        problems.append({
            'file': en_path,
            'type': 'JSON Error',
            'details': en_error
        })
        problematic_files.add(en_path)
        continue
    
    en_keys = set(en_data.keys())
    
    for lang in languages:
        lang_path = f'ui/lang/{folder}/{lang}.json'
        total_files_checked += 1
        
        lang_data, lang_error = load_json(lang_path)
        
        if lang_error:
            problems.append({
                'file': lang_path,
                'type': 'JSON Syntax Error',
                'details': lang_error
            })
            problematic_files.add(lang_path)
            continue
        
        lang_keys = set(lang_data.keys())
        
        # Check for missing keys
        missing_keys = en_keys - lang_keys
        if missing_keys:
            for key in sorted(missing_keys):
                problems.append({
                    'file': lang_path,
                    'type': 'Missing Key',
                    'key': key,
                    'english_value': en_data.get(key, 'N/A')
                })
            problematic_files.add(lang_path)
        
        # Check for extra keys (not in English)
        extra_keys = lang_keys - en_keys
        if extra_keys:
            for key in sorted(extra_keys):
                problems.append({
                    'file': lang_path,
                    'type': 'Extra Key (not in English)',
                    'key': key,
                    'value': lang_data.get(key, 'N/A')
                })
            problematic_files.add(lang_path)
        
        # Check for empty or suspicious translations
        for key in lang_keys:
            value = lang_data.get(key, '')
            if not value or value.strip() == '':
                problems.append({
                    'file': lang_path,
                    'type': 'Empty Translation',
                    'key': key
                })
                problematic_files.add(lang_path)

# Print results
print('='*80)
print('TRANSLATION AUDIT RESULTS')
print('='*80)
print(f'Total files checked: {total_files_checked}')
print(f'Total problems found: {len(problems)}')
print(f'Problematic files: {len(problematic_files)}')

if problems:
    print('')
    print('='*80)
    print('DETAILED PROBLEMS LIST')
    print('='*80)
    
    current_file = None
    for p in sorted(problems, key=lambda x: (x['file'], x.get('type', ''), x.get('key', ''))):
        if p['file'] != current_file:
            current_file = p['file']
            print('')
            print(f'FILE: {current_file}')
            print('-'*60)
        
        if p['type'] == 'JSON Syntax Error':
            print(f'  JSON ERROR: {p["details"]}')
        elif p['type'] == 'Missing Key':
            print(f'  MISSING: "{p["key"]}"')
            print(f'    English: "{p["english_value"]}"')
        elif p['type'] == 'Extra Key (not in English)':
            print(f'  EXTRA KEY: "{p["key"]}" = "{p["value"]}"')
        elif p['type'] == 'Empty Translation':
            print(f'  EMPTY: "{p["key"]}"')
    
    print('')
    print('='*80)
    print('SUMMARY BY FILE')
    print('='*80)
    for f in sorted(problematic_files):
        file_problems = [p for p in problems if p['file'] == f]
        print(f'{f}: {len(file_problems)} problems')
else:
    print('')
    print('No problems found!')
