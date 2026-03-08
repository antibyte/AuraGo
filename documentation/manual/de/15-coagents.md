# Kapitel 15: Co-Agenten

Co-Agenten sind parallele Sub-Agenten, die komplexe Aufgaben parallel bearbeiten. Sie nutzen ein separates LLM-Modell, haben eigene Limits und sind vom Haupt-Agenten isoliert.

---

## Was sind Co-Agenten?

Co-Agenten sind eigenständige Helfer-Agenten, die der Haupt-Agent (Main Agent) dynamisch spawnen kann. Sie arbeiten parallel an Teilaufgaben und liefern ihre Ergebnisse zurück.

```
┌────────────────────────────────────────────────────────────┐
│                        Main Agent                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                 │
│  │Personality│  │  Memory   │  │  Tools   │                 │
│  │  Engine   │  │  (R/W)   │  │ Dispatch │                 │
│  └──────────┘  └────┬─────┘  └──────────┘                 │
│                     │ READ-ONLY Snapshot                    │
│         ┌───────────┼───────────┐                          │
│         ▼           ▼           ▼                          │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐                    │
│  │Co-Agent 1│  │Co-Agent 2│  │Co-Agent 3│  (max_concurrent)│
│  │ Model B  │  │ Model B  │  │ Model B  │                  │
│  │ Task: A  │  │ Task: B  │  │ Task: C  │                  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘                 │
│       │ Result       │ Result      │ Result                │
│       └──────────────┴─────────────┘                       │
│                      ▼                                     │
│              CoAgentRegistry                               │
└────────────────────────────────────────────────────────────┘
```

### Kernprinzipien

| Prinzip | Beschreibung |
|---------|--------------|
| **Isolation** | Co-Agenten beeinflussen weder Personality-Engine noch Memory des Main-Agenten |
| **Read-Only Memory** | Co-Agenten erhalten einen Snapshot relevanter Informationen, schreiben aber nicht zurück |
| **Eigenes Limit** | Jeder Co-Agent hat eigenen Token-Counter und Circuit-Breaker |
| **Sicherheit** | Alle Security-Features (Guardian, Path-Traversal, etc.) gelten unverändert |
| **Keine User-Interaktion** | Co-Agenten kommunizieren nur über den Result-Channel mit dem Main-Agenten |
| **Deterministisch stoppbar** | Main-Agent kann jeden Co-Agenten per `context.Cancel()` sofort beenden |

---

## Wann Co-Agenten nutzen?

### Ideale Anwendungsfälle

| Szenario | Beispiel | Warum Co-Agenten? |
|----------|----------|-------------------|
| **Parallele Recherche** | "Recherchiere gleichzeitig: Go 1.26 Features, Docker Best Practices, Python CVEs" | 3x schneller als sequenziell |
| **Datenaggregation** | "Analysiere Logs von 5 verschiedenen Servern" | Isolierte Fehlerbehandlung pro Server |
| **Vergleichsstudie** | "Vergleiche Preise von 4 verschiedenen Anbietern" | Keine Beeinflussung durch vorherige Ergebnisse |
| **Multi-Source-Analyse** | "Extrahiere Daten aus PDF, Website und API" | Spezialisierte Tools pro Quelle |

### Wann NICHT nutzen

> ⚠️ **Warnung:** Co-Agenten sind NICHT geeignet für:
> - **Aufgaben mit Abhängigkeiten** (Task B braucht Ergebnis von Task A)
> - **Kontext-sensitive Operationen** (wo jeder Schritt auf dem vorherigen aufbaut)
> - **Sehr einfache Aufgaben** (Overhead zu hoch)
> - **Echtzeit-Interaktionen** (kein User-Feedback während der Ausführung)

---

## Konfiguration

### config.yaml – Co-Agent Section

```yaml
# Co-Agent System (optionale parallele Helfer)
co_agents:
  enabled: false                   # System aktivieren
  max_concurrent: 3                # Max. gleichzeitig laufende Co-Agenten
  
  # LLM-Konfiguration für Co-Agenten (eigenes Model)
  llm:
    provider: "openrouter"
    base_url: "https://openrouter.ai/api/v1"
    api_key: ""                    # Leer = fällt auf llm.api_key zurück
    model: "meta-llama/llama-3.1-8b-instruct:free"  # Günstigeres/schnelleres Model
  
  # Eigene Limits pro Co-Agent
  circuit_breaker:
    max_tool_calls: 10             # Max Tool-Calls pro Co-Agent-Auftrag
    timeout_seconds: 300           # Max Laufzeit pro Co-Agent (5 Min.)
    max_tokens: 0                  # Token-Budget (0 = unbegrenzt)
```

