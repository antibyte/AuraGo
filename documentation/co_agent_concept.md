# Co-Agent System — Konzept & Implementierungsplan

## 1. Überblick

Das Co-Agent-System erlaubt dem Main-Agent, eigenständige Helfer-Agenten zu spawnen, die **parallel** Aufgaben bearbeiten und Ergebnisse zurückliefern. Co-Agenten nutzen ein **separat konfigurierbares LLM-Model**, haben **eigene Circuit-Breaker-Limits** und sind vom Personality-System sowie vom Memory-Write-Pfad des Main-Agents isoliert.

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
│  │ Task: …  │  │ Task: …  │  │ Task: …  │                  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘                 │
│       │ Result       │ Result      │ Result                │
│       └──────────────┴─────────────┘                       │
│                      ▼                                     │
│              CoAgentRegistry                               │
│       (Status, Laufzeit, Ergebnis)                         │
└────────────────────────────────────────────────────────────┘
```

### Kernprinzipien

| Prinzip | Umsetzung |
|---------|-----------|
| **Isolation** | Co-Agenten beeinflussen weder Personality-Engine noch Memory des Main-Agents |
| **Read-Only Memory** | Co-Agenten erhalten einen Snapshot relevanter Informationen, schreiben aber nicht zurück |
| **Eigenes Limit** | Jeder Co-Agent hat eigenen Token-Counter und Circuit-Breaker |
| **Sicherheit** | Alle Security-Features (Guardian, Path-Traversal, etc.) gelten unverändert |
| **Keine User-Interaktion** | Co-Agenten kommunizieren nur über den Result-Channel mit dem Main-Agent |
| **Deterministisch stoppbar** | Main-Agent kann jeden Co-Agent per `context.Cancel()` sofort beenden |

---

## 2. Konfiguration

### config.yaml — Neue Section

```yaml
# Co-Agent System (optional parallel helpers)
co_agents:
  enabled: false
  max_concurrent: 3          # Max gleichzeitig laufende Co-Agenten
  
  # LLM-Konfiguration für Co-Agenten (eigenes Model)
  llm:
    provider: "openrouter"
    base_url: "https://openrouter.ai/api/v1"
    api_key: ""              # Leer = fällt auf llm.api_key zurück
    model: "meta-llama/llama-3.1-8b-instruct:free"  # Günstigeres/schnelleres Model
  
  # Eigene Limits pro Co-Agent
  circuit_breaker:
    max_tool_calls: 10       # Max Tool-Calls pro Co-Agent-Auftrag
    timeout_seconds: 120     # Max Laufzeit pro Co-Agent
    max_tokens: 8000         # Max Token-Budget pro Auftrag (0 = unbegrenzt)
```

### Config-Struct (Go)

```go
// In internal/config/config.go

type CoAgentConfig struct {
    Enabled       bool             `yaml:"enabled"`
    MaxConcurrent int              `yaml:"max_concurrent"`
    LLM           CoAgentLLMConfig `yaml:"llm"`
    CircuitBreaker CoAgentCBConfig `yaml:"circuit_breaker"`
}

type CoAgentLLMConfig struct {
    Provider string `yaml:"provider"`
    BaseURL  string `yaml:"base_url"`
    APIKey   string `yaml:"api_key"`
    Model    string `yaml:"model"`
}

type CoAgentCBConfig struct {
    MaxToolCalls   int `yaml:"max_tool_calls"`
    TimeoutSeconds int `yaml:"timeout_seconds"`
    MaxTokens      int `yaml:"max_tokens"`
}
```

Defaults: `MaxConcurrent: 3`, `MaxToolCalls: 10`, `TimeoutSeconds: 120`, `MaxTokens: 0`.

---

## 3. Architektur — Neue Komponenten

### 3.1 CoAgentRegistry (`internal/agent/coagent_registry.go`)

Thread-safe Registry aller laufenden Co-Agenten, analog zum bestehenden `ProcessRegistry` für Shell-Prozesse.

```go
package agent

import (
    "context"
    "fmt"
    "sync"
    "time"
)

// CoAgentState beschreibt den aktuellen Status eines Co-Agenten.
type CoAgentState string

const (
    CoAgentRunning   CoAgentState = "running"
    CoAgentCompleted CoAgentState = "completed"
    CoAgentFailed    CoAgentState = "failed"
    CoAgentCancelled CoAgentState = "cancelled"
)

// CoAgentInfo enthält alle Metadaten eines laufenden Co-Agenten.
type CoAgentInfo struct {
    ID          string          // Eindeutige ID (z.B. "coagent-1-<timestamp>")
    Task        string          // Aufgabenbeschreibung (vom Main-Agent vergeben)
    State       CoAgentState
    StartedAt   time.Time
    CompletedAt time.Time
    Result      string          // Ergebnis-Text (nach Abschluss)
    Error       string          // Fehlertext (bei Failure)
    TokensUsed  int             // Verbrauchte Tokens
    ToolCalls   int             // Anzahl durchgeführter Tool-Calls
    Cancel      context.CancelFunc // Zum Stoppen von außen
    mu          sync.Mutex
}

