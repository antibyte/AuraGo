# Verbesserungskonzept: AuraGo Memory System 2.0

## Executive Summary

Das aktuelle Memory-System von AuraGo ist technisch solide, aber zu **passiv**. Der Knowledge Graph wird kaum genutzt, es gibt keine automatische Konsolidierung von Erinnerungen und kein Tagebuch-System für langfristige Kontinuität. Dieser Bericht schlägt einen evolutionären, aber stark erweiterten Ansatz vor.

---

## Aktueller Stand (Analyse)

### Bestehende Komponenten

| Komponente | Technologie | Aktive Nutzung | Problem |
|------------|-------------|----------------|---------|
| **Short-Term Memory** | SQLite | ✅ Hoch | Sliding window nur, keine intelligente Konsolidierung |
| **Long-Term Memory** | VectorDB (chromem) | ✅ Hoch | Nur explizite RAG-Abfragen |
| **Knowledge Graph** | JSON-Datei | ❌ Niedrig | Keine automatische Bevölkerung, keine proaktive Nutzung |
| **Core Memory** | SQLite + Prompt | ✅ Hoch | Manuelle Pflege nötig |
| **User Profile** | SQLite | ✅ Mittel | Passive Profilierung |
| **Notes/Todos** | SQLite | ✅ Mittel | Keine Verknüpfung mit anderen Memory-Ebenen |
| **Temporal Memory** | SQLite | ✅ Mittel | Nur Patterns, keine Zeitachse |

### Warum wird der Knowledge Graph nicht genutzt?

1. **Hohe Reibung**: Der Agent muss explizit `knowledge_graph` Tool-Calls machen
2. **Keine automatische Extraktion**: Entitäten aus Gesprächen werden nicht automatisch erkannt
3. **Keine Integration im Prompt**: Der Graph ist im System-Prompt nicht sichtbar
4. **Fehlende Proaktivität**: Der Graph suggeriert keine Verbindungen oder Erkenntnisse

---

## Vision: Das "Lebendige Gedächtnis"

```
┌─────────────────────────────────────────────────────────────────┐
│                    JOURNAL / TAGESSPIEGEL                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │ Gestern     │  │ Heute       │  │ Wichtige    │              │
│  │ 15.03.2026  │  │ 16.03.2026  │  │ Meilensteine│              │
│  │ • SSH-Key   │  │ • Docker    │  │             │              │
│  │   Setup     │  │   Compose   │  │             │              │
│  │ • VPN Config│  │   erstellt  │  │             │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
├─────────────────────────────────────────────────────────────────┤
│              KNOWLEDGE GRAPH (Auto-bevölkert)                    │
│                                                                  │
│     [User:Andre]──uses──>[Tool:Docker]──for──>[Project:Aura]   │
│          │                      │                    │          │
│          │                 manages              contains        │
│          │                      │                    │          │
│     [Pref:Deutsch]      [Container:App]◄──────[File:compose.yml]│
│                                                                  │
├─────────────────────────────────────────────────────────────────┤
│              AUTOMATISMEN & REFLEKTION                           │
│  • Auto-Archivierung (STM → LTM) nach 24h                       │
│  • Entitätsextraktion pro Nachricht                             │
│  • Abendliche Zusammenfassung (Tagesjournal)                    │
│  • Wöchentliche Reflektion: "Was haben wir gelernt?"            │
└─────────────────────────────────────────────────────────────────┘
```

---

## Phase 1: Journal-System (Tagebuch)

### Konzept
Ein Journal ist eine **Zeitachse** wichtiger Ereignisse, die automatisch oder halb-automatisch geführt wird. Es ist KEIN Ersatz für STM/LTM, sondern eine **Meta-Ebene** für Kontext und Kontinuität.

### Datenbank-Schema

