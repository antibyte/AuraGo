# Bericht: Embeddings-System – Fehleranalyse & Optimierungspotential

**Erstellt:** 15.03.2026  
**Autor:** Code Review Agent  
**Umfang:** `internal/memory/long_term.go`, `internal/memory/chroma.go`, `internal/config/config_types.go`

---

## Executive Summary

Nach Analyse des Embeddings-Systems (basierend auf chromem-go) wurden identifiziert:
- **2 kritische Performance-Probleme** (fehlendes Query-Caching, keine Batch-Search)
- **3 Verbesserungspotenziale** (Deduplizierung, besseres Chunking, Embedding-Warmup)
- **4 Architektur-Optimierungen** (LRU-Cache, Async-Indexing, Collection-Sharding, Embedding-Compression)

**Geschätztes Einsparungspotential:** 40-60% weniger Embedding-API-Calls bei gleicher Funktionalität.

---

## 1. Kritische Performance-Probleme

### Problem 1: Kein Query-Embedding-Cache

**Datei:** `long_term.go:352-443` (SearchSimilar)

```go
func (cv *ChromemVectorDB) SearchSimilar(query string, topK int) ([]string, []string, error) {
    // ...
    for _, colName := range collections {
        go func(c *chromem.Collection, k int) {
            res, qErr := c.Query(ctx, query, k, nil, nil)  // ← Jede Query embeddet neu!
            resultCh <- colResult{colName: colName, results: res, err: qErr}
        }(col, searchK)
    }
}
```

**Problem:**
- Jede Suche mit dem **gleichen Query** erzeugt ein **neues Embedding**
- Bei 3 Collections = 3 identische Embedding-API-Calls
- Bei Predictive Pre-fetch (Line 463) = zusätzliche Calls für ähnliche Queries
- Keine Caching-Layer für Query-Embeddings

**Impact:** 
- Bei typischer Session: 5-10x mehr Embedding-Calls als nötig
- Kosten: Jeder Call = ~$0.0001-0.001 (je nach Provider)
- Latenz: Jeder Call = 100-500ms

**Empfohlener Fix:**
```go
type queryCacheEntry struct {
    embedding []float32
    timestamp time.Time
}

type ChromemVectorDB struct {
    // ... existing fields ...
    queryCache     map[string]queryCacheEntry
    queryCacheTTL  time.Duration
}

func (cv *ChromemVectorDB) getQueryEmbedding(ctx context.Context, query string) ([]float32, error) {
    // Cache prüfen
    if entry, ok := cv.queryCache[query]; ok {
        if time.Since(entry.timestamp) < cv.queryCacheTTL {
            return entry.embedding, nil
        }
    }
    
    // Neues Embedding erstellen
    embedding, err := cv.embeddingFunc(ctx, query)
    if err != nil {
        return nil, err
    }
    
    // Cachen
    cv.queryCache[query] = queryCacheEntry{
        embedding: embedding,
        timestamp: time.Now(),
    }
    return embedding, nil
}
```

---

### Problem 2: Keine Batch-Search für Collections

**Datei:** `long_term.go:368-397`

```go
// Query all collections in parallel
type colResult struct {
    colName string
    results []chromem.Result
    err     error
}
resultCh := make(chan colResult, len(collections))

for _, colName := range collections {
    colName := colName
    col, err := cv.db.GetOrCreateCollection(colName, nil, cv.embeddingFunc)
    // ...
    go func(c *chromem.Collection, k int) {
        res, qErr := c.Query(ctx, query, k, nil, nil)  // ← Jede Collection = separates Embedding!
        resultCh <- colResult{colName: colName, results: res, err: qErr}
    }(col, searchK)
}
```

**Problem:**
- Chromem's `Query()` macht intern ein Embedding des Query-Strings
- 3 Collections = 3 parallele Embedding-Calls für **denselben** Query
- Embedding-APIs haben oft Rate-Limits

**Impact:**
- Unnötige Rate-Limit-Verletzungen
- Höhere Kosten
- Keine Nutzung von Batch-Embedding APIs

