#!/usr/bin/env python3
"""
Prüfung ob das Projekt KI-gerecht aufgeteilt ist
"""

import os
from pathlib import Path
from collections import defaultdict

def analyze_project_structure():
    root = Path('.')
    
    # Alle Go-Dateien finden
    go_files = list(root.rglob('*.go'))
    
    # Dateien nach Kategorien
    categories = {
        'micro': (0, 100, 'Mikro'),
        'small': (101, 300, 'Klein'),
        'medium': (301, 500, 'Mittel'),
        'large': (501, 800, 'Gross'),
        'xlarge': (801, 1200, 'Sehr gross'),
        'xxl': (1201, 2000, 'XXL'),
        'monster': (2001, float('inf'), 'Monster')
    }
    
    stats = defaultdict(list)
    total_lines = 0
    
    for f in go_files:
        if 'vendor' in str(f) or 'node_modules' in str(f):
            continue
        try:
            with open(f, 'r', encoding='utf-8', errors='ignore') as file:
                lines = len(file.readlines())
                total_lines += lines
                
                for cat, (min_l, max_l, name) in categories.items():
                    if min_l <= lines <= max_l:
                        stats[cat].append((str(f.relative_to(root)), lines))
                        break
        except:
            pass
    
    # Sortieren
    for cat in stats:
        stats[cat].sort(key=lambda x: x[1], reverse=True)
    
    return stats, total_lines, len(go_files)

def check_package_structure():
    """Prüft die Package-Struktur"""
    internal_dir = Path('./internal')
    packages = []
    
    if internal_dir.exists():
        for pkg_dir in internal_dir.iterdir():
            if pkg_dir.is_dir():
                go_files = list(pkg_dir.rglob('*.go'))
                if go_files:
                    total_lines = 0
                    for f in go_files:
                        try:
                            with open(f, 'r', encoding='utf-8', errors='ignore') as file:
                                total_lines += len(file.readlines())
                        except:
                            pass
                    packages.append((pkg_dir.name, len(go_files), total_lines))
    
    packages.sort(key=lambda x: x[2], reverse=True)
    return packages

def generate_report():
    stats, total_lines, file_count = analyze_project_structure()
    packages = check_package_structure()
    
    report = []
    report.append('='*80)
    report.append('KI-GERECHTE PROJEKTSTRUKTUR - ANALYSE')
    report.append('='*80)
    report.append('')
    
    # KI-Gerecht definieren
    report.append('KRITERIEN FUER KI-GERECHT:')
    report.append('-'*80)
    report.append('Optimal:    < 300 Zeilen (einheitlich verarbeitbar)')
    report.append('Akzeptabel: < 500 Zeilen (noch gut ueberschaubar)')
    report.append('Grenzwertig: < 800 Zeilen (maximale KI-Kapazitaet)')
    report.append('Problematisch: > 800 Zeilen (Kontext-Probleme)')
    report.append('')
    
    # Gesamtstatistik
    report.append('GESAMTSTATISTIK:')
    report.append('-'*80)
    report.append(f'Go-Dateien gesamt: {file_count}')
    report.append(f'Gesamtzeilen: {total_lines:,}')
    report.append(f'Durchschnitt: {total_lines//file_count} Zeilen/Datei')
    report.append('')
    
    # Verteilung
    report.append('VERTEILUNG DER DATEIEN:')
    report.append('-'*80)
    
    ki_optimal = len(stats.get('micro', [])) + len(stats.get('small', []))
    ki_ok = len(stats.get('medium', []))
    ki_grenze = len(stats.get('large', []))
    ki_problem = len(stats.get('xlarge', [])) + len(stats.get('xxl', [])) + len(stats.get('monster', []))
    
    report.append(f'KI-Optimal (<300 Zeilen):    {ki_optimal:3} Dateien ({100*ki_optimal//file_count}%)')
    report.append(f'KI-OK (301-500 Zeilen):      {ki_ok:3} Dateien ({100*ki_ok//file_count}%)')
    report.append(f'Grenzwertig (501-800):       {ki_grenze:3} Dateien ({100*ki_grenze//file_count}%)')
    report.append(f'Problematisch (>800):        {ki_problem:3} Dateien ({100*ki_problem//file_count}%)')
    report.append('')
    
    # Gesamtbewertung
    report.append('GESAMTBEWERTUNG:')
    report.append('-'*80)
    ki_score = (ki_optimal + ki_ok*0.8 + ki_grenze*0.5) / file_count * 100
    report.append(f'KI-Gerecht-Score: {ki_score:.1f}%')
    
    if ki_score >= 80:
        report.append('Status: [GUT] Das Projekt ist weitgehend KI-gerecht.')
    elif ki_score >= 60:
        report.append('Status: [MITTEL] Verbesserungen empfohlen.')
    else:
        report.append('Status: [SCHLECHT] Wesentliche Aufteilung noetig.')
    report.append('')
    
    # Problemdateien
    report.append('PROBLEMDATEIEN (KI-Sicht):')
    report.append('-'*80)
    
    for cat in ['monster', 'xxl', 'xlarge']:
        files = stats.get(cat, [])
        if files:
            report.append('')
            report.append(f'{cat.upper()}:')
            for path, lines in files[:10]:
                report.append(f'  {lines:5} Zeilen: {path}')
    
    report.append('')
    
    # Package-Übersicht
    report.append('PACKAGE-STRUKTUR:')
    report.append('-'*80)
    for pkg, file_count_pkg, lines in packages[:15]:
        status = '[OK]' if lines < 3000 else '[WARN]' if lines < 5000 else '[KRITISCH]'
        report.append(f'{status} {pkg:25} {file_count_pkg:3} Dateien, {lines:5} Zeilen')
    
    report.append('')
    report.append('='*80)
    report.append('EMPFEHLUNGEN:')
    report.append('='*80)
    
    if stats.get('monster'):
        report.append('1. SOFORT: Monster-Dateien aufspalten')
        report.append('   - Siehe oben fuer konkrete Dateien')
        report.append('')
    
    if stats.get('xxl'):
        report.append('2. HOCH: XXL-Dateien refactoren')
        for path, lines in stats['xxl'][:3]:
            report.append(f'   - {path}')
        report.append('')
    
    report.append('3. MITTEL: Packages mit >3000 Zeilen aufsplitten')
    for pkg, fc, lines in packages:
        if lines > 3000:
            report.append(f'   - Package "{pkg}": {lines} Zeilen')
    
    report.append('')
    report.append('4. BEST PRACTICES fuer KI-Verarbeitung:')
    report.append('   - Eine Datei = Eine klare Verantwortung')
    report.append('   - Max. 300 Zeilen pro Datei')
    report.append('   - Klare Schnittstellen zwischen Modulen')
    report.append('   - Dokumentation der wichtigsten Funktionen')
    
    return '\n'.join(report)

if __name__ == '__main__':
    report = generate_report()
    with open('ki_gerecht_analyse.txt', 'w', encoding='utf-8') as f:
        f.write(report)
    print('Analyse erstellt: ki_gerecht_analyse.txt')