```sql
-- Journal-Einträge
CREATE TABLE journal_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_type TEXT NOT NULL,      -- 'milestone', 'learning', 'decision', 'error', 'preference'
    title TEXT NOT NULL,
    content TEXT,
    tags TEXT,                     -- JSON-Array
    importance INTEGER DEFAULT 2,  -- 1=low, 2=normal, 3=high, 4=critical
    date TEXT NOT NULL,            -- YYYY-MM-DD
    session_id TEXT,
    related_memories TEXT,         -- JSON-Array von VectorDB IDs
    auto_generated BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Tageszusammenfassungen (Auto-generiert)
CREATE TABLE daily_summaries (
    date TEXT PRIMARY KEY,
    summary TEXT,                  -- LLM-generierte Zusammenfassung
    key_topics TEXT,               -- JSON-Array
    tool_usage TEXT,               -- JSON-Objekt {tool: count}
    sentiment TEXT,                -- 'positive', 'neutral', 'frustrated'
    memory_count INTEGER,          -- Neue LTM-Einträge
    entity_count INTEGER,          -- Neue KG-Nodes
    generated_at DATETIME
);

-- Meilensteine (Manuell oder Auto-markiert)
CREATE TABLE milestones (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT,
    date_achieved TEXT,
    category TEXT,                 -- 'project', 'learning', 'integration', 'fix'
    impact_score INTEGER DEFAULT 1
);
```

### Automatismus: Journal-Reflektion

```go
// Trigger: Nach X Nachrichten oder am Ende einer Session
func (s *SQLiteMemory) AutoGenerateJournalEntry(sessionID string, messages []Message) error {
    // 1. Prüfe, ob wichtige Ereignisse passiert sind
    importantEvents := s.extractImportantEvents(messages)
    
    // 2. Für jedes wichtige Event: Journal-Eintrag erstellen
    for _, event := range importantEvents {
        entry := JournalEntry{
            EntryType:    event.Type,      // 'milestone', 'learning', 'decision'
            Title:        event.Title,
            Content:      event.Content,
            Tags:         event.Tags,
            Importance:   event.Importance,
            Date:         time.Now().Format("2006-01-02"),
            SessionID:    sessionID,
            AutoGenerated: true,
        }
        s.InsertJournalEntry(entry)
    }
    
    return nil
}
```

### Trigger für Journal-Einträge

| Trigger | Typ | Beispiel |
|---------|-----|----------|
| **Tool-Chain abgeschlossen** | Auto | "Docker-Setup erfolgreich" nach 5+ Docker-Calls |
| **Erfolgreiche Reparatur** | Auto | "SSH-Verbindung zu Server X wiederhergestellt" |
| **Neue Integration** | Auto | "Home Assistant API zum ersten Mal verwendet" |
| **Wichtige Entscheidung** | Halb-Auto | User bestätigt: "Das war die richtige Lösung" |
| **Core Memory Update** | Auto | Neue Präferenz oder Fakt gespeichert |
| **Fehler/Warnung** | Auto | "Docker-Build fehlgeschlagen - Workaround gefunden" |

### UI-Integration

```javascript
// Journal-Widget im Dashboard
{
  "type": "journal_timeline",
  "config": {
    "show_days": 7,
    "group_by": "date",
    "filters": ["milestone", "learning", "decision"]
  }
}
```

---

## Phase 2: Aktiver Knowledge Graph

### Problem-Analyse

Der aktuelle Graph ist **passiv** - er wartet auf Tool-Calls. Stattdessen sollte er **proaktiv** werden:

```
Aktuell:  User sagt etwas → Agent antwortet → (vielleicht KG-Update)
          ↓
Zukunft:  User sagt etwas → Agent erkennt Entität → Auto-Graph-Update
          → Graph suggeriert Beziehungen → Agent nutzt diese
```

### Auto-Entitätsextraktion

```go
// Nach jedem User-Input: Extrahiere potenzielle Entitäten
type EntityExtractor struct {
    llmClient llm.ChatClient
    kg        *memory.KnowledgeGraph
}

func (e *EntityExtractor) ExtractAndStore(text string) error {
    // Nur bei signifikanten Nachrichten (>50 chars, nicht nur "ok")
    if len(text) < 50 {
        return nil
    }
    
    prompt := fmt.Sprintf(`
Analysiere diesen Text und extrahiere ENTITÄTEN und BEZIEHUNGEN.
Extrahiere NUR signifikante Entitäten (Personen, Projekte, Tools, Orte, Präferenzen).

