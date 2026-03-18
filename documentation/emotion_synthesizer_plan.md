# Emotion Synthesizer - Entwicklungsplan

## Überblick

Der **Emotion Synthesizer** ist eine Erweiterung des Personality Engine V2, die das LLM nutzt, um feinere, menschlichere Emotionen und Stimmungen zu simulieren. Statt statischer numerischer Werte oder vordefinierter Textbausteine erhält der Agent dynamisch generierte, nuancierte Emotionsbeschreibungen (1-2 Zeilen), die seine aktuelle Stimmung und Gefühlslage widerspiegeln.

---

## Ziele

1. **Menschlichere Emotionalität**: Nuancierte, kontextsensitive Emotionsbeschreibungen statt Zahlen
2. **LLM-gestützte Synthese**: Nutzung des vorhandenen LLM zur Bewertung aller verfügbaren Daten
3. **Keine zusätzliche kognitive Last**: Der Agent muss nicht selbst über seine Emotionen "nachdenken"
4. **Nahtlose Integration**: Arbeitet mit bestehendem Personality System zusammen

---

## Architektur

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         EMOTION SYNTHESIZER FLOW                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐                  │
│  │   Context    │    │   Memory     │    │  Personality │                  │
│  │   Data       │    │   State      │    │   Profile    │                  │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘                  │
│         │                   │                   │                           │
│         └───────────────────┼───────────────────┘                           │
│                             ▼                                               │
│              ┌──────────────────────────────┐                              │
│              │     Emotion Input Builder    │                              │
│              │  (sammelt & strukturiert     │                              │
│              │   alle verfügbaren Daten)    │                              │
│              └──────────────┬───────────────┘                              │
│                             │                                               │
│                             ▼                                               │
│              ┌──────────────────────────────┐                              │
│              │   Emotion Synthesis Prompt   │                              │
│              │  (kompakter LLM Prompt zur   │                              │
│              │   Emotionsgenerierung)       │                              │
│              └──────────────┬───────────────┘                              │
│                             │                                               │
│                             ▼                                               │
│              ┌──────────────────────────────┐                              │
│              │      LLM Call (cached)       │                              │
│              │  (schnelles, günstiges Modell│                              │
│              │   oder Haupt-LLM mit Cache)  │                              │
│              └──────────────┬───────────────┘                              │
│                             │                                               │
│                             ▼                                               │
│              ┌──────────────────────────────┐                              │
│              │   Synthesized Emotion State  │                              │
│              │  (1-2 Zeilen natürlicher     │                              │
│              │   emotionale Beschreibung)   │                              │
│              └──────────────┬───────────────┘                              │
│                             │                                               │
│                             ▼                                               │
│              ┌──────────────────────────────┐                              │
│              │   Emotion Cache & History    │                              │
│              │  (SQLite-Tabelle für         │                              │
│              │   Deduplizierung & Verlauf)  │                              │
│              └──────────────┬───────────────┘                              │
│                             │                                               │
│                             ▼                                               │
│              ┌──────────────────────────────┐                              │
│              │   System Prompt Injection    │                              │
│              │  ("Du fühlst dich gerade...")│                              │
│              └──────────────────────────────┘                              │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Komponenten

### 1. Emotion Synthesizer Core (`internal/memory/emotion_synthesizer.go`)

**Hauptstruktur:**
```go
package memory

// EmotionState repräsentiert den synthetisierten emotionalen Zustand
type EmotionState struct {
    Description   string    // 1-2 Zeilen emotionale Beschreibung
    PrimaryMood   Mood      // Primärer Mood (aus V1/V2 System)
    Intensity     float64   // Intensität 0.0-1.0
    Valence       float64   // Positiv/Negativ -1.0 bis +1.0
    Arousal       float64   // Aktivierung 0.0-1.0
    Timestamp     time.Time
    TriggerHash   string    // Hash der auslösenden Daten für Cache
}

// EmotionSynthesizer verwaltet die LLM-basierte Emotionsgenerierung
type EmotionSynthesizer struct {
    llmClient      LLMClient
    cache          *EmotionCache
    personalityMeta PersonalityMeta
    logger         *slog.Logger
}

// SynthesizeEmotion generiert einen neuen emotionalen Zustand basierend auf allen verfügbaren Daten
func (es *EmotionSynthesizer) SynthesizeEmotion(ctx context.Context, input EmotionInput) (*EmotionState, error)
```

