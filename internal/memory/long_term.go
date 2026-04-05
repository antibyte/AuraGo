package memory

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"

	chromem "github.com/philippgille/chromem-go"
	"golang.org/x/sync/singleflight"
)

// ArchiveItem represents a single concept/content pair for batch archiving.
type ArchiveItem struct {
	Concept string `json:"concept"`
	Content string `json:"content"`
	Domain  string `json:"domain,omitempty"` // Phase C: optional domain tag for cross-domain learning
}

// VectorDB represents a generic vector database for long term storage
type VectorDB interface {
	StoreDocument(concept, content string) ([]string, error)
	StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error)
	StoreBatch(items []ArchiveItem) ([]string, error)
	SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error)
	// SearchMemoriesOnly searches only the aurago_memories collection.
	// Use this for lightweight lookups (e.g. predictive pre-fetch) that
	// should not pay the cost of scanning tool_guides and documentation.
	SearchMemoriesOnly(query string, topK int) ([]string, []string, error)
	GetByID(id string) (string, error)
	DeleteDocument(id string) error
	Count() int
	IsDisabled() bool
	Close() error
}

// queryCacheEntry stores a pre-computed query embedding with a timestamp for TTL expiry.
type queryCacheEntry struct {
	embedding []float32
	timestamp time.Time
}

// ChromemVectorDB implements VectorDB using chromem-go with persistence.
type ChromemVectorDB struct {
	db            *chromem.DB
	dataDir       string // persistent directory for the vector DB (used for version files)
	collection    *chromem.Collection
	logger        *slog.Logger
	mu            sync.RWMutex // Protects indexing operations; reads use RLock
	storeDocMu    sync.Mutex   // Serialises the dedup-check+store sequence in StoreDocumentWithDomain
	embeddingFunc chromem.EmbeddingFunc
	disabled      atomic.Bool  // Set when embedding pipeline fails; skips operations gracefully
	idCounter     atomic.Int64 // Monotonic counter for collision-free document IDs
	queryCache    map[string]queryCacheEntry
	queryCacheMu  sync.RWMutex
	queryCacheTTL time.Duration
	indexing      atomic.Int32       // Counter: >0 while async indexing is in progress
	dedupSem      chan struct{}      // semaphore to limit concurrent dedup checks
	sfGroup       singleflight.Group // deduplicates concurrent embedding API calls for the same query
}

func (cv *ChromemVectorDB) Close() error {
	// chromem-go v0.7.0 uses synchronous file-based persistence with no open file handles
	// or background resources that require cleanup. Each write operation persists immediately.
	// The DB struct holds only in-memory maps which are garbage-collected when references
	// are dropped. This no-op satisfies the VectorDB interface.
	cv.logger.Debug("Closing VectorDB (chromem-go uses synchronous writes, no cleanup needed)")
	return nil
}

// GetDB returns the underlying chromem.DB so other subsystems (e.g. KnowledgeGraph
// semantic index) can share the same open database handle instead of opening a second one.
func (cv *ChromemVectorDB) GetDB() *chromem.DB {
	return cv.db
}

// GetEmbeddingFunc returns the embedding function used by this VectorDB instance,
// allowing it to be shared with the KnowledgeGraph semantic index.
func (cv *ChromemVectorDB) GetEmbeddingFunc() chromem.EmbeddingFunc {
	return cv.embeddingFunc
}

// Count returns the total number of documents across all collections
// (aurago_memories, tool_guides, documentation).
// Returns the persisted count even when the embedding pipeline is disabled,
// because counting does not require embeddings.
func (cv *ChromemVectorDB) Count() int {
	cv.mu.RLock()
	defer cv.mu.RUnlock()
	total := cv.collection.Count() // aurago_memories

	// Include secondary collections that were indexed at startup
	for _, name := range []string{"tool_guides", "documentation"} {
		col, err := cv.db.GetOrCreateCollection(name, nil, cv.embeddingFunc)
		if err == nil {
			total += col.Count()
		}
	}
	return total
}

