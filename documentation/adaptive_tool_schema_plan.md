# Adaptives Tool-Schema-System

> **Konzept:** Dynamische Priorisierung basierend auf Nutzungsstatistik  
> **Ziel:** Top 10 Tools direkt verfügbar, Rest gruppiert, selbstlernend  
> **Erwartete Einsparung:** 60-70% bei optimaler UX

---

## 1. Kernkonzept

### Hybrid-Ansatz

```
┌─────────────────────────────────────────────────────────┐
│  SYSTEM PROMPT (Tools)                                  │
├─────────────────────────────────────────────────────────┤
│  🔥 HOT TOOLS (Top 10, individuell)                     │
│  - filesystem       [direkt, vollständiges Schema]      │
│  - manage_memory    [direkt, vollständiges Schema]      │
│  - docker           [direkt, vollständiges Schema]      │
│  - home_assistant   [direkt, vollständiges Schema]      │
│  - ... (6 weitere)                                    │
│                                                         │
│  📦 TOOL FAMILIEN (Rest, gruppiert)                     │
│  - infrastructure   [proxmox, ansible, tailscale]       │
│  - integrations     [adguard, ollama, mcp]              │
│  - media_ops        [tts, transcribe, registry]         │
│  - ...                                                │
└─────────────────────────────────────────────────────────┘
```

### Warum das funktioniert

| Aspekt | Vorteil |
|--------|---------|
| **Häufige Tools** | Direkter Zugriff, präzise Schemas, schnelle Erkennung durch LLM |
| **Seltene Tools** | Kompakt gruppiert, weniger Kontext-Overhead |
| **Adaptiv** | Passt sich an tatsächliches Nutzungsverhalten an |
| **Zeitliche Abklingung** | Alte Gewohnheiten werden vergessen, neue übernehmen |

---

## 2. Nutzungsstatistik-System

### Datenmodell

```go
// internal/agent/tool_usage_stats.go

package agent

import (
    "sync"
    "time"
)

// ToolUsageStats tracked Nutzungshäufigkeit mit zeitlicher Abklingung
type ToolUsageStats struct {
    mu sync.RWMutex
    
    // Einträge mit Zeitstempel für gewichtete Berechnung
    entries map[string]*ToolUsageEntry
    
    // Konfiguration
    config UsageStatsConfig
}

type ToolUsageEntry struct {
    ToolName      string
    Count         int       // Totale Aufrufe
    LastUsed      time.Time // Letzte Nutzung
    WeightedScore float64   // Berechneter Score mit zeitlicher Abklingung
}

type UsageStatsConfig struct {
    // Wie schnell vergessen wir alte Nutzung?
    // Halbwertszeit in Tagen (Default: 7)
    DecayHalfLifeDays float64
    
    // Wie viele "Hot Tools" direkt anbieten?
    HotToolCount int
    
    // Minimum Score um als "Hot" zu gelten
    MinHotScore float64
    
    // Persistenz-Intervall
    SaveInterval time.Duration
}

func DefaultUsageStatsConfig() UsageStatsConfig {
    return UsageStatsConfig{
        DecayHalfLifeDays: 7.0,  // Nach 7 Tagen halbiert sich der Wert
        HotToolCount:      10,
        MinHotScore:       1.0,  // Mindestens 1 Nutzung in der Halbwertszeit
        SaveInterval:      5 * time.Minute,
    }
}
```

### Score-Berechnung mit exponentiellem Verfall

```go
// Berechne gewichteten Score: Neuere Nutzung zählt mehr
func (e *ToolUsageEntry) CalculateScore(halfLifeDays float64) float64 {
    daysSinceLastUse := time.Since(e.LastUsed).Hours() / 24
    
    // Exponentieller Verfall: score = count * (0.5 ^ (days / halfLife))
    decayFactor := math.Pow(0.5, daysSinceLastUse/halfLifeDays)
    
    // Basis-Score + gewichtete Historie
    // Jede Nutzung zählt, aber verliert an Gewicht über Zeit
    e.WeightedScore = float64(e.Count) * decayFactor
    
    return e.WeightedScore
}

// Beispiel:
// - Tool A: 10x vor 1 Tag genutzt → Score: 10 * 0.5^(1/7) = 9.0
// - Tool B: 20x vor 14 Tagen genutzt → Score: 20 * 0.5^(14/7) = 5.0
// - Tool C: 5x vor 30 Tagen genutzt → Score: 5 * 0.5^(30/7) = 0.3 (vergessen)
```

