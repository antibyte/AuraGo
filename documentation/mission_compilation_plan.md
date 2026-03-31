# Mission Compilation Plan (Korrigiert)

## Zusammenfassung

Dieses Dokument beschreibt ein System um Missions vor der Ausführung durch ein LLM "vorzuverarbeiten". 

**WICHTIGE KORREKTUR:** Das System ist KEIN starres "Kompilieren" sondern eine **intelligente Vorbereitung**, die den Agenten unterstützt aber nicht ersetzt. Der Agent behält volle Flexibilität zur Laufzeit.

---

## Identifizierte Denkfehler im ursprünglichen Plan

### ❌ Denkfehler 1: "Alles ist vorab auflösbar"
**Problem:** Angenommen, alle Informationen können vor der Laufzeit gesammelt werden.

**Realität:**
- Dateipfade sind bekannt, aber Inhalte ändern sich
- API-Antworten sind zur Laufzeit erst verfügbar
- Bedingungen (if/then) können nicht alle vorhergesehen werden
- Fehler sind unvorhersehbar

**Korrektur:**
- Unterscheidung zwischen **statischem Kontext** (bekannt vorab) und **dynamischem Kontext** (nur zur Laufzeit)
- Der Agent muss weiterhin Tools zur Informationsgewinnung nutzen können

### ❌ Denkfehler 2: "Starrer Workflow"
**Problem:** Ein festgelegter Schritt-für-Schritt Workflow nimmt dem Agenten die Entscheidungsfreiheit.

**Realität:**
- Wenn Schritt 3 fehlschlägt, braucht der Agent Alternativen
- Manche Schritte können übersprungen werden wenn Bedingungen nicht zutreffen
- Neue Informationen können den Workflow ändern müssen

**Korrektur:**
- Workflow wird als **Leitfaden** nicht als strikte Anweisung behandelt
- Agent behält Autonomie über Ausführung
- Fehlerbehandlung bleibt generisch, nicht auf spezifische Fälle fixiert

### ❌ Denkfehler 3: "Alle Tool Manuals einbetten"
**Problem:** Annahme, dass alle potenziell benötigten Tool Manuals in das Compiled Prompt gehören.

**Realität:**
- Zu viele Tokens, teuer und langsam
- Manche Tools werden gar nicht gebraucht
- Agent kann bei Bedarf selbst Manuals nachschlagen (RAG)

**Korrektur:**
- Nur **high-confidence Tools** einbetten (sicher benötigt)
- **Tool Kategorien** erwähnen für den Fall der Fälle
- Nicht erwähnte Tools bleiben über RAG verfügbar

### ❌ Denkfehler 4: "Compilation ersetzt Agent-Denken"
**Problem:** Der Agent soll ein vorgekautes Paket ausführen ohne nachzudenken.

**Realität:**
- LLMs sind nicht deterministisch
- Edge Cases werden übersehen
- Kontext ändert sich während der Ausführung

**Korrektur:**
- Compiled Prompt ist **Vorbereitung** nicht **Ersatz**
- Agent muss weiterhin:
  - Situation analysieren
  - Entscheidungen treffen
  - Auf Fehler reagieren
  - Unvorhergesehene Tools nutzen

---

## Korrigiertes Konzept: "Mission Preparation"