// IsDisabled reports whether the embedding pipeline failed at startup.
// When true, new Store/Search operations will fail, but existing documents
// are still persisted and countable.
func (cv *ChromemVectorDB) IsDisabled() bool {
	return cv.disabled.Load()
}

// NewChromemVectorDB creates a new persistent Vector DB backed by chromem-go.
// It selects the embedding function based on the config:
//   - "internal": uses the main LLM provider's API (e.g., OpenRouter) for embeddings
//   - "external": uses a dedicated embedding endpoint (e.g., local Ollama)
func NewChromemVectorDB(cfg *config.Config, logger *slog.Logger) (*ChromemVectorDB, error) {
	db, err := chromem.NewPersistentDB(cfg.Directories.VectorDBDir, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create persistent vector DB: %w", err)
	}
	dataDir := cfg.Directories.VectorDBDir

	// Dynamic embedding function factory using chromem-go's native constructors
	var embeddingFunc chromem.EmbeddingFunc
	provider := cfg.Embeddings.Provider

	if provider == "disabled" || provider == "" {
		// Explicit opt-out via config — use a no-op func; disabled flag is set below.
		embeddingFunc = func(_ context.Context, _ string) ([]float32, error) {
			return nil, fmt.Errorf("embeddings are disabled")
		}
		logger.Info("VectorDB: embeddings disabled by configuration")
		provider = "disabled" // normalise for the check below
	} else {
		// Provider entry resolved by config.ResolveProviders — use resolved fields directly.
		// Fallback: if legacy "internal"/"external" values are still present (no migration),
		// handle them for backward compat.
		embedURL := cfg.Embeddings.BaseURL
		embedKey := cfg.Embeddings.APIKey
		embedModel := cfg.Embeddings.Model

		// Legacy compat: "internal" always uses the main LLM endpoint + credentials.
		// The embeddings.api_key field is irrelevant in this mode — always override
		// so a stale/dummy key never blocks the embedding pipeline.
		if provider == "internal" {
			embedURL = cfg.LLM.BaseURL
			embedKey = cfg.LLM.APIKey
			if embedModel == "" {
				embedModel = cfg.Embeddings.InternalModel
			}
		}
		// Legacy compat: "external" uses dedicated fields
		if provider == "external" {
			if embedURL == "" {
				embedURL = cfg.Embeddings.ExternalURL
			}
			if embedModel == "" {
				embedModel = cfg.Embeddings.ExternalModel
			}
		}

		if embedModel == "" {
			embedModel = "text-embedding-3-small"
		}

		embeddingFunc = chromem.NewEmbeddingFuncOpenAICompat(
			embedURL,
			embedKey,
			embedModel,
			nil, // Auto-detect normalization
		)
		logger.Info("VectorDB using embeddings provider", "provider", provider, "url", embedURL, "model", embedModel)
	}

	collection, err := db.GetOrCreateCollection("aurago_memories", nil, embeddingFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create collection: %w", err)
	}

	vdb := &ChromemVectorDB{
		db:            db,
		dataDir:       dataDir,
		collection:    collection,
		logger:        logger,
		embeddingFunc: embeddingFunc,
		queryCache:    make(map[string]queryCacheEntry),
		queryCacheTTL: 5 * time.Minute,
		dedupSem:      make(chan struct{}, 4), // max 4 concurrent dedup checks to avoid rate limits
	}

	// Phase 29: Startup validation — test the embedding pipeline with retries
	if provider == "disabled" {
		vdb.disabled.Store(true)
		logger.Info("VectorDB disabled by configuration, skipping embedding validation")
	} else {
		logger.Info("Validating embedding pipeline (with retries)...")
		validationStart := time.Now()
		vec, err := validateEmbeddingWithRetry(embeddingFunc, 3, logger)
		if err != nil {
			logger.Warn("Embedding pipeline validation failed after retries. Long-term memory will be disabled.", "error", err)
			vdb.disabled.Store(true)
		} else {
			latency := time.Since(validationStart)
			logger.Info("Embedding pipeline validated", "vector_dimensions", len(vec), "provider", provider, "docs", collection.Count(), "latency", latency)
			if latency > 500*time.Millisecond {
				logger.Warn("Local embeddings are slow. Consider enabling GPU passthrough (use_host_gpu) or using a cloud provider.", "latency", latency)
			}
		}
	}

	return vdb, nil
}

