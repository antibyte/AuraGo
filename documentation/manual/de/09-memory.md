# Kapitel 9: Gedächtnis & Wissen

AuraGo verfügt über ein mehrschichtiges Gedächtnissystem, das es der KI ermöglicht, Kontext zu bewahren und sich an vergangene Gespräche zu erinnern.

> ⚠️ **Hinweis:** Dieses Kapitel beschreibt die aktuelle Implementierung. Einige fortgeschrittene Features aus früheren Versionen (wie konfigurierbare RAG-Parameter) sind in der aktuellen Version nicht verfügbar.

---

## Übersicht des Gedächtnissystems

AuraGo implementiert ein hierarchisches Gedächtnismodell:

| Gedächtnistyp | Speichermedium | Zugriffsgeschwindigkeit | Verfallsdauer |
|---------------|----------------|------------------------|---------------|
| **Kurzzeitgedächtnis (STM)** | SQLite | Sehr schnell | Konfigurierbar (Standard: letzte Nachrichten) |
| **Kernspeicher (Core Memory)** | Markdown-Datei | Sofort | Permanent |
| **Langzeitgedächtnis (LTM)** | Vektordatenbank | Schnell | Permanent (wenn aktiviert) |

---

## Kurzzeitgedächtnis (STM)

Das Kurzzeitgedächtnis speichert die unmittelbare Gesprächshistorie in einer SQLite-Datenbank.

### Konfiguration

```yaml
# config.yaml - STM-Konfiguration
sqlite:
  short_term_path: "./data/short_term.db"  # Pfad zur SQLite-Datenbank
```

### Funktionsweise

- **Automatische Speicherung:** Jede Konversation wird automatisch in der SQLite-Datenbank gespeichert
- **Kontext-Fenster:** Der Agent berücksichtigt die letzten Nachrichten bei jeder Anfrage
- **Persistenz:** Überlebt Neustarts des Agents

---

## Kernspeicher (Core Memory)

Der Kernspeicher enthält permanente Fakten, die AuraGo über den Benutzer und die Umgebung wissen sollte.

### Speicherort

```
data/core_memory.md
```

### Struktur

Die Datei wird automatisch verwaltet. Der Agent kann Fakten hinzufügen mit:

```
Du: Merke dir: Mein Name ist Max und ich arbeite mit Go
Agent: ✅ In Core Memory gespeichert
```

### Konfiguration

```yaml
# config.yaml
agent:
  core_memory_max_entries: 200      # Maximale Anzahl Einträge (0 = unbegrenzt)
  core_memory_cap_mode: "soft"      # "soft" oder "hard"
```

| Modus | Beschreibung |
|-------|--------------|
| `soft` | Weiche Begrenzung, ältere Einträge werden priorisiert |
| `hard` | Harte Begrenzung, überschüssige Einträge werden entfernt |

---

## Langzeitgedächtnis (LTM) / Embeddings

> ⚠️ **Optional:** Dieses Feature ist standardmäßig deaktiviert (`embeddings.provider: "disabled"`), da es zusätzliche API-Aufrufe erfordert.

Das Langzeitgedächtnis ermöglicht semantische Suche über vergangene Gespräche mittels Vektordatenbank.

### Konfiguration

```yaml
# config.yaml - LTM/Embeddings-Konfiguration
embeddings:
  provider: "disabled"                    # Optionen: "disabled", "internal", oder Provider-ID
  internal_model: "qwen/qwen3-embedding-8b"  # Modell für internen Provider
  external_url: "http://localhost:11434/v1"  # Für externen Provider (z.B. Ollama)
  external_model: "nomic-embed-text"         # Modell für externen Provider
```

### Provider-Optionen

| Provider | Beschreibung |
|----------|--------------|
| `disabled` | Langzeitgedächtnis deaktiviert (Standard) |
| `internal` | Nutzt das Haupt-LLM für Embeddings |
| Provider-ID | Verwendet einen konfigurierten Provider-Eintrag |

### Speicherort

```
data/vectordb/       # Vektordatenbank (chromem-go)
sqlite:
  long_term_path: "./data/long_term.db"  # Langzeit-Speicher
```

---

## Memory Compression

Um den Kontext effizient zu nutzen, komprimiert AuraGo ältere Konversationen automatisch.

### Konfiguration

```yaml
# config.yaml
agent:
  memory_compression_char_limit: 60000   # Zeichen-Limit für Kompression (Standard: 60000)
```

Wenn die Konversationshistorie dieses Limit überschreitet:
1. Werden ältere Nachrichten zusammengefasst
2. Wichtige Fakten werden extrahiert
3. Der Kontext bleibt erhalten, aber kompakter