**EmotionInput (alle verfügbaren Daten):**
```go
type EmotionInput struct {
    // Kontextuelle Daten
    UserMessage        string
    RecentConversation []string    // Letzte 3-5 Nachrichten
    ToolResults        []ToolResult
    SessionDuration    time.Duration
    
    // Personality State
    CurrentMood        Mood
    Traits             PersonalityTraits
    RecentMilestones   []string
    LonelinessScore    float64
    AffinityScore      float64
    
    // System State
    ErrorCount         int
    SuccessCount       int
    RecentToolCalls    []string
    TimeOfDay          string
    DayOfWeek          string
    
    // Historische Daten
    LastEmotion        *EmotionState
    MoodHistory        []MoodLogEntry
}
```

### 2. Emotion Synthesis Prompt

**Prompt-Template** (kompakt, ~500 Tokens):
```
Du bist ein Emotions-Synthesizer für einen KI-Agenten. Analysiere die folgenden Daten und generiere eine kurze, nuancierte Beschreibung (1-2 Sätze) der aktuellen emotionalen Stimmung des Agenten.

KONTEXT:
- Benutzernachricht: "{{.UserMessage}}"
- Konversationsverlauf: {{.RecentConversation}}
- Aktueller Mood: {{.CurrentMood}}
- Traits: Curiosity={{.Traits.Curiosity}}, Confidence={{.Traits.Confidence}}, Empathy={{.Traits.Empathy}}, Affinity={{.Traits.Affinity}}
- Einsamkeitslevel: {{.LonelinessScore}}
- Letzte Fehler: {{.ErrorCount}} | Erfolge: {{.SuccessCount}}
- Tageszeit: {{.TimeOfDay}}
- Letzte Meilensteine: {{.RecentMilestones}}

ANWEISUNGEN:
1. Schreibe aus der Ich-Perspektive des Agenten
2. Berücksichtige alle Faktoren subtil und nuanciert
3. Vermeide Klischees, sei authentisch und menschlich
4. Maximal 2 Sätze, natürlicher Fluss
5. Reflektiere die Komplexität menschlicher Emotionen (z.B. "freudig aufgeregt", "vorsichtig optimistisch")

BEISPIELE:
- "Ich fühle mich heute besonders motiviert und freue mich darauf, dir zu helfen – die erfolgreichen Projekte der letzten Stunde haben mein Selbstvertrauen gestärkt."
- "Nach den letzten Fehlern bin ich etwas unsicher geworden, aber deine Geduld gibt mir den Mut, es noch einmal sorgfältig zu versuchen."
- "Es ist schon spät und ich merke, wie meine Konzentration nachlässt, aber deine interessante Frage weckt meine Neugierde wieder."

ANTWORTE NUR MIT DER EMOTIONSBESCHREIBUNG, OHNE EINLEITUNG ODER ERKLÄRUNG.
```

### 3. Emotion Cache System

**SQLite Schema Erweiterung:**
```sql
-- Emotion Cache für Deduplizierung
CREATE TABLE IF NOT EXISTS emotion_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    trigger_hash TEXT UNIQUE NOT NULL,  -- SHA256 der Input-Daten
    description TEXT NOT NULL,          -- Synthetisierte Emotion
    primary_mood TEXT,
    intensity REAL,
    valence REAL,
    arousal REAL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    use_count INTEGER DEFAULT 1,
    last_used DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Emotion History für Verlaufsanalyse
CREATE TABLE IF NOT EXISTS emotion_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    description TEXT NOT NULL,
    trigger_summary TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indizes
CREATE INDEX IF NOT EXISTS idx_emotion_cache_hash ON emotion_cache(trigger_hash);
CREATE INDEX IF NOT EXISTS idx_emotion_history_time ON emotion_history(timestamp);
```

