# Bericht: Memory-basierte automatische Prompt-Injektion

**Projekt:** AuraGo  
**Datum:** 15.03.2026  
**Autor:** Code-Analyse-Agent  
**Status:** Vollständige Architektur-Review

---

## Zusammenfassung

Das AuraGo-System implementiert ein **mehrstufiges Memory-System** mit automatischer Prompt-Injektion. Die Architektur ist grundsätzlich solide, hat aber Optimierungspotenzial bei der Verknüpfung zwischen **Memory Analysis LLM** und **Prompt Injection**.

**Wichtig:** Das System bremst den Agenten nicht aus und arbeitet zuverlässig. Die identifizierten Verbesserungen sind optionale Optimierungen.

---

## 1. Aktuelles System-Architektur

### 1.1 Datenfluss der automatischen Prompt-Injektion

```
User Message → VectorDB Search (6 candidates) 
                    ↓
         rerankWithRecency() (similarity + temporal)
                    ↓
         Top 3 Memories → flags.RetrievedMemories
                    ↓
         BuildSystemPrompt() → Prompt Injection
                    ↓
         LLM Call
                    ↓
         Async: runMemoryAnalysis() (post-response)
```

### 1.2 Kernkomponenten

| Komponente | Datei | Funktion |
|------------|-------|----------|
| **RAG Retrieval** | `internal/agent/agent_loop.go:422-443` | Semantic Search über VectorDB (Chroma) |
| **Recency Reranking** | `internal/agent/personality.go:27-70` | Kombiniert Similarity mit Zeit-Bonus |
| **Predictive Prefetch** | `internal/agent/agent_loop.go:454-474` | Temporale Muster + Tool-Transitions |
| **Prompt Builder** | `internal/prompts/builder.go:245-432` | Assembliert System Prompt mit Memory-Sektionen |
| **Memory Analysis** | `internal/agent/memory_analysis.go:59-185` | Async LLM-basierte Extraktion von Fakten |

### 1.3 Aktuelle Memory-Typen im Prompt

Aus `internal/prompts/builder.go` Zeilen 292-331:

```go
1. CORE MEMORY          → Immer injiziert (kritische Fakten)
2. RETRIEVED MEMORIES   → Nur wenn Tier != "minimal"  
3. PREDICTED CONTEXT    → Nur im "full" Tier
4. KNOWLEDGE CONTEXT    → Knowledge Graph Entities
5. ERROR PATTERNS       → Während Error Recovery
```

---

## 2. Analyse: Memory Analysis LLM vs. Prompt Injection

### 2.1 Aktuelle Isolation

Das **Memory Analysis LLM** (`runMemoryAnalysis`) läuft **asynchron nach der Response** und ist **komplett entkoppelt** vom Prompt-Injection-System:

```go
// internal/agent/agent_loop.go:1549-1556
// Real-time memory analysis: async post-response extraction
if cfg.MemoryAnalysis.Enabled && cfg.MemoryAnalysis.RealTime && !isEmpty {
    go func(userMsg, aResp, sid string) {
        analysisCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
        defer cancel()
        runMemoryAnalysis(analysisCtx, cfg, currentLogger, shortTermMem, kg, longTermMem, userMsg, aResp, sid)
    }(lastUserMsg, content, sessionID)
}
```

**Problem identifiziert:** Das Memory Analysis LLM extrahiert wertvolle Informationen, die **nie direkt in den aktuellen Prompt** gelangen - sie werden erst in die LTM gespeichert und müssen über RAG wieder gefunden werden.

### 2.2 Potenzial für Verbesserung

| Aspekt | Aktuell | Potenzial mit Memory Analysis LLM |
|--------|---------|-----------------------------------|
| **Query-Understanding** | Keyword-basierte Vector Search | Semantische Intent-Analyse |
| **Memory-Selektion** | Top-K nach Similarity | Relevanz-basierte Auswahl |
| **Kontext-Erweiterung** | Nur direkte Matches | Implizite Zusammenhänge |
| **Fehler-Erkennung** | Keine | "Du fragst nach X, aber hast Y im LTM" |

---

## 3. Sicherheits- und Performance-Analyse

### 3.1 Was das System GUT macht ✅