### Standardwerte

| Parameter | Default | Beschreibung |
|-----------|---------|--------------|
| `enabled` | `false` | Deaktiviert – muss explizit aktiviert werden |
| `max_concurrent` | `3` | Gleichzeitige Co-Agenten |
| `max_tool_calls` | `12` | Tool-Calls pro Auftrag |
| `timeout_seconds` | `300` | 5 Minuten Timeout |
| `max_tokens` | `0` | Unbegrenzt |

### Modell-Auswahl

> 💡 **Tipp:** Nutze für Co-Agenten ein kleineres, schnelleres Modell als den Main-Agenten:

| Main-Agent | Co-Agent (Empfohlen) | Grund |
|------------|----------------------|-------|
| GPT-4 | Llama-3.1-8B | Kosten sparen |
| Claude-3-Opus | Gemini-Flash | Geschwindigkeit |
| Arcee-Trinity | Qwen-2.5-7B | Lokale Modelle |

---

## Co-Agenten spawnen

### Über das Tool-System

Der Main-Agent spawnnt Co-Agenten über das `co_agent`-Tool mit der Operation `spawn`:

```json
{
  "action": "co_agent",
  "operation": "spawn",
  "task": "Recherchiere die neuesten Go 1.26 Features. Nutze web_search.",
  "context_hints": ["Wir arbeiten an einem Backend-Projekt", "Fokus auf Performance"]
}
```

### Antwort bei Erfolg

```json
{
  "status": "ok",
  "co_agent_id": "coagent-1",
  "available_slots": 2,
  "message": "Co-Agent gestartet. Nutze 'list' um Status zu prüfen und 'get_result' sobald fertig."
}
```

### Status-Überprüfung

```json
{
  "action": "co_agent",
  "operation": "list"
}
```

**Antwort:**
```json
{
  "status": "ok",
  "available_slots": 0,
  "max_slots": 3,
  "co_agents": [
    {
      "id": "coagent-1",
      "task": "Go 1.26 Features...",
      "state": "completed",
      "runtime": "34.2s",
      "tokens_used": 2100
    },
    {
      "id": "coagent-2",
      "task": "Docker Multi-Stage...",
      "state": "running",
      "runtime": "28.5s"
    }
  ]
}
```

### Ergebnis abrufen

```json
{
  "action": "co_agent",
  "operation": "get_result",
  "co_agent_id": "coagent-1"
}
```

**Antwort:**
```json
{
  "status": "ok",
  "co_agent_id": "coagent-1",
  "result": "## Go 1.26 Features\n1. Verbesserter Garbage Collector..."
}
```

---

## Co-Agenten verwalten

### Alle Operationen im Überblick

| Operation | Parameter | Beschreibung |
|-----------|-----------|--------------|
| `spawn` | `task`, `context_hints` | Startet neuen Co-Agenten |
| `list` | — | Zeigt alle Co-Agenten mit Status |
| `get_result` | `co_agent_id` | Holt Ergebnis eines abgeschlossenen Co-Agenten |
| `stop` | `co_agent_id` | Bricht laufenden Co-Agenten ab |
| `stop_all` | — | Bricht ALLE Co-Agenten ab |

### Co-Agenten stoppen

```json
// Einzelnen stoppen
{
  "action": "co_agent",
  "operation": "stop",
  "co_agent_id": "coagent-2"
}

// Alle stoppen (Notfall)
{
  "action": "co_agent",
  "operation": "stop_all"
}
```

### Zustände eines Co-Agenten

| Status | Bedeutung | Aktion möglich |
|--------|-----------|----------------|
| `running` | Läuft noch | `stop`, `list` |
| `completed` | Erfolgreich beendet | `get_result` |
| `failed` | Fehler aufgetreten | `get_result` (zeigt Fehler) |
| `cancelled` | Manuell abgebrochen | `list` |

