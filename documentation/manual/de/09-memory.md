# Kapitel 9: Gedächtnis & Wissen

AuraGo verfügt über ein ausgeklügeltes, mehrschichtiges Gedächtnissystem, das es der KI ermöglicht, Kontext zu bewahren, sich an vergangene Gespräche zu erinnern und Wissen effizient zu verwalten. Dieses Kapitel erklärt die verschiedenen Gedächtnistypen und deren Funktionsweise.

---

## Übersicht des Gedächtnissystems

AuraGo implementiert ein hierarchisches Gedächtnismodell mit fünf Hauptkomponenten:

| Gedächtnistyp | Speichermedium | Zugriffsgeschwindigkeit | Verfallsdauer |
|---------------|----------------|------------------------|---------------|
| **Kurzzeitgedächtnis (STM)** | SQLite | Sehr schnell | Konfigurierbar (Standard: 10 Nachrichten) |
| **Langzeitgedächtnis (LTM)** | Vektordatenbank (ChromaDB) | Schnell | Permanent |
| **Wissensgraph** | SQLite + JSON | Mittel | Permanent |
| **Kernspeicher** | YAML-Datei | Sofort | Permanent |
| **Notizen & Aufgaben** | SQLite | Schnell | Bis zur Löschung |

> 💡 Jedes Gedächtnissystem hat seine spezifische Stärke. Die Kombination aller Systeme ermöglicht AuraGo ein menschenähnliches Erinnerungsvermögen.

---

## Kurzzeitgedächtnis (STM)

Das Kurzzeitgedächtnis speichert die unmittelbare Gesprächshistorie und bildet den primären Kontext für jede Antwort.

### Funktionsweise

```yaml
# config.yaml - STM-Konfiguration
agent:
  memory:
    stm:
      enabled: true
      max_messages: 10        # Anzahl der gespeicherten Nachrichten
      compression_threshold: 20  # Ab wann komprimiert wird
```

### Merkmale

- **Sofortiger Zugriff**: Direkte Abfrage der letzten Nachrichten
- **Vollständiger Kontext**: Speichert sowohl Benutzer- als auch KI-Nachrichten
- **Automatische Rotation**: Alte Nachrichten werden automatisch ausgelagert

### Beispiel einer STM-Abfrage

```sql
-- Interne SQLite-Struktur
SELECT role, content, timestamp 
FROM conversation_history 
WHERE session_id = ? 
ORDER BY timestamp DESC 
LIMIT 10;
```

> 🔍 **Deep Dive: STM-Komprimierung**
> 
> Wenn die Nachrichtenanzahl das Threshold überschreitet, werden ältere Nachrichten zusammengefasst:
> - Behält wichtige Entitäten bei (Namen, Fakten, Entscheidungen)
> - Erstellt eine semantische Zusammenfassung
> - Lagert die Zusammenfassung in das LTM über

---

## Langzeitgedächtnis (LTM/RAG)

Das Langzeitgedächtnis basiert auf **Retrieval-Augmented Generation (RAG)** und ermöglicht semantische Suche über alle vergangenen Gespräche.

### Architektur

```
Benutzeranfrage
      ↓
[Embedding-Generierung] → Vektorrepräsentation
      ↓
[Ähnlichkeitssuche in ChromaDB]
      ↓
Relevante Erinnerungen gefunden
      ↓
[Kontextanreicherung] → LLM-Antwort
```

### Konfiguration

```yaml
# config.yaml - LTM/RAG-Konfiguration
rag:
  enabled: true
  embedding_model: "sentence-transformers/all-MiniLM-L6-v2"
  vector_store: "chroma"
  chunk_size: 512
  chunk_overlap: 128
  top_k: 5              # Anzahl der abgerufenen Erinnerungen
  similarity_threshold: 0.75
```

### Speicherung von Erinnerungen

| Aktion | Auslöser | Speicherort |
|--------|----------|-------------|
| Gesprächsende | Sitzung beendet | LTM-Datenbank |
| Manuelles Speichern | `/save` Befehl | LTM + Metadaten |
| Automatisch | Wichtige Erkenntnis erkannt | LTM mit Tags |
| Zusammenfassung | STM-Komprimierung | LTM als Summary |

### Semantische Suche

AuraGo wandelt Anfragen in Vektoren um und findet konzeptionell ähnliche Inhalte:

```python
# Beispiel: Semantische Ähnlichkeitssuche
# Anfrage: "Wie war nochmal meine Server-Konfiguration?"
# Gefunden: Gespräch vor 3 Wochen über nginx-Einrichtung
```

