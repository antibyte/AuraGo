# Umsetzungsplan: Headroom-Optimierungen für AuraGo — Revision 2

**Datum:** 2026-06-05
**Status:** Überarbeitet nach Review (`reports/headroom_plan_review.md`)
**Ziel:** Realistischer, sicherer, schrittweise rollbar

---

## Änderungen zur Version 1

| Problem in V1 | Lösung in V2 |
|---------------|--------------|
| Paket 5 (KV-Cache) redundant/falsch | **Gestrichen**. OpenAI-Caching existiert bereits; Anthropic-Caching ist separates Thema. |
| Paket 4 baut auf nichtexistierendem "Session-Ende" auf | Umgestaltet auf **Turn-basiert** (nach Recovery) + **zeitbasiert** (inaktive Sessions) + **on-reset**. |
| Paket 1: Operator-Präzedenz-Bug | **Korrigiert** — Klammern gesetzt. |
| Paket 2: Abruf ohne Limit | **Korrigiert** — 32 KB Abruf-Limit im Tool. |
| 15 neue Config-Optionen | **Reduziert auf 5** — nur `enabled`-Flags + 1–2 essentielle Parameter pro Paket. |
| Keine Beta-Modi | **Eingeführt** — jedes Paket hat `mode: "log_only" \| "active"`. |
| Zeitplan zu optimistisch | **Verdoppelt** — realistische Schätzungen mit Puffer. |
| `.claude/rules/` als Kern-Feature | **Gestrichen** — nur SQLite-Runtime-Inject. |
| `json.Unmarshal` in `interface{}` unreflektiert | **Eingeschränkt** — nur bei Inputs > 2000 Chars und JSON-Validierung vorab. |

---

## Übersicht der 4 Arbeitspakete (statt 5)

| # | Paket | Komplexität | Realistischer Aufwand | Impact | Beta-Modus |
|---|-------|-------------|----------------------|--------|------------|
| 1 | **SmartCrusher (JSON-Compressor)** | Niedrig-Mittel | 2–3 Tage | 🟡 Mittel-Hoch | `log_only` → `active` |
| 2 | **Reversible Compression (CCR)** | Mittel | 3–4 Tage | 🔴 Hoch | `log_only` → `active` |
| 3 | **Message Importance Scoring** | Mittel-Hoch | 4–5 Tage | 🔴 Hoch | **Nur** `log_only` für 1 Woche, dann `active` |
| 4 | **Turn-basiertes Lernen (headroom learn light)** | Mittel | 4–5 Tage | 🟡 Mittel-Hoch | `log_only` → `active` |

**Empfohlene Reihenfolge:** 1 → 2 (Store) → 3 (log_only) → 3 (active) → 2 (Tool) → 4

---

## Paket 1: SmartCrusher (JSON-Compressor)

### 1.1 Ziel
Universeller JSON-Compressor für Arrays of Objects, unabhängig vom Tool-Typ. Keine Änderungen am Agent-Loop.

### 1.2 Design

```go
// internal/tools/outputcompress/smart_crusher.go

const smartCrusherMinInputChars = 2000

type SmartCrusherConfig struct {
    Enabled  bool
    MaxRows  int // Default: 50
    TailRows int // Default: 5
    MaxCols  int // Default: 20
}

func smartCrushJSON(input string, cfg SmartCrusherConfig) (string, bool) {
    // 1. Schnelle Abbruchbedingungen
    if len(input) < smartCrusherMinInputChars {
        return input, false
    }
    trimmed := strings.TrimSpace(input)
    if !(strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{")) {
        return input, false
    }
    
    // 2. JSON validieren (billiger als Unmarshal)
    if !json.Valid([]byte(trimmed)) {
        return input, false
    }
    
    // 3. Unmarshal nur wenn wahrscheinlich lohnenswert
    var data interface{}
    if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
        return input, false
    }
    
    if arr, ok := data.([]interface{}); ok && len(arr) > 2 {
        if isArrayOfUniformObjects(arr) {
            return crushArrayOfObjects(arr, cfg), true
        }
    }
    
    // 4. Fallback: minifiziertes JSON (nur wenn signifikant kürzer)
    minified, _ := json.Marshal(data)
    if len(minified) < len(input)*0.85 {
        return string(minified), true
    }
    return input, false
}
```