---

## Use Cases und Beispiele

### Use Case 1: Parallele Recherche

**User-Anfrage:**
> "Recherchiere parallel: 1) Neueste Go 1.26 Features, 2) Docker Multi-Stage Best Practices, 3) Python 3.12 CVEs"

**Ausführung durch Main-Agent:**

```json
// Co-Agent 1 spawnen
{"action": "co_agent", "operation": "spawn", "task": "Recherchiere Go 1.26 Features"}
→ coagent-1

// Co-Agent 2 spawnen
{"action": "co_agent", "operation": "spawn", "task": "Recherchiere Docker Multi-Stage Best Practices"}
→ coagent-2

// Co-Agent 3 spawnen
{"action": "co_agent", "operation": "spawn", "task": "Recherchiere Python 3.12 CVEs"}
→ coagent-3
```

**Nach Abschluss:**
```json
{"action": "co_agent", "operation": "get_result", "co_agent_id": "coagent-1"}
{"action": "co_agent", "operation": "get_result", "co_agent_id": "coagent-2"}
{"action": "co_agent", "operation": "get_result", "co_agent_id": "coagent-3"}
```

**Gesamtlaufzeit:** ~35 Sekunden statt ~90 Sekunden sequenziell

### Use Case 2: Multi-Server Log-Analyse

```json
// Für jeden Server einen Co-Agenten
{"action": "co_agent", "operation": "spawn", 
 "task": "Analysiere /var/log/syslog auf server01 auf Fehler. Nutze ssh_execute."}

{"action": "co_agent", "operation": "spawn",
 "task": "Analysiere /var/log/syslog auf server02 auf Fehler. Nutze ssh_execute."}

{"action": "co_agent", "operation": "spawn",
 "task": "Analysiere /var/log/syslog auf server03 auf Fehler. Nutze ssh_execute."}
```

### Use Case 3: Daten-Extraktion aus verschiedenen Quellen

```json
// PDF-Analyse
{"action": "co_agent", "operation": "spawn",
 "task": "Extrahiere alle Tabellen aus report.pdf", "context_hints": ["Finanzbericht Q4"]}

// Website-Scraping
{"action": "co_agent", "operation": "spawn",
 "task": "Scrape Preise von example.com/produkte"}

// API-Abfrage
{"action": "co_agent", "operation": "spawn",
 "task": "Rufe REST-API ab: GET api.example.com/v2/stats"}
```

---

## Limitierungen und Constraints

### Architektonische Limits

| Limit | Wert | Erklärung |
|-------|------|-----------|
| **Max Concurrent** | Konfigurierbar (Default: 3) | Begrenzt RAM/CPU-Verbrauch |
| **Nested Co-Agents** | Verboten | Co-Agenten können keine Sub-Co-Agenten spawnen |
| **Memory Writes** | Verboten | Co-Agenten können Memory/Notes/KG nicht verändern |
| **Cron-Jobs** | Verboten | Kein Zugriff auf Scheduler |
| **User-Interaktion** | Keine | Co-Agenten nutzen `NoopBroker` |

### Tool-Blacklist für Co-Agenten

Co-Agenten können folgende Tools **NICHT** nutzen:

| Tool | Einschränkung |
|------|---------------|
| `manage_memory` | Kein Schreiben (add/update/delete) |
| `knowledge_graph` | Keine Write-Operationen |
| `manage_notes` | Kein Erstellen/Ändern von Notizen |
| `co_agent` | Keine Rekursion (keine Sub-Co-Agenten) |
| `follow_up` | Kein Self-Scheduling |
| `cron_scheduler` | Kein Cron-Zugriff |

### Zeitliche Limits

```yaml
# Default: 5 Minuten pro Co-Agent
circuit_breaker:
  timeout_seconds: 300
```

> ⚠️ **Warnung:** Wenn ein Co-Agent das Timeout erreicht, wird er hart beendet. Das Ergebnis ist dann unvollständig.

---

## Ressourcen-Management

### Speicher-Verbrauch