Text: "%s"

Antworte als JSON:
{
  "entities": [
    {"id": "unique_id", "label": "Name", "type": "person|project|tool|place|preference"}
  ],
  "relations": [
    {"source": "id1", "target": "id2", "relation": "verb"}
  ]
}

WENN keine signifikanten Entitäten: {"entities": [], "relations": []}
`, text)

    // Async-Call zum LLM (nicht blockierend)
    go e.processExtraction(prompt, text)
    return nil
}
```

### Graph-Integration im Prompt

```go
// In builder.go: Integriere relevante Graph-Ausschnitte
func (b *PromptBuilder) AddKnowledgeGraphContext(userMsg string) string {
    // Suche im Graphen nach relevanten Knoten
    relevantNodes := b.kg.Search(userMsg)
    
    // Begrenze auf die wichtigsten 3-5 Knoten mit ihren Nachbarn
    context := b.buildGraphContext(relevantNodes, 2) // Tiefe 2
    
    if context == "" {
        return ""
    }
    
    return fmt.Sprintf(`
## Relevante Beziehungen aus deinem Wissen
%s
Nutze diese Informationen für Kontext, aber bevorzuge aktuelle Anweisungen.
`, context)
}
```

### Graph-Visualisierung im Prompt

```
## Dein Wissensnetzwerk (relevante Ausschnitte)

Projekt "AuraGo" 
  ├── wird_entwickelt_mit: Go 1.26
  ├── läuft_auf: Docker
  ├── benutzt: SQLite (für STM)
  └── benutzt: Chromem (für LTM)

User "Andre"
  ├── bevorzugt: Deutsche Sprache
  ├── arbeitet_an: AuraGo
  └── hat_eingerichtet: Proxmox Cluster
```

### Wissens-Lücken-Erkennung

```go
// Erkenne, wenn der Graph unvollständig ist
func (kg *KnowledgeGraph) DetectKnowledgeGaps() []GapHint {
    var gaps []GapHint
    
    // Beispiel: Tool wird oft genutzt, aber keine Verbindung zum User
    for tool, count := range kg.getToolUsageCounts() {
        if count > 5 && !kg.hasUserToolRelation(tool) {
            gaps = append(gaps, GapHint{
                Type: "missing_relation",
                Suggestion: fmt.Sprintf("User frequently uses %s - consider adding preference relation", tool),
            })
        }
    }
    
    return gaps
}
```

---

## Phase 3: Automatische Memory-Konsolidierung

### Das Vergessens-Problem

Aktuell: Nachrichten werden einfach verworfen (nach N Einträgen).

Besser: Intelligente Konsolidierung von Short-Term → Long-Term.

### Konsolidierungs-Pipeline

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Nachrichten    │    │  Extraktion      │    │  Klassifizierung│
│  (SQLite)       │───▶│  • Entitäten     │───▶│  • Wichtig?     │
│                 │    │  • Fakten        │    │  • Temporär?    │
│  Älter 24h      │    │  • Entscheidungen│    │  • Persistieren?│
└─────────────────┘    └──────────────────┘    └────────┬────────┘
                                                        │
                           ┌────────────────────────────┼────────────────────────────┐
                           │                            │                            │
                           ▼                            ▼                            ▼
                   ┌───────────────┐          ┌───────────────┐              ┌───────────────┐
                   │  Core Memory  │          │  Long-Term    │              │  Journal      │
                   │  (Präferenzen)│          │  (Fakten)     │              │  (Ereignisse) │
                   └───────────────┘          └───────────────┘              └───────────────┘
```

### Konsolidierungs-Regeln

```go
type ConsolidationRule struct {
    Name        string
    Condition   func(msg Message) bool
    Destination string  // "core", "ltm", "journal", "discard"
    Transform   func(msg Message) string
}