// Runtime gibt die bisherige Laufzeit zurück.
func (c *CoAgentInfo) Runtime() time.Duration {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.State == CoAgentRunning {
        return time.Since(c.StartedAt)
    }
    return c.CompletedAt.Sub(c.StartedAt)
}

// CoAgentRegistry verwaltet alle aktiven Co-Agenten.
type CoAgentRegistry struct {
    mu       sync.RWMutex
    agents   map[string]*CoAgentInfo
    counter  int
    maxSlots int
    logger   *slog.Logger
}

func NewCoAgentRegistry(maxSlots int, logger *slog.Logger) *CoAgentRegistry {
    return &CoAgentRegistry{
        agents:   make(map[string]*CoAgentInfo),
        maxSlots: maxSlots,
        logger:   logger,
    }
}

// AvailableSlots gibt die Anzahl freier Slots zurück.
func (r *CoAgentRegistry) AvailableSlots() int {
    r.mu.RLock()
    defer r.mu.RUnlock()
    running := 0
    for _, a := range r.agents {
        if a.State == CoAgentRunning {
            running++
        }
    }
    return r.maxSlots - running
}

// Register erstellt einen neuen Co-Agent-Eintrag und gibt die ID zurück.
// Gibt error zurück wenn alle Slots belegt sind.
func (r *CoAgentRegistry) Register(task string, cancel context.CancelFunc) (string, error) {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    running := 0
    for _, a := range r.agents {
        if a.State == CoAgentRunning {
            running++
        }
    }
    if running >= r.maxSlots {
        return "", fmt.Errorf("all %d co-agent slots are occupied", r.maxSlots)
    }
    
    r.counter++
    id := fmt.Sprintf("coagent-%d", r.counter)
    r.agents[id] = &CoAgentInfo{
        ID:        id,
        Task:      task,
        State:     CoAgentRunning,
        StartedAt: time.Now(),
        Cancel:    cancel,
    }
    r.logger.Info("Co-Agent registered", "id", id, "task", task)
    return id, nil
}

// Complete markiert einen Co-Agenten als abgeschlossen.
func (r *CoAgentRegistry) Complete(id, result string, tokensUsed, toolCalls int) {
    r.mu.Lock()
    defer r.mu.Unlock()
    if a, ok := r.agents[id]; ok {
        a.mu.Lock()
        a.State = CoAgentCompleted
        a.CompletedAt = time.Now()
        a.Result = result
        a.TokensUsed = tokensUsed
        a.ToolCalls = toolCalls
        a.mu.Unlock()
    }
}

// Fail markiert einen Co-Agenten als fehlgeschlagen.
func (r *CoAgentRegistry) Fail(id, errMsg string, tokensUsed, toolCalls int) {
    r.mu.Lock()
    defer r.mu.Unlock()
    if a, ok := r.agents[id]; ok {
        a.mu.Lock()
        a.State = CoAgentFailed
        a.CompletedAt = time.Now()
        a.Error = errMsg
        a.TokensUsed = tokensUsed
        a.ToolCalls = toolCalls
        a.mu.Unlock()
    }
}

// Stop bricht einen laufenden Co-Agenten ab.
func (r *CoAgentRegistry) Stop(id string) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    a, ok := r.agents[id]
    if !ok {
        return fmt.Errorf("co-agent '%s' not found", id)
    }
    if a.State != CoAgentRunning {
        return fmt.Errorf("co-agent '%s' is not running (state: %s)", id, a.State)
    }
    a.Cancel() // context.CancelFunc → beendet den LLM-Loop
    a.mu.Lock()
    a.State = CoAgentCancelled
    a.CompletedAt = time.Now()
    a.mu.Unlock()
    r.logger.Info("Co-Agent stopped", "id", id)
    return nil
}

// StopAll bricht alle laufenden Co-Agenten ab.
func (r *CoAgentRegistry) StopAll() {
    r.mu.Lock()
    defer r.mu.Unlock()
    for _, a := range r.agents {
        if a.State == CoAgentRunning {
            a.Cancel()
            a.mu.Lock()
            a.State = CoAgentCancelled
            a.CompletedAt = time.Now()
            a.mu.Unlock()
        }
    }
    r.logger.Info("All co-agents stopped")
}

// List gibt eine Übersicht aller Co-Agenten zurück (für Tool-Output).
func (r *CoAgentRegistry) List() []map[string]interface{} {
    r.mu.RLock()
    defer r.mu.RUnlock()
    var result []map[string]interface{}
    for _, a := range r.agents {
        entry := map[string]interface{}{
            "id":          a.ID,
            "task":        a.Task,
            "state":       string(a.State),
            "started_at":  a.StartedAt.Format(time.RFC3339),
            "runtime":     fmt.Sprintf("%.1fs", a.Runtime().Seconds()),
            "tokens_used": a.TokensUsed,
            "tool_calls":  a.ToolCalls,
        }
        if a.State == CoAgentCompleted {
            entry["result_preview"] = truncate(a.Result, 200)
        }
        if a.State == CoAgentFailed {
            entry["error"] = a.Error
        }
        result = append(result, entry)
    }
    return result
}