### Neue Philosophie

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    MISSION PREPARATION PIPELINE                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ZIEL: Den Agenten EFFIZIENTER machen, nicht ENTBEHRLICH                    │
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐                  │
│  │   Mission    │    │  Cheat       │    │  Cheat       │                  │
│  │   Prompt     │ +  │  Sheet 1     │ +  │  Sheet 2...  │                  │
│  └──────────────┘    └──────────────┘    └──────────────┘                  │
│         │                   │                   │                           │
│         └───────────────────┴───────────────────┘                           │
│                             │                                               │
│                             ▼                                               │
│              ┌──────────────────────────────────────┐                       │
│              │   PREPARATION LLM CALL               │                       │
│              │                                      │                       │
│              │  Analyse:                            │                       │
│              │  - Welche TOOLS sind ESSENTIELL?     │                       │
│              │  - Was ist der BEWÄHRTE WORKFLOW?    │                       │
│              │  - Welche FALLSTRICKE sind bekannt?  │                       │
│              │  - Was sollte VORAB geladen werden?  │                       │
│              └──────────────────────────────────────┘                       │
│                             │                                               │
│                             ▼                                               │
│              ┌──────────────────────────────────────┐                       │
│              │   PREPARED MISSION CONTEXT           │                       │
│              │                                      │                       │
│              │  ✓ Sicher benötigte Tool-Manuals     │                       │
│              │  ✓ Bewährte Herangehensweise         │                       │
│              │  ✓ Bekannte Fallstricke              │                       │
│              │  ✓ Nützlicher Vorab-Kontext          │                       │
│              │  ✓ Erfahrungsbasierte Tips           │                       │
│              └──────────────────────────────────────┘                       │
│                             │                                               │
│                             ▼                                               │
│                   ┌──────────────────────┐                                  │
│                   │  Agent Execution     │                                  │
│                   │                      │                                  │
│                   │  + Volle Autonomie   │                                  │
│                   │  + Kann abweichen    │                                  │
│                   │  + Kann improvisieren│                                  │
│                   │  + Nutzt RAG bei Bedarf                              │
│                   └──────────────────────┘                                  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Unterscheidung: Statisch vs Dynamisch

| Aspekt | Statisch (kann vorbereitet werden) | Dynamisch (nur zur Laufzeit) |
|--------|-----------------------------------|------------------------------|
| **Tools** | Welche Tools *wahrscheinlich* benötigt werden | Ob ein Tool in spezifischem Fall funktioniert |
| **Workflow** | Bewährte Herangehensweise, Tipps | Konkrete Ausführungsentscheidungen |
| **Kontext** | Allgemeine Informationen, Konfiguration | Aktuelle Dateiinhalte, API-Antworten |
| **Fehler** | Bekannte Fallstricke | Unvorhergesehene Fehlersituationen |
| **Entscheidungen** | Decision Trees, Optionen | Welcher Pfad gewählt wird |

---

## Korrigierte Architektur

### 1. Datenstrukturen (korrigiert)

#### PreparedMission (ehemals CompiledMission)

```go
// PreparedMission represents pre-analyzed mission context
// This is ADVISORY - the agent maintains full autonomy
type PreparedMission struct {
    OriginalID        string    `json:"original_id"`
    Version           int       `json:"version"`
    PreparedAt        time.Time `json:"prepared_at"`
    
    // Task understanding (context, not instruction)
    TaskSummary       string    `json:"task_summary"`        // Restated for clarity
    SuccessCriteria   []string  `json:"success_criteria"`    // What success looks like
    
    // Tool guidance (likely needed tools, not exclusive list)
    EssentialTools    []ToolGuide `json:"essential_tools"`   // High-confidence tools
    RelatedToolCategories []string `json:"related_tool_categories"` // May also need
    
    // Workflow guidance (suggested approach, not rigid plan)
    SuggestedApproach string    `json:"suggested_approach"`  // General strategy
    CommonSteps       []StepGuide `json:"common_steps"`      // Typical steps (optional)
    DecisionPoints    []DecisionPoint `json:"decision_points"` // Known branches
    
    // Pitfalls and tips (experience from cheat sheets)
    KnownPitfalls     []Pitfall `json:"known_pitfalls"`      // Things to watch for
    ProTips           []string  `json:"pro_tips"`            // Best practices
    
    // Pre-load suggestions (what to load if possible)
    SuggestedPreloads []PreloadSuggestion `json:"suggested_preloads"`
    
    // Metadata
    Confidence        float64   `json:"confidence"`          // How sure we are (0-1)
    SourceChecksum    string    `json:"source_checksum"`
}

type ToolGuide struct {
    ToolName        string   `json:"tool_name"`
    LikelyActions   []string `json:"likely_actions"`      // Common actions for this mission
    WhyRelevant     string   `json:"why_relevant"`        // Why this tool is suggested
    KeyParameters   []string `json:"key_parameters"`      // Important params to remember
    ManualSummary   string   `json:"manual_summary"`      // Condensed manual (not full!)
}

type StepGuide struct {
    StepNumber      int      `json:"step_number"`
    Description     string   `json:"description"`
    TypicalTool     string   `json:"typical_tool,omitempty"`
    IsOptional      bool     `json:"is_optional"`         // Can be skipped
    Condition       string   `json:"condition,omitempty"` // When to execute
    Notes           string   `json:"notes"`
}

type DecisionPoint struct {
    Description     string   `json:"description"`
    Options         []string `json:"options"`
    Considerations  string   `json:"considerations"`
}

type Pitfall struct {
    Issue           string   `json:"issue"`
    Prevention      string   `json:"prevention"`
    Recovery        string   `json:"recovery"`            // How to recover if it happens
}

type PreloadSuggestion struct {
    Type            string   `json:"type"`                // "file", "config", "query"
    Target          string   `json:"target"`              // Path/query
    Purpose         string   `json:"purpose"`             // Why load this
    Optional        bool     `json:"optional"`            // Can skip if fails
}
```