// validateEmbeddingWithRetry attempts to validate the embedding pipeline up to maxRetries times
// with exponential backoff (1s, 4s, 9s). Returns the embedding vector on success.
func validateEmbeddingWithRetry(ef chromem.EmbeddingFunc, maxRetries int, logger *slog.Logger) ([]float32, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			backoff := time.Duration(i*i) * time.Second
			logger.Info("Retrying embedding validation...", "attempt", i+1, "backoff", backoff)
			time.Sleep(backoff)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		vec, err := ef(ctx, "startup validation test")
		cancel()
		if err == nil {
			return vec, nil
		}
		lastErr = err
		logger.Warn("Embedding validation attempt failed", "attempt", i+1, "error", err)
	}
	return nil, fmt.Errorf("embedding validation failed after %d attempts: %w", maxRetries, lastErr)
}

// StoreDocument stores a concept/content pair, auto-chunking large texts.
// Returns the list of stored document IDs.
func (cv *ChromemVectorDB) StoreDocument(concept, content string) ([]string, error) {
	return cv.StoreDocumentWithDomain(concept, content, "")
}

// StoreDocumentWithDomain stores a concept/content pair with an optional domain tag
// for cross-domain learning (Phase C). The domain helps categorize knowledge.
// Deduplication: skips storage if a very similar document already exists (similarity > 0.95).
func (cv *ChromemVectorDB) StoreDocumentWithDomain(concept, content, domain string) ([]string, error) {
	if cv.disabled.Load() {
		return nil, fmt.Errorf("VectorDB is disabled (embedding pipeline failed at startup)")
	}

	const maxContentBytes = 500 * 1024 // 500 KB per document
	if len(content) > maxContentBytes {
		cv.logger.Warn("Document content exceeds 500 KB limit, truncating", "concept", concept, "bytes", len(content))
		content = content[:maxContentBytes]
	}

	// Deduplication: serialise check+store so two concurrent calls for the
	// same concept cannot both pass the similarity gate before either is stored.
	cv.storeDocMu.Lock()
	if sim := cv.searchTopSimilarityScore(concept); sim > 0.95 {
		cv.storeDocMu.Unlock()
		cv.logger.Debug("Skipping duplicate concept (similarity > 0.95)", "concept", concept, "similarity", sim)
		return nil, nil
	}
	defer cv.storeDocMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullContent := concept + "\n\n" + content

	metadata := map[string]string{
		"concept":   concept,
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	}
	if domain != "" {
		metadata["domain"] = domain
	}

	// Small texts: store as a single document
	if len(fullContent) <= 4000 {
		docID := fmt.Sprintf("mem_%d_%d", time.Now().UnixMilli(), cv.idCounter.Add(1))
		doc := chromem.Document{
			ID:       docID,
			Metadata: metadata,
			Content:  fullContent,
		}
		if err := cv.collection.AddDocument(ctx, doc); err != nil {
			cv.logger.Error("Failed to store document in vector DB", "error", err)
			return nil, fmt.Errorf("failed to add document: %w", err)
		}
		cv.logger.Info("Stored document in long-term memory", "id", docID, "concept", concept, "domain", domain)
		return []string{docID}, nil
	}

	// Large texts: split into chunks and batch-store
	const maxChunks = 200
	chunks := chunkText(content, 3500, 200)
	if len(chunks) > maxChunks {
		cv.logger.Warn("Document produces too many chunks, capping", "concept", concept, "chunks", len(chunks), "max", maxChunks)
		chunks = chunks[:maxChunks]
	}
	baseCounter := cv.idCounter.Add(int64(len(chunks)))

	var docs []chromem.Document
	var storedIDs []string
	for i, chunk := range chunks {
		docID := fmt.Sprintf("mem_%d_%d_chunk_%d", time.Now().UnixMilli(), baseCounter-int64(len(chunks))+int64(i)+1, i)
		chunkMeta := map[string]string{
			"concept":     concept,
			"chunk_index": fmt.Sprintf("%d/%d", i+1, len(chunks)),
			"timestamp":   fmt.Sprintf("%d", time.Now().Unix()),
		}
		if domain != "" {
			chunkMeta["domain"] = domain
		}
		docs = append(docs, chromem.Document{
			ID:       docID,
			Metadata: chunkMeta,
			Content:  concept + "\n\n" + chunk,
		})
		storedIDs = append(storedIDs, docID)
	}

	// Batch-add all chunks in one call (sequential embedding to avoid rate limits)
	chunkCtx, chunkCancel := context.WithTimeout(context.Background(), calculateBatchTimeout(len(docs)))
	defer chunkCancel()
	if err := cv.collection.AddDocuments(chunkCtx, docs, 1); err != nil {
		cv.logger.Error("Failed to store chunked document", "error", err, "chunks", len(chunks))
		return nil, fmt.Errorf("failed to add chunked document (%d chunks): %w", len(chunks), err)
	}

	cv.logger.Info("Stored chunked document in long-term memory", "concept", concept, "domain", domain, "chunks", len(chunks), "total_chars", len(content))
	return storedIDs, nil
}