var DefaultRules = []ConsolidationRule{
    {
        Name: "User Preference",
        Condition: func(m Message) bool {
            return containsAny(m.Content, []string{
                "ich bevorzuge", "i prefer", "immer", "never", 
                "wichtig", "unwichtig", "mag ich", "mag ich nicht",
            })
        },
        Destination: "core",
        Transform: func(m Message) string {
            return extractPreference(m.Content)
        },
    },
    {
        Name: "Successful Setup",
        Condition: func(m Message) bool {
            return m.Role == "assistant" && 
                   containsAny(m.Content, []string{"erfolgreich", "success", "fertig", "completed"}) &&
                   len(m.ToolCalls) > 3  // Mehrere Tools verwendet
        },
        Destination: "journal",
        Transform: func(m Message) string {
            return fmt.Sprintf("Setup abgeschlossen: %s", summarizeTools(m.ToolCalls))
        },
    },
    // ... weitere Regeln
}
```

### Nächtliche Konsolidierung

```go
// Cron-Job: Jede Nacht um 3 Uhr
func (m *MemoryManager) NightlyConsolidation() error {
    // 1. Hole Nachrichten älter 24h
    oldMessages := m.shortTerm.GetMessagesOlderThan(24 * time.Hour)
    
    // 2. Gruppiere nach Session
    sessions := groupBySession(oldMessages)
    
    for _, session := range sessions {
        // 3. Extrahiere wichtige Fakten
        facts := m.extractor.ExtractFacts(session.Messages)
        
        // 4. Speichere in LTM
        for _, fact := range facts {
            m.longTerm.StoreDocument(fact.Concept, fact.Content)
        }
        
        // 5. Erstelle Journal-Eintrag für den Tag
        m.journal.CreateDailySummary(session.Date, facts)
        
        // 6. Archiviere (nicht löschen!) die Original-Nachrichten
        m.shortTerm.ArchiveMessages(session.ID)
    }
    
    return nil
}
```

---

## Phase 4: Reflektives Gedächtnis

### Konzept
Der Agent sollte in der Lage sein, **über seine Erfahrungen nachzudenken** und daraus zu lernen.

### Reflektions-Trigger

```go
type ReflectionTrigger struct {
    Type     string   // "end_of_day", "after_error", "on_milestone"
    Condition func() bool
    Action    func()
}

var ReflectionTriggers = []ReflectionTrigger{
    {
        Type: "end_of_day",
        Condition: func() bool {
            // Nach der letzten User-Interaktion des Tages
            return time.Now().Hour() >= 22 && hasNewMemoriesToday()
        },
        Action: generateEndOfDayReflection,
    },
    {
        Type: "after_error",
        Condition: func() bool {
            // Nach 3 aufeinanderfolgenden Fehlern
            return consecutiveErrors >= 3
        },
        Action: generateErrorPatternAnalysis,
    },
}
```

### End-of-Day Reflektion

```go
func generateEndOfDayReflection() {
    // 1. Sammle Daten des Tages
    journal := getJournalEntries(today)
    ltmAdds := getNewLTMEntries(today)
    errors := getErrors(today)
    successes := getSuccesses(today)
    
    // 2. Generiere Reflektion via LLM
    prompt := fmt.Sprintf(`
Du bist ein Reflektions-Assistent für einen AI Agenten.

Heutige Aktivitäten:
- %d neue Erinnerungen gespeichert
- %d erfolgreiche Operationen
- %d Fehler aufgetreten

Journal-Einträge:
%s

Generiere eine kurze Reflektion (2-3 Sätze):
1. Was wurde heute erreicht?
2. Was könnte verbessert werden?
3. Was sollte man sich merken?

Antworte im Stil eines Tagebucheintrags.
`, len(ltmAdds), len(successes), len(errors), formatJournal(journal))

    reflection := llm.Complete(prompt)
    
    // 3. Speichere Reflektion
    journalDB.InsertEntry(JournalEntry{
        EntryType: "reflection",
        Title: fmt.Sprintf("Reflektion %s", today),
        Content: reflection,
        Date: today,
        AutoGenerated: true,
    })
}
```

---

## Implementierungs-Roadmap

### Sprint 1: Journal-Foundation (2 Wochen)
- [ ] Datenbank-Schema für Journal
- [ ] Journal-API (CRUD)
- [ ] Dashboard-Widget
- [ ] Manuelle Journal-Einträge

### Sprint 2: Automatische Journal-Einträge (2 Wochen)
- [ ] Trigger-System für Auto-Einträge
- [ ] Tool-Chain-Erkennung
- [ ] Erfolgs/Fehler-Erkennung
- [ ] Tageszusammenfassung (Nachts)

### Sprint 3: Knowledge Graph 2.0 (3 Wochen)
- [ ] Auto-Entitätsextraktion (Hintergrund-Job)
- [ ] Graph-Integration im Prompt
- [ ] Graph-Visualisierung (Text)
- [ ] Wissens-Lücken-Erkennung

### Sprint 4: Konsolidierung (2 Wochen)
- [ ] Konsolidierungs-Regeln
- [ ] Nächtlicher Cron-Job
- [ ] STM → LTM Transfer
- [ ] Archivierung statt Löschung

### Sprint 5: Reflektion (2 Wochen)
- [ ] Reflektions-Engine
- [ ] End-of-Day Zusammenfassung
- [ ] Wochenbericht
- [ ] Pattern-Erkennung

---

## Technische Architektur

### Neue Komponenten

```
internal/
├── memory/
│   ├── journal.go           # Journal-Logik
│   ├── journal_types.go     # Structs
│   ├── consolidation.go     # Konsolidierungs-Engine
│   ├── reflection.go        # Reflektions-Engine
│   └── entity_extractor.go  # Auto-Extraktion
├── agent/
│   └── memory_automation.go # Trigger-Handler
└── prompts/
    └── builder_memory.go    # Memory-Integration im Prompt