### Tracking-Implementierung

```go
// ToolUsageStats Methoden

func (s *ToolUsageStats) RecordUsage(toolName string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    entry, exists := s.entries[toolName]
    if !exists {
        entry = &ToolUsageEntry{
            ToolName: toolName,
            Count:    0,
        }
        s.entries[toolName] = entry
    }
    
    entry.Count++
    entry.LastUsed = time.Now()
    
    // Score neu berechnen
    entry.CalculateScore(s.config.DecayHalfLifeDays)
}

func (s *ToolUsageStats) GetHotTools() []string {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    // Alle Scores aktualisieren
    var scored []*ToolUsageEntry
    for _, entry := range s.entries {
        score := entry.CalculateScore(s.config.DecayHalfLifeDays)
        if score >= s.config.MinHotScore {
            scored = append(scored, entry)
        }
    }
    
    // Nach Score sortieren (höchster zuerst)
    sort.Slice(scored, func(i, j int) bool {
        return scored[i].WeightedScore > scored[j].WeightedScore
    })
    
    // Top N zurückgeben
    limit := s.config.HotToolCount
    if len(scored) < limit {
        limit = len(scored)
    }
    
    result := make([]string, limit)
    for i := 0; i < limit; i++ {
        result[i] = scored[i].ToolName
    }
    
    return result
}

func (s *ToolUsageStats) GetStatsForDisplay() []ToolStatDisplay {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    var stats []ToolStatDisplay
    for _, entry := range s.entries {
        score := entry.CalculateScore(s.config.DecayHalfLifeDays)
        stats = append(stats, ToolStatDisplay{
            ToolName:      entry.ToolName,
            TotalUses:     entry.Count,
            LastUsed:      entry.LastUsed,
            CurrentScore:  score,
            IsHot:         score >= s.config.MinHotScore,
        })
    }
    
    // Nach Score sortieren
    sort.Slice(stats, func(i, j int) bool {
        return stats[i].CurrentScore > stats[j].CurrentScore
    })
    
    return stats
}

type ToolStatDisplay struct {
    ToolName     string
    TotalUses    int
    LastUsed     time.Time
    CurrentScore float64
    IsHot        bool
}
```

### Persistenz

```go
// Speichern in SQLite (ShortTermMem oder eigene Tabelle)

func (s *ToolUsageStats) Save(db *sql.DB) error {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // Tabelle leeren und neu befüllen
    _, err = tx.Exec("DELETE FROM tool_usage_stats")
    if err != nil {
        return err
    }
    
    stmt, err := tx.Prepare(`
        INSERT INTO tool_usage_stats (tool_name, count, last_used)
        VALUES (?, ?, ?)
    `)
    if err != nil {
        return err
    }
    defer stmt.Close()
    
    for _, entry := range s.entries {
        _, err = stmt.Exec(entry.ToolName, entry.Count, entry.LastUsed)
        if err != nil {
            return err
        }
    }
    
    return tx.Commit()
}

func (s *ToolUsageStats) Load(db *sql.DB) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    rows, err := db.Query(`
        SELECT tool_name, count, last_used 
        FROM tool_usage_stats
    `)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    for rows.Next() {
        var entry ToolUsageEntry
        err := rows.Scan(&entry.ToolName, &entry.Count, &entry.LastUsed)
        if err != nil {
            continue
        }
        s.entries[entry.ToolName] = &entry
    }
    
    return rows.Err()
}
```

---

## 3. Hybride Schema-Generierung

### Algorithmus