#### Tabellarisches Format (sicher)

```go
func crushArrayOfObjects(arr []interface{}, cfg SmartCrusherConfig) string {
    sharedKeys := findSharedKeys(arr, 0.5)
    if len(sharedKeys) == 0 || len(sharedKeys) > cfg.MaxCols {
        // Zu heterogen oder zu breit → minifizieren
        b, _ := json.Marshal(arr)
        return string(b)
    }
    
    var b strings.Builder
    b.WriteString("[JSON_ARRAY_COMPACT]\n")
    b.WriteString(escapeForTSV(strings.Join(sharedKeys, "\t")))
    b.WriteString("\n")
    
    limit := min(len(arr), cfg.MaxRows)
    tailStart := -1
    if len(arr) > limit+cfg.TailRows {
        tailStart = len(arr) - cfg.TailRows
    }
    
    for i, item := range arr {
        if i >= limit && i < tailStart {
            if i == limit {
                b.WriteString(fmt.Sprintf("... (%d rows omitted) ...\n", tailStart-limit))
            }
            continue
        }
        if obj, ok := item.(map[string]interface{}); ok {
            vals := make([]string, len(sharedKeys))
            for j, key := range sharedKeys {
                vals[j] = escapeForTSV(compactValue(obj[key]))
            }
            b.WriteString(strings.Join(vals, "\t"))
            b.WriteString("\n")
        }
    }
    return b.String()
}

func escapeForTSV(s string) string {
    s = strings.ReplaceAll(s, "\t", " ")
    s = strings.ReplaceAll(s, "\n", " ")
    s = strings.ReplaceAll(s, "\r", "")
    return s
}
```

#### Integration (kein `goto`)

```go
// internal/tools/outputcompress/compressor.go

// Nach TOONJSON, vor dem domain-spezifischen Switch:
if cfg.SmartCrusher.Enabled && len(output) >= smartCrusherMinInputChars {
    if crushed, ok := smartCrushJSON(output, cfg.SmartCrusher); ok {
        // Vergleiche mit TOONJSON-Ergebnis (falls vorhanden)
        if len(crushed) < len(result) {
            result = crushed
            filter = "smart-crusher"
        }
    }
}
```

### 1.3 Konfiguration (minimal)

```go
// config_types.go
SmartCrusher struct {
    Enabled  bool `yaml:"enabled"`
    MaxRows  int  `yaml:"max_rows"`  // Default: 50
} `yaml:"smart_crusher"`
```

`MaxRows` ist der einzige nutzerrelevante Parameter. `TailRows` (5) und `MaxCols` (20) werden hartkodiert.

### 1.4 Tests

- Gold-File-Tests mit realen Samples: `docker ps --format json`, GitHub API, `kubectl get pods -o json`
- Performance-Benchmark: 1 MB JSON → Ziel < 100 ms
- Rollback-Test: Wenn SmartCrusher expandiert oder nur < 5% spart → Original beibehalten

---

## Paket 2: Reversible Compression (CCR)

### 2.1 Ziel
Original-Tool-Outputs archivieren, damit das LLM bei Bedarf eine komprimierte Version zurückbekommen kann.

### 2.2 Design

#### SQLite-Tabelle

```sql
-- internal/memory/short_term_init.go (im Schema-Block)
CREATE TABLE IF NOT EXISTS compressed_tool_outputs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    tool_call_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    original_content TEXT NOT NULL,
    compressed_content TEXT NOT NULL,
    compression_ratio REAL,
    filter_used TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    accessed_at DATETIME,
    access_count INTEGER DEFAULT 0,
    UNIQUE(session_id, tool_call_id)
);
CREATE INDEX IF NOT EXISTS idx_cto_session ON compressed_tool_outputs(session_id);
CREATE INDEX IF NOT EXISTS idx_cto_created ON compressed_tool_outputs(created_at);
```

