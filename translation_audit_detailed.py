#!/usr/bin/env python3
import json
import os
import sys

def load_json(filepath):
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            return json.load(f), None
    except json.JSONDecodeError as e:
        return None, str(e)
    except Exception as e:
        return None, str(e)

languages = ['cs', 'da', 'de', 'el', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']
lang_names = {
    'cs': 'Czech', 'da': 'Danish', 'de': 'German', 'el': 'Greek', 'es': 'Spanish',
    'fr': 'French', 'hi': 'Hindi', 'it': 'Italian', 'ja': 'Japanese', 'nl': 'Dutch',
    'no': 'Norwegian', 'pl': 'Polish', 'pt': 'Portuguese', 'sv': 'Swedish', 'zh': 'Chinese'
}
folders = ['dashboard', 'gallery', 'media']

all_problems = []
total_files_checked = 0
problematic_files = set()

for folder in folders:
    en_path = f'ui/lang/{folder}/en.json'
    en_data, en_error = load_json(en_path)
    
    if en_error:
        all_problems.append({
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
            all_problems.append({
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
                all_problems.append({
                    'file': lang_path,
                    'type': 'Missing Key',
                    'key': key,
                    'english_value': en_data.get(key, 'N/A')
                })
            problematic_files.add(lang_path)
        
        # Check for extra keys
        extra_keys = lang_keys - en_keys
        if extra_keys:
            for key in sorted(extra_keys):
                all_problems.append({
                    'file': lang_path,
                    'type': 'Extra Key (not in English)',
                    'key': key,
                    'value': lang_data.get(key, 'N/A')
                })
            problematic_files.add(lang_path)
        
        # Check for English values in non-English files (suspicious)
        for key in lang_keys:
            en_val = en_data.get(key, '')
            lang_val = lang_data.get(key, '')
            
            # Skip if key is missing in English (already reported)
            if key not in en_keys:
                continue
                
            # Check for identical values with English (might be untranslated)
            # Skip technical terms and proper nouns
            if lang_val == en_val and lang != 'en':
                # Skip if looks like technical term
                if en_val in ['MQTT', 'TTS', 'SSE', 'API', 'CPU', 'RAM', 'Disk', 'GitHub', 'Docker', 'Discord', 
                               'Proxmox', 'Ansible', 'Ollama', 'Telegram', 'WebDAV', 'Webhooks', 'n8n',
                               'Koofr', 'FritzBox', 'Tailscale', 'Netlify', 'VirusTotal', 'MeshCentral',
                               'AdGuard', 'OneDrive', 'Cloudflare', 'A2A', 'MCP', 'S3']:
                    continue
                # Skip short values
                if len(en_val) <= 3:
                    continue
                # Skip values with special characters that should stay same
                if any(c in en_val for c in ['/', '(', '{', '@', '#', '\\u']):
                    continue
                # Skip all-caps values
                if en_val.isupper():
                    continue
                    
                all_problems.append({
                    'file': lang_path,
                    'type': 'Same as English (possibly untranslated)',
                    'key': key,
                    'value': lang_val
                })
                problematic_files.add(lang_path)
        
        # Special check for da.json - seems to have Portuguese content
        if lang == 'da' and folder == 'dashboard':
            # Load Portuguese for comparison
            pt_path = f'ui/lang/{folder}/pt.json'
            pt_data, _ = load_json(pt_path)
            if pt_data:
                matches_pt = 0
                for key in lang_keys:
                    if lang_data.get(key) == pt_data.get(key):
                        matches_pt += 1
                if matches_pt > 50:  # Suspicious number of matches
                    all_problems.append({
                        'file': lang_path,
                        'type': 'SUSPICIOUS: File may contain wrong language translations',
                        'details': f'{matches_pt} values match Portuguese (pt.json)'
                    })
                    problematic_files.add(lang_path)

# Write results to file
output_lines = []
output_lines.append('='*80)
output_lines.append('DETAILED TRANSLATION AUDIT RESULTS')
output_lines.append('='*80)
output_lines.append(f'Total files checked: {total_files_checked}')
output_lines.append(f'Total problems found: {len(all_problems)}')
output_lines.append(f'Problematic files: {len(problematic_files)}')

if all_problems:
    # Group by type
    by_type = {}
    for p in all_problems:
        t = p['type']
        if t not in by_type:
            by_type[t] = []
        by_type[t].append(p)
    
    output_lines.append('')
    output_lines.append('='*80)
    output_lines.append('SUMMARY BY PROBLEM TYPE')
    output_lines.append('='*80)
    for t in sorted(by_type.keys()):
        output_lines.append(f'{t}: {len(by_type[t])}')
    
    output_lines.append('')
    output_lines.append('='*80)
    output_lines.append('DETAILED PROBLEMS LIST')
    output_lines.append('='*80)
    
    current_file = None
    for p in sorted(all_problems, key=lambda x: (x['file'], x.get('type', ''), x.get('key', ''))):
        if p['file'] != current_file:
            current_file = p['file']
            output_lines.append('')
            output_lines.append(f'FILE: {current_file}')
            output_lines.append('-'*60)
        
        if p['type'] == 'JSON Syntax Error':
            output_lines.append(f'  JSON ERROR: {p["details"]}')
        elif p['type'] == 'Missing Key':
            output_lines.append(f'  MISSING: "{p["key"]}"')
            output_lines.append(f'    English: "{p["english_value"]}"')
        elif p['type'] == 'Extra Key (not in English)':
            output_lines.append(f'  EXTRA KEY: "{p["key"]}" = "{p["value"]}"')
        elif p['type'] == 'Same as English (possibly untranslated)':
            output_lines.append(f'  UNTRANSLATED: "{p["key"]}" = "{p["value"]}"')
        elif p['type'] == 'SUSPICIOUS: File may contain wrong language translations':
            output_lines.append(f'  SUSPICIOUS: {p["details"]}')
    
    output_lines.append('')
    output_lines.append('='*80)
    output_lines.append('SUMMARY BY FILE')
    output_lines.append('='*80)
    for f in sorted(problematic_files):
        file_problems = [p for p in all_problems if p['file'] == f]
        output_lines.append(f'{f}: {len(file_problems)} problems')
else:
    output_lines.append('')
    output_lines.append('No problems found!')

# Write to file
with open('translation_audit_report.txt', 'w', encoding='utf-8') as f:
    f.write('\n'.join(output_lines))

print('\n'.join(output_lines[:50]))
print(f'\n... (full report saved to translation_audit_report.txt)')