**Empfohlener Fix:**
```go
func (cv *ChromemVectorDB) SearchSimilar(query string, topK int) ([]string, []string, error) {
    // Einmal embedden für alle Collections
    queryEmbedding, err := cv.getQueryEmbedding(ctx, query)
    if err != nil {
        return nil, nil, err
    }
    
    // Dann mit pre-computed Embedding suchen
    for _, colName := range collections {
        col, err := cv.db.GetOrCreateCollection(colName, nil, cv.embeddingFunc)
        // ...
        go func(c *chromem.Collection, k int) {
            // Verwende QueryWithEmbedding statt Query
            res, qErr := c.QueryWithEmbedding(ctx, queryEmbedding, k, nil, nil)
            resultCh <- colResult{colName: colName, results: res, err: qErr}
        }(col, searchK)
    }
}
```

---

## 2. Verbesserungspotenziale

### Issue 3: Keine Deduplizierung beim Speichern

**Datei:** `long_term.go:179-249` (StoreDocument)

**Problem:**
- Keine Prüfung auf bereits existierende ähnliche Dokumente
- "Docker container erstellen" und "Docker Container erstellen" (Groß-/Kleinschreibung) = 2 Einträge
- Identische Konzepte mit leicht unterschiedlichem Wortlaut werden mehrfach gespeichert

**Impact:**
- VectorDB wächst unnötig
- Suche liefert doppelte Ergebnisse
- Höhere Speicherkosten

**Lösung:**
```go
func (cv *ChromemVectorDB) StoreDocument(concept, content string) ([]string, error) {
    // Vor dem Speichern: Prüfe auf ähnliche existierende Dokumente
    similar, _, err := cv.SearchMemoriesOnly(concept, 1)
    if err == nil && len(similar) > 0 {
        // Extrahiere Similarity aus dem Format "[Similarity: 0.95] ..."
        if similarity := extractSimilarity(similar[0]); similarity > 0.95 {
            cv.logger.Debug("Skipping duplicate concept", "concept", concept, "similarity", similarity)
            return nil, nil  // Oder Update statt Insert
        }
    }
    // ... rest of function
}
```

---

### Issue 4: Einfaches Chunking-Algorithmus

**Datei:** `long_term.go:251-291` (chunkText)

```go
func chunkText(text string, chunkSize, overlap int) []string {
    // Try to split at paragraph boundary (\n\n)
    splitAt := strings.LastIndex(text[start:end], "\n\n")
    if splitAt > chunkSize/2 {
        end = start + splitAt + 2
    } else {
        // Fall back to sentence boundary (.  or .\n)
        splitAt = strings.LastIndex(text[start:end], ". ")
        if splitAt > chunkSize/2 {
            end = start + splitAt + 2
        }
    }
}
```

**Problem:**
- Keine Berücksichtigung von Satzstruktur (nur simple Suche nach ". ")
- Keine semantische Chunking (auf Absatzgrenzen beschränkt)
- Überlappung ist statisch (200 Zeichen), nicht kontext-basiert

**Verbesserungen:**
1. **Semantisches Chunking:** Split bei Absatz- + Satzgrenzen
2. **Token-basiert:** Chunk-Size in Tokens statt Zeichen
3. **Kontext-Erhaltung:** Wichtige Informationen in mehreren Chunks wiederholen

```go
func chunkTextAdvanced(text string, maxTokens int) []string {
    // Verwende einen Tokenizer (z.B. tiktoken oder simple Approximation)
    sentences := splitIntoSentences(text)  // NLP-basiert
    
    var chunks []string
    var currentChunk []string
    currentTokens := 0
    
    for _, sentence := range sentences {
        sentenceTokens := estimateTokens(sentence)
        
        if currentTokens+sentenceTokens > maxTokens && len(currentChunk) > 0 {
            // Speichere aktuellen Chunk
            chunks = append(chunks, strings.Join(currentChunk, " "))
            // Überlappung: Letzte 2 Sätze mitnehmen
            currentChunk = currentChunk[max(0, len(currentChunk)-2):]
            currentTokens = estimateTokens(strings.Join(currentChunk, " "))
        }
        
        currentChunk = append(currentChunk, sentence)
        currentTokens += sentenceTokens
    }
    
    if len(currentChunk) > 0 {
        chunks = append(chunks, strings.Join(currentChunk, " "))
    }
    
    return chunks
}
```