---

## Wissen speichern und abrufen

### Manuelle Speicherung

```
Du: Merke dir: Ich bevorzuge PostgreSQL über MySQL
Agent: ✅ Gespeichert: "Ich bevorzuge PostgreSQL über MySQL"

Du: Notiere: Meeting mit Team am Freitag 14 Uhr
Agent: ✅ Notiz gespeichert
```

### Abfrage

```
Du: Was ist meine bevorzugte Datenbank?
Agent: Laut meiner Notiz bevorzugst du PostgreSQL über MySQL.
```

---

## Knowledge Indexing (Datei-Indexierung)

AuraGo kann lokale Dateien automatisch indexieren für schnellen Zugriff.

### Konfiguration

```yaml
# config.yaml
indexing:
  enabled: true
  directories:
    - "./knowledge"           # Zu indexierende Verzeichnisse
  poll_interval_seconds: 60   # Prüfintervall für Änderungen
  extensions:                 # Zu indexierende Dateitypen
    - .txt
    - .md
    - .json
    - .csv
    - .log
    - .yaml
    - .yml
```

### Verwendung

Lege Dateien im `knowledge/`-Verzeichnis ab. Der Agent kann dann darauf zugreifen:

```
Du: Was steht in der dokumentation.txt?
Agent: [Inhalt der indexierten Datei]
```

---

## Helper LLM — Automatisierte Wartung

Der Helper LLM ist ein sekundäres, kostengünstigeres LLM, das Hintergrundwartungsaufgaben erledigt, um den Hauptagenten schnell und effizient zu halten.

### Übersicht

```
┌──────────────────────────────────────────────────────────┐
│  Helper LLM — Hintergrundwartung                          │
├──────────────────────────────────────────────────────────┤
│                                                           │
│  • Turn-Analyse        → Extrahiert Fakten, Präferenzen │
│  • Tageszusammenfassung + KG → Zusammenfassung + Entitäten │
│  • Konsolidierung      → Stapel-Konsolidierung Gedächtnis│
│  • Speicherkomprimierung → Komprimiert Konversationshistorie│
│  • RAG-Stapel          → Stapel-RAG-Verarbeitung        │
│                                                           │
└──────────────────────────────────────────────────────────┘
```

### Operationen

| Operation | Beschreibung | Auslöser |
|-----------|--------------|----------|
| **Turn-Analyse** | Analysiert jeden Gesprächsstrang für Fakten, Präferenzen, Stimmung und ausstehende Aktionen | Nach jedem Strang |
| **Tageszusammenfassung + KG** | Tägliche Zusammenfassung + Wissensgraph-Extraktion | Tägliche Wartung |
| **Konsolidierung** | Konsolidiert Gesprächsstapel in Langzeitwissen | Periodisch |
| **Speicherkomprimierung** | Komprimiert alte Konversationshistorie | Zeichenlimit erreicht |
| **Inhaltszusammenfassungen** | Erstellt Zusammenfassungen von Inhalten | Bei Bedarf |
| **RAG-Stapel** | Stapelverarbeitung für Retrieval-Augmented Generation | Periodisch |

### Turn-Analyse

Nach jedem Gesprächsstrang analysiert der Helper LLM:

**Gedächtnisanalyse:**
- **Fakten**: Konkrete Fakten über Benutzer/Projekt/Umgebung
- **Präferenzen**: Benutzerpräferenzen, Gewohnheiten, Arbeitsabläufe
- **Korrekturen**: Aktualisierungen previously bekannter Informationen
- **Ausstehende Aktionen**: Zurückgestellte Folgemaßnahmen

**Aktivitätszusammenfassung:**
- Benutzerabsicht und -ziel
- Vom Agenten durchgeführte Aktionen
- Ergebnisse und wichtige Punkte
- Ausstehende Punkte
- Wichtigkeitsstufe (1-4)

**Persönlichkeitsanalyse:**
- Benutzerstimmung und -emotion
- Angemessene Antwortstimmung für nächste Interaktion
- Trait-Deltas (Neugier, Gründlichkeit, Kreativität, Empathie, etc.)
- Emotionszustandsbeschreibung

### Dashboard-Überwachung

Helper LLM-Statistiken im Dashboard → System-Tab anzeigen:

