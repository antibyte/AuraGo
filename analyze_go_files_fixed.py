#!/usr/bin/env python3
"""
Analyse aller Go-Dateien fuer KI-Verarbeitungsoptimierung
"""

import os
from pathlib import Path
from collections import defaultdict
from dataclasses import dataclass
from typing import List, Tuple

@dataclass
class GoFile:
    path: str
    lines: int
    size_kb: float
    category: str
    
    @property
    def relative_path(self) -> str:
        return self.path.replace('c:\\Users\\andre\\Documents\\repo\\AuraGo\\', '').replace('\\', '/')

# Kategorien fuer KI-Verarbeitung
CATEGORIES = {
    'ideal': (0, 300, '[OK] Ideal', 'Perfekt fuer KI-Verarbeitung'),
    'good': (301, 500, '[OK] Gut', 'Noch gut handhabbar'),
    'acceptable': (501, 800, '[WARN] Akzeptabel', 'Gross, aber noch OK'),
    'large': (801, 1200, '[WARN] Gross', 'Sollte refactored werden'),
    'too_large': (1201, 2000, '[KRITISCH] Zu gross', 'Aufspaltung empfohlen'),
    'critical': (2001, float('inf'), '[KRITISCH] Kritisch', 'Sofortige Aufspaltung noetig'),
}

def categorize(lines: int) -> str:
    for key, (min_lines, max_lines, label, desc) in CATEGORIES.items():
        if min_lines <= lines <= max_lines:
            return key
    return 'critical'

def get_category_label(category: str) -> str:
    return CATEGORIES[category][2]

def get_category_desc(category: str) -> str:
    return CATEGORIES[category][3]

def analyze_go_files():
    root = Path('c:/Users/andre/Documents/repo/AuraGo')
    go_files = list(root.rglob('*.go'))
    
    files_by_category = defaultdict(list)
    all_files = []
    
    total_lines = 0
    total_size = 0
    
    for filepath in go_files:
        if 'vendor' in str(filepath) or 'node_modules' in str(filepath):
            continue
            
        try:
            with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
                content = f.read()
                lines = len(content.split('\n'))
                size_kb = filepath.stat().st_size / 1024
                
                category = categorize(lines)
                go_file = GoFile(str(filepath), lines, size_kb, category)
                
                files_by_category[category].append(go_file)
                all_files.append(go_file)
                
                total_lines += lines
                total_size += size_kb
        except Exception as e:
            pass
    
    # Sortiere nach Groesse
    for cat in files_by_category:
        files_by_category[cat].sort(key=lambda x: x.lines, reverse=True)
    
    all_files.sort(key=lambda x: x.lines, reverse=True)
    
    return files_by_category, all_files, total_lines, total_size, len(go_files)