#### Erweiterung MissionV2 (korrigiert)

```go
type MissionV2 struct {
    // ... existing fields ...
    
    // Preparation (not compilation!)
    PreparedContext   *PreparedMission  `json:"prepared_context,omitempty"`
    PreparationStatus string            `json:"preparation_status,omitempty"` // none | prepared | stale | error
    LastPreparedAt    *time.Time        `json:"last_prepared_at,omitempty"`
    AutoPrepare       bool              `json:"auto_prepare,omitempty"`       // Auto-prepare on changes
    
    // IMPORTANT: Fallback always available
    RawPromptFallback bool              `json:"raw_prompt_fallback,omitempty"` // Use unprepared if prep fails
}
```

### 2. Preparation Service (korrigiert)

#### MissionPreparationService

```go
// MissionPreparationService creates advisory context for missions
type MissionPreparationService struct {
    db            *sql.DB
    llmClient     llm.Client
    toolRegistry  ToolRegistry
    config        PreparationConfig
}

type PreparationConfig struct {
    MaxEssentialTools     int           // Max tools to include (default: 3-5)
    IncludeManualSummary  bool          // Include condensed manual, not full
    MaxPreloadSuggestions int           // Don't overwhelm with preloads
    MinConfidence         float64       // Minimum confidence to accept preparation
    CacheDuration         time.Duration
}

// Prepare analyzes a mission and creates advisory context
func (s *MissionPreparationService) Prepare(
    ctx context.Context,
    mission *MissionV2,
) (*PreparedMission, error) {
    
    // 1. Gather source materials
    materials := s.gatherMaterials(mission)
    checksum := computeChecksum(materials)
    
    // 2. Check cache
    if mission.PreparedContext != nil && 
       mission.SourceChecksum == checksum &&
       time.Since(*mission.LastPreparedAt) < s.config.CacheDuration {
        return mission.PreparedContext, nil
    }
    
    // 3. LLM Analysis - with emphasis on uncertainty
    analysis, err := s.analyzeWithLLM(ctx, materials)
    if err != nil {
        return nil, fmt.Errorf("preparation failed: %w", err)
    }
    
    // 4. Validate confidence
    if analysis.Confidence < s.config.MinConfidence {
        // Still create preparation but mark as low-confidence
        analysis.Confidence = analysis.Confidence
    }
    
    // 5. Build preparation
    prepared := &PreparedMission{
        OriginalID:      mission.ID,
        Version:         1,
        PreparedAt:      time.Now(),
        TaskSummary:     analysis.TaskSummary,
        SuccessCriteria: analysis.SuccessCriteria,
        Confidence:      analysis.Confidence,
        SourceChecksum:  checksum,
    }
    
    // 6. Select essential tools (limited!)
    prepared.EssentialTools = s.selectEssentialTools(
        analysis.SuggestedTools, 
        s.config.MaxEssentialTools,
    )
    
    // 7. Add workflow guidance (flexible)
    prepared.SuggestedApproach = analysis.SuggestedApproach
    prepared.CommonSteps = s.makeStepsOptional(analysis.Steps)
    prepared.DecisionPoints = analysis.DecisionPoints
    
    // 8. Add pitfalls and tips
    prepared.KnownPitfalls = analysis.Pitfalls
    prepared.ProTips = analysis.Tips
    
    // 9. Suggest preloads (limited, optional)
    prepared.SuggestedPreloads = s.limitPreloads(
        analysis.Preloads,
        s.config.MaxPreloadSuggestions,
    )
    
    return prepared, nil
}

// selectEssentialTools limits tools to high-confidence ones only
func (s *MissionPreparationService) selectEssentialTools(
    candidates []ToolAnalysis, 
    max int,
) []ToolGuide {
    // Sort by confidence
    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].Confidence > candidates[j].Confidence
    })
    
    // Take only high-confidence ones
    var essential []ToolGuide
    for _, c := range candidates {
        if len(essential) >= max {
            break
        }
        if c.Confidence < 0.7 { // Only high confidence
            continue
        }
        essential = append(essential, ToolGuide{
            ToolName:      c.Name,
            LikelyActions: c.Actions,
            WhyRelevant:   c.Reasoning,
            KeyParameters: c.KeyParams,
            ManualSummary: s.summarizeManual(c.Name), // CONDENSED!
        })
    }
    
    return essential
}
```