> ⚠️ **Achtung**: Das LTM erfordert ausreichend RAM für den Embedding-Model. Bei Systemen mit wenig RAM kann die Performance leiden.

---

## Wissensgraph

Der Wissensgraph speichert Entitäten (Personen, Orte, Konzepte) und deren Beziehungen zueinander.

### Struktur

```json
{
  "entities": [
    {
      "id": "ent_001",
      "type": "person",
      "name": "Hans Müller",
      "attributes": {
        "rolle": "Teamleiter",
        "abteilung": "IT"
      }
    }
  ],
  "relationships": [
    {
      "source": "ent_001",
      "target": "ent_002",
      "type": "arbeitet_mit",
      "since": "2023-01-15"
    }
  ]
}
```

### Entitätstypen

| Typ | Beschreibung | Beispiele |
|-----|--------------|-----------|
| `person` | Menschen | Kollegen, Kunden, Autoren |
| `organization` | Organisationen | Firmen, Teams, Vereine |
| `project` | Projekte | Softwareprojekte, Events |
| `technology` | Technologien | Frameworks, Tools, Sprachen |
| `concept` | Abstrakte Konzepte | Methoden, Strategien |
| `location` | Orte | Büros, Server-Standorte |

### Automatische Extraktion

AuraGo analysiert Gespräche automatisch und extrahiert:

```yaml
# Beispiel einer automatisch erkannten Entität
entity_extraction:
  enabled: true
  extract_on:
    - introduction: "Ich bin...", "Mein Name ist..."
    - reference: "Hans hat gesagt...", "Bei der Firma XYZ..."
  confidence_threshold: 0.8
```

> 💡 **Tipp**: Verwenden Sie `/remember [Fakt]` für explizite Speicherung wichtiger Informationen.

---

## Kernspeicher (Core Memory)

Der Kernspeicher enthält permanente Fakten, die AuraGo über den Benutzer und die Umgebung wissen sollte.

### Speicherort

```
data/core_memory.yaml
```

### Struktur

```yaml
# data/core_memory.yaml
core_facts:
  user:
    name: "Max Mustermann"
    role: "DevOps Engineer"
    preferences:
      code_style: "PEP8"
      language: "de"
      detail_level: "technical"
  
  environment:
    primary_os: "Ubuntu 22.04"
    editor: "VS Code"
    infrastructure: "AWS"
  
  projects:
    current:
      name: "AuraGo Deployment"
      stack: ["Go", "SQLite", "Docker"]
      deadline: "2024-12-31"
```

### Zugriff im Prompt

Der Kernspeicher wird bei jedem Prompt automatisch eingefügt:

```
[Core Memory]
Benutzer: Max Mustermann, DevOps Engineer
Präferenzen: Technische Details, Deutsch
Projekt: AuraGo Deployment (Go, SQLite, Docker)

[Konversation]
...
```

---

## Notizen & Aufgaben

AuraGo kann strukturierte Notizen und Aufgaben verwalten mit Priorisierung und Kategorisierung.

### Notizen

#### Erstellen einer Notiz

```
Benutzer: Notiere: Meeting mit IT-Team am Donnerstag 14:00
AuraGo: ✅ Notiz gespeichert: "Meeting mit IT-Team am Donnerstag 14:00"
```

#### Notizstruktur

```json
{
  "id": "note_001",
  "content": "Meeting mit IT-Team am Donnerstag 14:00",
  "category": "meeting",
  "tags": ["it-team", "planung"],
  "priority": "normal",
  "created_at": "2024-01-15T10:30:00Z",
  "remind_at": "2024-01-18T13:45:00Z"
}
```

### Aufgaben (To-Dos)

#### Aufgabenverwaltung

| Befehl | Beschreibung |
|--------|--------------|
| `/todo add [Text]` | Neue Aufgabe erstellen |
| `/todo list` | Alle Aufgaben anzeigen |
| `/todo done [ID]` | Aufgabe als erledigt markieren |
| `/todo priority [ID] [hoch/mittel/niedrig]` | Priorität setzen |
| `/todo delete [ID]` | Aufgabe löschen |

#### Beispiel-Sitzung

```
Benutzer: /todo add Docker-Container für Produktion konfigurieren
AuraGo: ✅ Aufgabe #42 erstellt: "Docker-Container für Produktion konfigurieren"

Benutzer: /todo priority 42 hoch
AuraGo: 🔺 Priorität von Aufgabe #42 auf "hoch" gesetzt.

Benutzer: /todo list
AuraGo: 
📋 Offene Aufgaben:

🔴 Hoch:
  #42 Docker-Container für Produktion konfigurieren

🟡 Mittel:
  #38 Dokumentation aktualisieren

🟢 Niedrig:
  #15 README überarbeiten
```