| Komponente | Pro Co-Agent | Bei 3 Concurrent |
|------------|--------------|------------------|
| LLM-Context | ~2-8 MB | ~6-24 MB |
| History (ephemeral) | ~1-4 MB | ~3-12 MB |
| Goroutine-Stack | ~2 KB | ~6 KB |
| **Gesamt** | ~3-12 MB | ~9-36 MB |

### CPU-Nutzung

- Co-Agenten laufen in eigenen Goroutinen
- Parallelität durch Go-Scheduler verwaltet
- LLM-Calls sind Netzwerk-bound, nicht CPU-bound

### Token-Budget

```yaml
# Optional: Token-Limit pro Co-Agent
circuit_breaker:
  max_tokens: 8000          # Co-Agent muss bei Überschreitung zusammenfassen
```

> 💡 **Tipp:** Setze `max_tokens` für kostenkontrollierte Umgebungen. Der Co-Agent erhält dann eine Systemnachricht, seine Ergebnisse zu komprimieren.

---

## Sicherheitsaspekte

### Geltende Security-Features

| Feature | Gilt für Co-Agenten? | Implementierung |
|---------|---------------------|-----------------|
| Path-Traversal-Guards | ✅ Ja | Gleicher `DispatchToolCall` Code |
| Guardian | ✅ Ja | Gleiche Guardian-Instanz |
| Docker Name Validation | ✅ Ja | Gleicher Code in docker.go |
| HTTP Client Timeouts | ✅ Ja | Gleiche Clients/Timeouts |
| Vault Encryption | ✅ Ja | Gleiche Vault-Instanz |
| Shell Execution Sandbox | ✅ Ja | Gleiche Workspace-Beschränkungen |
| Circuit Breaker | ✅ Ja | Eigene, konfigurierbare Limits |
| Context Timeout | ✅ Ja | `context.WithTimeout` pro Co-Agent |

### Zusätzliche Schutzmaßnahmen

1. **Rekursions-Schutz:** Kein `co_agent`-Tool für Co-Agenten
2. **Slot-Limit:** Begrenzt parallele Ausführung
3. **Timeout:** Hartes Zeitlimit pro Co-Agent
4. **Token-Budget:** Optional begrenzbar
5. **Kein User-Kontakt:** `NoopBroker` statt Event-Broker
6. **Memory-Isolation:** Write-Ops per Tool-Blacklist blockiert

### Risiko-Analyse

| Risiko | Schwere | Mitigierung |
|--------|---------|-------------|
| Co-Agent überschreibt Datei, die Main-Agent liest | Mittel | Shared Workspace = akzeptables Risiko |
| Co-Agent verbraucht zu viele API-Tokens | Mittel | Token-Budget + Timeout |
| Co-Agent blockiert Server durch Shell-Prozess | Niedrig | Timeout + ProcessRegistry |
| Injection-Payload im Ergebnis | Niedrig | Main-Agent parsed als Plain-Text |
| Rate-Limiting durch parallele LLM-Calls | Mittel | `max_concurrent` begrenzen |

---

## Monitoring paralleler Agenten

### Über die Web-UI

Das Dashboard zeigt aktive Co-Agenten an:

```
┌─────────────────────────────────────────┐
│  Co-Agent Status                        │
├─────────────────────────────────────────┤
│  🟢 coagent-1  [completed]  34s         │
│  🟡 coagent-2  [running]    28s         │
│  🟡 coagent-3  [running]    22s         │
│                                         │
│  Slots: 0/3 belegt                      │
└─────────────────────────────────────────┘
```

### Über die API

```bash
# Status abfragen
curl http://localhost:8088/api/co-agents

# Einzelnen Co-Agenten abfragen
curl http://localhost:8088/api/co-agents/coagent-1/result
```

### Logging

Co-Agent-Aktivitäten werden mit `component=co-agent` geloggt:

```
2026-03-08T10:30:00Z [co-agent] [co_id=coagent-1] Co-Agent started task="Recherchiere..."
2026-03-08T10:30:34Z [co-agent] [co_id=coagent-1] Co-Agent completed tokens=2100
2026-03-08T10:30:34Z [co-agent] [co_id=coagent-2] Co-Agent stopped (timeout)
```

### Metriken