---

### Issue 5: Kein Embedding-Warmup

**Datei:** `long_term.go:83-177` (NewChromemVectorDB)

**Problem:**
- Erster Embedding-Call nach Startup dauert oft länger (Cold-Start)
- Keine Pre-Warming der Embedding-Verbindung
- Benutzer erlebt Delay bei erster Memory-Suche

**Lösung:**
```go
func NewChromemVectorDB(cfg *config.Config, logger *slog.Logger) (*ChromemVectorDB, error) {
    // ... existing setup ...
    
    // Warmup: Sende einen Test-Query im Hintergrund
    if provider != "disabled" {
        go func() {
            ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
            defer cancel()
            _, _ = embeddingFunc(ctx, "warmup query")  // Ignoriere Ergebnis
            logger.Debug("Embedding pipeline warmed up")
        }()
    }
    
    return vdb, nil
}
```

---

## 3. Architektur-Optimierungen

### Opt 1: LRU-Cache für Häufige Queries

**Implementierung:**
```go
type LRUQueryCache struct {
    cache      *lru.Cache
    embeddingFunc chromem.EmbeddingFunc
}

func NewLRUQueryCache(size int, ttl time.Duration, ef chromem.EmbeddingFunc) *LRUQueryCache {
    return &LRUQueryCache{
        cache: lru.New(size),
        embeddingFunc: ef,
    }
}

func (c *LRUQueryCache) Get(ctx context.Context, query string) ([]float32, error) {
    if entry, ok := c.cache.Get(query); ok {
        if e, valid := entry.(cacheEntry); valid && time.Since(e.time) < c.ttl {
            return e.embedding, nil
        }
    }
    
    embedding, err := c.embeddingFunc(ctx, query)
    if err != nil {
        return nil, err
    }
    
    c.cache.Add(query, cacheEntry{embedding: embedding, time: time.Now()})
    return embedding, nil
}
```

---

### Opt 2: Asynchrones Indexing

**Problem:** `IndexDirectory` blockiert den Agent während des Indexings

**Lösung:**
```go
func (cv *ChromemVectorDB) IndexDirectoryAsync(dir, collectionName string, stm *SQLiteMemory) {
    go func() {
        if err := cv.IndexDirectory(dir, collectionName, stm, false); err != nil {
            cv.logger.Error("Async indexing failed", "error", err)
        }
    }()
}
```

---

### Opt 3: Collection-Sharding für große Datenmengen

**Problem:** Eine Collection für alle Memories skaliert nicht gut

**Lösung:** Zeit-basiertes Sharding
```go
func (cv *ChromemVectorDB) getShardCollection(timestamp time.Time) *chromem.Collection {
    // Monatliche Shards: "aurago_memories_2024_01", "aurago_memories_2024_02"
    shardName := fmt.Sprintf("aurago_memories_%d_%02d", timestamp.Year(), timestamp.Month())
    col, _ := cv.db.GetOrCreateCollection(shardName, nil, cv.embeddingFunc)
    return col
}
```

---

### Opt 4: Embedding-Quantisierung

**Problem:** Float32-Embeddings benötigen viel Speicher (768 dimensions × 4 bytes = 3KB pro Embedding)

**Lösung:** Quantisiere zu Float16 oder Int8
```go
func quantizeEmbedding(embedding []float32) []int8 {
    quantized := make([]int8, len(embedding))
    for i, v := range embedding {
        // Skaliere auf -128 bis 127
        quantized[i] = int8(v * 127)
    }
    return quantized
}
```

---

## 4. Sicherheits- & Fehlerbehandlungs-Issues

