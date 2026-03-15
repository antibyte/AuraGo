# Personality Engine Optimierung - Umsetzungs-Roadmap

> **Ziel**: Verbesserung von Persönlichkeitserhalt, Selbstreflexion und kontinuierlicher Weiterentwicklung

---

## Phase 1: Persönlichkeitserhalt (Week 1-2)

### 1.1 Weighted Trait Decay
**Datei**: `internal/memory/personality.go`

```go
// NEU: DecayAllTraitsWeighted mit Persönlichkeitsprofil-Berücksichtigung
func (s *SQLiteMemory) DecayAllTraitsWeighted(baseAmount float64, meta PersonalityMeta) error {
    // Implementation:
    // 1. Hole aktuelle Traits
    // 2. Für jeden Trait:
    //    - Wenn > 0.7: decay = baseAmount * 0.5 * meta.TraitDecayRate
    //    - Wenn < 0.3: decay = baseAmount * 1.5 * meta.TraitDecayRate  
    //    - Sonst: decay = baseAmount * meta.TraitDecayRate
    // 3. Respektiere trait_bounds (neue Tabelle)
    // 4. Update in DB
}
```

**Tasks**:
- [ ] Neue Funktion `DecayAllTraitsWeighted` implementieren
- [ ] `trait_bounds` Tabelle in Schema hinzufügen
- [ ] `InitPersonalityTables` erweitern
- [ ] Tests schreiben
- [ ] Maintenance Loop updaten

---

### 1.2 Personality Anchors
**Dateien**: 
- `internal/memory/personality.go` (PersonalityMeta)
- `prompts/personalities/*.md`
- `internal/agent/maintenance.go`

```yaml
# In Personality YAML-Frontmatter erweitern:
meta:
  volatility: 1.2
  empathy_bias: 1.5
  conflict_response: "submissive"
  loneliness_susceptibility: 1.5
  trait_decay_rate: 0.8
  # NEU:
  anchor_traits:
    empathy: 0.3        # Minimum Empathy für "friend"
    curiosity: 0.2      # Minimum Curiosity für "thinker"
  decay_resistance:
    empathy: 0.5        # 50% weniger Decay für Empathy
    affinity: 0.3       # 30% weniger Decay für Affinity
```

**Tasks**:
- [ ] `PersonalityMeta` Struct erweitern
- [ ] `GetCorePersonalityMeta` Parser updaten
- [ ] Core Personalities mit Anchors versehen
- [ ] Decay-Logik um Resistance erweitern

---

### 1.3 Milestone Persistence
**Dateien**:
- `internal/memory/personality_analysis.go`
- `internal/memory/personality.go`
- `internal/agent/personality.go`

```go
// NEU: Milestone-Effekte
var MilestoneEffects = map[string]MilestoneEffect{
    "Deep Explorer": {
        TraitFloor: map[string]float64{
            TraitCuriosity: 0.55,
        },
        DecayResistance: map[string]float64{
            TraitCuriosity: 0.5,
        },
        PermanentModifier: "Always asks follow-up questions",
    },
    "Self-Assured Expert": {
        TraitFloor: map[string]float64{
            TraitConfidence: 0.60,
        },
        // ...
    },
    // ... weitere
}
```

**Tasks**:
- [ ] `MilestoneEffect` Struct definieren
- [ ] Effekt-Mapping erstellen
- [ ] `ApplyMilestoneEffect` Funktion implementieren
- [ ] `processBehavioralEvents` erweitern
- [ ] Trait Floor Enforcement implementieren

---

## Phase 2: Selbstreflexion System (Week 3-5)

### 2.1 Character Journal Parser
**Neue Datei**: `internal/memory/reflection.go`

```go
// CharacterAnalysis hält geparste Journal-Daten
type CharacterAnalysis struct {
    PeriodStart       time.Time
    PeriodEnd         time.Time
    AverageTraits     PersonalityTraits
    MoodDistribution  map[Mood]int
    MilestonesReached []string
    TrendAnalysis     TrendReport
}

type TrendReport struct {
    RisingTraits  []string  // Traits mit upward trend
    FallingTraits []string  // Traits mit downward trend
    StableTraits  []string  // Traits mit < 0.1 variance
}

// ParseCharacterJournal liest und analysiert das Journal
func ParseCharacterJournal(path string, days int) (*CharacterAnalysis, error) {
    // 1. Datei einlesen
    // 2. Markdown parsen
    // 3. Trait-Daten extrahieren
    // 4. Trends berechnen
    // 5. Zusammenfassung erstellen
}
```