### 3. Preparation Prompt (korrigiert)

```markdown
# Mission Preparation Task

Analyze the following mission and cheat sheets to create HELPFUL CONTEXT 
for an AI agent. This is ADVISORY - the agent will make final decisions.

## Input Materials

### Mission
```
{{MISSION_PROMPT}}
```

### Cheat Sheets
{{CHEATSHEETS}}

## Your Task

Create a PREPARATION GUIDE that will help an agent execute this mission efficiently.

### IMPORTANT PRINCIPLES:

1. **BE HUMBLE**: You cannot predict everything. Mark uncertainty clearly.
2. **BE CONCISE**: Don't include full manuals - summarize key points.
3. **BE FLEXIBLE**: Suggest, don't command. The agent decides.
4. **ACKNOWLEDGE LIMITS**: You don't know runtime conditions.

### Output Structure

```json
{
  "task_summary": "Clear restatement of what needs to be done",
  "success_criteria": ["List of what success looks like"],
  "confidence": 0.85,
  
  "suggested_tools": [
    {
      "name": "tool_name",
      "confidence": 0.95,
      "reasoning": "Why this tool is likely needed",
      "likely_actions": ["action1", "action2"],
      "key_parameters": ["param1", "param2"],
      "manual_summary": "2-3 sentences on how to use"
    }
  ],
  
  "suggested_approach": "General strategy paragraph",
  
  "common_steps": [
    {
      "step": 1,
      "description": "What typically happens here",
      "typical_tool": "tool_name",
      "is_optional": false,
      "condition": "When to do this"
    }
  ],
  
  "decision_points": [
    {
      "description": "Situation requiring decision",
      "options": ["Option A", "Option B"],
      "considerations": "What to consider"
    }
  ],
  
  "known_pitfalls": [
    {
      "issue": "What can go wrong",
      "prevention": "How to avoid",
      "recovery": "How to fix if it happens"
    }
  ],
  
  "pro_tips": [
    "Best practice 1",
    "Best practice 2"
  ],
  
  "preload_suggestions": [
    {
      "type": "file|config|query",
      "target": "path or query",
      "purpose": "Why this helps",
      "optional": true
    }
  ]
}
```

### Confidence Guidelines

- **0.9-1.0**: Very certain (e.g., "docker backup needs docker tool")
- **0.7-0.9**: Likely (e.g., "probably needs file operations")
- **0.5-0.7**: Uncertain (e.g., "might need API calls")
- **<0.5**: Don't include

### Rules

1. **Max 5 tools** in suggested_tools - only high confidence
2. **Steps are suggestions** - mark is_optional liberally
3. **Preloads are optional** - agent can ignore if they fail
4. **Acknowledge unknowns** - "if X then Y, but verify first"
5. **Keep manual summaries short** - 2-3 sentences max
```

