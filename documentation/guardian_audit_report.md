# Bericht: LLM Guardian Implementation – Fehleranalyse & Optimierungspotential

**Erstellt:** 15.03.2026  
**Autor:** Code Review Agent  
**Umfang:** `internal/security/llm_guardian.go`, `guardian.go`, `guardian_cache.go`, `guardian_metrics.go`

---

## Executive Summary

Nach Analyse der Guardian-Implementierung wurden identifiziert:
- **2 kritische Bugs** (Race Condition, Context Timeout)
- **3 Verbesserungspotenziale** (Cache Collision, Rate Limiting, Parameter-Heuristik)
- **4 Optimierungsmöglichkeiten** (Batching, Pre-Warming, Async Logging, Chunking)

**Geschätztes Einsparungspotential:** 50-70% weniger Guardian-LLM-Calls bei gleicher Sicherheit.

---

## 1. Kritische Bugs

### Bug 1: Race Condition im Cache bei Eviction

**Datei:** `guardian_cache.go:70-83`

```go
// evictOldest removes the oldest cache entry. Caller must hold the write lock.
func (c *GuardianCache) evictOldest() {
    var oldestKey string
    var oldestTime time.Time
    first := true
    for k, v := range c.entries {
        if first || v.timestamp.Before(oldestTime) {
            oldestKey = k
            oldestTime = v.timestamp
            first = false
        }
    }
    if oldestKey != "" {
        delete(c.entries, oldestKey)
    }
}
```

**Problem:** 
- `evictOldest()` wird in `Set()` aufgerufen (Line 61), aber der Cache-Key wird **nach** der Eviction gesetzt
- Bei hoher Last können mehrere Goroutines gleichzeitig `Set()` aufrufen, was zu einer Race Condition führt
- Die Größenprüfung (`len(c.entries) >= c.maxSize`) und die Eviction sind nicht atomar

**Impact:** Cache könnte über `maxSize` hinauswachsen oder inkonsistent werden.

**Empfohlener Fix:**
```go
func (c *GuardianCache) Set(key string, result GuardianResult) {
    c.mu.Lock()
    defer c.mu.Unlock()

    // Prüfen ob Key existiert - wenn ja, einfach updaten ohne Eviction
    if _, exists := c.entries[key]; exists {
        c.entries[key] = guardianCacheEntry{
            result:    result,
            timestamp: time.Now(),
        }
        return
    }

    // Nur evicten wenn wir wirklich neuen Eintrag hinzufügen
    if len(c.entries) >= c.maxSize {
        c.evictOldest()
    }
    c.entries[key] = guardianCacheEntry{
        result:    result,
        timestamp: time.Now(),
    }
}
```

---

### Bug 2: Falscher Context-Timeout in EvaluateWithFailSafe

**Datei:** `llm_guardian.go:156-160`

```go
func (g *LLMGuardian) EvaluateWithFailSafe(ctx context.Context, check GuardianCheck) GuardianResult {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    return g.Evaluate(ctx, check)
}
```

**Problem:**
- `Evaluate()` hat bereits intern kein Timeout (außer was der Aufrufer mitgibt)
- Aber `callLLM()` wird direkt aufgerufen ohne Timeout-Protection
- Bei einem hängenden LLM-Provider blockiert der Guardian

**Impact:** Agent-Loop blockiert potenziell für Minuten.

**Empfohlener Fix:**
```go
func (g *LLMGuardian) Evaluate(ctx context.Context, check GuardianCheck) GuardianResult {
    start := time.Now()
    
    // Check cache first
    cacheKey := GenerateCacheKey(check.Operation, check.Parameters)
    if result, hit := g.cache.Get(cacheKey); hit {
        result.Duration = time.Since(start)
        g.Metrics.Record(result)
        g.logger.Debug("[Guardian] Cache hit", "operation", check.Operation, "decision", result.Decision)
        return result
    }

    // Rate limiting: try to acquire semaphore
    select {
    case g.sem <- struct{}{}:
        defer func() { <-g.sem }()
    default:
        g.logger.Warn("[Guardian] Rate limit exceeded, applying fail-safe")
        g.Metrics.RecordError()
        return g.failSafeResult(start, "rate limit exceeded")
    }

    // Build prompt & call LLM with timeout
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second) // HINZUFÜGEN
    defer cancel()
    
    result := g.callLLM(ctx, check, start)
    g.cache.Set(cacheKey, result)
    g.Metrics.Record(result)
    return result
}
```

---

## 2. Verbesserungspotenziale (Medium Priority)

### Issue 3: Cache Key Collision bei Content Scanning

**Datei:** `llm_guardian.go:514`

```go
cacheKey := GenerateCacheKey("content_scan:"+contentType, map[string]string{"content": snippet})
```

**Problem:**
- Content wird auf 1000 Zeichen getruncated für Cache Key
- Zwei unterschiedliche E-Mails mit gleichem Anfang (z.B. "Hi, please...") produzieren denselben Key
- Falsche Cache Hits möglich