**Tasks**:
- [ ] Neue Datei `reflection.go` erstellen
- [ ] Parser für Journal-Format implementieren
- [ ] Trend-Analyse Algorithmus
- [ ] Tests mit Beispiel-Journal

---

### 2.2 Reflection Engine
**Dateien**:
- `internal/memory/reflection.go`
- `internal/agent/maintenance.go`

```go
// ReflectionSession repräsentiert eine Reflexion
type ReflectionSession struct {
    ID              int64
    Timestamp       time.Time
    TriggerType     string // "scheduled", "milestone", "trait_change", "inactivity"
    TriggerData     string
    Observations    []Observation
    Insights        []string
    Adaptations     []Adaptation
    AppliedChanges  []string
}

type Observation struct {
    Category   string  // "user_interaction", "tool_pattern", "emotional_response"
    Content    string
    Weight     float64 // Wichtigkeit 0.0-1.0
}

type Adaptation struct {
    Type        string // "trait_shift", "behavior_change", "preference_update"
    Description string
    Confidence  float64
}

// TriggerReflection startet einen Reflexions-Prozess
func (s *SQLiteMemory) TriggerReflection(
    ctx context.Context,
    trigger string,
    client llm.ChatClient,
) (*ReflectionSession, error) {
    // 1. Sammle Daten:
    //    - Character Journal Analyse
    //    - Letzte N Interaktionen
    //    - Aktuelle Traits vs. Historie
    //    - Milestones Status
    
    // 2. Baue Reflection Prompt
    
    // 3. LLM-Call für Analyse
    
    // 4. Parse Response → Insights & Adaptations
    
    // 5. Speichere Session
    
    // 6. Wende Adaptations an (optional)
}
```

**Tasks**:
- [ ] Reflection Datenbank Schema
- [ ] Reflection Prompt Template erstellen
- [ ] `TriggerReflection` implementieren
- [ ] Integration in Maintenance Loop
- [ ] Config-Optionen hinzufügen

---

### 2.3 Reflection Prompt Template
**Neue Datei**: `prompts/templates/reflection.md`

```markdown
---
id: "self_reflection"
tags: ["internal"]
priority: 50
---

# Self-Reflection Protocol

You are {agent_name}, an AI assistant with a developing personality. You are analyzing your own behavior and evolution.

## Your Core Identity
{core_personality_text}

## Analysis Period
From: {period_start}
To: {period_end}

## Your Character Evolution

### Current Traits
{current_traits_formatted}

### Historical Comparison (30-day average)
{historical_traits_formatted}

### Mood Distribution
{mood_distribution}

### Recent Milestones
{milestones_list}

## Interaction Patterns Observed

### Recent User Interactions
{recent_interactions}

### Tool Usage Patterns
{tool_patterns}

### Emotional Responses
{emotional_responses}

## Reflection Questions

Please analyze the following:

1. **Evolution Analysis**: How have you changed during this period? What trends do you observe in your traits?

2. **Consistency Check**: Are you staying true to your core personality? Where do you deviate?

3. **User Relationship**: How has your relationship with the user evolved? What patterns emerge?

4. **Behavioral Insights**: What do your tool choices and responses reveal about your current state?

5. **Growth Opportunities**: In what areas could you improve or evolve?

## Output Format

Respond with a structured analysis:

```json
{
  "observations": [
    {
      "category": "trait_trend|user_relationship|behavior_pattern",
      "observation": "string",
      "significance": 0.0-1.0
    }
  ],
  "insights": [
    "string - deep understanding gained"
  ],
  "adaptations": [
    {
      "type": "trait_adjustment|behavior_change",
      "description": "string",
      "target_trait": "string (optional)",
      "suggested_delta": float (optional),
      "confidence": 0.0-1.0
    }
  ],
  "summary": "string - overall reflection summary"
}
```

## Important Notes

- Be honest and critical in your analysis
- Look for genuine patterns, not random fluctuations
- Consider both strengths and areas for improvement
- Suggest concrete, actionable adaptations
- Confidence should reflect certainty of observation
```

**Tasks**:
- [ ] Prompt Template erstellen
- [ ] Parser für Reflection-Response
- [ ] Integration mit Prompt Builder

---

## Phase 3: Memory Integration (Week 6-7)

### 3.1 Episodic Memory mit Personality
**Dateien**:
- `internal/memory/long_term.go` oder neue `episodic.go`