// StoreDocumentWithEmbedding stores a document with a pre-computed embedding vector.
// This bypasses the text embedding function, allowing multimodal content (images, audio)
// to be stored with externally computed embeddings.
func (cv *ChromemVectorDB) StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error) {
	if cv.disabled.Load() {
		return "", fmt.Errorf("VectorDB is disabled (embedding pipeline failed at startup)")
	}
	if len(embedding) == 0 {
		return "", fmt.Errorf("embedding vector is empty")
	}

	cv.mu.Lock()
	defer cv.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	docID := fmt.Sprintf("mm_%d_%d", time.Now().UnixMilli(), cv.idCounter.Add(1))
	doc := chromem.Document{
		ID: docID,
		Metadata: map[string]string{
			"concept":    concept,
			"timestamp":  fmt.Sprintf("%d", time.Now().Unix()),
			"multimodal": "true",
		},
		Content:   content,
		Embedding: embedding,
	}

	if err := cv.collection.AddDocument(ctx, doc); err != nil {
		cv.logger.Error("Failed to store multimodal document", "error", err, "concept", concept)
		return "", fmt.Errorf("failed to add multimodal document: %w", err)
	}

	cv.logger.Info("Stored multimodal document", "id", docID, "concept", concept)
	return docID, nil
}

// chunkText splits a large text into smaller segments of roughly chunkSize characters,
// preferring paragraph (\n\n) or sentence boundaries. Adds overlap characters between chunks.
func chunkText(text string, chunkSize, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if chunkSize <= 0 {
		chunkSize = len(text)
		if chunkSize == 0 {
			return nil
		}
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 4
	}
	if len(text) <= chunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0

	for start < len(text) {
		end := start + chunkSize
		if end >= len(text) {
			if chunk := strings.TrimSpace(text[start:]); chunk != "" {
				chunks = append(chunks, chunk)
			}
			break
		}

		// Try to split at paragraph boundary (\n\n)
		splitAt := strings.LastIndex(text[start:end], "\n\n")
		if splitAt > chunkSize/2 {
			end = start + splitAt + 2 // include the double newline
		} else {
			// Fall back to sentence boundary (.  or .\n)
			splitAt = strings.LastIndex(text[start:end], ". ")
			if splitAt > chunkSize/2 {
				end = start + splitAt + 2
			}
			// else: hard cut at chunkSize
		}

		if chunk := strings.TrimSpace(text[start:end]); chunk != "" {
			chunks = append(chunks, chunk)
		}

		// Move forward with overlap
		start = end - overlap
		if start < 0 {
			start = 0
		}
	}

	return chunks
}

