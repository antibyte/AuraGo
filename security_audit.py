#!/usr/bin/env python3
"""
Umfassender Security Audit für AuraGo Projekt
"""

import os
import re
import json
from pathlib import Path
from collections import defaultdict
from dataclasses import dataclass, field
from typing import List, Dict, Tuple

@dataclass
class SecurityIssue:
    severity: str  # CRITICAL, HIGH, MEDIUM, LOW, INFO
    category: str
    file: str
    line: int
    description: str
    recommendation: str

class SecurityAuditor:
    def __init__(self, root_path: str):
        self.root = Path(root_path)
        self.issues: List[SecurityIssue] = []
        self.go_files = list(self.root.rglob("*.go"))
        self.config_files = list(self.root.rglob("*.yaml")) + list(self.root.rglob("*.yml")) + list(self.root.rglob("*.json"))
        
    def audit_all(self):
        """Führt alle Audit-Checks durch"""
        self.check_hardcoded_secrets()
        self.check_sql_injection()
        self.check_xss_vulnerabilities()
        self.check_weak_crypto()
        self.check_auth_issues()
        self.check_file_uploads()
        self.check_command_injection()
        self.check_error_handling()
        self.check_cors_issues()
        self.check_docker_security()
        self.check_config_security()
        self.check_dependencies()
        self.check_permission_issues()
        self.check_logging_issues()
        
    def check_hardcoded_secrets(self):
        """Prüft auf hardcodierte Secrets, API-Keys, Passwörter"""
        patterns = {
            'API Key': r'(?i)(api[_-]?key|apikey)\s*[:=]\s*["\'][a-zA-Z0-9_\-]{20,}["\']',
            'Password': r'(?i)(password|passwd|pwd)\s*[:=]\s*["\'][^"\']{4,}["\']',
            'Secret': r'(?i)(secret|token|auth_token)\s*[:=]\s*["\'][a-zA-Z0-9_\-]{10,}["\']',
            'AWS Key': r'AKIA[0-9A-Z]{16}',
            'Private Key': r'-----BEGIN (RSA |DSA |EC |OPENSSH )?PRIVATE KEY-----',
            'JWT Secret': r'(?i)(jwt|signature)[_-]?secret\s*[:=]\s*["\'][^"\']+["\']',
            'Database URL': r'(?i)(mongodb|postgres|mysql)://[^\s\"\']+:[^\s\"\']+@[^\s\"\']+',
        }
        
        exclude_files = ['vendor', 'node_modules', '.git', 'security_audit.py']
        
        for filepath in self.go_files + self.config_files:
            if any(ex in str(filepath) for ex in exclude_files):
                continue
                
            try:
                with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
                    content = f.read()
                    lines = content.split('\n')
                    
                for secret_type, pattern in patterns.items():
                    for i, line in enumerate(lines, 1):
                        if re.search(pattern, line):
                            # Prüfe ob es ein Beispiel/Placeholder ist
                            if not self._is_placeholder(line):
                                self.issues.append(SecurityIssue(
                                    severity='CRITICAL',
                                    category='Hardcoded Secret',
                                    file=str(filepath.relative_to(self.root)),
                                    line=i,
                                    description=f'Potenziell hardcodiertes {secret_type} gefunden',
                                    recommendation=f'{secret_type} in Umgebungsvariablen oder Secrets-Manager auslagern'
                                ))
            except Exception:
                pass
    
    def _is_placeholder(self, line: str) -> bool:
        """Prüft ob ein Wert ein Placeholder/Beispiel ist"""
        placeholders = ['example', 'placeholder', 'your-', 'change-me', 'xxx', '***', 'password123', 'admin', 'test']
        line_lower = line.lower()
        return any(p in line_lower for p in placeholders)
    
    def check_sql_injection(self):
        """Prüft auf SQL-Injection Schwachstellen"""
        patterns = [
            (r'(?i)(Query|Exec|Prepare)\s*\(\s*["\'].*\+', 'String-Konkatenation in SQL-Query'),
            (r'(?i)fmt\.Sprintf\s*\(\s*["\'].*SELECT|INSERT|UPDATE|DELETE', 'Sprintf in SQL-Query'),
            (r'(?i)(WHERE|AND|OR)\s+.*=\s*["\'].*\+', 'Dynamische WHERE-Klausel'),
        ]
        
        for filepath in self.go_files:
            try:
                with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
                    lines = f.readlines()
                    
                for i, line in enumerate(lines, 1):
                    for pattern, desc in patterns:
                        if re.search(pattern, line):
                            self.issues.append(SecurityIssue(
                                severity='HIGH',
                                category='SQL Injection',
                                file=str(filepath.relative_to(self.root)),
                                line=i,
                                description=desc,
                                recommendation='Verwende parameterized queries/prepared statements'
                            ))
            except Exception:
                pass
    
    def check_xss_vulnerabilities(self):
        """Prüft auf XSS Schwachstellen"""
        patterns = [
            (r'(?i)\.Write\s*\(\s*.*\+.*r\.URL\.Query\(\)', 'Unescaped Query-Parameter in Response'),
            (r'(?i)template\.HTML\s*\(\s*.*\)', 'template.HTML ohne Sanitization'),
            (r'(?i)\.InnerHTML\s*=', 'InnerHTML Zuweisung'),
            (r'(?i)document\.write\s*\(', 'document.write()'),
        ]
        
        for filepath in self.go_files:
            try:
                with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
                    lines = f.readlines()
                    
                for i, line in enumerate(lines, 1):
                    for pattern, desc in patterns:
                        if re.search(pattern, line):
                            self.issues.append(SecurityIssue(
                                severity='HIGH',
                                category='XSS Vulnerability',
                                file=str(filepath.relative_to(self.root)),
                                line=i,
                                description=desc,
                                recommendation='Eingaben validieren und escapen, Content Security Policy verwenden'
                            ))
            except Exception:
                pass
    
    def check_weak_crypto(self):
        """Prüft auf schwache Kryptographie"""
        patterns = [
            (r'(?i)md5\.', 'MD5 Hash verwendet (unsicher)'),
            (r'(?i)sha1\.', 'SHA1 Hash verwendet (unsicher)'),
            (r'(?i)des\.NewCipher', 'DES Verschlüsselung (unsicher)'),
            (r'(?i)rc4\.NewCipher', 'RC4 Verschlüsselung (unsicher)'),
            (r'(?i)rand\.Int|rand\.Seed', 'Unsichere Zufallszahlengenerierung'),
            (r'(?i)tls\.Config.*InsecureSkipVerify.*true', 'TLS-Zertifikatsprüfung deaktiviert'),
            (r'(?i)crypto/tls.*VersionSSL30', 'SSLv3 verwendet (veraltet)'),
        ]
        
        for filepath in self.go_files:
            try:
                with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
                    lines = f.readlines()
                    
                for i, line in enumerate(lines, 1):
                    for pattern, desc in patterns:
                        if re.search(pattern, line):
                            self.issues.append(SecurityIssue(
                                severity='HIGH',
                                category='Weak Cryptography',
                                file=str(filepath.relative_to(self.root)),
                                line=i,
                                description=desc,
                                recommendation='Verwende SHA-256/SHA-3, AES-GCM, crypto/rand, TLS 1.3'
                            ))
            except Exception:
                pass
    
    def check_auth_issues(self):
        """Prüft auf Authentifizierungsprobleme"""
        patterns = [
            (r'(?i)session.*timeout.*\d+.*minute', 'Session-Timeout zu lang'),
            (r'(?i)jwt.*expiry.*\d+.*day', 'JWT zu lange gültig'),
            (r'(?i)password.*length.*<\s*8', 'Passwortlänge zu kurz'),
            (r'(?i)bcrypt.*DefaultCost|bcrypt.*Cost.*<\s*10', 'Bcrypt-Kosten zu niedrig'),
            (r'(?i)auth.*bypass|skip.*auth', 'Authentifizierungsumgehung'),
        ]
        
        for filepath in self.go_files:
            try:
                with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
                    lines = f.readlines()
                    
                for i, line in enumerate(lines, 1):
                    for pattern, desc in patterns:
                        if re.search(pattern, line):
                            self.issues.append(SecurityIssue(
                                severity='MEDIUM',
                                category='Authentication Issue',
                                file=str(filepath.relative_to(self.root)),
                                line=i,
                                description=desc,
                                recommendation='Implementiere sichere Auth-Praktiken (kurze Sessions, starke Passwörter)'
                            ))
            except Exception:
                pass
    
    def check_command_injection(self):
        """Prüft auf Command Injection"""
        dangerous_funcs = [
            (r'(?i)exec\.Command\s*\(\s*[^,]+,\s*[^)]+\+', 'Dynamische Command-Argumente'),
            (r'(?i)os\.Exec|syscall\.Exec', 'System-Command-Ausführung'),
            (r'(?i)eval\s*\(', 'Eval-Aufruf'),
            (r'(?i)sh\s+-c', 'Shell-Execution'),
        ]
        
        for filepath in self.go_files:
            try:
                with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
                    lines = f.readlines()
                    
                for i, line in enumerate(lines, 1):
                    for pattern, desc in dangerous_funcs:
                        if re.search(pattern, line):
                            self.issues.append(SecurityIssue(
                                severity='CRITICAL',
                                category='Command Injection',
                                file=str(filepath.relative_to(self.root)),
                                line=i,
                                description=desc,
                                recommendation='Validiere alle Eingaben, verwende Whitelists, keine Shell-Execution'
                            ))
            except Exception:
                pass
    
    def check_file_uploads(self):
        """Prüft auf unsichere Datei-Uploads"""
        patterns = [
            (r'(?i)multipart\.FormFile', 'Datei-Upload ohne Validierung'),
            (r'(?i)SaveUploadedFile|ioutil\.WriteFile.*upload', 'Datei-Upload-Verarbeitung'),
        ]
        
        for filepath in self.go_files:
            try:
                with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
                    content = f.read()
                    lines = content.split('\n')
                    
                has_upload = False
                has_validation = False
                
                for i, line in enumerate(lines):
                    if re.search(r'(?i)multipart\.FormFile|SaveUploadedFile', line):
                        has_upload = True
                    if re.search(r'(?i)\.extension|mimetype|content.type|magic', line):
                        has_validation = True
                
                if has_upload and not has_validation:
                    self.issues.append(SecurityIssue(
                        severity='HIGH',
                        category='Insecure File Upload',
                        file=str(filepath.relative_to(self.root)),
                        line=0,
                        description='Datei-Upload ohne erkennbare Validierung',
                        recommendation='Validiere Dateityp, Größe, MIME-Type; speichere außerhalb Document-Root'
                    ))
            except Exception:
                pass
    
    def check_error_handling(self):
        """Prüft auf unsichere Fehlerbehandlung"""
        patterns = [
            (r'(?i)fmt\.Println\s*\(\s*err', 'Fehlerdetails werden ausgegeben'),
            (r'(?i)log\.Printf\s*\(\s*["\'].*%s.*["\'].*err', 'Fehlerdetails im Log'),
            (r'(?i)return.*err\.Error\(\)', 'Interne Fehlerdetails an Client'),
        ]
        
        for filepath in self.go_files:
            try:
                with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
                    lines = f.readlines()
                    
                for i, line in enumerate(lines, 1):
                    for pattern, desc in patterns:
                        if re.search(pattern, line):
                            self.issues.append(SecurityIssue(
                                severity='MEDIUM',
                                category='Information Disclosure',
                                file=str(filepath.relative_to(self.root)),
                                line=i,
                                description=desc,
                                recommendation='Generische Fehlermeldungen an Client, Details nur intern loggen'
                            ))
            except Exception:
                pass
    
    def check_cors_issues(self):
        """Prüft auf CORS-Misconfiguration"""
        patterns = [
            (r'(?i)AllowAllOrigins.*true', 'CORS erlaubt alle Origins'),
            (r'(?i)AllowOrigins.*\*', 'Wildcard in CORS-Origins'),
            (r'(?i)Access-Control-Allow-Origin.*\*', 'Wildcard CORS-Header'),
        ]
        
        for filepath in self.go_files:
            try:
                with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
                    lines = f.readlines()
                    
                for i, line in enumerate(lines, 1):
                    for pattern, desc in patterns:
                        if re.search(pattern, line):
                            self.issues.append(SecurityIssue(
                                severity='MEDIUM',
                                category='CORS Misconfiguration',
                                file=str(filepath.relative_to(self.root)),
                                line=i,
                                description=desc,
                                recommendation='Spezifische Origins whitelisten, keine Wildcards'
                            ))
            except Exception:
                pass
    
    def check_docker_security(self):
        """Prüft Docker-Sicherheit"""
        dockerfiles = list(self.root.rglob("Dockerfile*"))
        
        for dockerfile in dockerfiles:
            try:
                with open(dockerfile, 'r', encoding='utf-8', errors='ignore') as f:
                    content = f.read()
                    lines = content.split('\n')
                
                # Prüfe auf Root-User
                if 'USER' not in content:
                    self.issues.append(SecurityIssue(
                        severity='MEDIUM',
                        category='Docker Security',
                        file=str(dockerfile.relative_to(self.root)),
                        line=0,
                        description='Container läuft als root (kein USER Statement)',
                        recommendation='Füge "USER app" hinzu für nicht-privilegierten User'
                    ))
                
                # Prüfe auf Secrets im Build
                for i, line in enumerate(lines, 1):
                    if re.search(r'(?i)(password|secret|key)\s*=\s*[^\s]+', line) and 'ARG' in line:
                        self.issues.append(SecurityIssue(
                            severity='HIGH',
                            category='Docker Security',
                            file=str(dockerfile.relative_to(self.root)),
                            line=i,
                            description='Potenzielles Secret in Docker-Build-Argument',
                            recommendation='Verwende Buildkit Secrets oder Runtime-Variablen'
                        ))
            except Exception:
                pass
    
    def check_config_security(self):
        """Prüft Konfigurationsdateien auf Sicherheitsprobleme"""
        config_files = list(self.root.glob("*.yaml")) + list(self.root.glob("*.yml")) + list(self.root.glob("config.*"))
        
        for config_file in config_files:
            try:
                with open(config_file, 'r', encoding='utf-8', errors='ignore') as f:
                    content = f.read()
                
                # Prüfe auf Debug-Modus
                if re.search(r'(?i)debug\s*:\s*true', content):
                    self.issues.append(SecurityIssue(
                        severity='MEDIUM',
                        category='Configuration Issue',
                        file=str(config_file.relative_to(self.root)),
                        line=0,
                        description='Debug-Modus ist aktiviert',
                        recommendation='Debug-Modus in Produktion deaktivieren'
                    ))
                
                # Prüfe auf fehlende HTTPS
                if re.search(r'(?i)https?://localhost|http://[^/]', content) and not re.search(r'(?i)development|dev', str(config_file)):
                    self.issues.append(SecurityIssue(
                        severity='MEDIUM',
                        category='Configuration Issue',
                        file=str(config_file.relative_to(self.root)),
                        line=0,
                        description='HTTP statt HTTPS konfiguriert',
                        recommendation='HTTPS für alle Kommunikation erzwingen'
                    ))
            except Exception:
                pass
    
    def check_dependencies(self):
        """Prüft Abhängigkeiten"""
        go_mod = self.root / 'go.mod'
        if go_mod.exists():
            try:
                with open(go_mod, 'r', encoding='utf-8') as f:
                    content = f.read()
                
                # Liste bekannter verwundbarer Pakete (Beispiele)
                vulnerable_packages = [
                    ('github.com/gin-gonic/gin', '1.7.0', 'CVE-2020-28483'),
                    ('github.com/dgrijalva/jwt-go', '3.2.0', 'CVE-2020-26160'),
                ]
                
                for pkg, version, cve in vulnerable_packages:
                    if pkg in content and version in content:
                        self.issues.append(SecurityIssue(
                            severity='HIGH',
                            category='Vulnerable Dependency',
                            file='go.mod',
                            line=0,
                            description=f'Bekannte verwundbare Abhängigkeit: {pkg}@{version} ({cve})',
                            recommendation=f'Upgrade auf neueste Version von {pkg}'
                        ))
            except Exception:
                pass
    
    def check_permission_issues(self):
        """Prüft auf Berechtigungsprobleme"""
        # Prüfe auf 777 Berechtigungen
        for path in self.root.rglob("*"):
            try:
                if path.is_file():
                    stat = path.stat()
                    mode = oct(stat.st_mode)[-3:]
                    if mode == '777':
                        self.issues.append(SecurityIssue(
                            severity='LOW',
                            category='Permission Issue',
                            file=str(path.relative_to(self.root)),
                            line=0,
                            description=f'Datei hat 777 Berechtigungen',
                            recommendation='Berechtigungen auf 644 (Dateien) oder 755 (Verzeichnisse) setzen'
                        ))
            except Exception:
                pass
    
    def check_logging_issues(self):
        """Prüft auf Logging-Probleme"""
        # Prüfe ob sensitive Daten geloggt werden
        sensitive_patterns = [
            (r'(?i)log.*password|log.*secret|log.*token', 'Sensiblen Daten werden geloggt'),
            (r'(?i)log.*credit.*card|log.*ssn|log.*social', 'PII wird geloggt'),
        ]
        
        for filepath in self.go_files:
            try:
                with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
                    lines = f.readlines()
                    
                for i, line in enumerate(lines, 1):
                    for pattern, desc in sensitive_patterns:
                        if re.search(pattern, line):
                            self.issues.append(SecurityIssue(
                                severity='HIGH',
                                category='Sensitive Data Logging',
                                file=str(filepath.relative_to(self.root)),
                                line=i,
                                description=desc,
                                recommendation='Keine sensitiven Daten loggen, verwende Maskierung'
                            ))
            except Exception:
                pass
    
    def generate_report(self) -> str:
        """Generiert den Audit-Bericht"""
        self.audit_all()
        
        report = []
        report.append('='*80)
        report.append('SECURITY AUDIT REPORT - AuraGo')
        report.append('='*80)
        report.append('')
        
        # Zusammenfassung
        severity_counts = defaultdict(int)
        category_counts = defaultdict(int)
        
        for issue in self.issues:
            severity_counts[issue.severity] += 1
            category_counts[issue.category] += 1
        
        report.append('ZUSAMMENFASSUNG')
        report.append('-'*80)
        report.append(f'Geprüfte Go-Dateien: {len(self.go_files)}')
        report.append(f'Gefundene Sicherheitsprobleme: {len(self.issues)}')
        report.append('')
        
        report.append('Nach Schweregrad:')
        for sev in ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'INFO']:
            count = severity_counts.get(sev, 0)
            symbol = '🔴' if sev == 'CRITICAL' else '🟠' if sev == 'HIGH' else '🟡' if sev == 'MEDIUM' else '🔵'
            report.append(f'  {symbol} {sev}: {count}')
        report.append('')
        
        report.append('Nach Kategorie:')
        for cat, count in sorted(category_counts.items(), key=lambda x: x[1], reverse=True):
            report.append(f'  {cat}: {count}')
        report.append('')
        
        # Detaillierte Probleme
        if self.issues:
            report.append('='*80)
            report.append('DETAILLIERTE FINDINGS')
            report.append('='*80)
            report.append('')
            
            # Sortiere nach Schweregrad
            severity_order = {'CRITICAL': 0, 'HIGH': 1, 'MEDIUM': 2, 'LOW': 3, 'INFO': 4}
            sorted_issues = sorted(self.issues, key=lambda x: severity_order.get(x.severity, 5))
            
            for issue in sorted_issues:
                symbol = '🔴' if issue.severity == 'CRITICAL' else '🟠' if issue.severity == 'HIGH' else '🟡' if issue.severity == 'MEDIUM' else '🔵'
                report.append(f'{symbol} [{issue.severity}] {issue.category}')
                report.append(f'  Datei: {issue.file}:{issue.line}')
                report.append(f'  Beschreibung: {issue.description}')
                report.append(f'  Empfehlung: {issue.recommendation}')
                report.append('')
        
        # Empfehlungen
        report.append('='*80)
        report.append('SICHERHEITSEMPFEHLUNGEN')
        report.append('='*80)
        report.append('')
        
        report.append('1. DRingende Maßnahmen (Critical/High):')
        report.append('   - Alle hardcodierten Secrets entfernen')
        report.append('   - SQL-Injection-Schwachstellen beheben')
        report.append('   - Input-Validierung implementieren')
        report.append('')
        
        report.append('2. Architektur-Empfehlungen:')
        report.append('   - Secrets-Manager verwenden (Vault, AWS Secrets Manager)')
        report.append('   - Zentrale Input-Validierung')
        report.append('   - Content Security Policy implementieren')
        report.append('   - Rate-Limiting für APIs')
        report.append('')
        
        report.append('3. Prozess-Empfehlungen:')
        report.append('   - Regelmäßige Security-Scans in CI/CD')
        report.append('   - Dependabot für Abhängigkeiten aktivieren')
        report.append('   - Security-Code-Reviews')
        report.append('   - Penetration-Tests vor Releases')
        report.append('')
        
        report.append('='*80)
        report.append('ENDE DES SECURITY AUDITS')
        report.append('='*80)
        
        return '\n'.join(report)

if __name__ == '__main__':
    auditor = SecurityAuditor('.')
    report = auditor.generate_report()
    
    with open('security_audit_report.txt', 'w', encoding='utf-8') as f:
        f.write(report)
    
    print('Security Audit abgeschlossen: security_audit_report.txt')