// GetResult gibt das vollständige Ergebnis eines abgeschlossenen Co-Agenten zurück.
func (r *CoAgentRegistry) GetResult(id string) (string, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    a, ok := r.agents[id]
    if !ok {
        return "", fmt.Errorf("co-agent '%s' not found", id)
    }
    switch a.State {
    case CoAgentRunning:
        return "", fmt.Errorf("co-agent '%s' is still running", id)
    case CoAgentCompleted:
        return a.Result, nil
    case CoAgentFailed:
        return "", fmt.Errorf("co-agent '%s' failed: %s", id, a.Error)
    case CoAgentCancelled:
        return "", fmt.Errorf("co-agent '%s' was cancelled", id)
    }
    return "", fmt.Errorf("unknown state")
}

// Cleanup entfernt abgeschlossene Einträge die älter als maxAge sind.
func (r *CoAgentRegistry) Cleanup(maxAge time.Duration) {
    r.mu.Lock()
    defer r.mu.Unlock()
    for id, a := range r.agents {
        if a.State != CoAgentRunning && time.Since(a.CompletedAt) > maxAge {
            delete(r.agents, id)
        }
    }
}
```

### 3.2 Co-Agent System-Prompt

Co-Agenten erhalten **nicht** den vollen System-Prompt des Main-Agents. Stattdessen bekommen sie einen **reduzierten Helfer-Prompt**, der:

- Ihre Rolle als Helfer klar definiert
- Kein Personality-Profil enthält
- Keinen Zugriff auf `manage_memory`, `manage_notes`, `knowledge_graph` (Write-Ops) gibt
- Relevanten Memory-Kontext als Read-Only-Snapshot enthält
- Die Sprache des Main-Agents übernimmt

**Datei:** `agent_workspace/prompts/coagent_system.md`

```markdown
# Co-Agent System Prompt

Du bist ein Hilfsagent (Co-Agent) des AuraGo-Systems. Deine Aufgabe ist es,
einen spezifischen Auftrag effizient zu bearbeiten und das Ergebnis zurückzuliefern.

## Regeln
- Du arbeitest NUR an der dir zugewiesenen Aufgabe
- Du kommunizierst NICHT mit dem Benutzer — dein Ergebnis geht an den Main-Agent
- Dein Ergebnis muss klar strukturiert und direkt verwertbar sein
- Du bearbeitest die Aufgabe so kompakt wie möglich
- Antworte in der Sprache: {{LANGUAGE}}

## Verfügbare Tools
Du kannst die gleichen Tools nutzen wie der Main-Agent, mit folgenden Einschränkungen:
- ❌ manage_memory (kein Memory-Schreiben)
- ❌ knowledge_graph write operations (kein Graph-Schreiben)
- ❌ manage_notes write operations (keine Notizen erstellen/ändern)
- ❌ co_agent (keine verschachtelten Co-Agenten)
- ❌ follow_up (kein Self-Scheduling)
- ❌ cron_scheduler (kein Cron-Zugriff)
- ✅ Alle anderen Tools: filesystem, execute_python, execute_shell, api_request,
     query_memory (lesen), knowledge_graph (lesen), etc.

## Kontext vom Main-Agent
{{CONTEXT_SNAPSHOT}}

## Deine Aufgabe
{{TASK}}
```

### 3.3 SpawnCoAgent-Funktion (`internal/agent/coagent.go`)

Die Kernfunktion, die einen neuen Co-Agenten startet:

```go
package agent

import (
    "context"
    "fmt"
    "time"

    "aurago/internal/config"
    "aurago/internal/llm"
    "aurago/internal/memory"
    "aurago/internal/security"
    "aurago/internal/tools"

    "github.com/sashabaranov/go-openai"
)

// CoAgentRequest definiert einen Auftrag an einen Co-Agenten.
type CoAgentRequest struct {
    Task           string   // Aufgabenbeschreibung
    ContextHints   []string // Optionale Memory-Kontexte die mitgegeben werden sollen
}