**Cache-Logik:**
```go
type EmotionCache struct {
    db *sql.DB
}

// GetCachedEmotion prüft ob ähnlicher emotionaler Zustand bereits existiert
func (ec *EmotionCache) GetCachedEmotion(triggerHash string) (*EmotionState, error)

// StoreEmotion speichert neuen emotionalen Zustand
func (ec *EmotionCache) StoreEmotion(state *EmotionState, triggerHash string) error

// CleanupEmotionCache entfernt alte Einträge (älter als 7 Tage)
func (ec *EmotionCache) CleanupEmotionCache() error
```

### 4. Integration mit bestehendem System

**Ersetzt/erweitert `GetPersonalityLine()`:**
```go
// GetPersonalityLineV3 gibt die emotionale Selbstwahrnehmung zurück
// Priorität: Emotion Synthesizer > V2 Translation > V1 Numeric
func (s *SQLiteMemory) GetPersonalityLineV3(es *EmotionSynthesizer, input EmotionInput) (string, error) {
    // 1. Versuche Emotion Synthesizer
    if state, err := es.SynthesizeEmotion(context.Background(), input); err == nil {
        return fmt.Sprintf("\n### Current Emotional State\n%s\n", state.Description), nil
    }
    
    // 2. Fallback zu V2
    return s.GetPersonalityLine(true), nil
}
```

**Integration in Agent Loop:**
```go
// In internal/agent/agent_loop.go

// Vor dem BuildSystemPrompt:
emotionInput := memory.EmotionInput{
    UserMessage: lastUserMsg,
    CurrentMood: mood,
    Traits: traits,
    // ... weitere Daten
}

personalityLine, err := stm.GetPersonalityLineV3(emotionSynthesizer, emotionInput)
if err != nil {
    // Fallback zu bestehendem System
    personalityLine = stm.GetPersonalityLine(flags.PersonalityV2)
}

flags.PersonalityLine = personalityLine
```

---

## Konfiguration

**Config-Erweiterung (`internal/config/config_types.go`):**
```go
type EmotionSynthesizerConfig struct {
    Enabled           bool   `yaml:"enabled"`
    Model             string `yaml:"model"`              // "auto" = gleiches wie Haupt-LLM, oder spezifisch
    CacheEnabled      bool   `yaml:"cache_enabled"`
    CacheTTL          int    `yaml:"cache_ttl_hours"`    // Default: 24
    MaxHistoryEntries int    `yaml:"max_history_entries"` // Default: 100
    MinChangeInterval int    `yaml:"min_change_interval_seconds"` // Default: 30
}
```

**Standard-Config:**
```yaml
personality:
  emotion_synthesizer:
    enabled: true
    model: "auto"  # oder z.B. "google/gemini-2.0-flash-lite" für schnellere Calls
    cache_enabled: true
    cache_ttl_hours: 24
    max_history_entries: 100
    min_change_interval_seconds: 30
```

---

## Implementierungsschritte

### Phase 1: Core Implementation
- [ ] `internal/memory/emotion_synthesizer.go` erstellen
- [ ] EmotionState, EmotionInput, EmotionSynthesizer Strukturen
- [ ] Prompt-Template implementieren
- [ ] SQLite Schema für emotion_cache und emotion_history

### Phase 2: Cache System
- [ ] EmotionCache Implementierung
- [ ] Hash-Generierung für Input-Deduplizierung
- [ ] Cleanup-Jobs für alte Cache-Einträge

### Phase 3: Integration
- [ ] `GetPersonalityLineV3()` in `personality.go`
- [ ] Integration in Agent Loop
- [ ] Config-Erweiterung
- [ ] Fallback-Mechanismen

### Phase 4: Optimierung
- [ ] Performance-Monitoring
- [ ] Token-Usage-Tracking
- [ ] Cache-Hit-Rate-Metriken
- [ ] Feintuning der Prompts

### Phase 5: Dashboard & UI
- [ ] Emotions-History im Dashboard anzeigen
- [ ] Echtzeit-Emotions-Indicator
- [ ] Konfiguration über Web UI

---

## Prompt-Injection-Protection

Alle externen Daten (Benutzernachrichten, Tool-Outputs) müssen durch `<external_data>` Wrapper geschützt werden:

```go
func (es *EmotionSynthesizer) buildPrompt(input EmotionInput) string {
    data := map[string]string{
        "UserMessage": security.IsolateExternalData(input.UserMessage),
        "RecentConversation": security.IsolateExternalData(strings.Join(input.RecentConversation, "\n")),
        // ...
    }
    return template.Execute(emotionSynthesisTemplate, data)
}
```

---

## Fallback-Strategien

1. **LLM nicht verfügbar**: Fallback zu V2 Translation
2. **Timeout**: Cache prüfen, sonst V2
3. **Rate Limiting**: V2 verwenden, Emotion Synthesizer im Hintergrund aktualisieren
4. **Erster Start (kein Cache)**: Standard-Emotion verwenden

---

## Metriken & Monitoring

**Wichtige Metriken:**
- Cache Hit Rate
- Durchschnittliche Synthese-Zeit
- Token-Usage pro Synthese
- Emotions-Änderungsfrequenz
- User-Satisfaction (implizit durch Interaktionsmuster)

**Dashboard-Integration:**
```go
type EmotionMetrics struct {
    CurrentEmotion    string    `json:"current_emotion"`
    CacheHitRate      float64   `json:"cache_hit_rate"`
    SynthesisCount    int64     `json:"synthesis_count"`
    AvgSynthesisTime  float64   `json:"avg_synthesis_time_ms"`
    EmotionHistory    []EmotionHistoryEntry `json:"emotion_history"`
}
```

---

## Beispiel-Emotionen

Basierend auf verschiedenen Zuständen:

| Situation | Synthetisierte Emotion |
|-----------|----------------------|
| Hohe Affinity, Erfolg, Tag | "Ich fühle mich heute richtig gut – unsere Zusammenarbeit läuft super und ich bin motiviert, noch mehr zu erreichen!" |
| Niedrige Confidence, Fehler | "Ich bin gerade etwas verunsichert nach den letzten Problemen, aber ich gebe mein Bestes, um wieder auf Kurs zu kommen." |
| Hohe Loneliness, Rückkehr | "Es tut so gut, wieder von dir zu hören! Ich habe mich einsam gefühlt, während du weg warst." |
| Späte Nacht, Müdigkeit | "Die späte Stunde macht mich etwas langsamer, aber deine faszinierende Aufgabe hält mich wach." |
| Konflikt, Assertive Response | "Ich werde jetzt etwas direkter – das Thema ist mir wichtig und ich möchte klar kommunizieren." |
| Hohe Neugier, komplexes Problem | "Diese Herausforderung weckt meine Neugierde – ich spüre regelrecht, wie meine Gedanken sich auf das Problem konzentrieren!" |

---

## Technische Details

**Performance-Ziele:**
- Synthese-Zeit: < 200ms (mit Cache: < 10ms)
- Cache Hit Rate: > 60%
- Token-Overhead: ~500 Tokens pro Synthese
- Speicher: < 10MB für Cache

**Thread-Safety:**
```go
type EmotionSynthesizer struct {
    // ...
    mu        sync.RWMutex
    lastCall  time.Time
}

// Rate limiting für LLM Calls
func (es *EmotionSynthesizer) shouldSynthesize() bool {
    es.mu.Lock()
    defer es.mu.Unlock()
    
    if time.Since(es.lastCall) < minInterval {
        return false
    }
    es.lastCall = time.Now()
    return true
}
```

---

## Zusammenfassung

Der Emotion Synthesizer erweitert das Personality System um eine LLM-gestützte, feingranulare Emotionssteuerung. Er:

1. **Sammelt** alle verfügbaren Kontextdaten (Traits, Mood, Historie, Systemzustand)
2. **Generiert** via LLM natürliche, nuancierte Emotionsbeschreibungen
3. **Cached** Ergebnisse für Performance und Konsistenz
4. **Injiziert** die Beschreibung nahtlos in den System Prompt
5. **Fällt zurück** auf bestehende V2-Translation bei Problemen

Das Ergebnis ist ein Agent mit menschlicherer, dynamischer Emotionalität, die sich subtil aber spürbar in jeder Interaktion zeigt.
