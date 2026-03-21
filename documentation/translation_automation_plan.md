# Translation Automation Plan

## Ziel

Automatische Übersetzung aller UI-Texte aus der deutschen Vorlage (`de.json`) in alle 15 Zielsprachen, inklusive Korrektur bestehender fehlerhafter Übersetzungen. Verwendung von Google Translate über die kostenlose `deep-translator` Python-Bibliothek (bereits installiert).

---

## Bestandsaufnahme

### Struktur
- **11 Übersetzungs-Verzeichnisse**: `chat`, `cheatsheets`, `common`, `config`, `dashboard`, `gallery`, `help`, `invasion`, `media`, `missions`, `setup`
- **16 Sprachen**: cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh
- **1.193 Keys** pro Sprache (in `de.json`)
- **~19.000 Übersetzungen** gesamt (16 × 1.193)

### Bekannte Probleme
| Problem | Beispiele | Umfang |
|---------|-----------|--------|
| Unübersetzte Werte (identisch mit EN) | `gallery/fr.json`: 19/19 = EN | Gallery komplett, Dashboard teilweise |
| Werte teilweise identisch mit DE | `dashboard/nl.json`: 60/287 = DE | NL, FR besonders betroffen |
| Englische Wörter in fremden Sprachen | `help/es.json` enthält "Manage", "enabled" | Vereinzelt in help, missions, gallery |
| Uneinheitliche Übersetzungsqualität | Manche via AI, manche via Copy-Paste | Alle Sprachen außer DE |

### Referenz-Hierarchie
```
de.json  →  Primäre Quelle (handgepflegt, maßgeblich)
en.json  →  Sekundäre Quelle (manuell gepflegt, als Kontext nutzbar)
*.json   →  Zielsprachen (automatisch aus DE übersetzt)
```

---

## Technologie

### deep-translator + Google Translate
- **Paket**: `deep-translator` 1.11.4 (bereits in `.venv` installiert)
- **Backend**: Google Translate (kostenlos, kein API-Key nötig)
- **Batch-Fähig**: `translate_batch()` für mehrere Texte gleichzeitig
- **Rate Limits**: ~100 Requests/Minute, 5.000 Zeichen/Request → ausreichend für ~19K Texte

### Sprach-Mapping
| AuraGo Code | Google Code |
|-------------|-------------|
| cs | cs |
| da | da |
| de | de (Quelle) |
| el | el |
| en | en |
| es | es |
| fr | fr |
| hi | hi |
| it | it |
| ja | ja |
| nl | nl |
| no | no |
| pl | pl |
| pt | pt |
| sv | sv |
| zh | zh-CN |

---

## Ablauf des Scripts

### Phase 1: Laden & Analysieren
1. Alle `de.json` aus jedem Verzeichnis laden → Master-Keys
2. Alle `{lang}.json` laden → bestehende Übersetzungen
3. Für jedes Key-Value-Paar feststellen:
   - **Fehlt** → muss übersetzt werden
   - **Identisch mit DE** → wahrscheinlich nie übersetzt → neu übersetzen
   - **Identisch mit EN** → wahrscheinlich Fallback → neu übersetzen
   - **Enthält Wörter anderer Sprachen** → verdächtig → neu übersetzen
   - **Bereits korrekt** → beibehalten

### Phase 2: Übersetzen
1. Pro Sprache + Verzeichnis: Alle zu übersetzenden Werte sammeln
2. Batch-Übersetzung via `GoogleTranslator.translate_batch()` (max. 50 pro Batch wegen Rate Limits)
3. Zwischen Batches 1 Sekunde Pause
4. Emojis und Platzhalter (`{0}`, `%s`, HTML-Tags) erhalten:
   - Vor Übersetzung extrahieren
   - Nach Übersetzung wieder einfügen
5. Spezielle Begriffe nicht übersetzen:
   - Produktnamen: "AuraGo", "Fritz!Box", "Docker", "Proxmox", "Home Assistant", "Telegram", "Discord", "MQTT"
   - Technische Terme: "SSH", "API", "SSL/TLS", "HTTPS", "cron", "webhook"

### Phase 3: Validierung
1. **Vollständigkeitscheck**: Alle Keys aus DE vorhanden?
2. **Leerstring-Check**: Keine leeren Übersetzungen?
3. **Platzhalter-Check**: Gleiche Platzhalter wie in DE?
4. **Emoji-Check**: Führende Emojis beibehalten?
5. **Längen-Check**: Übersetzung nicht >3x länger als Original?
6. **Selbsttest**: Kein DE/EN-Text mehr in fremden Sprachen?

### Phase 4: Schreiben
1. JSON alphabetisch nach Keys sortieren
2. UTF-8 ohne BOM schreiben
3. Konsistente JSON-Formatierung (2-Space-Indent)
4. Diff-Report erstellen: Welche Keys wurden geändert?