```go
// internal/agent/adaptive_tool_builder.go

package agent

func BuildAdaptiveToolSchemas(
    stats *ToolUsageStats,
    families map[string]ToolFamily,
    individualBuilders map[string]ToolBuilder,
    ff ToolFeatureFlags,
) []openai.Tool {
    
    var schemas []openai.Tool
    
    // 1. Hot Tools (Top 10) - individuelle Schemas
    hotTools := stats.GetHotTools()
    hotToolSet := make(map[string]bool)
    
    for _, toolName := range hotTools {
        hotToolSet[toolName] = true
        
        // Individuelles Schema bauen
        if builder, ok := individualBuilders[toolName]; ok {
            if tool := builder(ff); tool.Function != nil {
                schemas = append(schemas, tool)
            }
        }
    }
    
    // 2. Restliche Tools in Familien gruppieren
    // Aber: Wenn ein Tool in einer Familie "Hot" ist, 
    // die anderen in der Familie aber nicht,
    // dann trotzdem die ganze Familie als Gruppe?
    // → Ja, sonst haben wir Inkonsistenzen
    
    // Familien filtern: Nur die, die keine Hot Tools enthalten
    // ODER: Familien aufbauen aus den nicht-hot Tools
    
    coldToolsByFamily := make(map[string][]string)
    
    for familyName, family := range families {
        for _, toolName := range family.Tools {
            if !hotToolSet[toolName] {
                // Dieses Tool ist nicht hot → zur Familie hinzufügen
                coldToolsByFamily[familyName] = append(
                    coldToolsByFamily[familyName], 
                    toolName,
                )
            }
        }
    }
    
    // 3. Familien-Schemas bauen (nur aus Cold Tools)
    for familyName, toolNames := range coldToolsByFamily {
        if len(toolNames) == 0 {
            continue // Alle Tools dieser Familie sind hot
        }
        
        // Familie mit den verbleibenden Tools bauen
        family := families[familyName]
        tool := buildPartialFamilyTool(family, toolNames, ff)
        if tool.Function != nil {
            schemas = append(schemas, tool)
        }
    }
    
    return schemas
}
```

### Konkrete Implementierung

```go
// internal/agent/adaptive_tools.go

type AdaptiveToolManager struct {
    stats             *ToolUsageStats
    individualBuilders map[string]func(ToolFeatureFlags) openai.Tool
    families          map[string]ToolFamily
}

func NewAdaptiveToolManager(db *sql.DB) *AdaptiveToolManager {
    stats := NewToolUsageStats(DefaultUsageStatsConfig())
    stats.Load(db) // Vorherige Statistik laden
    
    return &AdaptiveToolManager{
        stats: stats,
        individualBuilders: map[string]func(ToolFeatureFlags) openai.Tool{
            "filesystem":       buildFilesystemTool,
            "shell":            buildShellTool,
            "docker":           buildDockerTool,
            "manage_memory":    buildManageMemoryTool,
            "home_assistant":   buildHomeAssistantTool,
            "query_memory":     buildQueryMemoryTool,
            "system_metrics":   buildSystemMetricsTool,
            "execute_skill":    buildExecuteSkillTool,
            "generate_image":   buildGenerateImageTool,
            "send_image":       buildSendImageTool,
        },
        families: map[string]ToolFamily{
            "infrastructure": {
                Name: "infrastructure",
                Tools: []string{"proxmox", "ansible", "tailscale", "cloudflare_tunnel"},
            },
            "integrations": {
                Name: "integrations", 
                Tools: []string{"adguard", "ollama", "mcp", "meshcentral"},
            },
            "media_secondary": {
                Name: "media_secondary",
                Tools: []string{"transcribe_audio", "tts", "media_registry"},
            },
            "development": {
                Name: "development",
                Tools: []string{"homepage", "github", "netlify", "document_creator"},
            },
            // ... weitere Familien
        },
    }
}

func (m *AdaptiveToolManager) GetToolSchemas(ff ToolFeatureFlags) []openai.Tool {
    hotTools := m.stats.GetHotTools()
    
    var schemas []openai.Tool
    usedTools := make(map[string]bool)
    
    // 1. Hot Tools als individuelle Schemas
    for _, toolName := range hotTools {
        if builder, ok := m.individualBuilders[toolName]; ok {
            if tool := builder(ff); tool.Function != nil {
                schemas = append(schemas, tool)
                usedTools[toolName] = true
            }
        }
    }
    
    // 2. Restliche Tools in Familien
    for familyName, family := range m.families {
        var availableOps []FamilyOperation
        
        for _, op := range family.Operations {
            // Nur Operationen hinzufügen, deren Tool nicht schon hot ist
            if !usedTools[op.OrigToolName] {
                availableOps = append(availableOps, op)
            }
        }
        
        if len(availableOps) > 0 {
            tool := buildFamilyTool(familyName, family.Description, availableOps, ff)
            schemas = append(schemas, tool)
        }
    }
    
    return schemas
}

func (m *AdaptiveToolManager) RecordToolUsage(toolName string) {
    m.stats.RecordUsage(toolName)
}

func (m *AdaptiveToolManager) GetHotTools() []string {
    return m.stats.GetHotTools()
}

func (m *AdaptiveToolManager) SaveStats(db *sql.DB) error {
    return m.stats.Save(db)
}
```

