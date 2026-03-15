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
	StoreBatch(items []ArchiveItem) ([]string, error)
	SearchSimilar(query string, topK int) ([]string, []string, error)
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
	collection    *chromem.Collection
	logger        *slog.Logger
	mu            sync.RWMutex // Protects indexing operations; reads use RLock
	embeddingFunc chromem.EmbeddingFunc
	disabled      atomic.Bool  // Set when embedding pipeline fails; skips operations gracefully
	idCounter     atomic.Int64 // Monotonic counter for collision-free document IDs
	queryCache    map[string]queryCacheEntry
	queryCacheMu  sync.RWMutex
	queryCacheTTL time.Duration
	indexing      atomic.Bool // True while async indexing is in progress
}

func (cv *ChromemVectorDB) Close() error {
	// Chromem-go's persistent DB doesn't have an explicit Close() method in current versions,
	// but we implement it to satisfy the interface and allow for future cleanup.
	cv.logger.Info("Closing VectorDB (no-op for chromem)")
	return nil
}

// Count returns the total number of documents across all collections
// (aurago_memories, tool_guides, documentation).
// Returns the persisted count even when the embedding pipeline is disabled,
// because counting does not require embeddings.
func (cv *ChromemVectorDB) Count() int {
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

		// Legacy compat: "internal" uses main LLM endpoint + internal_model
		if provider == "internal" {
			if embedURL == "" {
				embedURL = cfg.LLM.BaseURL
			}
			if embedKey == "" {
				embedKey = cfg.LLM.APIKey
			}
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
		collection:    collection,
		logger:        logger,
		embeddingFunc: embeddingFunc,
		queryCache:    make(map[string]queryCacheEntry),
		queryCacheTTL: 5 * time.Minute,
	}

	// Phase 29: Startup validation — test the embedding pipeline with retries
	if provider == "disabled" {
		vdb.disabled.Store(true)
		logger.Info("VectorDB disabled by configuration, skipping embedding validation")
	} else {
		logger.Info("Validating embedding pipeline (with retries)...")
		vec, err := validateEmbeddingWithRetry(embeddingFunc, 3, logger)
		if err != nil {
			logger.Warn("Embedding pipeline validation failed after retries. Long-term memory will be disabled.", "error", err)
			vdb.disabled.Store(true)
		} else {
			logger.Info("Embedding pipeline validated", "vector_dimensions", len(vec), "provider", provider, "docs", collection.Count())
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

	// Deduplication: check if a very similar concept already exists
	if similar, _, err := cv.SearchMemoriesOnly(concept, 1); err == nil && len(similar) > 0 {
		if sim := extractSimilarityScore(similar[0]); sim > 0.95 {
			cv.logger.Debug("Skipping duplicate concept (similarity > 0.95)", "concept", concept, "similarity", sim)
			return nil, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullContent := concept + "\n\n" + content

	metadata := map[string]string{"concept": concept}
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
	chunks := chunkText(content, 3500, 200)
	baseCounter := cv.idCounter.Add(int64(len(chunks)))

	var docs []chromem.Document
	var storedIDs []string
	for i, chunk := range chunks {
		docID := fmt.Sprintf("mem_%d_%d_chunk_%d", time.Now().UnixMilli(), baseCounter-int64(len(chunks))+int64(i)+1, i)
		chunkMeta := map[string]string{
			"concept":     concept,
			"chunk_index": fmt.Sprintf("%d/%d", i+1, len(chunks)),
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

// chunkText splits a large text into smaller segments of roughly chunkSize characters,
// preferring paragraph (\n\n) or sentence boundaries. Adds overlap characters between chunks.
func chunkText(text string, chunkSize, overlap int) []string {
	if len(text) <= chunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0

	for start < len(text) {
		end := start + chunkSize
		if end >= len(text) {
			chunks = append(chunks, strings.TrimSpace(text[start:]))
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

		chunks = append(chunks, strings.TrimSpace(text[start:end]))

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
func (cv *ChromemVectorDB) StoreBatch(items []ArchiveItem) ([]string, error) {
	if cv.disabled.Load() {
		return nil, fmt.Errorf("VectorDB is disabled (embedding pipeline failed at startup)")
	}

	var allIDs []string
	var smallDocs []chromem.Document
	var smallIDs []string

	for _, item := range items {
		fullContent := item.Concept + "\n\n" + item.Content
		metadata := map[string]string{"concept": item.Concept}
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
func (cv *ChromemVectorDB) SearchSimilar(query string, topK int) ([]string, []string, error) {
	if cv.disabled.Load() {
		return nil, nil, nil // Graceful degradation: return empty results
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Compute query embedding once and reuse across all collections
	queryEmbedding, err := cv.getQueryEmbedding(ctx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compute query embedding: %w", err)
	}

	collections := []string{"aurago_memories", "tool_guides", "documentation"}

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
		cr := <-resultCh
		if cr.err != nil {
			cv.logger.Warn("Failed to query collection", "collection", cr.colName, "error", cr.err)
			continue
		}
		for _, result := range cr.results {
			if result.Similarity > 0.3 {
				domainHint := ""
				if d, ok := result.Metadata["domain"]; ok && d != "" {
					domainHint = fmt.Sprintf(" [Domain: %s]", d)
				}
				// Tag documentation/tools results for the LLM
				if cr.colName != "aurago_memories" {
					domainHint = fmt.Sprintf(" [%s]", cr.colName)
				}

				cv.logger.Debug("Retrieved memory", "collection", cr.colName, "id", result.ID, "similarity", result.Similarity)
				allResults = append(allResults, rankedResult{
					text:       fmt.Sprintf("[Similarity: %.2f]%s %s", result.Similarity, domainHint, result.Content),
					docID:      result.ID,
					similarity: result.Similarity,
				})
			}
		}
	}

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
		if r.Similarity > 0.3 {
			texts = append(texts, fmt.Sprintf("[Similarity: %.2f] %s", r.Similarity, r.Content))
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	doc, err := cv.collection.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	return doc.Content, nil
}

// DeleteDocument removes a specific document from the VectorDB by its ID.
func (cv *ChromemVectorDB) DeleteDocument(id string) error {
	if cv.disabled.Load() {
		return fmt.Errorf("VectorDB is disabled")
	}
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

	embedding, err := cv.embeddingFunc(ctx, query)
	if err != nil {
		return nil, err
	}

	cv.queryCacheMu.Lock()
	cv.queryCache[query] = queryCacheEntry{embedding: embedding, timestamp: time.Now()}
	// Evict old entries if cache grows too large (> 200 entries)
	if len(cv.queryCache) > 200 {
		now := time.Now()
		for k, v := range cv.queryCache {
			if now.Sub(v.timestamp) > cv.queryCacheTTL {
				delete(cv.queryCache, k)
			}
		}
	}
	cv.queryCacheMu.Unlock()

	return embedding, nil
}

// extractSimilarityScore extracts the similarity value from a formatted search result string.
// Expected format: "[Similarity: 0.95] ..."
func extractSimilarityScore(result string) float64 {
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
	return cv.indexing.Load()
}