```
Helper LLM Statistiken
┌─────────────────────────────────────────────────────────┐
│  Status: Aktiviert ✓                                    │
│  Letztes Update: Vor 2 Minuten                          │
│                                                           │
│  Anfragen: 1.234      LLM-Aufrufe: 567               │
│  Cache-Treffer: 432 (35%)  Fallbacks: 12             │
│                                                           │
│  Gesparte Aufrufe: 89       Stapel-Artikel: 456      │
│                                                           │
│  Operationen:                                           │
│  • Turn-Analyse: 1.234 erfolgreich                     │
│  • Tageszusammenfassung + KG: 28 erfolgreich           │
│  • Konsolidierung: 5 erfolgreich                         │
│  • Speicherkomprimierung: 3 ausgelöst                  │
│  • Inhaltszusammenfassungen: 45 erfolgreich            │
│  • RAG-Stapel: 89 erfolgreich                          │
└─────────────────────────────────────────────────────────┘
```

### Konfiguration

```yaml
llm:
  helper_enabled: true              # Helper LLM aktivieren (empfohlen)
  helper_provider: ""               # leer = Haupt-Provider; oder Provider-ID
  helper_model: ""                  # leer = auto; oder spezifisches kostengünstiges Modell
```

> 💡 **Empfehlung:** Aktiviere den Helper LLM und weise ihm ein kleineres, kostengünstiges Modell zu (z. B. `google/gemini-2.0-flash-001`). Ohne Helper sind viele Hintergrundfunktionen wie Turn-Analyse, Konsolidierung und Wissensgraph-Extraktion nicht vollständig verfügbar.

| Parameter | Standard | Beschreibung |
|-----------|----------|--------------|
| `helper_enabled` | `false` | Helper LLM aktivieren/deaktivieren |
| `helper_provider` | `""` | Provider-ID (leer = Haupt-Provider) |
| `helper_model` | `""` | Modell (leer = Haupt-Modell) |

### Fehlerbehebung

| Problem | Ursache | Lösung |
|---------|---------|--------|
| Helper LLM zeigt "deaktiviert" | Feature nicht aktiviert | Setze `helper_llm.enabled: true` |
| Hohe Helper-LLM-Kosten | Zu häufige Operationen | Operationsfrequenz reduzieren |
| Keine Turn-Analyse | Provider unterstützt keine Function Calls | Fähiges Modell verwenden |
| Zu hohe Cache-Miss-Rate | Cache häufig geleert | Cache-Speichereinstellungen prüfen |

---

## Best Practices

### Für Benutzer

#### ✅ Empfohlene Praktiken

1. **Wichtige Fakten explizit speichern**
   ```
   Merke dir: Mein Server läuft auf Ubuntu 22.04 LTS
   ```

2. **Embeddings aktivieren für komplexe Projekte**
   ```yaml
   embeddings:
     provider: "internal"  # oder externer Provider
   ```

3. **Core Memory regelmäßig aufräumen**
   - Überprüfe `data/core_memory.md`
   - Entferne veraltete Informationen

#### ❌ Zu vermeiden

- Übermäßig lange Nachrichten ohne Struktur
- Speicherung sensibler Daten (Passwörter, API-Keys)
- Ignorieren der `memory_compression_char_limit` bei langsamen Antworten

### Für Administratoren

#### Backup-Strategie

```bash
# Wichtige Dateien sichern
cp data/core_memory.md backup/
cp data/short_term.db backup/
cp -r data/vectordb backup/  # Falls Embeddings aktiviert
```

#### Performance-Optimierung

| Problem | Lösung |
|---------|--------|
| Hoher RAM-Verbrauch | `embeddings.provider: disabled` setzen |
| Langsame Antworten | `memory_compression_char_limit` reduzieren |
| Große Datenbank | SQLite VACUUM ausführen |

---

## Zusammenfassung

| Feature | Konfiguration | Standard |
|---------|--------------|----------|
| **Kurzzeitgedächtnis** | `sqlite.short_term_path` | Aktiviert |
| **Kernspeicher** | `agent.core_memory_*` | Aktiviert |
| **Langzeitgedächtnis** | `embeddings.provider` | Deaktiviert |
| **Datei-Indexierung** | `indexing.enabled` | Aktiviert |
| **Kompression** | `agent.memory_compression_char_limit` | 50000 Zeichen |

> 💡 **Profi-Tipp:** Für die meisten Anwendungsfälle reichen Kurzzeitgedächtnis und Core Memory. Aktiviere Embeddings nur, wenn du eine große Menge historischer Konversationen durchsuchen musst.

---

**Vorheriges Kapitel:** [Kapitel 8: Integrationen](./08-integrations.md)  
**Nächstes Kapitel:** [Kapitel 10: Persönlichkeit](./10-personality.md)