---

## Wie Gedächtnis in Gesprächen funktioniert

### Der Gedächtnis-Lebenszyklus

```
┌─────────────────────────────────────────────────────────────┐
│                    GESPRÄCHSSTART                           │
│  1. Kernspeicher laden                                       │
│  2. Relevante LTM-Einträge abrufen                          │
│  3. Aktuelle Aufgaben/Notizen prüfen                        │
└──────────────────────┬──────────────────────────────────────┘
                       ↓
┌─────────────────────────────────────────────────────────────┐
│                    WÄHREND DES GESPRÄCHS                    │
│  4. STM aktualisieren (neue Nachrichten)                    │
│  5. Entitäten extrahieren → Wissensgraph                    │
│  6. Wichtige Fakten erkennen                                │
└──────────────────────┬──────────────────────────────────────┘
                       ↓
┌─────────────────────────────────────────────────────────────┐
│                    GESPRÄCHSENDE                            │
│  7. STM-Komprimierung → Summary                             │
│  8. Summary + Entities in LTM speichern                     │
│  9. Core Memory Updates prüfen                              │
└─────────────────────────────────────────────────────────────┘
```

### Kontextaufbau für jeden Prompt

```python
def build_context():
    context = []
    
    # 1. Kernspeicher (immer an erster Stelle)
    context.append(load_core_memory())
    
    # 2. Relevante Langzeiterinnerungen (RAG)
    relevant_memories = search_ltm(user_query)
    context.append(relevant_memories)
    
    # 3. Aktuelle Aufgaben (falls relevant)
    active_todos = get_active_todos()
    context.append(active_todos)
    
    # 4. Kurzzeitgedächtnis (letzte Nachrichten)
    stm = get_stm_messages(limit=10)
    context.append(stm)
    
    # 5. Aktuelle Anfrage
    context.append(user_query)
    
    return concat_context(context)
```

---

## Wissen speichern und abrufen

### Manuelle Speicherung

#### Über Befehle

```
/remember [Fakt]              # Einzelnen Fakt speichern
/save "[Titel]" [Inhalt]      # Strukturierte Speicherung
/note [Inhalt]                # Schnellnotiz erstellen
```

#### Im Gespräch

```
Benutzer: Bitte merke dir, dass ich PostgreSQL bevorzuge.
AuraGo: ✅ Gespeichert: "Benutzer bevorzugt PostgreSQL"

Benutzer: Wie war nochmal meine Datenbank-Präferenz?
AuraGo: Laut meiner Notiz bevorzugst du PostgreSQL.
```

### Wissensabfrage

#### Direkte Abfragen

```
/recall [Suchbegriff]         # LTM durchsuchen
/notes                        # Alle Notizen anzeigen
/entity [Name]                # Entitätsdetails anzeigen
```

#### Implizite Abfragen

AuraGo erkennt automatisch, wenn auf gespeichertes Wissen zugegriffen werden sollte:

```
Benutzer: Wie geht es dem Projekt?
AuraGo: Beziehst du dich auf "AuraGo Deployment"? 
        Stand: Docker-Container ist zu 80% konfiguriert.
        Offene Aufgabe #42 mit hoher Priorität.
```

---

## Gedächtnis-Optimierung

### Performance-Tuning

#### Vector-Datenbank-Optimierung

```yaml
rag:
  # Für bessere Performance
  embedding_batch_size: 32
  index_type: "hnsw"        # Approximate nearest neighbor
  ef_construction: 200      # Genauigkeit vs. Geschwindigkeit
  m: 16                     # Verbindungen pro Knoten
```

#### SQLite-Optimierung

```yaml
memory:
  sqlite:
    pragma:
      - "PRAGMA journal_mode=WAL"      # Write-Ahead Logging
      - "PRAGMA synchronous=NORMAL"    # Balance Sicherheit/Speed
      - "PRAGMA cache_size=-64000"     # 64MB Cache
```

### Speicherplatz-Management

| Komponente | Aufräumstrategie | Intervall |
|------------|------------------|-----------|
| STM | Automatisch (FIFO) | Bei jedem Prompt |
| LTM | Alter + Relevanz | Wöchentlich |
| Wissensgraph | Verwaiste Entitäten | Monatlich |
| Notizen | Erledigte archivieren | Manuell |
| Logs | Rotation nach Größe | Täglich |