| Metrik | Quelle | Nutzen |
|--------|--------|--------|
| Laufzeit | Registry | Performance-Analyse |
| Tokens Used | Registry | Kostenkontrolle |
| Tool Calls | Registry | Effizienz-Messung |
| Success Rate | Logs | Zuverlässigkeit |

---

## 🔍 Deep Dive: Lifecycle eines Co-Agenten

```
Main-Agent                          Co-Agent Goroutine
    │                                      │
    ├─ spawn("Analysiere X")               │
    │   ├─ Slot prüfen ✓                   │
    │   ├─ context.WithTimeout()           │
    │   ├─ Registry.Register() ──────────► │ Goroutine startet
    │   └─ return co_agent_id              │ ├─ Eigener LLM-Client
    │                                      │ ├─ System-Prompt (Helfer)
    │   ... Main-Agent arbeitet weiter ... │ ├─ RunSyncAgentLoop()
    │                                      │ │  ├─ LLM Call
    ├─ list() ← Status: "running"         │ │  ├─ Tool Dispatch
    │                                      │ │  ├─ LLM Call
    │                                      │ │  └─ Final Answer
    │                                      │ └─ Registry.Complete(result)
    │                                      │
    ├─ list() ← Status: "completed"        │
    ├─ get_result(id) ← Ergebnis           │
    └─ Ergebnis in Antwort integrieren     │
```

### Ressourcen-Sharing

| Ressource | Sharing-Modell | Begründung |
|-----------|---------------|------------|
| **Config** | Shallow Copy mit Overrides | Eigene Limits, Personality deaktiviert |
| **LLM Client** | Eigene Instanz | Anderes Model, eigene Rate-Limits |
| **ShortTermMem** | Shared, Read-Only | SQLite ist thread-safe |
| **LongTermMem** | Shared, Read-Only | VectorDB mit RWMutex |
| **Vault** | Shared Reference | Thread-safe, API-Keys benötigt |
| **HistoryManager** | Eigene Instanz | Ephemeral (nicht persistent) |
| **CronManager** | nil | Kein Cron-Zugriff |

---

## Troubleshooting

### Problem: "all 3 co-agent slots are occupied"

**Lösung:**
```json
// Laufende Co-Agenten prüfen
{"action": "co_agent", "operation": "list"}

// Alte Co-Agenten aufräumen oder warten
{"action": "co_agent", "operation": "stop_all"}
```

**Prävention:** Erhöhe `max_concurrent` in der Config:
```yaml
co_agents:
  max_concurrent: 5
```

### Problem: Co-Agent hängt (timeout)

**Symptom:** Status bleibt `running`, aber keine Fortschritte

**Lösung:**
```json
// Manuell stoppen
{"action": "co_agent", "operation": "stop", "co_agent_id": "coagent-X"}
```

**Prävention:** Reduziere `timeout_seconds` oder `max_tool_calls`:
```yaml
circuit_breaker:
  timeout_seconds: 120
  max_tool_calls: 8
```

### Problem: Ergebnis ist leer

**Ursachen:**
1. Task war zu vage
2. Co-Agent hat Timeout erreicht
3. LLM hat keinen Content zurückgegeben

**Debug:**
```json
// Mit context_hints mehr Kontext geben
{"action": "co_agent", "operation": "spawn",
 "task": "...",
 "context_hints": ["Sehr präzise Anweisung", "Erwartetes Format: Markdown-Liste"]}
```

---

## Zusammenfassung

| Aspekt | Empfehlung |
|--------|------------|
| **Aktivierung** | `co_agents.enabled: true` |
| **Modell** | Günstigeres/schnelleres als Main-Agent |
| **Concurrent** | 3 für Standard-Workload, 5+ für Server |
| **Timeout** | 120–300 Sekunden je nach Task |
| **Token-Budget** | 0 (unbegrenzt) oder 8000+ bei Kostenkontrolle |
| **Einsatz** | Parallele, unabhängige Aufgaben |
| **Vermeiden** | Abhängige Tasks, Echtzeit-Interaktion |

---

> 💡 **Nächste Schritte:**
> - **[Sicherheit](14-sicherheit.md)** – Co-Agenten sicher konfigurieren
> - **[Konfiguration](07-konfiguration.md)** – Alle Parameter im Detail
> - **[Tools](06-tools.md)** – Verfügbare Werkzeuge kennenlernen