#### Store als SQLiteMemory-Methode

```go
// internal/memory/compressed_output_store.go

func (s *SQLiteMemory) StoreCompressedOutput(ctx context.Context, out *CompressedToolOutput) error {
    // Vor dem Speichern: Secrets scrubben
    out.OriginalContent = security.ScrubSensitive(out.OriginalContent)
    // ... INSERT OR REPLACE ...
}

func (s *SQLiteMemory) RetrieveCompressedOutput(ctx context.Context, sessionID, toolCallID string) (*CompressedToolOutput, error)
func (s *SQLiteMemory) CleanupCompressedOutputs(ctx context.Context, maxAge time.Duration) (int64, error)
```

#### Integration in Tool-Output-Flow

```go
// internal/agent/tool_execution_policy.go

func finalizeToolExecution(
    ctx context.Context,
    tc ToolCall,
    rawContent string,        // ← das ist das Original
    guardianBlocked bool,
    // ...
) {
    originalContent := rawContent  // Backup vor Kompression
    
    if !guardianBlocked && cfg != nil {
        compCfg := outputcompress.Config{ /* ... */ }
        rawContent, compStats = outputcompress.Compress(trackingTC.Action, trackingTC.Command, rawContent, compCfg)
        
        // CCR: Nur im native Path und nur wenn wirklich komprimiert wurde
        if tc.NativeCallID != "" && compStats.Ratio < 0.95 && shortTermMem != nil {
            _ = shortTermMem.StoreCompressedOutput(ctx, &memory.CompressedToolOutput{
                SessionID:         sessionID,
                ToolCallID:        tc.NativeCallID,
                ToolName:          trackingTC.Action,
                OriginalContent:   originalContent,
                CompressedContent: rawContent,
                CompressionRatio:  compStats.Ratio,
                FilterUsed:        compStats.FilterUsed,
            })
        }
    }
    
    policyResult := applyToolOutputPolicy(rawContent, limit, scope)
    // ...
}
```

> **Wichtig:** Nur speichern, wenn `compStats.Ratio < 0.95` (mindestens 5% Kompression). Sonst lohnt sich der Speicher-Overhead nicht.

#### Abruf-Tool (mit Limit)

```go
// internal/tools/retrieve_original.go

const maxRetrievableOriginalChars = 32000

func RetrieveOriginalOutput(ctx context.Context, args map[string]interface{}, stm *memory.SQLiteMemory, sessionID string) (string, error) {
    toolCallID, _ := args["tool_call_id"].(string)
    reason, _ := args["reason"].(string)
    
    out, err := stm.RetrieveCompressedOutput(ctx, sessionID, toolCallID)
    if err != nil {
        return "", fmt.Errorf("no archived original found for tool_call_id=%s", toolCallID)
    }
    
    _ = stm.MarkCompressedOutputAccessed(ctx, out.ID)
    
    content := out.OriginalContent
    truncated := false
    if len(content) > maxRetrievableOriginalChars {
        content = content[:maxRetrievableOriginalChars] + 
            fmt.Sprintf("\n[TRUNCATED: original was %d chars, retrieved first %d]",
                len(out.OriginalContent), maxRetrievableOriginalChars)
        truncated = true
    }
    
    header := fmt.Sprintf("[ORIGINAL OUTPUT for %s — filter=%s ratio=%.2f%s]\n",
        out.ToolName, out.FilterUsed, out.CompressionRatio,
        map[bool]string{true: " retrieved_partially", false: ""}[truncated])
    
    if reason != "" {
        slog.Debug("CCR retrieval", "tool_call_id", toolCallID, "reason", reason, "filter", out.FilterUsed)
    }
    
    return header + content, nil
}
```

#### Tool-Registrierung

Das Tool wird **bedingt** registriert — nur wenn in der aktuellen Session mindestens ein komprimierter Output existiert:

```go
// In agent_loop_tools.go oder loop-Initialisierung
if cfg.Agent.OutputCompression.Reversible.Enabled {
    hasCompressed, _ := shortTermMem.HasCompressedOutputsForSession(sessionID)
    if hasCompressed {
        req.Tools = append(req.Tools, retrieveOriginalToolDefinition)
    }
}
```

### 2.3 Konfiguration (minimal)

```go
// config_types.go
Reversible struct {
    Enabled     bool `yaml:"enabled"`
    MaxAgeHours int  `yaml:"max_age_hours"` // Default: 24
} `yaml:"reversible"`
```

Kein `MaxStoreMB` — zeitbasiertes Cleanup reicht. Kein komplexes Speicherlimit.

### 2.4 Cleanup

```go
// In agent_loop.go oder als Hintergrund-Goroutine beim Start
go func() {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()
    for range ticker.C {
        if shortTermMem != nil {
            deleted, _ := shortTermMem.CleanupCompressedOutputs(ctx, 24*time.Hour)
            if deleted > 0 {
                logger.Debug("CCR cleanup", "deleted_rows", deleted)
            }
        }
    }
}()
```

---

## Paket 3: Message Importance Scoring

### 3.1 Ziel
Nachrichten nicht mehr rein chronologisch trimmen, sondern nach Wichtigkeit. **Zuerst nur loggen** (`log_only`), dann nach Validierung aktivieren.

### 3.2 Design

#### Scoring (bug-korrigiert)

```go
// internal/agent/message_scoring.go

type MessageImportance int

const (
    ImportanceCritical MessageImportance = 4
    ImportanceHigh     MessageImportance = 3
    ImportanceMedium   MessageImportance = 2
    ImportanceLow      MessageImportance = 1
    ImportanceFiller   MessageImportance = 0
)

func ScoreMessage(msg openai.ChatCompletionMessage, prevMsg *openai.ChatCompletionMessage) (MessageImportance, string) {
    content := messageText(msg)
    lower := strings.ToLower(content)
    
    switch msg.Role {
    case openai.ChatMessageRoleSystem:
        return ImportanceCritical, "system"
        
    case openai.ChatMessageRoleUser:
        if len(content) < 20 && (strings.Contains(lower, "yes") || strings.Contains(lower, "ok")) {
            return ImportanceLow, "short_ack"
        }
        return ImportanceHigh, "user_intent"
        
    case openai.ChatMessageRoleAssistant:
        if len(msg.ToolCalls) > 0 {
            return ImportanceMedium, "tool_calls"
        }
        if len(content) < 50 {
            return ImportanceLow, "short_ack"
        }
        // Heuristik: Enthält Plan/Entscheidung?
        if containsPlanningMarker(lower) {
            return ImportanceHigh, "plan"
        }
        return ImportanceMedium, "response"
        
    case openai.ChatMessageRoleTool:
        if isToolError(content) {
            return ImportanceHigh, "tool_error"
        }
        // Utility-Erkennung über vorherige ToolCalls
        if prevMsg != nil && prevMsg.Role == openai.ChatMessageRoleAssistant {
            for _, tc := range prevMsg.ToolCalls {
                if isUtilityTool(tc.Function.Name) && len(content) < 500 {
                    return ImportanceLow, "utility_output"
                }
            }
        }
        return ImportanceMedium, "tool_result"
    }
    
    return ImportanceMedium, "default"
}

func containsPlanningMarker(s string) bool {
    markers := []string{"plan:", "i will", "decision:", "let me", "approach:", "strategy:", "next steps"}
    for _, m := range markers {
        if strings.Contains(s, m) {
            return true
        }
    }
    return false
}

func isUtilityTool(name string) bool {
    switch name {
    case "execute_shell", "ssh_exec", "execute_sudo":
        return true
    }
    return false
}
```

#### Trimming-Algorithmus (präzisiert)