```

### Konfiguration

```yaml
# config.yaml
memory:
  journal:
    enabled: true
    auto_entries: true
    daily_summary: true
    summary_time: "03:00"  # Nächtliche Zusammenfassung
    
  knowledge_graph:
    enabled: true
    auto_extraction: true
    extraction_threshold: 0.7  # Konfidenz für Auto-Extraktion
    include_in_prompt: true
    max_nodes_in_prompt: 5
    
  consolidation:
    enabled: true
    interval_hours: 24
    rules: "default"  # oder "aggressive", "minimal"
    
  reflection:
    enabled: true
    end_of_day: true
    end_of_week: true
    on_error_pattern: true
```

---

## Erwartete Verbesserungen

| Metrik | Aktuell | Ziel | Wirkung |
|--------|---------|------|---------|
| **KG-Nutzung** | <5% der Sessions | >60% der Sessions | Bessere Kontextbehaltung über Sessions |
| **Memory-Erhalt** | 20 Nachrichten | "Unbegrenzt" durch LTM | Kein Vergessen wichtiger Fakten |
| **Kontinuität** | Niedrig | Hoch | Der Agent "erinnert sich" an Projekte |
| **User Experience** | Chatbot | Persönlicher Assistent | Emotionaler Bezug |
| **Fehlerwiederholung** | Oft | Selten | Lernen aus Fehlern |

---

## Risiken & Mitigation

| Risiko | Wahrscheinlichkeit | Mitigation |
|--------|-------------------|------------|
| **Zu viele Auto-Einträge** | Mittel | Qualitäts-Thresholds, User-Feedback-Loop |
| **Falsche Entitätsextraktion** | Hoch | Konfidenz-Scores, manuelle Korrektur-UI |
| **Token-Overhead im Prompt** | Mittel | Begrenzung auf Top-N Nodes, Caching |
| **LLM-Kosten für Extraktion** | Mittel | Lokales Modell für Extraktion, Batching |
| **Datenbank-Wachstum** | Niedrig | Archivierung, Kompression, Pruning |

---

## Fazit

Das vorgeschlagene System transformiert AuraGo von einem **zustandslosen Chatbot** zu einem **lernenden Assistenten** mit:

1. **Kontinuität** durch das Journal
2. **Strukturiertem Wissen** durch den aktiven Knowledge Graph
3. **Langzeitgedächtnis** durch automatische Konsolidierung
4. **Selbstverbesserung** durch reflektive Analysen

Die Implementierung ist **inkrementell** möglich - jedes Sprint liefert sofortigen Wert, ohne auf spätere Phasen warten zu müssen.