// StoreBatch stores multiple concept/content pairs. Small documents are collected
// and batch-embedded in a single parallel call; large texts are chunked individually.
// Performs deduplication by skipping items whose concept is already stored (similarity > 0.95).
func (cv *ChromemVectorDB) StoreBatch(items []ArchiveItem) ([]string, error) {
	if cv.disabled.Load() {
		return nil, fmt.Errorf("VectorDB is disabled (embedding pipeline failed at startup)")
	}

	const maxBatchItemBytes = 500 * 1024 // 500 KB per item

	// Deduplication: run concept-similarity checks with bounded concurrency to avoid rate limiting.
	type dedupResult struct {
		idx  int
		item ArchiveItem
		keep bool
	}
	resultsCh := make(chan dedupResult, len(items))
	for i, item := range items {
		if len(item.Content) > maxBatchItemBytes {
			cv.logger.Warn("StoreBatch: item content exceeds 500 KB limit, truncating", "concept", item.Concept, "bytes", len(item.Content))
			item.Content = item.Content[:maxBatchItemBytes]
		}
		i, item := i, item // capture
		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					cv.logger.Error("StoreBatch: dedup goroutine panicked", "panic", rec)
					resultsCh <- dedupResult{idx: i, item: item, keep: true}
				}
			}()
			// Acquire semaphore to limit concurrent embedding calls
			cv.dedupSem <- struct{}{}
			defer func() { <-cv.dedupSem }()

			keep := true
			if sim := cv.searchTopSimilarityScore(item.Concept); sim > 0.95 {
				cv.logger.Debug("StoreBatch: skipping duplicate concept", "concept", item.Concept, "similarity", sim)
				keep = false
			}
			resultsCh <- dedupResult{idx: i, item: item, keep: keep}
		}()
	}
	// Collect results preserving original order
	type indexedItem struct {
		idx  int
		item ArchiveItem
	}
	kept := make([]indexedItem, 0, len(items))
	for range items {
		r := <-resultsCh
		if r.keep {
			kept = append(kept, indexedItem{r.idx, r.item})
		}
	}
	sort.Slice(kept, func(a, b int) bool { return kept[a].idx < kept[b].idx })
	uniqueItems := make([]ArchiveItem, 0, len(kept))
	for _, k := range kept {
		uniqueItems = append(uniqueItems, k.item)
	}

	if len(uniqueItems) == 0 {
		return nil, nil
	}

	var allIDs []string
	var smallDocs []chromem.Document
	var smallIDs []string

	for _, item := range uniqueItems {
		fullContent := item.Concept + "\n\n" + item.Content
		metadata := map[string]string{
			"concept":   item.Concept,
			"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
		}
		if item.Domain != "" {
			metadata["domain"] = item.Domain
		}

		if len(fullContent) <= 4000 {
			// Collect small docs for batch embedding
			docID := fmt.Sprintf("mem_%d_%d", time.Now().UnixMilli(), cv.idCounter.Add(1))
			smallDocs = append(smallDocs, chromem.Document{
				ID:       docID,
				Metadata: metadata,
				Content:  fullContent,
			})
			smallIDs = append(smallIDs, docID)
		} else {
			// Large items need chunking — delegate to single-item store
			ids, err := cv.StoreDocumentWithDomain(item.Concept, item.Content, item.Domain)
			if err != nil {
				cv.logger.Error("Failed to store large batch item", "concept", item.Concept, "error", err)
				return allIDs, fmt.Errorf("failed to store batch item %q: %w", item.Concept, err)
			}
			allIDs = append(allIDs, ids...)
		}
	}

	// Batch-add all small documents in one parallel call
	if len(smallDocs) > 0 {
		concurrency := len(smallDocs)
		if concurrency > 4 {
			concurrency = 4
		}
		ctx, cancel := context.WithTimeout(context.Background(), calculateBatchTimeout(len(smallDocs)))
		defer cancel()
		if err := cv.collection.AddDocuments(ctx, smallDocs, concurrency); err != nil {
			cv.logger.Error("Failed to batch-add documents", "error", err, "count", len(smallDocs))
			return allIDs, fmt.Errorf("failed to batch-add %d documents: %w", len(smallDocs), err)
		}
		allIDs = append(allIDs, smallIDs...)
	}

	cv.logger.Info("Stored batch in long-term memory", "count", len(items), "total_docs", len(allIDs), "batched_small", len(smallDocs))
	return allIDs, nil
}