def generate_split_recommendations(files: List[GoFile]) -> List[Tuple[str, str]]:
    """Generiere spezifische Aufteilungsempfehlungen"""
    recommendations = []
    
    large_files = [f for f in files if f.category in ['too_large', 'critical']]
    
    for gf in large_files:
        path = gf.relative_path
        lines = gf.lines
        
        if 'agent.go' in path and 'internal/agent' in path:
            recommendations.append((
                path,
                f"6.152 Zeilen - Aufteilen in:\n"
                f"  - agent_core.go (Hauptstruktur)\n"
                f"  - agent_tools.go (Tool-Handling)\n"
                f"  - agent_messaging.go (Kommunikation)\n"
                f"  - agent_workflow.go (Workflow-Logik)\n"
                f"  - agent_utils.go (Hilfsfunktionen)"
            ))
        elif 'config.go' in path:
            recommendations.append((
                path,
                f"1.617 Zeilen - Aufteilen in:\n"
                f"  - config_load.go (Laden/Validierung)\n"
                f"  - config_types.go (Strukturen)\n"
                f"  - config_merge.go (Merge-Logik)"
            ))
        elif 'server.go' in path and 'internal/server' in path:
            recommendations.append((
                path,
                f"1.236 Zeilen - Aufteilen in:\n"
                f"  - server_core.go (HTTP-Setup)\n"
                f"  - server_middleware.go (Middleware)\n"
                f"  - server_routes.go (Routing)"
            ))
        elif 'homepage.go' in path:
            recommendations.append((
                path,
                f"1.203 Zeilen - Aufteilen in:\n"
                f"  - homepage_scraper.go (Scraping)\n"
                f"  - homepage_processor.go (Verarbeitung)\n"
                f"  - homepage_storage.go (Speicherung)"
            ))
        elif 'builder.go' in path and 'prompts' in path:
            recommendations.append((
                path,
                f"1.199 Zeilen - Aufteilen in:\n"
                f"  - prompt_templates.go (Templates)\n"
                f"  - prompt_builder.go (Builder-Logik)\n"
                f"  - prompt_utils.go (Hilfsfunktionen)"
            ))
        elif 'handlers.go' in path and 'server' in path:
            recommendations.append((
                path,
                f"1.100 Zeilen - Aufteilen in:\n"
                f"  - handlers_chat.go (Chat-Handler)\n"
                f"  - handlers_api.go (API-Handler)"
            ))
        elif 'short_term.go' in path or 'personality.go' in path:
            recommendations.append((
                path,
                f"{lines} Zeilen - Aufteilen in Core + Operations"
            ))
        elif 'netlify.go' in path or 'docker.go' in path:
            recommendations.append((
                path,
                f"{lines} Zeilen - Aufteilen in Client + Operations"
            ))
        else:
            # Generische Empfehlung
            parts = max(2, lines // 500)
            recommendations.append((
                path,
                f"{lines} Zeilen - Aufteilen in ~{parts} Dateien (je ~{lines//parts} Zeilen)"
            ))
    
    return recommendations

def generate_report():
    files_by_category, all_files, total_lines, total_size, file_count = analyze_go_files()
    
    report = []
    report.append("=" * 80)
    report.append("GO-DATEIEN ANALYSE - KI-VERARBEITUNG OPTIMIERUNG")
    report.append("=" * 80)
    report.append("")
    report.append(f"Gesamtdaten:")
    report.append(f"  Dateien: {file_count}")
    report.append(f"  Gesamtzeilen: {total_lines:,}")
    report.append(f"  Durchschnitt: {total_lines // file_count} Zeilen/Datei")
    report.append(f"  Gesamtgroesse: {total_size:.2f} KB")
    report.append("")
    
    # Verteilung
    report.append("-" * 80)
    report.append("VERTEILUNG NACH KATEGORIEN:")
    report.append("-" * 80)
    report.append("")
    
    for cat in ['ideal', 'good', 'acceptable', 'large', 'too_large', 'critical']:
        files = files_by_category.get(cat, [])
        if files:
            label = get_category_label(cat)
            desc = get_category_desc(cat)
            report.append(f"{label} ({len(files)} Dateien) - {desc}")
            
            if cat in ['too_large', 'critical', 'large']:
                report.append("  Dateien:")
                for gf in files[:5]:
                    report.append(f"    - {gf.relative_path}: {gf.lines} Zeilen")
                if len(files) > 5:
                    report.append(f"    ... und {len(files) - 5} weitere")
            report.append("")
    
    # Top 20 groesste Dateien
    report.append("-" * 80)
    report.append("TOP 20 GROESSTE DATEIEN:")
    report.append("-" * 80)
    report.append("")
    report.append(f"{'Datei':<55} {'Zeilen':>8} {'KB':>8} {'Status':<20}")
    report.append("-" * 80)
    
    for gf in all_files[:20]:
        filename = gf.relative_path
        if len(filename) > 54:
            filename = "..." + filename[-51:]
        status = get_category_label(gf.category)
        report.append(f"{filename:<55} {gf.lines:>8} {gf.size_kb:>8.1f} {status:<20}")
    
    # Aufteilungsempfehlungen
    report.append("")
    report.append("=" * 80)
    report.append("AUSSPALTUNGSEMPFEHLUNGEN")
    report.append("=" * 80)
    report.append("")
    
    recommendations = generate_split_recommendations(all_files)
    
    for path, rec in recommendations:
        report.append(f"-> {path}")
        report.append(f"   {rec}")
        report.append("")
    
    # Prioritaeten
    report.append("-" * 80)
    report.append("PRIORITAETEN FUER REFACTORING:")
    report.append("-" * 80)
    report.append("")
    report.append("1. SOFORT (Kritisch - >2000 Zeilen):")
    critical = files_by_category.get('critical', [])
    if critical:
        for gf in critical:
            report.append(f"   [KRITISCH] {gf.relative_path} ({gf.lines} Zeilen)")
    else:
        report.append("   Keine")
    
    report.append("")
    report.append("2. HOCH (Zu gross - 1200-2000 Zeilen):")
    too_large = files_by_category.get('too_large', [])
    if too_large:
        for gf in too_large:
            report.append(f"   [KRITISCH] {gf.relative_path} ({gf.lines} Zeilen)")
    else:
        report.append("   Keine")
    
    report.append("")
    report.append("3. MITTEL (Gross - 800-1200 Zeilen):")
    large = files_by_category.get('large', [])
    if large:
        for gf in large[:10]:
            report.append(f"   [WARN] {gf.relative_path} ({gf.lines} Zeilen)")
        if len(large) > 10:
            report.append(f"   ... und {len(large) - 10} weitere")
    
    report.append("")
    report.append("=" * 80)
    report.append("ZIEL: Alle Dateien unter 800 Zeilen fuer optimale KI-Verarbeitung")
    report.append("=" * 80)
    
    return "\n".join(report)

if __name__ == "__main__":
    report = generate_report()
    
    # Speichern
    with open("go_analysis_report.txt", "w", encoding="utf-8") as f:
        f.write(report)
    
    print("Bericht erstellt: go_analysis_report.txt")