### 4. Final Prompt Format (korrigiert)

```markdown
# MISSION

{{MISSION_PROMPT}}

---

## 📚 PREPARATION CONTEXT (Advisory)

> The following context was prepared to help you work more efficiently.
> You MAY use it, modify it, or ignore it as the situation requires.
> You have full autonomy to make decisions.

### 🎯 Task Summary
{{prepared.task_summary}}

### ✅ Success Criteria
{{prepared.success_criteria}}

### 🛠️ Likely Needed Tools
{{#each prepared.essential_tools}}
- **{{name}}**: {{manual_summary}}
  - Typical actions: {{likely_actions}}
  - Key params: {{key_parameters}}
{{/each}}

> Other tools may be needed. Use RAG or query_memory if unsure.

### 🧭 Suggested Approach
{{prepared.suggested_approach}}

### 📝 Typical Steps (Optional Guidelines)
{{#each prepared.common_steps}}
{{step}}. {{description}} {{#if is_optional}}(Optional){{/if}}
   {{#if condition}}- When: {{condition}}{{/if}}
   {{#if typical_tool}}- Tool: {{typical_tool}}{{/if}}
{{/each}}

### ⚠️ Known Pitfalls
{{#each prepared.known_pitfalls}}
- **{{issue}}**
  - Prevention: {{prevention}}
  - Recovery: {{recovery}}
{{/each}}

### 💡 Pro Tips
{{#each prepared.pro_tips}}
- {{this}}
{{/each}}

### 📥 Suggested Pre-Loads (Optional)
{{#each prepared.suggested_preloads}}
- {{type}}: `{{target}}` - {{purpose}} {{#if optional}}(Optional){{/if}}
{{/each}}

---

## ⚡ EXECUTION

Proceed with the mission using your judgment. 
- You can follow the suggested approach or adapt as needed
- You can use tools not listed above
- You can skip suggested steps if inappropriate
- Handle errors as you see fit

**Remember**: The preparation context is a starting point, not a script.
```

### 5. Integration (korrigiert)

```go
// processNext executes the next mission
func (m *MissionManagerV2) processNext() {
    // ... existing code ...
    
    prompt := mission.Prompt
    
    // Try to use prepared context
    if mission.PreparedContext != nil && mission.PreparationStatus == "prepared" {
        // Check if still valid
        if mission.LastPreparedAt != nil && 
           time.Since(*mission.LastPreparedAt) < m.prepConfig.CacheDuration {
            
            // Render prepared context as advisory
            preparedPrompt := m.renderPreparedContext(mission.PreparedContext)
            prompt = preparedPrompt + "\n\n---\n\n## ORIGINAL MISSION\n\n" + mission.Prompt
        } else {
            // Mark as stale, trigger background refresh
            mission.PreparationStatus = "stale"
            go m.refreshPreparation(mission)
        }
    }
    
    // Fallback: Simple cheat sheet attachment
    if mission.PreparedContext == nil && len(mission.CheatsheetIDs) > 0 {
        if extra := CheatsheetGetMultiple(mission.CheatsheetIDs); extra != "" {
            prompt = mission.Prompt + "\n\n## REFERENCE MATERIALS\n" + extra
        }
    }
    
    // ... execute ...
}

// renderPreparedContext creates the advisory prompt
func (m *MissionManagerV2) renderPreparedContext(prep *PreparedMission) string {
    var sb strings.Builder
    
    sb.WriteString("# MISSION\n\n")
    // ... template rendering ...
    
    return sb.String()
}
```

---

## Fehlerbehandlung & Fallbacks

### Szenarien und Lösungen