// SearchSimilar finds the topK most semantically similar documents across all relevant collections.
// Results from all collections are merged, sorted by similarity, and trimmed to topK globally.
// Uses a query embedding cache to avoid redundant embedding API calls.
func (cv *ChromemVectorDB) SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error) {
	if cv.disabled.Load() {
		return nil, nil, nil // Graceful degradation: return empty results
	}

	cv.mu.RLock()
	defer cv.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Compute query embedding once and reuse across all collections
	queryEmbedding, err := cv.getQueryEmbedding(ctx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compute query embedding: %w", err)
	}

	allCollections := []string{"aurago_memories", "tool_guides", "documentation"}
	var collections []string
	if len(excludeCollections) > 0 {
		excludeSet := make(map[string]bool, len(excludeCollections))
		for _, e := range excludeCollections {
			excludeSet[e] = true
		}
		for _, c := range allCollections {
			if !excludeSet[c] {
				collections = append(collections, c)
			}
		}
	} else {
		collections = allCollections
	}

	type rankedResult struct {
		text       string
		docID      string
		similarity float32
	}

	// Query all collections in parallel using pre-computed embedding.
	type colResult struct {
		colName string
		results []chromem.Result
		err     error
	}
	resultCh := make(chan colResult, len(collections))

	for _, colName := range collections {
		colName := colName // capture
		col, err := cv.db.GetOrCreateCollection(colName, nil, cv.embeddingFunc)
		if err != nil {
			resultCh <- colResult{colName: colName, err: err}
			continue
		}
		if col.Count() == 0 {
			cv.logger.Debug("Collection empty, skipping search", "collection", colName)
			resultCh <- colResult{colName: colName}
			continue
		}
		cv.logger.Info("Searching collection", "collection", colName, "docs", col.Count())
		searchK := topK
		if searchK > col.Count() {
			searchK = col.Count()
		}
		go func(c *chromem.Collection, k int) {
			res, qErr := c.QueryEmbedding(ctx, queryEmbedding, k, nil, nil)
			resultCh <- colResult{colName: colName, results: res, err: qErr}
		}(col, searchK)
	}

	var allResults []rankedResult
	for range collections {
		var cr colResult
		select {
		case cr = <-resultCh:
		case <-ctx.Done():
			cv.logger.Warn("SearchSimilar: context deadline exceeded, returning partial results", "collected", len(allResults))
			goto finalizeResults
		}
		if cr.err != nil {
			cv.logger.Warn("Failed to query collection", "collection", cr.colName, "error", cr.err)
			continue
		}
		for _, result := range cr.results {
			sim := result.Similarity
			if tsStr, ok := result.Metadata["timestamp"]; ok {
				if ts, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
					ageDays := float32(time.Since(time.Unix(ts, 0)).Hours() / 24.0)
					if ageDays > 0 {
						// Gentle decay: -1% per day, max -30%
						decay := ageDays * 0.01
						if decay > 0.30 {
							decay = 0.30
						}
						sim = sim * (1.0 - decay)
					}
				}
			}

			if sim > 0.3 {
				domainHint := ""
				if d, ok := result.Metadata["domain"]; ok && d != "" {
					domainHint = fmt.Sprintf(" [Domain: %s]", d)
				}
				// Tag documentation/tools results for the LLM
				if cr.colName != "aurago_memories" {
					domainHint = fmt.Sprintf(" [%s]", cr.colName)
				}

				cv.logger.Debug("Retrieved memory", "collection", cr.colName, "id", result.ID, "raw_sim", result.Similarity, "decayed_sim", sim)

				formattedText := result.Content
				if domainHint != "" {
					formattedText = domainHint + " " + result.Content
				}

				allResults = append(allResults, rankedResult{
					text:       formattedText,
					docID:      result.ID,
					similarity: sim,
				})
			}
		}
	}