| Aspekt | Implementierung | Bewertung |
|--------|-----------------|-----------|
| **Async Processing** | `go func()` für Memory Analysis | ✅ Nicht-blockierend |
| **Timeouts** | Überall `context.WithTimeout()` | ✅ Keine Hänger |
| **Graceful Degradation** | `if longTermMem != nil` Checks | ✅ Funktioniert ohne LTM |
| **Token Budget** | `budgetShed()` progressives Entfernen | ✅ Keine Context-Overflows |
| **Tier-System** | full/compact/minimal Adaptive | ✅ Skaliert mit Länge |
| **Recency Boost** | Zeit-basiertes Re-Ranking | ✅ Aktuelle Infos priorisiert |

### 3.2 Verbesserungsbedarf ⚠️

| Problem | Ort | Risiko | Lösung |
|---------|-----|--------|--------|
| **Memory Analysis ist "fire and forget"** | `memory_analysis.go:59` | Ergebnisse fließen nicht in aktuelle Anfrage ein | Query-Expansion (siehe 4.1) |
| **Keine Query-Expansion** | `agent_loop.go:428` | User muss exakte Keywords verwenden | Intent-Analyse |
| **RAG Retrieval ist "dumm"** | `long_term.go:397` | Keine Verknüpfung von Konzepten | LLM-basiertes Re-Ranking |

---

## 4. Empfohlene Verbesserungen

### 4.1 Option A: Pre-Query Memory Enhancement (Empfohlen - Minimaler Impact)

**Idee:** Das Memory Analysis LLM vor der Haupt-LLM-Anfrage nutzen, um die Query zu erweitern/verfeinern.

```go
// Konzept: Vor dem RAG-Retrieval
func enhanceQueryWithLLM(ctx context.Context, userMsg string, cfg *Config) (enhancedQuery string, relatedConcepts []string) {
    // Schneller LLM-Call (500 tokens max)
    // Beispiel-Prompt:
    // "User fragt: 'Wie war das mit dem Server?'
    //  Extrahiere: 1) Was weißt du über den User's Server aus dem Kontext?
    //              2) Welche verwandten Konzepte könnten relevant sein?"
    
    // Nutzt das konfigurierte MemoryAnalysis-LLM (kann günstiger sein als Haupt-LLM)
}
```

**Vorteile:**
- Keine Blockierung (schneller Call, < 1s)
- Besseres Retrieval durch erweiterte Queries
- Nutzt bestehende MemoryAnalysis-Konfiguration

### 4.2 Option B: Relevance-Scoring durch LLM (Mittlerer Impact)

**Idee:** Die 6 RAG-Kandidaten vor dem Re-Ranking durch das Memory Analysis LLM bewerten lassen.

```go
// Nach VectorDB.SearchSimilar(), vor rerankWithRecency()
candidates, _ := longTermMem.SearchSimilar(lastUserMsg, 6)

// LLM-basierte Relevanz-Prüfung (parallel, mit Timeout)
relevantIDs := filterRelevantWithLLM(ctx, lastUserMsg, candidates, 100*time.Millisecond)

// Danach normales Re-Ranking
ranked := rerankWithRecency(relevantIDs, ...)
```

**Vorteile:**
- Höhere Qualität der injizierten Memories
- Filtert "False Positives" der Vector-Search heraus

**Risiko:** 
- Zusätzliche Latenz (100-200ms)
- Fehler bei Timeout-Handling

### 4.3 Option C: Hybrides System mit Caching (Maximaler Impact, komplexer)

**Idee:** Häufige Themen/Intents cachen und das Memory Analysis LLM nur für neue/unklare Queries nutzen.

---

## 5. Konkrete Implementierungsempfehlung

### Phase 1: Query Intent Analysis (Sofort umsetzbar)

**Änderung in `internal/agent/agent_loop.go` vor Zeile 428:**

```go
// Aktuell:
memories, docIDs, err := longTermMem.SearchSimilar(lastUserMsg, 6)

// Neu: Intent-basierte Expansion wenn MemoryAnalysis aktiv
searchQuery := lastUserMsg
if cfg.MemoryAnalysis.Enabled && len(lastUserMsg) > 20 {
    // Quick Intent Extraction (nutzt ggf. günstigeres Analysis-LLM)
    searchQuery = expandQueryWithIntent(lastUserMsg, stm)
}
memories, docIDs, err := longTermMem.SearchSimilar(searchQuery, 6)
```