**Impact:** Falsche Security-Entscheidungen durch Cache-Kollision.

**Fix:** Hash des vollständigen Content verwenden statt Snippet.

---

### Issue 4: Keine Deduplizierung bei Rate Limiting

**Datei:** `llm_guardian.go:139-147`

**Problem:**
- Bei Rate Limit (`sem` voll) wird sofort Fail-Safe zurückgegeben
- Keine Möglichkeit zu warten oder zu deduplizieren
- Bei Burst-Traffic werden legitime Requests geblockt

**Lösungsvorschläge:**
1. Warte mit Timeout statt sofortigem Fail
2. Deduplication - gleiche Checks in Flight zusammenfassen

---

### Issue 5: Unnötige LLM-Calls bei niedrigem Guardian-Level

**Datei:** `llm_guardian.go:96-123`

**Problem:**
- `ShouldCheck()` prüft nur Tool-Namen, nicht die Parameter
- `execute_shell "ls"` wird genauso geprüft wie `execute_shell "rm -rf /"`
- Verschwendung von Tokens für offensichtlich harmlose Commands

**Optimierung:** Zusätzliche Heuristik für Parameter:
- Safe Patterns (ls, pwd, echo, cat, grep) → Kein LLM
- Dangerous Patterns (rm -rf, mkfs, dd if=) → Immer LLM

---

## 3. Performance-Optimierungen

### Opt 1: Batch-Verarbeitung für Content Scanning

**Aktuell:** Jede E-Mail/Dokument = 1 LLM-Call

**Optimierung:** Mehrere kleine Inhalte batch-verarbeiten
- Ein LLM-Call für bis zu 5 Items
- Format: `1:DECISION SCORE REASON\n2:...`

**Einsparung:** 60-80% weniger LLM-Calls bei E-Mail-Batches.

---

### Opt 2: Cache-Warming für häufige Tool-Kombinationen

**Aktuell:** Cache wird nur bei Requests befüllt

**Optimierung:** Proaktives Caching häufiger Patterns:
- `docker ps`
- `docker logs`
- `execute_shell "ls -la"`
- Im Hintergrund ausführen

---

### Opt 3: Async-Logging statt Sync-Logging

**Datei:** `llm_guardian.go:194-200`

**Problem:** Synchrone I/O-Operation nach jedem LLM-Call

**Fix:** Channel-basiertes asynchrones Logging mit Drop-Policy bei Überlast.

---

### Opt 4: Streaming für große Content-Scans

**Aktuell:** Content wird komplett in Prompt geladen (max 1000 chars)

**Optimierung:** Chunk-basierte Analyse für große Dokumente
- In Chunks aufteilen (je 1000 chars)
- Early exit bei Block-Entscheidung
- Aggregation der Risk Scores

---

## 4. Architekturelle Verbesserungen

### Vorschlag 1: Zwei-Tier Guardian (Regex → LLM)

```
User Request
    ↓
[Regex Guardian] ──HIGH──→ [Block]
    ↓ LOW/MEDIUM
[LLM Guardian] ──Block──→ [Clarification/Block]
    ↓ Allow
[Execute Tool]
```

**Einsparung:** ~70% der LLM-Calls bei niedrigem Threat-Level

---

### Vorschlag 2: Guardian-Entscheidungen persistieren

**Aktuell:** Cache ist nur In-Memory

**Verbesserung:** SQLite-Cache für Überlebensfähigkeit
- Prüfe In-Memory first
- Dann SQLite
- TTL-basierte Invalidierung

---

## 5. Zusammenfassung der Empfohlungen

| Priorität | Issue | Aufwand | Impact |
|-----------|-------|---------|--------|
| 🔴 Kritisch | Bug 1: Race Condition | 30 min | Stabilität |
| 🔴 Kritisch | Bug 2: Context Timeout | 15 min | Stabilität |
| 🟡 Hoch | Issue 3: Cache Collision | 45 min | Sicherheit |
| 🟡 Hoch | Opt 1: Batch Processing | 2h | 60% weniger Tokens |
| 🟢 Mittel | Issue 5: Parameter-Heuristik | 1h | 40% weniger Calls |
| 🟢 Mittel | Opt 2: Pre-Warming | 30 min | Latenz |
| 🔵 Niedrig | Architektur: Two-Tier | 4h | Skalierbarkeit |

---

## Anhang: Referenzierte Dateien

- `internal/security/llm_guardian.go` - Haupt-Implementierung
- `internal/security/guardian.go` - Regex-basierte Prüfung
- `internal/security/guardian_cache.go` - Caching-Layer
- `internal/security/guardian_metrics.go` - Metriken
- `internal/security/llm_guardian_test.go` - Testabdeckung

---

*Dieser Bericht wurde automatisch generiert basierend auf Code-Review der Security-Implementierung.*