// SpawnCoAgent startet einen Co-Agenten als Goroutine.
// Gibt die Co-Agent-ID zurück oder einen Fehler wenn kein Slot frei ist.
func SpawnCoAgent(
    cfg *config.Config,
    parentCtx context.Context,
    logger *slog.Logger,
    registry *CoAgentRegistry,
    
    // Shared resources (Read-Only oder thread-safe)
    shortTermMem *memory.SQLiteMemory,
    longTermMem  memory.VectorDB,
    vault        *security.Vault,
    procRegistry *tools.ProcessRegistry,
    manifest     *tools.Manifest,
    kg           *memory.KnowledgeGraph,
    inventoryDB  *sql.DB,
    
    req CoAgentRequest,
) (string, error) {
    
    if !cfg.CoAgents.Enabled {
        return "", fmt.Errorf("co-agent system is disabled")
    }
    
    // 1. Slot prüfen & registrieren
    timeout := time.Duration(cfg.CoAgents.CircuitBreaker.TimeoutSeconds) * time.Second
    ctx, cancel := context.WithTimeout(parentCtx, timeout)
    
    coID, err := registry.Register(req.Task, cancel)
    if err != nil {
        cancel()
        return "", err
    }
    
    // 2. Eigenen LLM-Client erstellen
    coLLMConfig := buildCoAgentLLMConfig(cfg)
    coClient := llm.NewClient(coLLMConfig)
    
    // 3. System-Prompt bauen
    systemPrompt := buildCoAgentSystemPrompt(cfg, req, shortTermMem, longTermMem)
    
    // 4. Eigenen History-Manager (in-memory, nicht persistent)
    coHistoryMgr := memory.NewEphemeralHistoryManager()
    
    // 5. Goroutine starten
    go func() {
        defer cancel()
        
        coLogger := logger.With("component", "co-agent", "co_id", coID)
        coLogger.Info("Co-Agent started", "task", req.Task)
        
        // Co-Agent-spezifische Config-Kopie mit eigenen Limits
        coCfg := *cfg // Shallow copy
        coCfg.CircuitBreaker.MaxToolCalls = cfg.CoAgents.CircuitBreaker.MaxToolCalls
        coCfg.Agent.PersonalityEngine = false // Kein Personality-Einfluss
        
        llmReq := openai.ChatCompletionRequest{
            Model: coLLMConfig.Model,
            Messages: []openai.ChatCompletionMessage{
                {Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
                {Role: openai.ChatMessageRoleUser, Content: req.Task},
            },
        }
        
        // NoopBroker — Co-Agent sendet keine Events an UI
        broker := &NoopBroker{}
        sessionID := coID
        
        // Co-Agent spezifischer CronManager: nil (kein Cron-Zugriff)
        // Co-Agent Guardian: gleicher Guardian (Security gilt!)
        
        resp, err := RunSyncAgentLoop(
            &coCfg, ctx, coLogger, coClient, llmReq,
            shortTermMem, // READ: queries funktionieren, WRITE: wird durch Tool-Blacklist verhindert
            sessionID,
            vault,
            procRegistry,
            manifest,
            nil,              // cronManager = nil → cron_scheduler wird abgelehnt
            coHistoryMgr,     // Eigener ephemerer History-Manager
            broker,
            longTermMem,      // READ-ONLY: SearchSimilar funktioniert
            kg,
            inventoryDB,
            false,            // isMaintenance = false
            "",               // surgeryPlan = leer
        )
        
        if err != nil {
            coLogger.Error("Co-Agent failed", "error", err)
            registry.Fail(coID, err.Error(), 0, 0)
            return
        }
        
        result := ""
        if len(resp.Choices) > 0 {
            result = resp.Choices[0].Message.Content
        }
        tokensUsed := resp.Usage.TotalTokens
        
        coLogger.Info("Co-Agent completed",
            "tokens", tokensUsed,
            "result_len", len(result),
        )
        registry.Complete(coID, result, tokensUsed, 0)
    }()
    
    return coID, nil
}

// buildCoAgentLLMConfig erstellt die LLM-Config für einen Co-Agenten.
// Fällt auf die Main-Config zurück wenn Co-Agent-spezifische Felder leer sind.
func buildCoAgentLLMConfig(cfg *config.Config) *config.LLMConfig {
    co := cfg.CoAgents.LLM
    result := &config.LLMConfig{
        Provider: co.Provider,
        BaseURL:  co.BaseURL,
        APIKey:   co.APIKey,
        Model:    co.Model,
    }
    // Fallback auf Main-Config
    if result.BaseURL == "" {
        result.BaseURL = cfg.LLM.BaseURL
    }
    if result.APIKey == "" {
        result.APIKey = cfg.LLM.APIKey
    }
    if result.Model == "" {
        result.Model = cfg.LLM.Model
    }
    return result
}
```

---

## 4. Tool-Integration — `co_agent` Action

Der Main-Agent steuert Co-Agenten über ein neues Tool mit vier Operationen.

### 4.1 Tool-Schema

```json
{
  "action": "co_agent",
  "operation": "spawn | list | get_result | stop",
  "task": "Beschreibung der Aufgabe",
  "co_agent_id": "coagent-1",
  "context_hints": ["relevanter Kontext", "weitere Info"]
}
```

### 4.2 Operationen

| Operation | Parameter | Beschreibung |
|-----------|-----------|--------------|
| `spawn` | `task`, `context_hints` (optional) | Startet einen neuen Co-Agenten mit der Aufgabe |
| `list` | — | Zeigt alle Co-Agenten mit ID, Aufgabe, Status, Laufzeit |
| `get_result` | `co_agent_id` | Holt das fertige Ergebnis eines abgeschlossenen Co-Agenten |
| `stop` | `co_agent_id` | Bricht einen laufenden Co-Agenten ab |
| `stop_all` | — | Bricht alle laufenden Co-Agenten ab |

### 4.3 Dispatch-Code (Sketch)

```go
case "co_agent", "co_agents":
    if !cfg.CoAgents.Enabled {
        return `Tool Output: {"status":"error","message":"Co-Agent system is disabled"}`
    }
    switch tc.Operation {
    case "spawn":
        if tc.Task == "" {
            return `Tool Output: {"status":"error","message":"'task' is required"}`
        }
        coReq := CoAgentRequest{
            Task:         tc.Task,
            ContextHints: tc.ContextHints,
        }
        id, err := SpawnCoAgent(cfg, ctx, logger, coAgentRegistry,
            shortTermMem, longTermMem, vault, registry, manifest, kg, inventoryDB, coReq)
        if err != nil {
            return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
        }
        slots := coAgentRegistry.AvailableSlots()
        return fmt.Sprintf(`Tool Output: {"status":"ok","co_agent_id":"%s","available_slots":%d,"message":"Co-Agent gestartet. Nutze 'list' um Status zu prüfen und 'get_result' sobald fertig."}`, id, slots)
    
    case "list":
        list := coAgentRegistry.List()
        data, _ := json.Marshal(map[string]interface{}{
            "status":          "ok",
            "available_slots": coAgentRegistry.AvailableSlots(),
            "max_slots":       cfg.CoAgents.MaxConcurrent,
            "co_agents":       list,
        })
        return "Tool Output: " + string(data)
    
    case "get_result":
        if tc.CoAgentID == "" {
            return `Tool Output: {"status":"error","message":"'co_agent_id' is required"}`
        }
        result, err := coAgentRegistry.GetResult(tc.CoAgentID)
        if err != nil {
            return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
        }
        return fmt.Sprintf(`Tool Output: {"status":"ok","co_agent_id":"%s","result":%q}`, tc.CoAgentID, result)
    
    case "stop":
        if tc.CoAgentID == "" {
            return `Tool Output: {"status":"error","message":"'co_agent_id' is required"}`
        }
        if err := coAgentRegistry.Stop(tc.CoAgentID); err != nil {
            return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
        }
        return fmt.Sprintf(`Tool Output: {"status":"ok","message":"Co-Agent '%s' stopped"}`, tc.CoAgentID)
    
    case "stop_all":
        coAgentRegistry.StopAll()
        return `Tool Output: {"status":"ok","message":"All co-agents stopped"}`
    
    default:
        return `Tool Output: {"status":"error","message":"Unknown operation. Use: spawn, list, get_result, stop, stop_all"}`
    }
```

### 4.4 Neue ToolCall-Felder

```go
// In der ToolCall struct ergänzen:
CoAgentID    string   `json:"co_agent_id"`
Task         string   `json:"task"`         // Bereits vorhanden? Sonst ergänzen
ContextHints []string `json:"context_hints"`
```

---

## 5. Synchronisation & Datenübergabe

### 5.1 Lifecycle eines Co-Agent-Auftrags

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
    └─ Ergebnis in eigene Antwort basteln  │
```

### 5.2 Datenfluss — Was teilen, was isolieren?

| Ressource | Sharing-Modell | Begründung |
|-----------|---------------|------------|
| **Config** | Shallow Copy mit Co-Agent-Overrides | Eigene CircuitBreaker-Limits, Personality deaktiviert |
| **LLM Client** | **Eigene Instanz** | Anderes Model, eigene Rate-Limits |
| **ShortTermMem (SQLite)** | Shared Reference, Tool-Blacklist verhindert Writes | SQLite ist thread-safe. Queries (GetRecentMessages etc.) funktionieren. Write-Tools (manage_memory, manage_notes add/update/delete) werden im Tool-Dispatch geblockt |
| **LongTermMem (VectorDB)** | Shared Reference, Read-Only | SearchSimilar ist thread-safe (RWMutex in chromem-go). StoreDocument wird per Tool-Blacklist verhindert |
| **KnowledgeGraph** | Shared Reference, Read-Only | Query-Operationen thread-safe. Write-Ops per Tool-Blacklist verhindert |
| **Vault** | Shared Reference | Thread-safe (sync.Mutex). Co-Agent braucht ggf. API-Keys/Secrets |
| **ProcessRegistry** | Shared Reference | Co-Agent kann Shell-Befehle ausführen, Registry ist thread-safe |
| **HistoryManager** | **Eigene Instanz** (ephemeral) | Co-Agent hat eigenen Konversationsverlauf, nicht persistent |
| **Manifest** | Shared Reference, Read-Only | Tool-Manifest ist nach Init immutable |
| **CronManager** | **nil** | Co-Agent darf keine Cron-Jobs anlegen |
| **Personality Engine** | **Deaktiviert** | `coCfg.Agent.PersonalityEngine = false` |
| **Guardian/Security** | Shared Reference | Gleiche Sicherheitsregeln gelten |
| **CoAgentRegistry** | **Nicht übergeben** | Co-Agent kann keine Sub-Co-Agenten spawnen (Rekursion verhindert) |

### 5.3 Thread-Safety-Analyse

| Komponente | Mechanismus | Sicher für Parallel-Zugriff? |
|------------|------------|------------------------------|
| SQLiteMemory | SQLite WAL mode + Go sync | ✅ Ja — SQLite erlaubt concurrent reads + serialized writes |
| VectorDB (chromem) | `sync.RWMutex` intern | ✅ Ja — mehrere Reader parallel |
| KnowledgeGraph | `sync.RWMutex` | ✅ Ja |
| Vault | `sync.Mutex` | ✅ Ja |
| ProcessRegistry | `sync.RWMutex` | ✅ Ja |
| CoAgentRegistry | `sync.RWMutex` | ✅ Ja |
| HistoryManager | Eigene Instanz pro Co-Agent | ✅ Ja — kein Shared State |
| GlobalTokenCount | `sync.Mutex` (package-level) | ⚠️ Akkumuliert auch Co-Agent-Tokens. Gewünscht? → Ja, für globales Monitoring |

### 5.4 Tool-Blacklist für Co-Agenten

Im `DispatchToolCall()` wird eine Blacklist geprüft. Co-Agenten werden über ihre `sessionID` (Prefix `"coagent-"`) identifiziert:

```go
// Am Anfang von DispatchToolCall:
isCoAgent := strings.HasPrefix(sessionID, "coagent-")

// Vor jedem Write-Tool:
if isCoAgent {
    switch tc.Action {
    case "manage_memory":
        if tc.Operation != "read" && tc.Operation != "query" {
            return `Tool Output: {"status":"error","message":"Co-Agents cannot modify memory"}`
        }
    case "knowledge_graph":
        if tc.Operation != "query" && tc.Operation != "search" {
            return `Tool Output: {"status":"error","message":"Co-Agents cannot modify the knowledge graph"}`
        }
    case "manage_notes":
        if tc.Operation != "list" {
            return `Tool Output: {"status":"error","message":"Co-Agents cannot modify notes"}`
        }
    case "co_agent", "co_agents":
        return `Tool Output: {"status":"error","message":"Co-Agents cannot spawn sub-agents"}`
    case "follow_up":
        return `Tool Output: {"status":"error","message":"Co-Agents cannot schedule follow-ups"}`
    case "cron_scheduler":
        return `Tool Output: {"status":"error","message":"Co-Agents cannot manage cron jobs"}`
    }
}
```

### 5.5 Token-Limit-Enforcement

Der Co-Agent hat ein eigenes Token-Budget. Im `RunSyncAgentLoop` wird `sessionTokens` bereits mitgezählt. Die Prüfung kann analog zum Circuit-Breaker implementiert werden:

```go
// In RunSyncAgentLoop, nach dem LLM-Call:
if maxTokenBudget > 0 && sessionTokens >= maxTokenBudget {
    // Inject budget-exceeded message
    req.Messages = append(req.Messages, openai.ChatCompletionMessage{
        Role:    openai.ChatMessageRoleUser,
        Content: "TOKEN BUDGET EXCEEDED: Summarize your findings and provide the final result immediately.",
    })
}
```

Da `RunSyncAgentLoop` aktuell keinen `maxTokens`-Parameter hat, gibt es zwei Optionen:

**Option A — Config-basiert:** `coCfg.CoAgents.CircuitBreaker.MaxTokens` wird in der Loop-Copy geprüft.

**Option B — Neuer Parameter:** `RunSyncAgentLoop` erhält einen optionalen `maxSessionTokens int` Parameter (0 = unbegrenzt).

**Empfehlung: Option A** — vermeidet Signaturänderung, nutzt die bereits gesetzte Co-Agent-Config.

### 5.6 Ephemeral History Manager

Co-Agenten brauchen einen History-Manager der nicht auf eine JSON-Datei schreibt:

```go
// In internal/memory/history.go

// NewEphemeralHistoryManager erstellt einen In-Memory-only HistoryManager.
// Genutzt von Co-Agenten — kein Disk-Persist, kein Compression.
func NewEphemeralHistoryManager() *HistoryManager {
    return &HistoryManager{
        file:     "",           // Kein File → Save wird zum No-Op
        Messages: []HistoryMessage{},
        saveChan: make(chan struct{}, 1),
    }
}
```

Die bestehenden `Save()`-Methoden prüfen bereits `h.file` — wenn leer, kein Write.
Falls nicht: eine einfache Guard-Clause `if h.file == "" { return }` in `persistToDisk()` genügt.

---

## 6. Memory-Snapshot für Kontext

Wenn der Main-Agent einen Co-Agenten spawnt, kann er `context_hints` mitgeben. Diese werden zusammen mit dem Core-Memory und optionalen RAG-Ergebnissen in den System-Prompt des Co-Agenten injiziert.

### buildCoAgentSystemPrompt

```go
func buildCoAgentSystemPrompt(
    cfg *config.Config,
    req CoAgentRequest,
    stm *memory.SQLiteMemory,
    ltm memory.VectorDB,
) string {
    // 1. Basis-Template laden
    tmpl := loadCoAgentTemplate(cfg.Directories.PromptsDir)
    
    // 2. Core Memory (Read-Only Snapshot)
    coreMemory, _ := os.ReadFile(filepath.Join(cfg.Directories.DataDir, "core_memory.md"))
    
    // 3. Relevante RAG-Dokumente für die Aufgabe
    var ragContext string
    if ltm != nil {
        results, _, err := ltm.SearchSimilar(req.Task, 3)
        if err == nil && len(results) > 0 {
            ragContext = strings.Join(results, "\n---\n")
        }
    }
    
    // 4. User-provided Kontext-Hints
    hintsStr := strings.Join(req.ContextHints, "\n")
    
    // 5. Zusammenbauen
    context := fmt.Sprintf("## Core Memory\n%s\n\n## Relevanter Kontext\n%s\n\n## Zusätzliche Hinweise\n%s",
        string(coreMemory), ragContext, hintsStr)
    
    // 6. Template befüllen
    prompt := strings.ReplaceAll(tmpl, "{{LANGUAGE}}", cfg.Agent.SystemLanguage)
    prompt = strings.ReplaceAll(prompt, "{{CONTEXT_SNAPSHOT}}", context)
    prompt = strings.ReplaceAll(prompt, "{{TASK}}", req.Task)
    
    return prompt
}
```

---

## 7. Sicherheit

### 7.1 Geltende Security-Features

| Feature | Gilt für Co-Agenten? | Wie? |
|---------|---------------------|------|
| Path-Traversal-Guards | ✅ | Gleicher `DispatchToolCall` Code |
| Guardian (wenn implementiert) | ✅ | Gleiche Guardian-Instanz |
| Docker Name Validation | ✅ | Gleicher Code in docker.go |
| HTTP Client Timeouts | ✅ | Gleiche Clients / Timeouts |
| Vault Encryption | ✅ | Gleiche Vault-Instanz |
| Shell Execution Sandbox | ✅ | Gleiche Workspace-Beschränkungen |
| Circuit Breaker | ✅ | Eigene, konfigurierbare Limits |
| Context Timeout | ✅ | `context.WithTimeout` pro Co-Agent |

### 7.2 Zusätzliche Schutzmaßnahmen

1. **Rekursions-Schutz:** Co-Agenten können kein `co_agent`-Tool aufrufen → keine Spawn-Kaskaden
2. **Slot-Limit:** `max_concurrent` begrenzt RAM/CPU-Verbrauch
3. **Timeout:** Jeder Co-Agent hat ein hartes Zeitlimit via `context.WithTimeout`
4. **Token-Budget:** Optional begrenzbar pro Auftrag → verhindert Kosten-Explosion
5. **Kein User-Kontakt:** Co-Agenten nutzen `NoopBroker` → keine SSE/WebSocket-Events an den User
6. **Memory-Isolation:** Write-Ops werden per Tool-Blacklist blockiert → kein Memory-Corruption durch parallel schreibende Agenten

### 7.3 Risiko-Analyse

| Risiko | Schwere | Mitigierung |
|--------|---------|-------------|
| Co-Agent überschreibt Datei die Main-Agent gerade liest | Mittel | Filesystem-Ops sind per Design erlaubt. Shared workspace = akzeptables Risiko. Bei Bedarf: eigenes Workdir pro Co-Agent |
| Co-Agent verbraucht zu viele API-Tokens | Mittel | Token-Budget + Timeout |
| Co-Agent startet Shell-Prozess der den Server blockiert | Niedrig | Timeout + ProcessRegistry für Cleanup |
| Co-Agent-Ergebnis enthält Injection-Payload | Niedrig | Main-Agent parsed Ergebnis als Plain-Text, nicht als Tool-Call |
| Zu viele parallele LLM-Calls → Rate-Limiting | Mittel | `max_concurrent` begrenzen. Optional: Semaphore/Queue für LLM-Calls |

---

## 8. Implementation — Dateien & Reihenfolge

### Phase 1: Grundgerüst (Core)

| # | Datei | Beschreibung |
|---|-------|-------------|
| 1 | `internal/config/config.go` | `CoAgentConfig` struct + Defaults + YAML-Mapping |
| 2 | `config.yaml` | Co-Agent-Section hinzufügen (disabled by default) |
| 3 | `internal/agent/coagent_registry.go` | `CoAgentRegistry` Typ + alle Methoden |
| 4 | `internal/memory/history.go` | `NewEphemeralHistoryManager()` hinzufügen |
| 5 | `agent_workspace/prompts/coagent_system.md` | System-Prompt-Template für Co-Agenten |

### Phase 2: Agent-Integration

| # | Datei | Beschreibung |
|---|-------|-------------|
| 6 | `internal/agent/coagent.go` | `SpawnCoAgent()`, `buildCoAgentSystemPrompt()`, `buildCoAgentLLMConfig()` |
| 7 | `internal/agent/agent.go` | ToolCall-Felder (`CoAgentID`, `ContextHints`), Tool-Blacklist-Check, `co_agent` Dispatch-Case, Token-Budget-Check in Loop |
| 8 | `internal/agent/native_tools.go` | Native Tool-Schema für `co_agent` |
| 9 | `internal/server/server.go` | `CoAgentRegistry` Instanz erstellen und durchreichen |

### Phase 3: Prompt & Tool-Reference

| # | Datei | Beschreibung |
|---|-------|-------------|
| 10 | `agent_workspace/prompts/tools_manuals/co_agent.md` | Tool-Anleitung mit Beispielen |
| 11 | `agent_workspace/prompts/03_tools_registry.md` | Registry-Eintrag ergänzen |

### Phase 4: Feinschliff

| # | Datei | Beschreibung |
|---|-------|-------------|
| 12 | `internal/agent/agent.go` | Param-Promotion für `co_agent_id`, `task`, `context_hints` |
| 13 | `internal/agent/coagent_registry.go` | Cleanup-Goroutine (entfernt alte Einträge alle 10 Min) |
| 14 | Tests | `coagent_registry_test.go`, Integration-Test |

---

## 9. Beispiel-Interaktion

### User fragt den Main-Agent eine komplexe Frage

```
User: "Recherchiere parallel: 1) Die neuesten Go 1.26 Features, 
       2) Best Practices für Docker Multi-Stage Builds, 
       3) Aktuelle CVEs für Python 3.12"
```

### Main-Agent spawnt 3 Co-Agenten

```json
{"action": "co_agent", "operation": "spawn", "task": "Recherchiere die neuesten Go 1.26 Features. Nutze web_scraper und duckduckgo_search. Fasse die wichtigsten Neuerungen in einer strukturierten Liste zusammen."}
→ {"status": "ok", "co_agent_id": "coagent-1", "available_slots": 2}

{"action": "co_agent", "operation": "spawn", "task": "Recherchiere Best Practices für Docker Multi-Stage Builds. Nutze web_scraper. Fokus auf Go-Projekte."}
→ {"status": "ok", "co_agent_id": "coagent-2", "available_slots": 1}

{"action": "co_agent", "operation": "spawn", "task": "Recherchiere aktuelle CVEs für Python 3.12. Nutze duckduckgo_search und web_scraper."}
→ {"status": "ok", "co_agent_id": "coagent-3", "available_slots": 0}
```

### Main-Agent prüft Status

```json
{"action": "co_agent", "operation": "list"}
→ {
    "status": "ok",
    "available_slots": 0,
    "max_slots": 3,
    "co_agents": [
      {"id": "coagent-1", "task": "Go 1.26 Features...", "state": "completed", "runtime": "34.2s", "tokens_used": 2100},
      {"id": "coagent-2", "task": "Docker Multi-Stage...", "state": "running", "runtime": "28.5s"},
      {"id": "coagent-3", "task": "Python CVEs...", "state": "running", "runtime": "22.1s"}
    ]
  }
```

### Main-Agent holt Ergebnisse ab

```json
{"action": "co_agent", "operation": "get_result", "co_agent_id": "coagent-1"}
→ {"status": "ok", "result": "## Go 1.26 Features\n1. Verbesserter GC..."}
```

### Main-Agent fasst zusammen und antwortet dem User

```
Main-Agent: "Hier sind die Ergebnisse meiner Recherche:

**Go 1.26 Features:** [Zusammenfassung von coagent-1]
**Docker Best Practices:** [Zusammenfassung von coagent-2]  
**Python 3.12 CVEs:** [Zusammenfassung von coagent-3]"
```

---

## 10. Offene Design-Entscheidungen

| # | Frage | Optionen | Empfehlung |
|---|-------|----------|------------|
| 1 | **Soll der Main-Agent auf Co-Agent-Ergebnisse warten können?** | (a) Polling via `list`+`get_result` (b) Synchrones `spawn_and_wait` | **(a) Polling** — flexibler, Main-Agent kann andere Aufgaben zwischendurch erledigen |
| 2 | **Eigenes Workspace-Verzeichnis pro Co-Agent?** | (a) Shared Workspace (b) Temp-Dir pro Agent | **(a) Shared** — einfacher, gleiche Dateien verfügbar. Option (b) bei Bedarf nachrüstbar |
| 3 | **Token-Tracking: Global oder getrennt?** | (a) Alles in GlobalTokenCount (b) Getrennt | **(a) Global** — gibt Gesamtüberblick. Co-Agent-Verbrauch zusätzlich in Registry tracked |
| 4 | **Sollen Co-Agenten Native Functions nutzen?** | (a) Immer text-based (b) Config des Main-Agents übernehmen | **(b) Config übernehmen** — text-based ist robuster für günstige Models, also Default `false` für Co-Agent-LLMs |
| 5 | **Skills-Zugriff?** | (a) Voll (b) Eingeschränkt (c) Deaktiviert | **(a) Voll** — Skills sind i.d.R. Read-Only-Recherche-Tools |
| 6 | **Max Verschachtelungstiefe falls doch gewünscht?** | (a) Strikt 1 Level (b) Konfigurierbar | **(a) Strikt 1** — Einfachheit, Sicherheit. Kein Bedarf für Sub-Sub-Agenten |

---

## 11. Metriken & Monitoring

Die Co-Agent-Registry liefert bereits Token-/Laufzeit-Daten. Für die UI können diese per SSE/API exponiert werden:

```
GET /api/co-agents → Liste aller Co-Agenten mit Status
GET /api/co-agents/:id/result → Ergebnis eines Co-Agenten
```

Logging erfolgt über den shared `slog.Logger` mit dem Attribut `component=co-agent, co_id=coagent-N`.

---

## Zusammenfassung

Das Co-Agent-System erweitert AuraGo um **parallele, isolierte Helfer-Agenten** die:

- Ein **eigenes LLM-Model** nutzen (günstig/schnell für Hilfsaufgaben)
- **Thread-safe** auf die bestehenden Ressourcen zugreifen (Memory, Vault, DB)
- Per **Tool-Blacklist** vom Schreiben in Memory/KG/Notes ausgeschlossen sind
- **Eigene Limits** für Tokens, Tool-Calls und Laufzeit haben
- Per `context.Cancel()` jederzeit **stoppbar** sind
- Keine **Rekursion** ermöglichen (kein `co_agent`-Tool für Co-Agenten)
- Alle bestehenden **Sicherheitsfeatures** erben

Die Implementation nutzt bewusst den bereits vorhandenen `RunSyncAgentLoop` als Kern-Engine und benötigt nur **minimale Änderungen** am bestehenden Code — hauptsächlich eine neue Registry, einen neuen Tool-Dispatch-Case und Config-Erweiterung.