---

## Script-Design

### Datei: `scripts/translate_ui.py`

```
scripts/
  translate_ui.py          # Hauptscript
  translation_report.json  # Generierter Bericht (in .gitignore)
```

### Kommandozeilen-Interface

```bash
# Alles neu übersetzen (Force-Modus)
python scripts/translate_ui.py --force

# Nur fehlende/verdächtige übersetzen (Standard)
python scripts/translate_ui.py

# Nur bestimmte Sprache
python scripts/translate_ui.py --lang fr

# Nur bestimmtes Verzeichnis
python scripts/translate_ui.py --dir missions

# Nur analysieren, nichts schreiben (Dry-Run)
python scripts/translate_ui.py --dry-run

# Bestehende Übersetzungen validieren
python scripts/translate_ui.py --validate-only

# Verbose-Ausgabe
python scripts/translate_ui.py --verbose
```

### Kernlogik (Pseudocode)

```python
GOOGLE_LANG_MAP = {"zh": "zh-CN", ...}  # Rest 1:1
KEEP_WORDS = {"AuraGo", "Fritz!Box", "Docker", ...}
EMOJI_PATTERN = re.compile(r'^(\p{Emoji}+\s*)')
PLACEHOLDER_PATTERN = re.compile(r'\{[^}]+\}|%[sd]')

def should_retranslate(value, de_value, en_value, lang):
    """Prüft ob ein Wert neu übersetzt werden muss."""
    if not value or value == de_value:
        return True  # Fehlt oder identisch mit Deutsch
    if value == en_value and lang != 'en':
        return True  # Englischer Fallback
    if contains_foreign_words(value, lang):
        return True  # Falsche Sprache erkannt
    return False

def translate_batch_safe(texts, source, target, batch_size=50):
    """Batch-Übersetzung mit Rate-Limiting und Retry."""
    results = []
    for i in range(0, len(texts), batch_size):
        batch = texts[i:i+batch_size]
        # Emojis/Platzhalter extrahieren
        cleaned, metadata = extract_special(batch)
        # Übersetzen
        translated = GoogleTranslator(source=source, target=target)
                     .translate_batch(cleaned)
        # Emojis/Platzhalter wiederherstellen
        restored = restore_special(translated, metadata)
        results.extend(restored)
        time.sleep(1)  # Rate-Limit
    return results

def process_directory(dir_name, langs, force=False):
    """Verarbeitet ein Übersetzungsverzeichnis."""
    de = load_json(f"ui/lang/{dir_name}/de.json")
    en = load_json(f"ui/lang/{dir_name}/en.json")
    
    for lang in langs:
        existing = load_json(f"ui/lang/{dir_name}/{lang}.json")
        to_translate = {}
        
        for key, de_value in de.items():
            en_value = en.get(key, "")
            current = existing.get(key, "")
            
            if force or should_retranslate(current, de_value, en_value, lang):
                to_translate[key] = de_value
        
        if to_translate:
            keys = list(to_translate.keys())
            values = list(to_translate.values())
            translated = translate_batch_safe(values, 'de', LANG_MAP[lang])
            
            for key, trans in zip(keys, translated):
                existing[key] = trans
        
        save_json(f"ui/lang/{dir_name}/{lang}.json", existing)
```

### Spezialbehandlung für en.json

Englisch wird **separat** behandelt:
- EN-Übersetzungen sind teilweise handgepflegt und hochwertig
- Nur Keys die fehlen oder offensichtlich Deutsch enthalten werden übersetzt
- `--force` überspringt EN standardmäßig (separates `--include-en` Flag nötig)

---

## Emoji- und Platzhalter-Schutz

Viele Values beginnen mit Emojis (z.B. `"🔌 Gerät verbunden"`) oder enthalten Platzhalter.

### Strategie
```python
def extract_special(text):
    """Extrahiert Emojis am Anfang und Platzhalter."""
    prefix_emoji = ""
    # Führende Emojis extrahieren
    match = re.match(r'^([\U0001F300-\U0001FAFF\u2600-\u27BF\u2B50]+\s*)', text)
    if match:
        prefix_emoji = match.group(1)
        text = text[len(prefix_emoji):]
    
    # Platzhalter durch Marker ersetzen
    placeholders = []
    def save_placeholder(m):
        placeholders.append(m.group(0))
        return f"XLPH{len(placeholders)}X"
    
    text = re.sub(r'\{[^}]+\}|%[sd]|<[^>]+>', save_placeholder, text)
    
    return text, {"emoji": prefix_emoji, "placeholders": placeholders}

def restore_special(text, metadata):
    """Stellt Emojis und Platzhalter wieder her."""
    # Platzhalter zurück
    for i, ph in enumerate(metadata["placeholders"], 1):
        text = text.replace(f"XLPH{i}X", ph)
    # Emoji vorne dran
    return metadata["emoji"] + text
```