finalizeResults:

	// Sort by similarity descending and enforce global topK limit
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].similarity > allResults[j].similarity
	})
	if len(allResults) > topK {
		allResults = allResults[:topK]
	}

	var allMemories []string
	var allDocIDs []string
	for _, r := range allResults {
		allMemories = append(allMemories, r.text)
		allDocIDs = append(allDocIDs, r.docID)
	}

	return allMemories, allDocIDs, nil
}

// SearchMemoriesOnly searches only the aurago_memories collection. Much cheaper than
// SearchSimilar because it skips the tool_guides and documentation collections.
// Intended for use cases like predictive pre-fetch where documentation hits add no value.
// Uses the query embedding cache to avoid redundant API calls.
func (cv *ChromemVectorDB) SearchMemoriesOnly(query string, topK int) ([]string, []string, error) {
	if cv.disabled.Load() {
		return nil, nil, nil
	}

	cv.mu.RLock()
	defer cv.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	col, err := cv.db.GetOrCreateCollection("aurago_memories", nil, cv.embeddingFunc)
	if err != nil || col.Count() == 0 {
		return nil, nil, nil
	}

	searchK := topK
	if searchK > col.Count() {
		searchK = col.Count()
	}

	queryEmbedding, err := cv.getQueryEmbedding(ctx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compute query embedding: %w", err)
	}

	results, err := col.QueryEmbedding(ctx, queryEmbedding, searchK, nil, nil)
	if err != nil {
		return nil, nil, err
	}

	var texts []string
	var ids []string
	for _, r := range results {
		sim := r.Similarity
		if tsStr, ok := r.Metadata["timestamp"]; ok {
			if ts, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
				ageDays := float32(time.Since(time.Unix(ts, 0)).Hours() / 24.0)
				if ageDays > 0 {
					decay := ageDays * 0.01
					if decay > 0.30 {
						decay = 0.30
					}
					sim = sim * (1.0 - decay)
				}
			}
		}

		if sim > 0.3 {
			texts = append(texts, r.Content)
			ids = append(ids, r.ID)
		}
	}
	return texts, ids, nil
}

// GetByID retrieves a document's full content by its ID.
func (cv *ChromemVectorDB) GetByID(id string) (string, error) {
	if cv.disabled.Load() {
		return "", fmt.Errorf("VectorDB is disabled")
	}
	cv.mu.RLock()
	defer cv.mu.RUnlock()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	doc, err := cv.collection.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	return doc.Content, nil
}

// searchTopSimilarityScore returns the decayed similarity score of the closest existing
// document in the aurago_memories collection for the given concept, or 0 if no match.
// It is used internally for dedup checks and does NOT format results with a prefix string,
// unlike SearchSimilar/SearchMemoriesOnly. Callers must not hold storeDocMu when calling
// SearchMemoriesOnly (which this replaces), but since this method creates its own context
// and does not acquire any other lock, it is safe to call while storeDocMu is held.
func (cv *ChromemVectorDB) searchTopSimilarityScore(concept string) float32 {
	if cv.disabled.Load() {
		return 0
	}
	col, err := cv.db.GetOrCreateCollection("aurago_memories", nil, cv.embeddingFunc)
	if err != nil || col.Count() == 0 {
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	queryEmbedding, err := cv.getQueryEmbedding(ctx, concept)
	if err != nil {
		return 0
	}

	results, err := col.QueryEmbedding(ctx, queryEmbedding, 1, nil, nil)
	if err != nil || len(results) == 0 {
		return 0
	}

	sim := results[0].Similarity
	if tsStr, ok := results[0].Metadata["timestamp"]; ok {
		if ts, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
			ageDays := float32(time.Since(time.Unix(ts, 0)).Hours() / 24.0)
			if ageDays > 0 {
				decay := ageDays * 0.01
				if decay > 0.30 {
					decay = 0.30
				}
				sim = sim * (1.0 - decay)
			}
		}
	}
	return sim
}