```go
// Pseudocode — ersetzt den ContextGuard in agent_loop.go

func trimByImportance(messages []openai.ChatCompletionMessage, maxHistoryTokens int, model string) ([]openai.ChatCompletionMessage, bool) {
    if len(messages) <= 6 {
        return messages, false
    }
    
    // 1. Scores berechnen (System + letzte 4 werden nie getrimmt)
    scored := make([]scoredItem, 0, len(messages)-5)
    for i := 1; i < len(messages)-4; i++ {
        score, reason := ScoreMessage(messages[i], getPrevMessage(messages, i))
        scored = append(scored, scoredItem{
            idx:    i,
            score:  score,
            reason: reason,
            tokens: estimateTokens(messages[i], model), // aus compRes oder Cache
        })
    }
    
    // 2. Sortiere: niedrigster Score zuerst, bei Gleichstand älteste zuerst
    sort.SliceStable(scored, func(a, b int) bool {
        if scored[a].score != scored[b].score {
            return scored[a].score < scored[b].score
        }
        return scored[a].idx < scored[b].idx
    })
    
    // 3. Entferne nacheinander, bis unter Budget
    currentTokens := totalTokens(messages, model)
    removed := make(map[int]bool)
    
    for _, item := range scored {
        if currentTokens <= maxHistoryTokens {
            break
        }
        if item.score >= ImportanceCritical {
            break // Nichts mehr entfernbar
        }
        
        // Tool-Call-Gruppen-Prüfung
        if isPartOfToolCallGroup(messages, item.idx) {
            groupIndices := getToolCallGroupIndices(messages, item.idx)
            groupScore := maxScoreInGroup(scored, groupIndices)
            if groupScore > item.score {
                // Gruppe hat höheren Score → überspringen
                continue
            }
            // Entferne ganze Gruppe
            for _, gi := range groupIndices {
                if !removed[gi] {
                    removed[gi] = true
                    currentTokens -= estimateTokens(messages[gi], model)
                }
            }
        } else {
            removed[item.idx] = true
            currentTokens -= item.tokens
        }
    }
    
    // 4. Rekonstruieren
    result := make([]openai.ChatCompletionMessage, 0, len(messages)-len(removed))
    for i, m := range messages {
        if !removed[i] {
            result = append(result, m)
        }
    }
    
    // 5. Recap für entfernte Messages generieren
    dropped := extractDroppedMessages(messages, removed)
    if len(dropped) > 0 {
        recap := buildTrimmedContextRecap(dropped, maxHistoryTokens-currentTokens)
        result = injectRecap(result, recap)
    }
    
    return result, len(removed) > 0
}
```

### 3.3 Beta-Modus: `log_only`

```go
// config_types.go
ImportanceScoring struct {
    Enabled bool   `yaml:"enabled"`
    Mode    string `yaml:"mode"` // "log_only" | "active"
} `yaml:"importance_scoring"`
```

Im `log_only`-Modus:
- Scores werden berechnet
- Es wird geloggt: `"would_drop: idx=3 score=1 reason=utility_output tokens=45"`
- Das tatsächliche Trimming bleibt chronologisch
- Nach 1 Woche oder 50 Sessions: Review der Logs, dann auf `active` schalten

### 3.4 Konfiguration (minimal)

```go
ImportanceScoring struct {
    Enabled bool   `yaml:"enabled"`
    Mode    string `yaml:"mode"` // Default: "log_only"
} `yaml:"importance_scoring"`
```

---

## Paket 4: Turn-basiertes Lernen (headroom learn light)

### 4.1 Ziel
Nach wiederholten Fehlern oder erfolgreicher Recovery wird eine **konkrete Handlungsregel** generiert und in SQLite persistiert. Kein AGENTS.md-Schreiben, kein Session-Ende-Event.

### 4.2 Trigger-Strategie (3 Trigger)

#### Trigger A: Recovery-Erfolg (Turn-basiert)
Wenn ein Tool fehlschlägt, aber im **selben Turn** mit angepassten Parametern erfolgreich ist:

```go
// In tool_execution_policy.go, nach erfolgreichem Recovery:
if recoveryState.justRecovered() && shortTermMem != nil {
    _ = shortTermMem.RecordResolution(trackingTC.Action, lastError, "Succeeded with adjusted parameters")
    
    // Nur wenn gleicher Fehler in dieser Session schon mal aufgetreten
    count, _ := shortTermMem.GetErrorCountInSession(sessionID, trackingTC.Action, lastError)
    if count >= 2 && cfg.Agent.AutoLearning.Enabled {
        go generateLearnedRule(ctx, shortTermMem, trackingTC.Action, lastError, "Succeeded with adjusted parameters")
    }
}
```

#### Trigger B: Konsekutive Fehler (Circuit-Breaker-Nähe)
Wenn `ConsecutiveErrorCount >= 3` für denselben Tool/Error:

```go
// In recovery_state.go oder agent_loop.go
if recoveryState.ConsecutiveErrorCount >= 3 && cfg.Agent.AutoLearning.Enabled {
    go generateLearnedRule(ctx, shortTermMem, lastTool, lastError, "")
}
```

#### Trigger C: On-Reset
Wenn der User `/reset` ausführt:

```go
// In commands/reset.go oder handlers.go
if cfg.Agent.AutoLearning.Enabled && cfg.Agent.AutoLearning.Mode == "active" {
    go runSessionRetro(ctx, shortTermMem, sessionID)
}
```

> **Wichtig:** Trigger A und B laufen **asynchron** (Goroutine mit 10s Timeout). Sie blockieren den Nutzer nicht.

### 4.3 Regel-Generierung (robust, klein)

```go
// internal/agent/learned_rules.go

func generateLearnedRule(ctx context.Context, stm *memory.SQLiteMemory, toolName, errorPattern, resolution string) {
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    
    // Kein LLM-Call nötig, wenn die Regel trivial ist
    if resolution != "" {
        rule := inferRuleFromResolution(toolName, errorPattern, resolution)
        if rule != nil {
            _ = stm.UpsertLearnedRule(rule)
            return
        }
    }
    
    // Nur für komplexe Fälle: Mini-LLM-Call
    prompt := fmt.Sprintf(`Given this recurring error, write ONE concise rule (< 30 words).
Tool: %s
Error: %s
Fix: %s
Rule:`, toolName, errorPattern, resolution)
    
    // LLM call mit max 100 Tokens, temperature 0.1
    // Antwort wird als reiner Text gespeichert — kein striktes Parsing
}
```

### 4.4 Regel-Format (einfach, kompakt)

```go
type LearnedRule struct {
    ID        int64
    ToolName  string
    Pattern   string   // normalisierter Fehler
    Rule      string   // z.B. "docker port conflict: run 'docker ps' first"
    Confidence float64 // 0.5 bei erstmaliger Erstellung
    Hits      int      // wie oft die Regel seitdem zutraf
    Misses    int      // wie oft sie irrelevant war
    Active    bool
    CreatedAt time.Time
}
```

### 4.5 Injektion (Top-5, adaptiv)

```go
// In prompts/builder.go
if flags.LearnedRulesContext != "" && tier != "minimal" {
    finalPrompt.WriteString("# LEARNED RULES\n")
    finalPrompt.WriteString("Apply proactively if relevant to the current task.\n")
    finalPrompt.WriteString(security.IsolateExternalData(flags.LearnedRulesContext))
    finalPrompt.WriteString("\n\n")
}
```

Nur die **Top-5** Regeln werden injiziert, gewichtet nach:
1. Relevanz zum aktuellen Tool-Set (Adaptive Tool Filtering)
2. Confidence
3. Recency

Max. 200 Tokens für den gesamten `# LEARNED RULES`-Block.

### 4.6 Konfiguration (minimal)

```go
// config_types.go
AutoLearning struct {
    Enabled bool   `yaml:"enabled"`
    Mode    string `yaml:"mode"` // "log_only" | "active"
} `yaml:"auto_learning"`
```