---

## 4. Integration in Agent Loop

### Tool-Usage Tracking

```go
// internal/agent/agent_loop.go

func ExecuteAgentLoop(ctx context.Context, req openai.ChatCompletionRequest, 
    runCfg RunConfig, ... ) (openai.ChatCompletionResponse, error) {
    
    // ... Setup ...
    
    // Adaptive Tool Manager initialisieren (oder aus RunConfig)
    adaptiveTools := runCfg.AdaptiveToolManager
    
    // Tool-Schemas dynamisch bauen
    if useAdaptiveTools {
        req.Tools = adaptiveTools.GetToolSchemas(ff)
        logger.Info("[AdaptiveTools] Built schemas", 
            "hot_tools", len(adaptiveTools.GetHotTools()),
            "total_schemas", len(req.Tools))
    }
    
    // ... Loop ...
    
    for {
        // ... Tool Call ausführen ...
        
        if tc.IsTool {
            // Tool-Nutzung tracken
            adaptiveTools.RecordToolUsage(tc.Action)
            
            // Dispatch...
            result := DispatchToolCall(ctx, tc, ...)
            
            // Periodisch speichern
            if time.Since(lastSave) > 5*time.Minute {
                adaptiveTools.SaveStats(runCfg.DB)
                lastSave = time.Now()
            }
        }
    }
}
```

### Dispatch mit Hot vs. Family

```go
// internal/agent/adaptive_dispatch.go

func DispatchAdaptiveTool(ctx context.Context, call ToolCall, 
    adaptiveTools *AdaptiveToolManager, cfg *config.Config, ...) string {
    
    // 1. Prüfen ob es ein Hot Tool ist (individuelles Dispatch)
    hotTools := adaptiveTools.GetHotTools()
    if contains(hotTools, call.Action) {
        // Direktes Dispatch zu ursprünglichem Handler
        return dispatchIndividualTool(ctx, call, cfg, ...)
    }
    
    // 2. Sonst Family Dispatch
    return dispatchFamilyTool(ctx, call, cfg, ...)
}

func dispatchIndividualTool(ctx context.Context, call ToolCall, ...) string {
    // Ursprüngliche Dispatch-Logik
    switch call.Action {
    case "filesystem":
        return tools.Filesystem(call.Operation, call.Params)
    case "docker":
        return tools.Docker(call.Operation, call.Params)
    // ...
    }
}
```

---

## 5. Konfiguration & Tuning

### Config-Optionen

```yaml
# config.yaml
agent:
  adaptive_tools:
    enabled: true
    
    # Wie viele Hot Tools direkt anbieten?
    hot_tool_count: 10
    
    # Halbwertszeit in Tagen (wie schnell vergessen wir?)
    # Niedriger = schnellerer Wechsel, höher = konservativer
    decay_half_life_days: 7
    
    # Minimum um als Hot zu gelten
    min_hot_score: 0.5
    
    # Tools die IMMER Hot sind (Override)
    always_hot:
      - filesystem
      - manage_memory
    
    # Tools die NIE Hot sein dürfen (immer in Familie)
    never_hot:
      - execute_sudo
      - delete_system
    
    # Persistenz
    save_interval_minutes: 5
```

### Dashboard-Anzeige