**Zeit-Impact:** +50-100ms nur bei aktivem MemoryAnalysis  
**Qualitäts-Impact:** Deutlich besseres Retrieval für komplexe Queries

### Phase 2: Smart Memory Injection (Optional)

**Neue Funktion in `internal/agent/memory_analysis.go`:**

```go
// AnalyzeQueryContext nutzt das MemoryAnalysis-LLM um zu bestimmen,
// welche Informationen für die aktuelle Query relevant wären
func AnalyzeQueryContext(ctx context.Context, cfg *Config, query string, candidates []string) ([]string, error) {
    // Schneller LLM-Call mit striktem Timeout
    // Rückgabe: Indizes der relevanten Candidates
}
```

---

## 6. Config-Optionen für Memory Analysis

Aktuell verfügbare Einstellungen (aus `internal/config/config_types.go`):

```yaml
memory_analysis:
  enabled: true                    # Master-Switch
  real_time: true                  # Nach jeder Response analysieren
  provider: "openrouter"           # Dedizierter Provider (optional)
  model: "google/gemini-flash"     # Günstiges Modell für Analysis
  auto_confirm_threshold: 0.92     # Confidence für Auto-Speicherung
  weekly_reflection: true          # Wöchentliche Zusammenfassung
  reflection_day: "sunday"
```

**Empfohlene Optimale Konfiguration:**

```yaml
# Für bessere Memory-Qualität ohne Performance-Verlust:
memory_analysis:
  enabled: true
  real_time: true
  provider: "openrouter"           # Oder lokales Ollama für Privacy
  model: "google/gemini-2.0-flash-001"  # Schnell & günstig
  auto_confirm_threshold: 0.85     # Etwas niedriger = mehr Memories
```

---

## 7. Fazit

| Kriterium | Bewertung |
|-----------|-----------|
| **System funktioniert korrekt** | ✅ Ja, keine kritischen Bugs |
| **Performance** | ✅ Gut, keine Blockierungen |
| **Nutzt Memory Analysis LLM optimal?** | ⚠️ Nein, läuft isoliert |
| **Verbesserungspotenzial** | 🟡 Mittel bis hoch |

### 7.1 Wesentliche Erkenntnisse

1. **Keine Gefahr für Agent-Performance:** Das System ist asynchron und non-blocking implementiert
2. **Memory Analysis ist "nachgelagert":** Extrahierte Fakten helfen erst bei FUTURE Queries
3. **RAG funktioniert gut:** Vector-Search + Recency-Boost liefert relevante Ergebnisse
4. **Tier-System ist effektiv:** Automatische Reduktion bei langen Konversationen

### 7.2 Empfohlene nächste Schritte

| Priorität | Maßnahme | Impact | Aufwand |
|-----------|----------|--------|---------|
| 🟢 Hoch | Query-Intent-Analyse (Phase 1) | Mittel | 2-3h |
| 🟡 Mittel | LLM-basiertes Re-Ranking | Hoch | 4-6h |
| 🔴 Niedrig | Hybrides Cache-System | Hoch | 1-2 Tage |

---

## Anhang: Code-Referenzen

### A.1 Memory Analysis Prompt

```go
// internal/agent/memory_analysis.go:31-56
const memoryAnalysisPrompt = `You are a memory extraction assistant. Analyze the following conversation exchange and extract any information worth remembering.

Extract:
1. **Facts**: Concrete facts about the user, their environment, preferences, or projects
2. **Preferences**: User preferences, habits, or workflows
3. **Corrections**: Corrections to previously known information

For each extracted item, provide:
- content: The factual statement to remember
- category: A short category label
- confidence: How confident you are this is worth storing (0.0 to 1.0)

Respond ONLY with valid JSON in this exact format:
{"facts":[],"preferences":[],"corrections":[]}`
```

### A.2 Prompt Injection Reihenfolge

```go
// internal/prompts/builder.go:245-432 - Reihenfolge der Memory-Injektion:
1. Core Memory (immer)
2. Session Todo Items (immer wenn vorhanden)
3. Retrieved Memories (außer minimal tier)
4. Predicted Memories (nur full tier)
5. Knowledge Context (außer minimal tier)
6. Error Pattern Context (bei Error State)
```

---

*Ende des Berichts*