| Szenario | Lösung |
|----------|--------|
| Preparation schlägt fehl | Fallback zu rohem Prompt + Cheat Sheets |
| Niedrige Confidence (<0.5) | Markieren als "low-confidence guidance" |
| Vorbereitete Schritte funktionieren nicht | Agent improvisiert mit vollem Tool-Zugriff |
| Suggested Tool nicht verfügbar | Agent wählt Alternative |
| Preload schlägt fehl | Ignorieren, zur Laufzeit laden |
| Mission ändert sich während Ausführung | Agent adaptiert normal |

### Fallback Chain

```
1. Prepared Mission (high confidence) 
   → 2. Prepared Mission (low confidence, marked)
      → 3. Raw Prompt + Cheat Sheets attached
         → 4. Raw Prompt only
```

---

## Beispiel: Backup Mission

### Vorher (ohne Preparation)

```
Mission: "Führe wöchentliches Backup durch"

Agent muss:
1. Aus Cheat Sheet verstehen was zu tun ist
2. Sich merken welche Tools nötig sind
3. Bei Bedarf Manuals nachlesen
4. Workflow planen
5. Ausführen

Probleme:
- Jede Ausführung = gleiche Analyse
- Cheat Sheet Inhalte können übersehen werden
- Keine Erfahrung aus vorherigen Runs
```

### Nachher (mit Preparation)

```markdown
# MISSION
Führe wöchentliches Backup durch

---

## 📚 PREPARATION CONTEXT (Advisory)

### 🎯 Task Summary
Erstelle Backups aller Docker Volumes und lade sie zu S3 hoch. 
Behalte nur die letzten 4 Backups.

### ✅ Success Criteria
- Alle Volumes als .tar.gz gesichert
- Upload zu S3 erfolgreich
- Alte Backups (>4) gelöscht

### 🛠️ Likely Needed Tools
- **docker**: Für Volume-Zugriff und Backup-Erstellung
  - docker volume ls
  - docker run mit volume mounts
- **s3_storage**: Für Upload und Lifecycle Management
  - upload, list, delete actions

> You may need filesystem tools if local temp storage is used.

### 🧭 Suggested Approach
1. Liste alle Volumes
2. Für jedes Volume: erstelle tar.gz in temp Verzeichnis
3. Lade zu S3 hoch
4. Liste S3 Inhalt und lösche alte Backups

### 📝 Typical Steps
1. Liste Volumes (docker)
2. Erstelle Backup-Verzeichnis (optional, filesystem)
3. Für jedes Volume: Backup erstellen (docker)
4. Upload zu S3 (s3_storage)
5. Cleanup alte Backups (s3_storage)

### ⚠️ Known Pitfalls
- **Große Volumes**: Backup kann lange dauern
  - Prevention: Prüfe Volume-Größe vorher
  - Recovery: Warte oder überspringe große Volumes
- **S3 Upload-Limit**: Einzelne Dateien >5GB brauchen Multipart
  - Prevention: Prüfe Dateigröße
  - Recovery: Nutze multipart upload falls verfügbar

### 💡 Pro Tips
- Nutze alpine:latest Image für tar - es ist klein
- Benenne Backups mit Timestamp: volume_YYYYMMDD.tar.gz
- Prüfe S3 Bucket-Größe vor Cleanup

### 📥 Suggested Pre-Loads (Optional)
- config: docker.socket path (usually /var/run/docker.sock)
- query: aktuelle Volumes (wird wahrscheinlich ohnehin gemacht)

---

## ⚡ EXECUTION
Proceed with the mission using your judgment...
```

**Vorteile:**
- Agent weiß sofort welche Tools wahrscheinlich nötig sind
- Bekannte Probleme sind dokumentiert
- Trotzdem volle Flexibilität
- Kann Schritte überspringen oder hinzufügen

---

## Kosten-Nutzen-Analyse (realistisch)

### Wann lohnt sich Preparation?