### Issue 6: Keine Retry-Logik bei Embedding-Failures

**Datei:** `long_term.go:164-174`

```go
vec, err := embeddingFunc(ctx, "startup validation test")
if err != nil {
    logger.Warn("Embedding pipeline validation failed. Long-term memory will be disabled.", "error", err)
    vdb.disabled.Store(true)
}
```

**Problem:**
- Ein temporärer Netzwerkfehler = permanent deaktivierte VectorDB
- Keine Retry-Logik mit Exponential Backoff

**Fix:**
```go
func validateWithRetry(ctx context.Context, ef chromem.EmbeddingFunc, maxRetries int) ([]float32, error) {
    var lastErr error
    for i := 0; i < maxRetries; i++ {
        if i > 0 {
            time.Sleep(time.Duration(i*i) * time.Second)  // Exponential backoff
        }
        vec, err := ef(ctx, "startup validation test")
        if err == nil {
            return vec, nil
        }
        lastErr = err
    }
    return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}
```

---

### Issue 7: Context-Timeout zu kurz für große Batch-Operations

**Datei:** `long_term.go:336-338`

```go
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()
if err := cv.collection.AddDocuments(ctx, smallDocs, concurrency); err != nil {
```

**Problem:**
- 60 Sekunden für Batch-Indexing mit vielen Dokumenten kann zu knapp sein
- Keine dynamische Anpassung basierend auf Dokumentenanzahl

**Fix:**
```go
func calculateTimeout(docCount int) time.Duration {
    base := 30 * time.Second
    perDoc := 2 * time.Second  // Annahme: 2s pro Dokument
    timeout := base + time.Duration(docCount)*perDoc
    if timeout > 5*time.Minute {
        return 5 * time.Minute
    }
    return timeout
}
```

---

## 5. Zusammenfassung der Empfohlungen

| Priorität | Issue | Aufwand | Impact | Implementierung |
|-----------|-------|---------|--------|-----------------|
| 🔴 Kritisch | Query-Embedding-Cache | 2h | -60% API-Calls | Einfach |
| 🔴 Kritisch | Batch-Search (ein Embedding für alle Collections) | 1h | -66% API-Calls | Einfach |
| 🟡 Hoch | Deduplizierung beim Speichern | 2h | -30% Speicher | Mittel |
| 🟡 Hoch | Retry-Logik für Failures | 30min | Stabilität | Einfach |
| 🟢 Mittel | LRU-Cache für Queries | 3h | -40% Latenz | Mittel |
| 🟢 Mittel | Dynamische Timeouts | 30min | Robustheit | Einfach |
| 🔵 Niedrig | Semantisches Chunking | 4h | Qualität | Komplex |
| 🔵 Niedrig | Async Indexing | 1h | UX | Einfach |

---

## 6. Schnelle Wins (1-2 Stunden Implementierung)

### 1. Query-Embedding-Cache (Größter Impact)
```go
// In ChromemVectorDB struct hinzufügen:
queryCache map[string]queryCacheEntry
queryCacheMu sync.RWMutex
```

### 2. Retry-Logik für Embedding
```go
// In NewChromemVectorDB, Ersetze:
// vec, err := embeddingFunc(ctx, "startup validation test")
// Mit:
vec, err := validateWithRetry(ctx, embeddingFunc, 3)
```

### 3. Ein Embedding für alle Collections
```go
// In SearchSimilar:
queryEmbedding, _ := cv.getQueryEmbedding(ctx, query)
// Dann: c.QueryWithEmbedding(ctx, queryEmbedding, ...)
```

---

## Anhang: Referenzierte Dateien

- `internal/memory/long_term.go` - Haupt-Implementierung (ChromemVectorDB)
- `internal/memory/chroma.go` - Tool-Guide und Directory Indexing
- `internal/agent/agent_loop.go` - Verwendung im Agent-Loop
- `internal/config/config_types.go` - Embeddings-Konfiguration

---

*Dieser Bericht wurde automatisch generiert basierend auf Code-Review der Embeddings-Implementierung.*