Im `log_only`-Modus:
- Regeln werden generiert und in `learned_rules` mit `active = false` gespeichert
- Es wird geloggt: `"would_inject: rule=X confidence=0.8"`
- Keine Injektion in den System Prompt

### 4.7 DB-Schema

```sql
-- In error_learning.go oder short_term_init.go
CREATE TABLE IF NOT EXISTS learned_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tool_name TEXT NOT NULL,
    pattern TEXT NOT NULL,
    rule TEXT NOT NULL,
    confidence REAL DEFAULT 0.5,
    hits INTEGER DEFAULT 0,
    misses INTEGER DEFAULT 0,
    active BOOLEAN DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_learned_tool ON learned_rules(tool_name);
CREATE INDEX IF NOT EXISTS idx_learned_active ON learned_rules(active);
```

Migration für `error_patterns` (Session-ID):
```go
// In InitErrorLearningTable() oder applySQLiteMemoryMigrations()
migrateAddColumn(db, logger, "error_patterns", "session_id", "TEXT DEFAULT ''")
```

---

## Gesamt-Zeitplan (realistisch)

| Woche | Paket(e) | Deliverable | Modus |
|-------|----------|-------------|-------|
| **Woche 1** | Paket 1 (SmartCrusher) | JSON-Compressor aktiv, Tests, Benchmarks | `active` |
| **Woche 2** | Paket 2 (CCR Store) | Archivierung läuft, Cleanup aktiv | `active` |
| **Woche 3** | Paket 3 (Importance log_only) | Scores werden berechnet und geloggt | `log_only` |
| **Woche 4** | Paket 2 (CCR Tool) + Paket 3 Review | `retrieve_original_output` Tool; Review der Importance-Logs | `active` / `log_only` |
| **Woche 5** | Paket 3 (Importance active) | Score-basiertes Trimming aktiv nach Validierung | `active` |
| **Woche 6** | Paket 4 (AutoLearning log_only) | Regeln werden generiert, nicht injiziert | `log_only` |
| **Woche 7** | Paket 4 Review + ggf. active | Review gelernte Regeln, dann Injektion aktivieren | `active` |
| **Woche 8** | Integration & Tests | End-to-End, Regressionstests, Dokumentation | — |

**Gesamtaufwand:** ~8 Wochen (statt 5 in V1), davon 3 Wochen Beta-/Review-Phase.

---

## Rückwärtskompatibilität & Rollout

### Datenbank
- Alle neuen Tabellen: `CREATE TABLE IF NOT EXISTS`
- Neue Spalten: `migrateAddColumn` mit `pragma_table_info`-Prüfung
- Bestehende Daten bleiben unberührt

### Config
- Neue Optionen haben sinnvolle Defaults
- `config.yaml` muss nicht manuell angepasst werden

### Rollback-Strategie
- Jedes Paket hat `enabled: false` als Not-Aus
- Beta-Modus `log_only` erlaubt Deaktivierung ohne Verhaltensänderung
- CCR-Store: Einfach `CleanupCompressedOutputs(ctx, 0)` → alles leer

---

## Erfolgsmetriken (messbar)

| Metrik | Ziel | Messung | Zeitraum |
|--------|------|---------|----------|
| SmartCrusher Kompressionsrate | -10% Token-Count bei JSON-Outputs | Vorher/Nachher für gleiche Tool-Calls | Woche 1 |
| CCR Archivierung | > 80% der komprimierten Outputs archiviert | `SELECT COUNT(*) FROM compressed_tool_outputs` | Woche 2 |
| Importance Score-Qualität | < 5% "falsch getrimmt" (manueller Review) | Log-Analyse: wichtige Messages mit Score < 2 | Woche 3–4 |
| Context-Trimming-Qualität | Weniger "vergessene" Fakten im Review | Manueller Review von 20 Sessions | Woche 5 |
| Gelernte Regeln | > 50% nützlich (Hit-Rate) | `hits / (hits + misses)` | Woche 6–7 |
| Token-Verbrauch pro Session | -8% | `budget.Tracker`-Daten | Woche 8 |