| Faktor | Lohnt sich | Lohnt nicht |
|--------|-----------|-------------|
| **Mission Häufigkeit** | Mehrfach pro Woche | Einmalig |
| **Komplexität** | >3 Cheat Sheets oder >500 Zeilen | Einfach, ein Cheat Sheet |
| **Kritikalität** | Produktions-Backups | Experimente |
| **Cheat Sheet Qualität** | Gut strukturiert, mit Fallstricken | Unstrukturiert |
| **Agent Erfahrung** | Neuer Agent | Hat Mission schon oft gemacht |

### Token-Kosten

| Phase | Tokens (geschätzt) | Hinweis |
|-------|-------------------|---------|
| Preparation (einmalig) | 2-5k Input, 1-2k Output | Amortisiert sich bei wiederholter Nutzung |
| Execution mit Prep | 1-2k weniger pro Run | Keine Tool-Analyse nötig |
| Break-even | Nach 3-5 Runs | Bei täglicher Mission: lohnt sich sofort |

### Empfehlung

- **Auto-Prepare**: Bei allen Scheduled Missions (täglich/wöchentlich)
- **On-Demand**: Bei manuellen Missions mit >2 Cheat Sheets
- **Kein Prepare**: Bei einmaligen oder sehr einfachen Missions

---

## Implementierungsphasen (korrigiert)

### Phase 1: Grundlegende Preparation (Woche 1)

- [ ] Datenstrukturen erstellen (PreparedMission)
- [ ] Preparation Service Grundgerüst
- [ ] Preparation Prompt Template
- [ ] Fallback zu Raw Prompt

### Phase 2: Integration & Fallbacks (Woche 2)

- [ ] MissionV2 Erweiterung
- [ ] Integration in processNext
- [ ] Fallback Chain implementieren
- [ ] Confidence-Handling

### Phase 3: UI & API (Woche 3)

- [ ] API Endpunkte (/prepare, /prepared)
- [ ] Preparation Status in UI
- [ ] Confidence Anzeige
- [ ] Manual Override (Ignore Preparation)

### Phase 4: Optimierung (Woche 4)

- [ ] Tool Detection verbessern
- [ ] Pitfall Erkennung
- [ ] Performance Tuning
- [ ] Feedback Loop (erfolgreiche Preps lernen)

---

## Appendix

### A. Unterschied: Compilation vs Preparation

| Aspekt | Compilation (falsch) | Preparation (richtig) |
|--------|---------------------|----------------------|
| **Ziel** | Ersetzt Agent-Denken | Unterstützt Agent |
| **Flexibilität** | Starr, deterministisch | Flexibel, adaptiv |
| **Tool-Auswahl** | Fix definiert | Vorgeschlagen |
| **Workflow** | Strikte Schritte | Optionale Guidelines |
| **Fehler** | Vordefinierte Pfade | Agent entscheidet |
| **LLM Role** | Ausführend | Entscheidend |

### B. Datenbank Schema (korrigiert)

```sql
-- Prepared mission context (advisory only)
CREATE TABLE IF NOT EXISTS prepared_missions (
    id                 TEXT PRIMARY KEY,
    mission_id         TEXT NOT NULL UNIQUE,
    version            INTEGER NOT NULL DEFAULT 1,
    prepared_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    valid_until        DATETIME,
    source_checksum    TEXT NOT NULL,
    prepared_data      TEXT NOT NULL,  -- JSON PreparedMission
    confidence         REAL NOT NULL DEFAULT 0.0,
    status             TEXT NOT NULL DEFAULT 'active',  -- active | stale | error
    error_message      TEXT,
    token_cost         INTEGER,
    preparation_time_ms INTEGER,
    
    FOREIGN KEY (mission_id) REFERENCES missions_v2(id) ON DELETE CASCADE
);

CREATE INDEX idx_prepared_mission_id ON prepared_missions(mission_id);
CREATE INDEX idx_prepared_status ON prepared_missions(status);
```

---

*Dokument erstellt: 2026-03-31*  
*Version: 2.0 (korrigiert)*  
*Status: Bereit für Review*