---

## Rate-Limiting & Fehlerbehandlung

```python
MAX_RETRIES = 3
BATCH_SIZE = 50
SLEEP_BETWEEN_BATCHES = 1.0  # Sekunden
SLEEP_ON_RATE_LIMIT = 30.0   # Sekunden

def translate_with_retry(translator, batch):
    for attempt in range(MAX_RETRIES):
        try:
            return translator.translate_batch(batch)
        except Exception as e:
            if "429" in str(e) or "rate" in str(e).lower():
                time.sleep(SLEEP_ON_RATE_LIMIT)
            else:
                time.sleep(2 ** attempt)
    raise RuntimeError(f"Translation failed after {MAX_RETRIES} retries")
```

---

## Report-Ausgabe

Nach Durchlauf generiert das Script einen Report:

```json
{
  "timestamp": "2026-03-21T14:30:00Z",
  "mode": "smart",
  "stats": {
    "total_keys": 1193,
    "languages_processed": 15,
    "translations_added": 234,
    "translations_corrected": 567,
    "translations_kept": 17207,
    "errors": 0
  },
  "changes": {
    "fr": {
      "dashboard": {"changed": 83, "added": 0, "kept": 204},
      "gallery": {"changed": 19, "added": 0, "kept": 0}
    }
  },
  "validation": {
    "missing_keys": [],
    "empty_values": [],
    "placeholder_mismatches": [],
    "suspicious_lengths": []
  }
}
```

Konsolenausgabe:
```
=== AuraGo Translation Automation ===
Source: de.json (German)
Target: 15 languages
Mode:   Smart (retranslate suspicious values)

[chat      ] cs: 0 changed, da: 0 changed, ...
[dashboard ] cs: 57 changed ✓, da: 61 changed ✓, ...
[gallery   ] ALL: 19 changed ✓ (was 100% EN fallback)
...

=== Summary ===
✓ 801 translations corrected
✓ 0 missing keys filled
✓ 17,207 translations verified OK
⚠ 0 errors

Report saved: scripts/translation_report.json
```

---

## Zeitabschätzung

| Phase | Keys | Geschwindigkeit | Dauer |
|-------|------|-----------------|-------|
| Analyse | 19.088 | Sofort (lokal) | ~2s |
| Übersetzung (Smart) | ~3.000 verdächtige | 50/Batch, 1s Pause | ~2 Min |
| Übersetzung (Force) | ~17.900 (alle außer DE) | 50/Batch, 1s Pause | ~8 Min |
| Validierung | 19.088 | Sofort (lokal) | ~2s |
| Schreiben | 176 Dateien | Sofort (lokal) | ~1s |

**Gesamt: ~2–8 Minuten** je nach Modus.

---

## Integration in Workflow

### Bei neuen Features
1. Entwickler fügt Keys zu `de.json` hinzu
2. Entwickler fügt Keys zu `en.json` hinzu (manuell, da Qualität wichtig)
3. `python scripts/translate_ui.py` generiert alle anderen Sprachen automatisch

### Regelmäßige Qualitätsprüfung
```bash
# Validierung ohne Änderungen
python scripts/translate_ui.py --validate-only

# Alles komplett neu übersetzen bei Qualitätszweifeln
python scripts/translate_ui.py --force
```

### CI-Integration (optional, Zukunft)
- Pre-commit Hook: `--validate-only` prüft ob alle Keys vorhanden
- GitHub Action: Automatische Übersetzung bei Änderungen an `de.json`

---

## Einschränkungen & Risiken

| Risiko | Mitigation |
|--------|------------|
| Google Translate Qualität variiert | DE als Quelle (besser als EN für viele Sprachen) |
| Rate-Limiting bei vielen Requests | Batching + Sleep + Retry |
| Technische Terme falsch übersetzt | KEEP_WORDS-Liste, Platzhalter-Schutz |
| Kontextverlust bei kurzen Strings | Akzeptabel für UI-Labels |
| Google API-Änderungen | deep-translator abstrahiert, Fallback auf andere Backends möglich (MyMemory, LibreTranslate) |
| Emojis werden verändert | Expliziter Emoji-Schutz vor/nach Übersetzung |

---

## Nächste Schritte

1. **Script erstellen**: `scripts/translate_ui.py` implementieren
2. **Testlauf**: `--dry-run` auf aktuellen Bestand
3. **Vollübersetzung**: `--force` für saubere Basis
4. **Review**: Stichproben in der UI prüfen
5. **Commit**: Alle korrigierten Übersetzungen committen
6. **.gitignore**: `scripts/translation_report.json` aufnehmen