// DeleteDocument removes a specific document from the VectorDB by its ID.
func (cv *ChromemVectorDB) DeleteDocument(id string) error {
	if cv.disabled.Load() {
		return fmt.Errorf("VectorDB is disabled")
	}
	cv.mu.Lock()
	defer cv.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return cv.collection.Delete(ctx, nil, nil, id)
}

// getQueryEmbedding returns a cached embedding for the query string, or computes a new one.
// The cache uses a TTL to avoid stale embeddings. This saves redundant API calls
// when the same query is used across multiple collections.
func (cv *ChromemVectorDB) getQueryEmbedding(ctx context.Context, query string) ([]float32, error) {
	cv.queryCacheMu.RLock()
	if entry, ok := cv.queryCache[query]; ok && time.Since(entry.timestamp) < cv.queryCacheTTL {
		cv.queryCacheMu.RUnlock()
		return entry.embedding, nil
	}
	cv.queryCacheMu.RUnlock()

	res, err, _ := cv.sfGroup.Do(query, func() (interface{}, error) {
		return cv.embeddingFunc(ctx, query)
	})
	if err != nil {
		return nil, err
	}
	embedding := res.([]float32)

	cv.queryCacheMu.Lock()
	cv.queryCache[query] = queryCacheEntry{embedding: embedding, timestamp: time.Now()}
	// Evict old entries if cache grows too large (> 200 entries).
	// First pass: remove expired entries. If none expired, remove the oldest entry
	// to enforce a hard cap and prevent unbounded growth under unique-query load.
	const queryCacheMaxSize = 200
	if len(cv.queryCache) > queryCacheMaxSize {
		now := time.Now()
		var toDelete []string
		var oldestKey string
		var oldestTime time.Time
		for k, v := range cv.queryCache {
			if now.Sub(v.timestamp) > cv.queryCacheTTL {
				toDelete = append(toDelete, k)
			} else if oldestKey == "" || v.timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.timestamp
			}
		}
		if len(toDelete) > 0 {
			for _, k := range toDelete {
				delete(cv.queryCache, k)
			}
		} else if oldestKey != "" {
			// No expired entries — evict the oldest one to stay under the hard cap.
			delete(cv.queryCache, oldestKey)
		}
	}
	cv.queryCacheMu.Unlock()

	return embedding, nil
}

// ExtractSimilarityScore extracts the similarity value from a formatted search result string.
// Expected format: "[Similarity: 0.95] ..."
// Returns 0 if the format is invalid. Values are clamped to [0.0, 1.0].
func ExtractSimilarityScore(result string) float64 {
	const prefix = "[Similarity: "
	idx := strings.Index(result, prefix)
	if idx < 0 {
		return 0
	}
	start := idx + len(prefix)
	end := strings.Index(result[start:], "]")
	if end < 0 {
		return 0
	}
	val, err := strconv.ParseFloat(result[start:start+end], 64)
	if err != nil {
		return 0
	}
	if val < 0 {
		return 0
	}
	if val > 1 {
		return 1
	}
	return val
}

// calculateBatchTimeout returns a dynamic timeout based on the number of documents.
// Base: 30s + 2s per document, capped at 5 minutes.
func calculateBatchTimeout(docCount int) time.Duration {
	timeout := 30*time.Second + time.Duration(docCount)*2*time.Second
	if timeout > 5*time.Minute {
		return 5 * time.Minute
	}
	return timeout
}

// IsIndexing reports whether async indexing is currently in progress.
func (cv *ChromemVectorDB) IsIndexing() bool {
	return cv.indexing.Load() > 0
}