```go
// UI Endpoint für Tool-Statistik
func handleToolStats(w http.ResponseWriter, r *http.Request) {
    stats := adaptiveTools.GetStatsForDisplay()
    
    response := map[string]interface{}{
        "hot_tools": adaptiveTools.GetHotTools(),
        "all_stats": stats,
        "total_tools_tracked": len(stats),
    }
    
    json.NewEncoder(w).Encode(response)
}
```

**UI-Anzeige:**
```
┌─────────────────────────────────────┐
│ Tool Usage Statistics               │
├─────────────────────────────────────┤
│ 🔥 HOT TOOLS (Top 10)               │
│ 1. filesystem       Score: 45.2     │
│ 2. docker           Score: 38.7     │
│ 3. home_assistant   Score: 22.1     │
│ ...                                 │
├─────────────────────────────────────┤
│ 📊 RECENT ACTIVITY                  │
│ filesystem: 5x today                │
│ docker: 3x today                    │
│ ansible: 1x (last: 2 weeks ago)     │
└─────────────────────────────────────┘
```

---

## 6. Beispiel-Verhalten über Zeit

### Szenario: Neuer User

```
Tag 1: Alle Tools gleichberechtigt (keine Historie)
       → 10 zufällige Tools als "Hot", Rest gruppiert
       
Tag 2-3: User nutzt hauptsächlich docker und home_assistant
       → Diese steigen in Hot-Liste auf
       
Tag 7:  home_assistant wird kaum noch genutzt
       → Fällt aus Hot-Liste (Decay)
       
Tag 14: User entdeckt github Tool, nutzt es häufig
       → github steigt auf, ein anderes fällt raus
```

### Score-Berechnung Beispiel

```
Tool: filesystem
- Count: 50
- Last used: vor 1 Tag
- Score: 50 * 0.5^(1/7) = 45.2 🔥

Tool: ansible
- Count: 20
- Last used: vor 21 Tagen
- Score: 20 * 0.5^(21/7) = 2.5 ❄️

Tool: github
- Count: 5
- Last used: vor 2 Stunden
- Score: 5 * 0.5^(0.08/7) = 5.0 (steigt gerade)
```

---

## 7. Vorteile dieser Lösung

| Aspekt | Vorteil |
|--------|---------|
| **Selbstlernend** | Passt sich automatisch an Nutzungsverhalten an |
| **Keine Konfiguration nötig** | Funktioniert Out-of-the-Box |
| **Fair** | Häufig genutzte Tools bekommen besseren Kontext |
| **Gedächtnis** | Vergisst alte Gewohnheiten, lernt Neue |
| **Kontrollierbar** | Config-Overrides für spezielle Tools |
| **Transparent** | Dashboard zeigt warum welche Tools hot sind |

---

## 8. Implementierungs-Checkliste

### Phase 1: Core (3-4 Tage)

- [ ] `ToolUsageStats` Struktur implementieren
- [ ] Score-Berechnung mit exponentiellem Verfall
- [ ] SQLite Persistenz
- [ ] `AdaptiveToolManager` erstellen

### Phase 2: Integration (2-3 Tage)

- [ ] Hybride Schema-Generierung (Hot + Families)
- [ ] Dispatch-Logik anpassen
- [ ] Integration in `agent_loop.go`
- [ ] Usage Tracking bei jedem Tool-Call

### Phase 3: UI & Tuning (1-2 Tage)

- [ ] Dashboard-Endpoint für Statistik
- [ ] Config-Optionen
- [ ] Tests mit verschiedenen Nutzungsmustern

---

## 9. Erwartete Ergebnisse

| Szenario | Tokens | Hot Tools | Familien | Gesamt Schemas |
|----------|--------|-----------|----------|----------------|
| Neuer User | ~20.000 | 10 | ~8 | 18 |
| Docker-Power-User | ~20.000 | 10 (inkl. docker) | ~6 | 16 |
| Home-Automation-User | ~20.000 | 10 (inkl. HA) | ~6 | 16 |

**Token-Ersparnis:** 40-60% (durch Familien)  
**Qualitätsgewinn:** Hot Tools haben präzisere Schemas  
**Adaptivität:** Vollautomatisch, kein manuelles Tuning nötig

---

*Dieses System kombiniert die Effizienz der Tool-Familien mit der Präzision individueller Schemas für häufig genutzte Tools - vollständig selbstlernend.*