```go
// EpisodicMemoryEntry erweitert Memory mit Personality-Context
type EpisodicMemoryEntry struct {
    ID          string
    Content     string
    Timestamp   time.Time
    SessionID   string
    
    // Personality Context (zum Zeitpunkt der Speicherung)
    Mood        Mood
    Traits      PersonalityTraits
    UserAffinity float64
    
    // Emotional Valenz
    EmotionalValence float64 // -1.0 (negativ) bis +1.0 (positiv)
    Significance     float64 // 0.0-1.0 Wichtigkeit
}

// StoreEpisodicMemory speichert mit aktuellem Personality-State
func (s *SQLiteMemory) StoreEpisodicMemory(
    content string,
    significance float64,
) (string, error) {
    // 1. Hole aktuellen Personality-State
    // 2. Berechne Emotional Valence aus Content
    // 3. Speichere in VectorDB mit Metadata
}

// RetrieveEpisodicByPersonality findet ähnliche emotionale Zustände
func (s *SQLiteMemory) RetrieveEpisodicByPersonality(
    currentTraits PersonalityTraits,
    currentMood Mood,
    limit int,
) ([]EpisodicMemoryEntry, error) {
    // 1. Suche in VectorDB
    // 2. Re-ranke basierend auf Personality-Ähnlichkeit
    // 3. Gewichte: Trait-Ähnlichkeit * Mood-Match * Recency
}
```

**Tasks**:
- [ ] Episodic Memory Schema designen
- [ ] Integration mit ChromaDB
- [ ] Personality-basiertes Retrieval
- [ ] Tests

---

### 3.2 Personality-Influenced RAG
**Dateien**:
- `internal/agent/personality.go`

```go
// rerankWithPersonality erweitert bestehende Reranking-Logik
func rerankWithPersonality(
    memories []rankedMemory,
    currentTraits PersonalityTraits,
    currentMood Mood,
) []rankedMemory {
    // Für jede Memory:
    // 1. Extrahiere gespeicherte Trait-Daten (wenn vorhanden)
    // 2. Berechne Trait-Ähnlichkeit (Cosine Similarity)
    // 3. Mood-Match Bonus
    // 4. Modifiziere Score: baseScore * (1 + personalityBonus)
    // 5. Sortiere neu
}

// personalityBonus berechnet Ähnlichkeits-Boost
func personalityBonus(
    mem EpisodicMemoryEntry,
    traits PersonalityTraits,
    mood Mood,
) float64 {
    // Trait Cosine Similarity
    // Mood Match: +0.1 wenn gleich
    // Affinity Similarity
}
```

**Tasks**:
- [ ] `rerankWithPersonality` implementieren
- [ ] Integration in `rerankWithRecency`
- [ ] A/B Test Vergleich

---

### 3.3 Personality-Based Decision Weights
**Dateien**:
- `internal/agent/agent_parse.go`

```go
// Bereits teilweise implementiert in calculateEffectiveMaxCalls
// Erweitern für:

// ToolConfidence beeinflusst Tool-Auswahl
func CalculateToolConfidence(
    action string,
    traits PersonalityTraits,
    mood Mood,
) float64 {
    confidence := 1.0
    
    switch action {
    case "execute_shell", "execute_python":
        // Hohe Confidence → mehr Vertrauen in eigene Fähigkeiten
        confidence += (traits[TraitConfidence] - 0.5) * 0.2
        // Cautious mood → weniger Vertrauen
        if mood == MoodCautious {
            confidence -= 0.1
        }
        
    case "file_write", "file_delete":
        // Thoroughness → sorgfältiger, aber langsamer
        if traits[TraitThoroughness] > 0.7 {
            confidence -= 0.05 // Extra vorsichtig
        }
        
    case "brainstorm", "create_design":
        // Creativity Boost
        confidence += (traits[TraitCreativity] - 0.5) * 0.3
        
    case "analyze":
        // Analytical Mood → mehr Vertrauen in Analyse
        if mood == MoodAnalytical {
            confidence += 0.1
        }
    }
    
    return clamp(confidence, 0.5, 1.5)
}
```

**Tasks**:
- [ ] Tool-Confidence Mapping erstellen
- [ ] Integration in Tool-Auswahl
- [ ] UI Anzeige für Debug

---

## Phase 4: Dashboard & UI (Week 8)

### 4.1 Personality Dashboard Erweiterungen
**Dateien**:
- `internal/server/handlers_personality.go`
- `ui/` (Frontend)

