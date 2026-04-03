import json
from pathlib import Path
import re

# Specific word replacements for German umlaut fixes
REPLACEMENTS = [
    (r'fuer ', 'für '),
    (r'ueber', 'über'),
    (r'Ueber', 'Über'),
    (r'Moeglichkeit', 'Möglichkeit'),
    (r'moeglich', 'möglich'),
    (r'guenstig', 'günstig'),
    (r'abhaengig', 'abhängig'),
    (r'vollstaendig', 'vollständig'),
    (r'ausschoepf', 'ausschöpf'),
    (r'Gespraech', 'Gespräch'),
    (r'gespraech', 'gespräch'),
    (r'veraend', 'veränd'),
    (r'Veraend', 'Veränd'),
    (r'zurueck', 'zurück'),
    (r'Zurueck', 'Zurück'),
    (r'oeffn', 'öffn'),
    (r'Oeffn', 'Öffn'),
    (r'koenn', 'könn'),
    (r'Koenn', 'Könn'),
    (r'schoen', 'schön'),
    (r'Schoen', 'Schön'),
    (r'frueh', 'früh'),
    (r'Frueh', 'Früh'),
    (r'waehr', 'währ'),
    (r'Waehr', 'Währ'),
    (r'entspraech', 'entspräch'),
    (r'Entspraech', 'Entspräch'),
    (r'naehr', 'nähr'),
    (r'Naehr', 'Nähr'),
    (r'haeufig', 'häufig'),
    (r'Haeufig', 'Häufig'),
    (r'oeffentlich', 'öffentlich'),
    (r'Oeffentlich', 'Öffentlich'),
    (r'beschraenk', 'beschränk'),
    (r'Beschraenk', 'Beschränk'),
    (r'aufzaehl', 'aufzähl'),
    (r'Aufzaehl', 'Aufzähl'),
    (r'durchfuehr', 'durchführ'),
    (r'Durchfuehr', 'Durchführ'),
    (r'fortgeschritt', 'fortgeschritt'),  # already correct
    (r'verbesser', 'verbesser'),  # already correct
    (r'zusaetz', 'zusätz'),
    (r'Zusaetz', 'Zusätz'),
    (r'zugreif', 'zugreif'),  # already correct
    (r'gewuensch', 'gewünsch'),
    (r'Gewuensch', 'Gewünsch'),
    (r'ausgewaehl', 'ausgewähl'),
    (r'Ausgewaehl', 'Ausgewähl'),
    (r'befaellig', 'befällig'),  # rare
    (r'baeck', 'bäck'),
    (r'loesch', 'lösch'),
    (r'Loesch', 'Lösch'),
    (r'eroeffn', 'eröffn'),
    (r'Eroeffn', 'Eröffn'),
    (r'ueberwach', 'überwach'),
    (r'Ueberwach', 'Überwach'),
    (r'ueberpruef', 'überprüf'),
    (r'Ueberpruef', 'Überprüf'),
    (r'uebertrag', 'übertrag'),
    (r'Uebertrag', 'Übertrag'),
    (r'uebernehmbar', 'übernehmbar'),
    (r'Uebernehmbar', 'Übernehmbar'),
    (r'uebernimm', 'übernimm'),
    (r'Uebernimm', 'Übernimm'),
    (r'uebernommen', 'übernommen'),
    (r'Uebernommen', 'Übernommen'),
    (r'uebernehmen', 'übernehmen'),
    (r'Uebernehmen', 'Übernehmen'),
    (r'zufaellig', 'zufällig'),
    (r'Zufaellig', 'Zufällig'),
    (r'oeffnungs', 'öffnungs'),
    (r'Oeffnungs', 'Öffnungs'),
    (r'schliess', 'schließ'),
    (r'Schliess', 'Schließ'),
    (r'entschliess', 'entschließ'),
    (r'Entschliess', 'Entschließ'),
    (r'verschliess', 'verschließ'),
    (r'Verschliess', 'Verschließ'),
    (r'beschliess', 'beschließ'),
    (r'Beschliess', 'Beschließ'),
    (r'sauber', 'sauber'),  # correct
    (r'naechst', 'nächst'),
    (r'Naechst', 'Nächst'),
    (r'tatsaechlich', 'tatsächlich'),
    (r'Tatsaechlich', 'Tatsächlich'),
    (r'einfuehr', 'einführ'),
    (r'Einfuehr', 'Einführ'),
    (r'ausfuehr', 'ausführ'),
    (r'Ausfuehr', 'Ausführ'),
    (r'aufuehr', 'aufführ'),  # careful
    (r'duerf', 'dürf'),
    (r'Duerf', 'Dürf'),
    (r'genueg', 'genüg'),
    (r'Genueg', 'Genüg'),
    (r'gefuehl', 'gefühl'),
    (r'Gefuehl', 'Gefühl'),
    (r'lueck', 'lück'),
    (r'Lueck', 'Lück'),
    (r'fuehr', 'führ'),
    (r'Fuehr', 'Führ'),
    (r'zuruecksetz', 'zurücksetz'),
    (r'Zuruecksetz', 'Zurücksetz'),
    (r'zurueckgesetzt', 'zurückgesetzt'),
    (r'zuruecksetzen', 'zurücksetzen'),
    (r'bevorzug', 'bevorzug'),  # correct
    (r'vergleich', 'vergleich'),  # correct
    (r'erreich', 'erreich'),  # correct
    (r'schwer', 'schwer'),  # correct
    (r'voellig', 'völlig'),
    (r'Voellig', 'Völlig'),
    (r'haendisch', 'händisch'),
    (r'Haendisch', 'Händisch'),
    (r'haendler', 'händler'),
    (r'Haendler', 'Händler'),
    (r'manuell', 'manuell'),  # correct
    (r'aktualisier', 'aktualisier'),  # correct
    (r'beispiel', 'beispiel'),  # correct
    (r'eingeschraenk', 'eingeschränk'),
    (r'Eingeschraenk', 'Eingeschränk'),
    (r'beschraenkung', 'Beschränkung'),
    (r'Beschraenkung', 'Beschränkung'),
    (r'angeblich', 'angeblich'),  # correct
    (r'beeinfluss', 'beeinfluss'),  # correct
    (r'beeintraecht', 'beeinträcht'),
    (r'Beeintraecht', 'Beeinträcht'),
    (r'ueberschreit', 'überschreit'),
    (r'Ueberschreit', 'Überschreit'),
    (r'ueberschritten', 'überschritten'),
    (r'ueberschritten', 'überschritten'),
    (r'uebersetz', 'übersetz'),
    (r'Uebersetz', 'Übersetz'),
    (r'ueberpruef', 'überprüf'),
    (r'uebersicht', 'übersicht'),
    (r'Uebersicht', 'Übersicht'),
    (r'ueberpruef', 'überprüf'),
    (r'ueberwach', 'überwach'),
    (r'ueberzeug', 'überzeug'),
    (r'Ueberzeug', 'Überzeug'),
    (r'zusaetzlich', 'zusätzlich'),
    (r'zusaetzliche', 'zusätzliche'),
    (r'zurueckgegriffen', 'zurückgegriffen'),
    (r'zurueckgekehrt', 'zurückgekehrt'),
    (r'zurueckgesetzt', 'zurückgesetzt'),
    (r'zuruecksetzen', 'zurücksetzen'),
    (r'zurueckzufuehren', 'zurückzuführen'),
    (r'zurueckzufuhren', 'zurückzuführen'),
]

fixed_files = []

for base in [Path('ui/lang/config'), Path('ui/lang/dashboard')]:
    for path in base.rglob('de.json'):
        with open(path, 'r', encoding='utf-8') as f:
            text = f.read()
        
        original = text
        for pattern, replacement in REPLACEMENTS:
            text = re.sub(pattern, replacement, text)
        
        if text != original:
            with open(path, 'w', encoding='utf-8') as f:
                f.write(text)
            fixed_files.append(str(path))

print(f"Fixed {len(fixed_files)} files:")
for f in fixed_files:
    print(f"  {f}")