### Aufräumbefehle

```
/memory cleanup              # Alte Einträge bereinigen
/memory vacuum               # Datenbank optimieren
/memory export [format]      # Backup erstellen
/memory import [file]        # Backup wiederherstellen
```

---

## Persistent Summary Compression

Die Zusammenfassungskomprimierung reduziert den Speicherbedarf und erhält gleichzeitig wichtige Informationen.

### Komprimierungsstufen

```
Original (10 Nachrichten):
  ├─ Benutzer: Hallo
  ├─ AuraGo: Guten Tag!
  ├─ Benutzer: Ich brauche Hilfe bei Docker
  ├─ AuraGo: Gerne! Was möchten Sie wissen?
  ├─ Benutzer: Wie erstelle ich ein Dockerfile?
  ...

Nach Komprimierung:
  Summary: "Benutzer benötigt Hilfe bei Docker. 
            Spezifisch: Dockerfile-Erstellung."
```

### Konfiguration

```yaml
memory:
  compression:
    enabled: true
    strategy: "semantic"      # semantic | extractive | hybrid
    preserve:
      - user_preferences
      - technical_decisions
      - action_items
      - named_entities
    compression_ratio: 0.3    # Ziel: 30% der Originalgröße
```

> 🔍 **Deep Dive: Komprimierungsalgorithmus**
> 
> 1. **Entity-Erkennung**: Behalte alle Named Entities bei
> 2. **Fakten-Extraktion**: Identifiziere Aussagesätze
> 3. **Intention-Erkennung**: Was wollte der Benutzer erreichen?
> 4. **Kontext-Zusammenfassung**: Paraphrasiere den Rest

---

## Best Practices für Wissensmanagement

### Für Benutzer

#### ✅ Empfohlene Praktiken

1. **Wichtige Fakten explizit speichern**
   ```
   /remember Mein Server läuft auf Ubuntu 22.04 LTS
   ```

2. **Strukturierte Notizen verwenden**
   ```
   /note Projekt Alpha: Deadline 15.03., Budget 50k€, Team: 5 Personen
   ```

3. **Regelmäßig aufräumen**
   ```
   /memory cleanup --older-than 90d
   ```

4. **Kategorien für Notizen nutzen**
   - `#meeting` für Besprechungen
   - `#code` für Code-Snippets
   - `#idea` für Ideen
   - `#todo` für Aufgaben

#### ❌ Zu vermeiden

- Übermäßig lange Nachrichten ohne Struktur
- Speicherung sensibler Daten ohne Verschlüsselung
- Vernachlässigung des Kernspeichers für wichtige Präferenzen

### Für Administratoren

#### Backup-Strategie

```bash
# Tägliches Backup
0 2 * * * /opt/aurago/bin/aurago-cli memory export --format=json --output=/backup/memory-$(date +%Y%m%d).json

# Wöchentliche Vakuumierung
0 3 * * 0 /opt/aurago/bin/aurago-cli memory vacuum
```

#### Monitoring

```yaml
# monitoring.yaml
memory_alerts:
  - condition: "sqlite_size > 1GB"
    action: "notify_admin"
  - condition: "vector_db_ram > 4GB"
    action: "scale_resources"
  - condition: "compression_ratio < 0.2"
    action: "review_config"
```

### Migration und Portabilität

```bash
# Export aller Gedächtnisdaten
aurago-cli memory export --all --output=aurago_memory_backup.json

# Import auf neuem System
aurago-cli memory import --source=aurago_memory_backup.json --merge-strategy=upsert
```

---

## Zusammenfassung

AuraGos Gedächtnissystem bietet:

| Feature | Nutzen |
|---------|--------|
| **STM** | Sofortiger Kontext für flüssige Gespräche |
| **LTM/RAG** | Langfristiges Lernen und Erinnern |
| **Wissensgraph** | Strukturierte Beziehungen zwischen Konzepten |
| **Kernspeicher** | Permanente, schnell verfügbare Präferenzen |
| **Notizen/Aufgaben** | Organisation und Nachverfolgung |

> 💡 **Profi-Tipp**: Kombinieren Sie die verschiedenen Gedächtnistypen für maximale Effektivität. Speichern Sie wichtige Präferenzen im Kernspeicher, nutzen Sie Notizen für kurzfristige Aufgaben, und lassen Sie LTM und Wissensgraph im Hintergrund automatisch wachsen.

---

**Nächstes Kapitel:** [Kapitel 10: Persönlichkeit](./10-personality.md)