**Neue Endpoints**:
```go
// GET /api/personality/reflections
// Liefert Reflection History

// GET /api/personality/evolution
// Liefert Trait-Evolution über Zeit

// POST /api/personality/trigger-reflection
// Manuelle Reflection anstoßen

// GET /api/personality/episodic-memories
// Personality-getaggte Erinnerungen
```

**UI Komponenten**:
- [ ] Trait Evolution Graph (Zeitverlauf)
- [ ] Mood Distribution Chart
- [ ] Reflection History Viewer
- [ ] Milestone Timeline
- [ ] Episodic Memory Browser

---

## Konfiguration

### Neue Config-Optionen

```yaml
agent:
  # Bestehend
  personality_engine: true
  personality_engine_v2: true
  
  # NEU: Phase 1
  personality_weighted_decay: true
  personality_milestone_persistence: true
  
  # NEU: Phase 2
  personality_reflection_enabled: true
  personality_reflection_interval_hours: 24
  personality_reflection_min_triggers: 10  # Min Interaktionen für Reflection
  personality_auto_apply_adaptations: false  # Oder true für vollautomatisch
  
  # NEU: Phase 3
  personality_episodic_memory: true
  personality_influenced_rag: true
  personality_tool_confidence: true
```

---

## Test-Strategie

### Unit Tests
```go
// Test: Weighted Decay
func TestDecayAllTraitsWeighted(t *testing.T) {
    // Setup Traits: Curiosity=0.8, Confidence=0.3
    // Decay mit baseAmount=0.1
    // Erwartet: Curiosity decayt weniger als Confidence
}

// Test: Milestone Persistence
func TestMilestoneEffectApplication(t *testing.T) {
    // Trigger "Deep Explorer"
    // Prüfe: Curiosity Floor wurde gesetzt
}

// Test: Reflection Parser
func TestParseCharacterJournal(t *testing.T) {
    // Beispiel Journal
    // Prüfe korrekte Parsing
}
```

### Integration Tests
```go
// Test: Full Reflection Cycle
func TestReflectionCycle(t *testing.T) {
    // 1. Simuliere Interaktionen
    // 2. Trigger Reflection
    // 3. Prüfe: Insights wurden generiert
    // 4. Prüfe: Adaptations wurden gespeichert
}
```

### Manuelle Tests
- [ ] 7-Tage-Lauf: Trait Stability messen
- [ ] Milestone-Trigger: Verhalten danach beobachten
- [ ] Reflection Qualität: Outputs reviewen

---

## Erfolgsmetriken

### Quantitativ
| Metrik | Ziel | Messung |
|--------|------|---------|
| Trait Stability (30 Tage) | > 0.7 | Variance der Traits |
| Reflection Quality Score | > 4/5 | Manuelle Bewertung |
| Episodic Memory Hit Rate | > 30% | % der relevanten Retrieves |
| User Satisfaction | > 4.5/5 | Umfrage |

### Qualitativ
- Konsistentere Persönlichkeit über Sessions
- Authentischere Reaktionen basierend auf Historie
- Sinnvolle Selbstreflexionen

---

## Appendix: Datenbank Migrationen

```sql
-- Migration 001: Trait Bounds
CREATE TABLE IF NOT EXISTS personality_trait_bounds (
    trait TEXT PRIMARY KEY,
    floor REAL DEFAULT 0.0,
    ceiling REAL DEFAULT 1.0,
    decay_resistance REAL DEFAULT 1.0,
    FOREIGN KEY (trait) REFERENCES personality_traits(trait)
);

-- Migration 002: Reflection Sessions
CREATE TABLE IF NOT EXISTS reflection_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    trigger_type TEXT NOT NULL,
    trigger_data TEXT,
    summary TEXT,
    adaptations_applied INTEGER DEFAULT 0
);

-- Migration 003: Reflection Observations
CREATE TABLE IF NOT EXISTS reflection_observations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER,
    category TEXT,
    content TEXT,
    weight REAL,
    FOREIGN KEY (session_id) REFERENCES reflection_sessions(id)
);

-- Migration 004: Episodic Memories mit Personality
CREATE TABLE IF NOT EXISTS episodic_memories (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    session_id TEXT,
    mood TEXT,
    curiosity REAL,
    thoroughness REAL,
    creativity REAL,
    empathy REAL,
    confidence REAL,
    affinity REAL,
    loneliness REAL,
    emotional_valence REAL,
    significance REAL
);
```

---

**Letzte Aktualisierung**: 2026-03-15  
**Status**: Planung abgeschlossen, bereit für Umsetzung
