package memory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
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

// SearchResult preserves the semantic score alongside the text returned to callers.
type SearchResult struct {
	Text       string
	DocID      string
	Similarity float64
}

// VectorDB represents a generic vector database for long term storage
type VectorDB interface {
	StoreDocument(concept, content string) ([]string, error)
	StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error)
	// StoreDocumentInCollection stores a document in a specific collection (used by FileIndexer).
	StoreDocumentInCollection(concept, content, collection string) ([]string, error)
	// StoreDocumentWithEmbeddingInCollection stores a document with pre-computed embedding in a specific collection.
	StoreDocumentWithEmbeddingInCollection(concept, content string, embedding []float32, collection string) (string, error)
	StoreBatch(items []ArchiveItem) ([]string, error)
	SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error)
	// SearchMemoriesOnly searches only the aurago_memories collection.
	// Use this for lightweight lookups (e.g. predictive pre-fetch) that
	// should not pay the cost of scanning tool_guides and documentation.
	SearchMemoriesOnly(query string, topK int) ([]string, []string, error)
	// GetByID retrieves a document from a specific collection by its ID.
	// This is the collection-aware variant needed by the FileIndexer.
	GetByIDFromCollection(id, collection string) (string, error)
	GetByID(id string) (string, error)
	DeleteDocument(id string) error
	DeleteDocumentFromCollection(id, collection string) error
	Count() int
	IsDisabled() bool
	IsReady() bool
	Close() error
	// StoreCheatsheet stores a cheatsheet in the vector DB with cs_id metadata.
	// The cheatsheet is stored with auto-chunking for large content and uses
	// the cheatsheet ID as the unique document identifier for upsert semantics.
	StoreCheatsheet(id, name, content string, attachments ...string) error
	// DeleteCheatsheet removes all vector entries associated with a cheatsheet ID
	// by using the cs_id metadata filter.
	DeleteCheatsheet(id string) error
	// RegisterCollections registers multiple collections.
	RegisterCollections(collections []string)
}

// ScoredVectorDB is implemented by vector stores that can return raw similarity
// scores without encoding them into user-visible memory text.
type ScoredVectorDB interface {
	SearchSimilarScored(query string, topK int, excludeCollections ...string) ([]SearchResult, error)
	SearchMemoriesOnlyScored(query string, topK int) ([]SearchResult, error)
}

// queryCacheEntry stores a pre-computed query embedding with a timestamp for TTL expiry.
type queryCacheEntry struct {
	embedding []float32
	timestamp time.Time
}

// ErrVectorDBNotReady is returned when embedding validation is still running.
var ErrVectorDBNotReady = errors.New("VectorDB is not ready (embedding validation in progress)")

// ErrVectorDBDisabled is returned when the embedding pipeline failed at startup.
var ErrVectorDBDisabled = errors.New("VectorDB is disabled (embedding pipeline failed at startup)")

// ErrVectorDBClosed is returned after Close starts shutting the vector DB down.
var ErrVectorDBClosed = errors.New("VectorDB is closed")

const maxBatchItemBytes = 500 * 1024 // 500 KB per batch item

var vectorDBCloseWait = 10 * time.Second

// ChromemVectorDB implements VectorDB using chromem-go with persistence.
type ChromemVectorDB struct {
	db                     *chromem.DB
	dataDir                string // persistent directory for the vector DB (used for version files)
	collection             *chromem.Collection
	logger                 *slog.Logger
	mu                     sync.RWMutex   // Protects indexing operations; reads use RLock
	lifecycleMu            sync.Mutex     // Serializes operation registration with Close.
	conceptLocks           [64]sync.Mutex // Striped locks to serialise checks/stores per concept bucket
	embeddingFunc          chromem.EmbeddingFunc
	embeddingFingerprint   string
	ready                  atomic.Bool  // Set once startup embedding validation reaches a final state
	disabled               atomic.Bool  // Set when embedding pipeline fails; skips operations gracefully
	closed                 atomic.Bool  // Set once Close begins; rejects new store/search operations.
	idCounter              atomic.Int64 // Monotonic counter for collision-free document IDs
	queryCache             map[string]queryCacheEntry
	queryCacheMu           sync.RWMutex
	queryCacheTTL          time.Duration
	indexing               atomic.Int32       // Counter: >0 while async indexing is in progress
	indexingWg             sync.WaitGroup     // Tracks in-flight async indexing goroutines
	validationWg           sync.WaitGroup     // Tracks startup embedding validation.
	dedupSem               chan struct{}      // semaphore to limit concurrent dedup checks
	storeWg                sync.WaitGroup     // Tracks in-flight single-document store operations
	batchWg                sync.WaitGroup     // Tracks in-flight StoreBatch goroutines
	searchWg               sync.WaitGroup     // Tracks in-flight parallel search goroutines
	sfGroup                singleflight.Group // deduplicates concurrent embedding API calls for the same query
	fileIndexerCollections map[string]struct{}
	fiColMu                sync.RWMutex
}

func (cv *ChromemVectorDB) Close() error {
	cv.lifecycleMu.Lock()
	cv.closed.Store(true)
	cv.ready.Store(false)
	cv.lifecycleMu.Unlock()
	if cv.logger != nil {
		cv.logger.Debug("Closing VectorDB, waiting for in-flight validation, indexing, and batch operations...")
	}
	done := waitForAsync(func() {
		cv.validationWg.Wait()
		cv.storeWg.Wait()
		cv.indexingWg.Wait()
		cv.batchWg.Wait()
		cv.searchWg.Wait()
	})
	select {
	case <-done:
		if cv.logger != nil {
			cv.logger.Debug("All in-flight VectorDB operations completed")
		}
	case <-time.After(vectorDBCloseWait):
		err := fmt.Errorf("close vector db: timed out waiting for in-flight operations")
		if cv.logger != nil {
			cv.logger.Warn("Close timed out waiting for in-flight VectorDB operations", "error", err)
		}
		return err
	}
	return nil
}

func (cv *ChromemVectorDB) beginTrackedOperation(wg *sync.WaitGroup) (func(), error) {
	cv.lifecycleMu.Lock()
	defer cv.lifecycleMu.Unlock()
	if cv.closed.Load() {
		return nil, ErrVectorDBClosed
	}
	wg.Add(1)
	return wg.Done, nil
}

func waitForWaitGroup(wg *sync.WaitGroup, timeout time.Duration) bool {
	done := waitForAsync(wg.Wait)
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func waitForAsync(wait func()) chan struct{} {
	done := make(chan struct{})
	go func() {
		wait()
		close(done)
	}()
	return done
}

func cloneFloat32Slice(in []float32) []float32 {
	if in == nil {
		return nil
	}
	out := make([]float32, len(in))
	copy(out, in)
	return out
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

// EmbeddingFingerprint identifies the embedding provider/model used for newly
// stored documents. Callers can persist it to detect stale embeddings after
// model or endpoint changes.
func (cv *ChromemVectorDB) EmbeddingFingerprint() string {
	return cv.embeddingFingerprint
}

// Count returns the total number of documents across all collections
// (aurago_memories, tool_guides, documentation, and file_indexer collections).
// Returns the persisted count even when the embedding pipeline is disabled,
// because counting does not require embeddings.
func (cv *ChromemVectorDB) Count() int {
	doneCount, err := cv.beginTrackedOperation(&cv.searchWg)
	if err != nil {
		return 0
	}
	defer doneCount()

	cv.mu.RLock()
	db := cv.db
	primary := cv.collection
	logger := cv.logger
	cv.mu.RUnlock()

	if db == nil || primary == nil {
		return 0
	}

	total := primary.Count() // aurago_memories
	collections := db.ListCollections()

	// Include secondary collections that were indexed at startup
	for _, name := range []string{"tool_guides", "documentation"} {
		if col, ok := collections[name]; ok && col != nil {
			total += col.Count()
		} else if logger != nil {
			logger.Debug("Expected VectorDB collection missing during count", "collection", name)
		}
	}

	// Include FileIndexer collections (file_index + registered custom collections)
	cv.fiColMu.RLock()
	fiCollections := make([]string, 0, len(cv.fileIndexerCollections)+1)
	fiCollections = append(fiCollections, "file_index")
	for col := range cv.fileIndexerCollections {
		if col != "file_index" {
			fiCollections = append(fiCollections, col)
		}
	}
	cv.fiColMu.RUnlock()

	for _, name := range fiCollections {
		if col, ok := collections[name]; ok && col != nil {
			total += col.Count()
		} else if logger != nil {
			logger.Debug("Expected VectorDB collection missing during count", "collection", name)
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

// IsReady reports whether startup embedding validation has completed.
func (cv *ChromemVectorDB) IsReady() bool {
	return cv.ready.Load()
}

func (cv *ChromemVectorDB) requireReadyForStore() error {
	if cv.closed.Load() {
		return ErrVectorDBClosed
	}
	if !cv.ready.Load() {
		return ErrVectorDBNotReady
	}
	if cv.disabled.Load() {
		return ErrVectorDBDisabled
	}
	return nil
}

func (cv *ChromemVectorDB) requireReadyForSearch() error {
	if cv.closed.Load() {
		return ErrVectorDBClosed
	}
	if !cv.ready.Load() {
		return ErrVectorDBNotReady
	}
	if cv.disabled.Load() {
		return ErrVectorDBDisabled
	}
	return nil
}

func buildEmbeddingFingerprint(provider, baseURL, model string) string {
	parts := []string{
		strings.TrimSpace(provider),
		strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		strings.TrimSpace(model),
	}
	return strings.Join(parts, "|")
}

func (cv *ChromemVectorDB) addEmbeddingMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	if cv.embeddingFingerprint != "" {
		metadata["embedding_fingerprint"] = cv.embeddingFingerprint
	}
	return metadata
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
	embeddingFingerprint := ""
	provider := cfg.Embeddings.Provider
	localEmbeddingProvider := isLocalEmbeddingProvider(cfg)

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
		embeddingFingerprint = buildEmbeddingFingerprint(provider, embedURL, embedModel)

		// Warn early if the API key is empty — the 401 from the provider would
		// otherwise be the only hint and doesn't clearly say "key missing".
		if embedKey == "" {
			vaultKey := "provider_" + provider + "_api_key"
			logger.Warn("[VectorDB] Embeddings API key is empty — check vault entry",
				"provider", provider, "vault_key", vaultKey,
				"hint", "Re-enter the API key via Config UI → Providers → "+provider)
		}

		embeddingFunc = withEmbeddingRetry(chromem.NewEmbeddingFuncOpenAICompat(
			embedURL,
			embedKey,
			embedModel,
			nil, // Auto-detect normalization
		), logger)
		logger.Info("VectorDB using embeddings provider", "provider", provider, "url", embedURL, "model", embedModel)
	}

	collection, err := db.GetOrCreateCollection("aurago_memories", nil, embeddingFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create collection: %w", err)
	}

	vdb := &ChromemVectorDB{
		db:                     db,
		dataDir:                dataDir,
		collection:             collection,
		logger:                 logger,
		embeddingFunc:          embeddingFunc,
		embeddingFingerprint:   embeddingFingerprint,
		queryCache:             make(map[string]queryCacheEntry),
		queryCacheTTL:          5 * time.Minute,
		dedupSem:               make(chan struct{}, 16),
		fileIndexerCollections: make(map[string]struct{}),
	}

	// Phase 29: Startup validation — test the embedding pipeline asynchronously
	// so that slow remote embedding providers do not block server startup.
	// vdb.disabled starts as false (atomic.Bool zero value). If the background
	// validation fails, it is set to true, causing subsequent VectorDB operations
	// to gracefully fail until the process is restarted.
	if provider == "disabled" {
		vdb.disabled.Store(true)
		vdb.ready.Store(true)
		logger.Info("VectorDB disabled by configuration, skipping embedding validation")
	} else {
		logger.Info("Validating embedding pipeline (async)...")
		vdb.validationWg.Add(1)
		go func() {
			defer vdb.validationWg.Done()
			defer vdb.ready.Store(true)
			validationStart := time.Now()
			vec, err := validateEmbeddingWithRetry(embeddingFunc, 3, logger)
			if err != nil {
				logger.Warn("Embedding pipeline validation failed after retries. Long-term memory will be disabled.", "error", err)
				vdb.disabled.Store(true)
			} else {
				latency := time.Since(validationStart)
				logger.Info("Embedding pipeline validated", "vector_dimensions", len(vec), "provider", provider, "docs", collection.Count(), "latency", latency)
				if localEmbeddingProvider && latency > 500*time.Millisecond {
					logger.Warn("Local embeddings are slow. Consider enabling GPU passthrough (use_host_gpu) or using a cloud provider.", "latency", latency)
				}
			}
		}()
	}

	return vdb, nil
}

func isLocalEmbeddingProvider(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	providerType := strings.ToLower(strings.TrimSpace(cfg.Embeddings.ProviderType))
	if providerType == "ollama" {
		return true
	}
	if cfg.Embeddings.LocalOllama.Enabled {
		return true
	}

	rawURL := strings.TrimSpace(cfg.Embeddings.BaseURL)
	if rawURL == "" {
		rawURL = strings.TrimSpace(cfg.Embeddings.ExternalURL)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "ollama"
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

// getConceptMutex returns the Mutex for a given concept based on its hash value.
func (cv *ChromemVectorDB) getConceptMutex(concept string) *sync.Mutex {
	var hash uint32 = 5381
	for i := 0; i < len(concept); i++ {
		hash = ((hash << 5) + hash) + uint32(concept[i])
	}
	idx := hash % 64
	return &cv.conceptLocks[idx]
}

// StoreDocumentWithDomain stores a concept/content pair with an optional domain tag
// for cross-domain learning (Phase C). The domain helps categorize knowledge.
// Deduplication: skips storage if a very similar document already exists (similarity > 0.95).
func (cv *ChromemVectorDB) StoreDocumentWithDomain(concept, content, domain string) ([]string, error) {
	doneStore, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return nil, err
	}
	defer doneStore()

	if err := cv.requireReadyForStore(); err != nil {
		return nil, err
	}

	// Deduplication: serialise check+store per concept bucket so concurrent
	// calls for the same concept cannot both pass the similarity gate.
	mu := cv.getConceptMutex(concept)
	mu.Lock()
	if docID, sim := cv.searchTopSimilarMemory(concept); sim > 0.95 {
		if docID != "" {
			mu.Unlock()
			cv.logger.Debug("Skipping duplicate concept (similarity > 0.95)", "concept", concept, "similarity", sim, "doc_id", docID)
			return []string{docID}, nil
		}
		cv.logger.Warn("Duplicate search returned high similarity without doc ID; storing new document", "concept", concept, "similarity", sim)
	}
	defer mu.Unlock()
	return cv.storeDocumentLocked(concept, content, domain)
}

// storeDocumentLocked stores a document in aurago_memories.
// The caller must hold the concept's striped mutex. This method serializes the
// actual collection mutation so concurrent writers cannot race chromem state.
func (cv *ChromemVectorDB) storeDocumentLocked(concept, content, domain string) ([]string, error) {
	const maxContentBytes = 500 * 1024 // 500 KB per document
	if len(content) > maxContentBytes {
		cv.logger.Warn("Document content exceeds 500 KB limit, truncating", "concept", concept, "bytes", len(content))
		content = truncateUTF8Bytes(content, maxContentBytes)
	}

	cv.mu.Lock()
	defer cv.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullContent := buildContentString(concept, content)

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
			Metadata: cv.addEmbeddingMetadata(metadata),
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
			Metadata: cv.addEmbeddingMetadata(chunkMeta),
			Content:  buildContentString(concept, chunk),
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

// StoreDocumentInCollection stores a document in a specific collection.
// This is used by the FileIndexer to route documents to per-directory collections.
func (cv *ChromemVectorDB) StoreDocumentInCollection(concept, content, collection string) ([]string, error) {
	doneStore, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return nil, err
	}
	defer doneStore()

	return cv.storeDocumentInCollectionWithDomain(concept, content, collection, "")
}

// storeDocumentInCollectionWithDomain is the internal implementation for collection-aware storage.
func (cv *ChromemVectorDB) storeDocumentInCollectionWithDomain(concept, content, collection, domain string) ([]string, error) {
	if err := cv.requireReadyForStore(); err != nil {
		return nil, err
	}
	if collection == "" {
		collection = "aurago_memories"
	}

	// Track this collection for FileIndexer document lookups
	cv.registerFileIndexerCollection(collection)

	const maxContentBytes = 500 * 1024
	if len(content) > maxContentBytes {
		cv.logger.Warn("Document content exceeds 500 KB limit, truncating", "concept", concept, "bytes", len(content))
		content = truncateUTF8Bytes(content, maxContentBytes)
	}

	cv.mu.Lock()
	defer cv.mu.Unlock()

	col, err := cv.db.GetOrCreateCollection(collection, nil, cv.embeddingFunc)
	if err != nil {
		return nil, fmt.Errorf("get/create collection %s: %w", collection, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullContent := buildContentString(concept, content)

	metadata := map[string]string{
		"concept":     concept,
		"timestamp":   fmt.Sprintf("%d", time.Now().Unix()),
		"source_type": "file_indexer",
		"collection":  collection,
	}
	if domain != "" {
		metadata["domain"] = domain
	}

	// Small texts: store as a single document
	if len(fullContent) <= 4000 {
		docID := fmt.Sprintf("file_%d_%d", time.Now().UnixMilli(), cv.idCounter.Add(1))
		doc := chromem.Document{
			ID:       docID,
			Metadata: cv.addEmbeddingMetadata(metadata),
			Content:  fullContent,
		}
		if err := col.AddDocument(ctx, doc); err != nil {
			cv.logger.Error("Failed to store document in collection", "collection", collection, "error", err)
			return nil, fmt.Errorf("failed to add document: %w", err)
		}
		cv.logger.Info("Stored document in collection", "collection", collection, "id", docID, "concept", concept)
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
		docID := fmt.Sprintf("file_%d_%d_chunk_%d", time.Now().UnixMilli(), baseCounter-int64(len(chunks))+int64(i)+1, i)
		chunkMeta := map[string]string{
			"concept":     concept,
			"chunk_index": fmt.Sprintf("%d/%d", i+1, len(chunks)),
			"timestamp":   fmt.Sprintf("%d", time.Now().Unix()),
			"source_type": "file_indexer",
			"collection":  collection,
		}
		if domain != "" {
			chunkMeta["domain"] = domain
		}
		docs = append(docs, chromem.Document{
			ID:       docID,
			Metadata: cv.addEmbeddingMetadata(chunkMeta),
			Content:  buildContentString(concept, chunk),
		})
		storedIDs = append(storedIDs, docID)
	}

	// Batch-add all chunks in one call
	chunkCtx, chunkCancel := context.WithTimeout(context.Background(), calculateBatchTimeout(len(docs)))
	defer chunkCancel()
	if err := col.AddDocuments(chunkCtx, docs, 1); err != nil {
		cv.logger.Error("Failed to store chunked document in collection", "collection", collection, "error", err)
		return nil, fmt.Errorf("failed to add chunked document: %w", err)
	}

	cv.logger.Info("Stored chunked document in collection", "collection", collection, "concept", concept, "chunks", len(chunks))
	return storedIDs, nil
}

// StoreDocumentWithEmbedding stores a document with a pre-computed embedding vector.
// This bypasses the text embedding function, allowing multimodal content (images, audio)
// to be stored with externally computed embeddings.
func (cv *ChromemVectorDB) StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error) {
	doneStore, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return "", err
	}
	defer doneStore()

	if err := cv.requireReadyForStore(); err != nil {
		return "", err
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
		Metadata: cv.addEmbeddingMetadata(map[string]string{
			"concept":    concept,
			"timestamp":  fmt.Sprintf("%d", time.Now().Unix()),
			"multimodal": "true",
		}),
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

// StoreDocumentWithEmbeddingInCollection stores a document with a pre-computed embedding vector
// in a specific collection. This is used by the FileIndexer for multimodal content.
func (cv *ChromemVectorDB) StoreDocumentWithEmbeddingInCollection(concept, content string, embedding []float32, collection string) (string, error) {
	doneStore, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return "", err
	}
	defer doneStore()

	if err := cv.requireReadyForStore(); err != nil {
		return "", err
	}
	if len(embedding) == 0 {
		return "", fmt.Errorf("embedding vector is empty")
	}
	if collection == "" {
		collection = "aurago_memories"
	}

	// Track this collection for FileIndexer document lookups
	cv.registerFileIndexerCollection(collection)

	cv.mu.Lock()
	defer cv.mu.Unlock()

	col, err := cv.db.GetOrCreateCollection(collection, nil, cv.embeddingFunc)
	if err != nil {
		return "", fmt.Errorf("get/create collection %s: %w", collection, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	docID := fmt.Sprintf("mm_%d_%d", time.Now().UnixMilli(), cv.idCounter.Add(1))
	doc := chromem.Document{
		ID: docID,
		Metadata: cv.addEmbeddingMetadata(map[string]string{
			"concept":     concept,
			"timestamp":   fmt.Sprintf("%d", time.Now().Unix()),
			"multimodal":  "true",
			"source_type": "file_indexer",
			"collection":  collection,
		}),
		Content:   content,
		Embedding: embedding,
	}

	if err := col.AddDocument(ctx, doc); err != nil {
		cv.logger.Error("Failed to store multimodal document in collection", "collection", collection, "error", err, "concept", concept)
		return "", fmt.Errorf("failed to add multimodal document: %w", err)
	}

	cv.logger.Info("Stored multimodal document in collection", "collection", collection, "id", docID, "concept", concept)
	return docID, nil
}

// StoreCheatsheet stores a cheatsheet document with its ID as a unique identifier.
// It uses the cheatsheet ID in the document ID for upsert semantics: calling
// StoreCheatsheet again for the same ID will replace the existing document.
// The cheatsheet is stored with cs_type="cheatsheet" metadata for filtering.
func (cv *ChromemVectorDB) StoreCheatsheet(id, name, content string, attachments ...string) error {
	doneStore, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return err
	}
	defer doneStore()

	if err := cv.requireReadyForStore(); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("cheatsheet ID is required")
	}

	cv.mu.Lock()
	defer cv.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First delete any existing cheatsheet docs with this ID (upsert semantics)
	if err := cv.collection.Delete(ctx, map[string]string{"cs_type": "cheatsheet", "cs_id": id}, nil); err != nil {
		cv.logger.Warn("Failed to delete existing cheatsheet docs before store", "cs_id", id, "error", err)
		return fmt.Errorf("delete existing cheatsheet docs %s: %w", id, err)
	}

	fullContent := buildContentString(name, content)
	if len(attachments) > 0 {
		if fullContent != "" {
			fullContent += "\n\n"
		}
		fullContent += "Attachments:\n" + strings.Join(attachments, "\n\n---\n\n")
	}

	metadata := map[string]string{
		"cs_type":   "cheatsheet",
		"cs_id":     id,
		"cs_name":   name,
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	}

	// Small texts: store as a single document
	if len(fullContent) <= 4000 {
		docID := fmt.Sprintf("cs_%s", id)
		doc := chromem.Document{
			ID:       docID,
			Metadata: cv.addEmbeddingMetadata(metadata),
			Content:  fullContent,
		}
		if err := cv.collection.AddDocument(ctx, doc); err != nil {
			cv.logger.Error("Failed to store cheatsheet in vector DB", "error", err, "cs_id", id)
			return fmt.Errorf("failed to add cheatsheet document: %w", err)
		}
		cv.logger.Info("Stored cheatsheet in vector DB", "id", docID, "cs_id", id, "cs_name", name)
		return nil
	}

	// Large texts: split into chunks and batch-store
	const maxChunks = 50
	chunks := chunkText(fullContent, 3500, 200)
	if len(chunks) > maxChunks {
		cv.logger.Warn("Cheatsheet produces too many chunks, capping", "cs_id", id, "chunks", len(chunks), "max", maxChunks)
		chunks = chunks[:maxChunks]
	}

	var docs []chromem.Document
	for i, chunk := range chunks {
		docID := fmt.Sprintf("cs_%s_chunk_%d", id, i)
		chunkMeta := map[string]string{
			"cs_type":     "cheatsheet",
			"cs_id":       id,
			"cs_name":     name,
			"chunk_index": fmt.Sprintf("%d/%d", i+1, len(chunks)),
			"timestamp":   fmt.Sprintf("%d", time.Now().Unix()),
		}
		docs = append(docs, chromem.Document{
			ID:       docID,
			Metadata: cv.addEmbeddingMetadata(chunkMeta),
			Content:  name + " (" + fmt.Sprintf("%d/%d", i+1, len(chunks)) + ")\n\n" + chunk,
		})
	}

	// Batch-add all chunks in one call (sequential embedding to avoid rate limits)
	chunkCtx, chunkCancel := context.WithTimeout(context.Background(), calculateBatchTimeout(len(docs)))
	defer chunkCancel()
	if err := cv.collection.AddDocuments(chunkCtx, docs, 1); err != nil {
		cv.logger.Error("Failed to store chunked cheatsheet", "error", err, "cs_id", id, "chunks", len(chunks))
		return fmt.Errorf("failed to add chunked cheatsheet (%d chunks): %w", len(chunks), err)
	}

	cv.logger.Info("Stored chunked cheatsheet in vector DB", "cs_id", id, "cs_name", name, "chunks", len(chunks))
	return nil
}

// DeleteCheatsheet removes all vector entries associated with a cheatsheet ID
// by using the cs_id metadata filter.
func (cv *ChromemVectorDB) DeleteCheatsheet(id string) error {
	doneStore, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return err
	}
	defer doneStore()

	if err := cv.requireReadyForStore(); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("cheatsheet ID is required")
	}

	cv.mu.Lock()
	defer cv.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := cv.collection.Delete(ctx, map[string]string{"cs_type": "cheatsheet", "cs_id": id}, nil); err != nil {
		cv.logger.Error("Failed to delete cheatsheet from vector DB", "cs_id", id, "error", err)
		return fmt.Errorf("failed to delete cheatsheet documents: %w", err)
	}

	cv.logger.Info("Deleted cheatsheet from vector DB", "cs_id", id)
	return nil
}

// chunkText splits a large text into smaller segments of roughly chunkSize characters,
// preferring paragraph (\n\n) or sentence boundaries. Adds overlap characters between chunks.
func chunkText(text string, chunkSize, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	if chunkSize <= 0 {
		chunkSize = len(runes)
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
	if len(runes) <= chunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0

	for start < len(runes) {
		end := start + chunkSize
		if end >= len(runes) {
			if chunk := strings.TrimSpace(string(runes[start:])); chunk != "" {
				chunks = append(chunks, chunk)
			}
			break
		}

		// Try to split at paragraph boundary (\n\n) or sentence boundary (. )
		chunkRunes := runes[start:end]
		chunkStr := string(chunkRunes)

		splitAt := strings.LastIndex(chunkStr, "\n\n")
		if splitAt > len(chunkStr)/2 {
			runeSplitAt := len([]rune(chunkStr[:splitAt]))
			end = start + runeSplitAt + 2 // include the double newline
		} else {
			splitAt = strings.LastIndex(chunkStr, ". ")
			if splitAt > len(chunkStr)/2 {
				runeSplitAt := len([]rune(chunkStr[:splitAt]))
				end = start + runeSplitAt + 2 // include the dot and space
			}
		}

		if chunk := strings.TrimSpace(string(runes[start:end])); chunk != "" {
			chunks = append(chunks, chunk)
		}

		// Move forward with overlap, ensuring we always progress
		nextStart := end - overlap
		if nextStart <= start {
			nextStart = end
		}
		start = nextStart
	}

	return chunks
}

// StoreBatch stores multiple concept/content pairs. Small documents are collected
// and batch-embedded in a single parallel call; large texts are chunked individually.
// Performs deduplication by skipping items whose concept is already stored (similarity > 0.95).
func (cv *ChromemVectorDB) StoreBatch(items []ArchiveItem) ([]string, error) {
	if err := cv.requireReadyForStore(); err != nil {
		return nil, err
	}
	doneBatch, err := cv.beginTrackedOperation(&cv.batchWg)
	if err != nil {
		return nil, err
	}
	defer doneBatch()
	if len(items) == 0 {
		return nil, nil
	}

	type batchResult struct {
		idx int
		ids []string
		err error
	}

	resultsCh := make(chan batchResult, len(items))
	jobs := make(chan batchResult)
	workerCount := cv.storeBatchConcurrency(len(items))
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer workers.Done()
			for job := range jobs {
				i := job.idx
				item := items[i]
				sent := false
				func() {
					defer func() {
						if rec := recover(); rec != nil {
							cv.logger.Error("StoreBatch: goroutine panicked", "panic", rec)
							if !sent {
								resultsCh <- batchResult{idx: i, err: fmt.Errorf("panic: %v", rec)}
							}
						}
					}()
					cv.dedupSem <- struct{}{}
					defer func() { <-cv.dedupSem }()

					mu := cv.getConceptMutex(item.Concept)
					mu.Lock()
					defer mu.Unlock()

					var ids []string
					var err error
					if docID, sim := cv.searchTopSimilarMemory(item.Concept); sim > 0.95 {
						if docID != "" {
							cv.logger.Debug("StoreBatch: skipping duplicate concept", "concept", item.Concept, "similarity", sim, "doc_id", docID)
							ids = []string{docID}
						} else {
							cv.logger.Warn("StoreBatch: duplicate search returned high similarity without doc ID; storing new document", "concept", item.Concept, "similarity", sim)
							ids, err = cv.storeDocumentLocked(item.Concept, item.Content, item.Domain)
						}
					} else {
						ids, err = cv.storeDocumentLocked(item.Concept, item.Content, item.Domain)
					}
					resultsCh <- batchResult{idx: i, ids: ids, err: err}
					sent = true
				}()
			}
		}()
	}
	for i, item := range items {
		if len(item.Content) > maxBatchItemBytes {
			cv.logger.Warn("StoreBatch: item content exceeds 500 KB limit, truncating", "concept", item.Concept, "bytes", len(item.Content))
			items[i].Content = truncateArchiveItemContent(item.Content)
		}
		jobs <- batchResult{idx: i}
	}
	close(jobs)
	workers.Wait()
	close(resultsCh)

	byIndex := make([][]string, len(items))
	var firstErr error
	for r := range resultsCh {
		if r.err != nil && firstErr == nil {
			firstErr = r.err
		}
		byIndex[r.idx] = append(byIndex[r.idx], r.ids...)
	}
	var allIDs []string
	for _, ids := range byIndex {
		allIDs = append(allIDs, ids...)
	}

	if firstErr != nil {
		return allIDs, firstErr
	}

	cv.logger.Info("Stored batch in long-term memory", "count", len(items), "total_docs", len(allIDs))
	return allIDs, nil
}

func (cv *ChromemVectorDB) storeBatchConcurrency(itemCount int) int {
	if itemCount <= 0 {
		return 0
	}
	limit := 16
	if cv.dedupSem != nil && cap(cv.dedupSem) > 0 {
		limit = cap(cv.dedupSem)
	}
	if itemCount < limit {
		return itemCount
	}
	return limit
}

func (cv *ChromemVectorDB) SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error) {
	results, err := cv.SearchSimilarScored(query, topK, excludeCollections...)
	if err != nil {
		return nil, nil, err
	}
	texts, ids := splitSearchResults(results)
	return texts, ids, nil
}

// SearchSimilarScored finds the topK most semantically similar documents across
// all relevant collections and preserves their decayed similarity scores.
func (cv *ChromemVectorDB) SearchSimilarScored(query string, topK int, excludeCollections ...string) ([]SearchResult, error) {
	doneSearch, err := cv.beginTrackedOperation(&cv.searchWg)
	if err != nil {
		return nil, err
	}
	defer doneSearch()

	if err := cv.requireReadyForSearch(); err != nil {
		return nil, err
	}

	cv.mu.RLock()
	defer cv.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Compute query embedding once and reuse across all collections
	queryEmbedding, err := cv.getQueryEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to compute query embedding: %w", err)
	}

	allCollections := []string{"aurago_memories", "tool_guides", "documentation"}
	cv.fiColMu.RLock()
	fileCollections := make([]string, 0, len(cv.fileIndexerCollections)+1)
	fileCollections = append(fileCollections, "file_index")
	for col := range cv.fileIndexerCollections {
		if col != "file_index" {
			fileCollections = append(fileCollections, col)
		}
	}
	cv.fiColMu.RUnlock()
	allCollections = append(allCollections, fileCollections...)
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

	// Query all collections in parallel using pre-computed embedding.
	type colResult struct {
		colName string
		results []chromem.Result
		err     error
	}
	resultCh := make(chan colResult, len(collections))

	// Track in-flight goroutines so we can wait for them before returning.
	// This ensures clean shutdown when context is cancelled or deadline exceeded.
	var wg sync.WaitGroup

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
		wg.Add(1)
		go func(c *chromem.Collection, k int) {
			defer wg.Done()
			doneSearch, trackErr := cv.beginTrackedOperation(&cv.searchWg)
			if trackErr != nil {
				resultCh <- colResult{colName: colName, err: trackErr}
				return
			}
			defer doneSearch()
			res, qErr := c.QueryEmbedding(ctx, queryEmbedding, k, nil, nil)
			resultCh <- colResult{colName: colName, results: res, err: qErr}
		}(col, searchK)
	}

	var allResults []SearchResult
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

				allResults = append(allResults, SearchResult{
					Text:       formattedText,
					DocID:      result.ID,
					Similarity: float64(sim),
				})
			}
		}
	}

finalizeResults:
	cancel()
	wg.Wait()

	// Sort by similarity descending and enforce global topK limit
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Similarity > allResults[j].Similarity
	})
	if len(allResults) > topK {
		allResults = allResults[:topK]
	}

	return allResults, nil
}

// SearchMemoriesOnly searches only the aurago_memories collection. Much cheaper than
// SearchSimilar because it skips the tool_guides and documentation collections.
// Intended for use cases like predictive pre-fetch where documentation hits add no value.
// Uses the query embedding cache to avoid redundant API calls.
func (cv *ChromemVectorDB) SearchMemoriesOnly(query string, topK int) ([]string, []string, error) {
	results, err := cv.SearchMemoriesOnlyScored(query, topK)
	if err != nil {
		return nil, nil, err
	}
	texts, ids := splitSearchResults(results)
	return texts, ids, nil
}

// SearchMemoriesOnlyScored searches only aurago_memories and preserves scores.
func (cv *ChromemVectorDB) SearchMemoriesOnlyScored(query string, topK int) ([]SearchResult, error) {
	doneSearch, err := cv.beginTrackedOperation(&cv.searchWg)
	if err != nil {
		return nil, err
	}
	defer doneSearch()

	if err := cv.requireReadyForSearch(); err != nil {
		return nil, err
	}

	cv.mu.RLock()
	defer cv.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	col, err := cv.db.GetOrCreateCollection("aurago_memories", nil, cv.embeddingFunc)
	if err != nil || col.Count() == 0 {
		return nil, nil
	}

	searchK := topK
	if searchK > col.Count() {
		searchK = col.Count()
	}

	queryEmbedding, err := cv.getQueryEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to compute query embedding: %w", err)
	}

	results, err := col.QueryEmbedding(ctx, queryEmbedding, searchK, nil, nil)
	if err != nil {
		return nil, err
	}

	var searchResults []SearchResult
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
			searchResults = append(searchResults, SearchResult{
				Text:       r.Content,
				DocID:      r.ID,
				Similarity: float64(sim),
			})
		}
	}
	return searchResults, nil
}

func splitSearchResults(results []SearchResult) ([]string, []string) {
	texts := make([]string, 0, len(results))
	ids := make([]string, 0, len(results))
	for _, result := range results {
		texts = append(texts, result.Text)
		ids = append(ids, result.DocID)
	}
	return texts, ids
}

// GetByID retrieves a document's full content by its ID.
// For FileIndexer documents (IDs starting with "file_" or "mm_"), it automatically
// falls back to the file_index collection if not found in aurago_memories.
// This maintains backward compatibility while supporting collection-aware FileIndexer lookups.
func (cv *ChromemVectorDB) GetByID(id string) (string, error) {
	doneSearch, err := cv.beginTrackedOperation(&cv.searchWg)
	if err != nil {
		return "", err
	}
	defer doneSearch()

	if err := cv.requireReadyForStore(); err != nil {
		return "", err
	}
	cv.mu.RLock()
	defer cv.mu.RUnlock()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Try aurago_memories first (backward compatible)
	doc, err := cv.collection.GetByID(ctx, id)
	if err == nil {
		return doc.Content, nil
	}

	// For FileIndexer documents, fall back to all registered FileIndexer collections.
	// FileIndexer uses "file_" or "mm_" prefixes for document IDs.
	// Collections are registered via registerFileIndexerCollection when documents are stored.
	if strings.HasPrefix(id, "file_") || strings.HasPrefix(id, "mm_") {
		cv.fiColMu.RLock()
		collections := make([]string, 0, len(cv.fileIndexerCollections)+1)
		collections = append(collections, "file_index") // always try default first
		for col := range cv.fileIndexerCollections {
			if col != "file_index" {
				collections = append(collections, col)
			}
		}
		cv.fiColMu.RUnlock()

		for _, colName := range collections {
			col, colErr := cv.db.GetOrCreateCollection(colName, nil, cv.embeddingFunc)
			if colErr != nil {
				continue
			}
			doc, err = col.GetByID(ctx, id)
			if err == nil {
				return doc.Content, nil
			}
		}
	}

	return "", fmt.Errorf("document not found: %w", err)
}

// registerFileIndexerCollection tracks a collection that contains FileIndexer documents.
// This allows GetByID to find FileIndexer documents in custom per-directory collections
// during the fallback lookup phase.
func (cv *ChromemVectorDB) registerFileIndexerCollection(collection string) {
	cv.fiColMu.Lock()
	defer cv.fiColMu.Unlock()
	cv.fileIndexerCollections[collection] = struct{}{}
}

// RegisterCollections registers multiple collections under fileIndexerCollections on startup.
func (cv *ChromemVectorDB) RegisterCollections(collections []string) {
	cv.fiColMu.Lock()
	defer cv.fiColMu.Unlock()
	for _, col := range collections {
		if col != "" {
			cv.fileIndexerCollections[col] = struct{}{}
		}
	}
}

// GetByIDFromCollection retrieves a document from a specific collection by its ID.
func (cv *ChromemVectorDB) GetByIDFromCollection(id, collection string) (string, error) {
	doneSearch, err := cv.beginTrackedOperation(&cv.searchWg)
	if err != nil {
		return "", err
	}
	defer doneSearch()

	if err := cv.requireReadyForStore(); err != nil {
		return "", err
	}
	cv.mu.RLock()
	defer cv.mu.RUnlock()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	col, err := cv.db.GetOrCreateCollection(collection, nil, cv.embeddingFunc)
	if err != nil {
		return "", fmt.Errorf("get collection %s: %w", collection, err)
	}
	doc, err := col.GetByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("document not found in collection %s: %w", collection, err)
	}
	return doc.Content, nil
}

// searchTopSimilarityScore returns the decayed similarity score of the closest existing
// document in the aurago_memories collection for the given concept, or 0 if no match.
// It is used internally for dedup checks and does NOT format results with a prefix string,
// unlike SearchSimilar/SearchMemoriesOnly. This method holds cv.mu.RLock for the duration
// of its operation. Callers may hold a concept lock, but must release this read lock before
// taking the vector write lock for storage.
func (cv *ChromemVectorDB) searchTopSimilarityScore(concept string) float32 {
	_, sim := cv.searchTopSimilarMemory(concept)
	return sim
}

func (cv *ChromemVectorDB) searchTopSimilarMemory(concept string) (string, float32) {
	if !cv.ready.Load() || cv.disabled.Load() {
		return "", 0
	}
	// Hold read lock for consistency with SearchSimilar/SearchMemoriesOnly,
	// which both protect cv.db and cv.embeddingFunc accesses with cv.mu.RLock().
	cv.mu.RLock()
	defer cv.mu.RUnlock()

	col, err := cv.db.GetOrCreateCollection("aurago_memories", nil, cv.embeddingFunc)
	if err != nil || col.Count() == 0 {
		return "", 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	queryEmbedding, err := cv.getQueryEmbedding(ctx, concept)
	if err != nil {
		return "", 0
	}

	results, err := col.QueryEmbedding(ctx, queryEmbedding, 1, nil, nil)
	if err != nil || len(results) == 0 {
		return "", 0
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
	return results[0].ID, sim
}

// DeleteDocument removes a specific document from the VectorDB by its ID.
// SQLite tracking cleanup is owned by SQLiteMemory so callers can decide whether
// to preserve memory_meta rows for archived-memory review workflows.
func (cv *ChromemVectorDB) DeleteDocument(id string) error {
	doneStore, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return err
	}
	defer doneStore()

	if err := cv.requireReadyForStore(); err != nil {
		return err
	}
	cv.mu.Lock()
	defer cv.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return cv.collection.Delete(ctx, nil, nil, id)
}

// DeleteDocumentWithCleanup removes a document and signals that the caller should
// also clean up SQLite tracking tables. Returns the docID so the caller can
// perform the SQLite cleanup after this call succeeds.
func (cv *ChromemVectorDB) DeleteDocumentWithCleanup(id string) error {
	if err := cv.DeleteDocument(id); err != nil {
		return err
	}
	return nil
}

// DeleteDocumentFromCollection removes a specific document from a named collection.
func (cv *ChromemVectorDB) DeleteDocumentFromCollection(id, collection string) error {
	doneStore, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return err
	}
	defer doneStore()

	if err := cv.requireReadyForStore(); err != nil {
		return err
	}
	cv.mu.Lock()
	defer cv.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	col, err := cv.db.GetOrCreateCollection(collection, nil, cv.embeddingFunc)
	if err != nil {
		return fmt.Errorf("get collection %s: %w", collection, err)
	}
	return col.Delete(ctx, nil, nil, id)
}

// getQueryEmbedding returns a cached embedding for the query string, or computes a new one.
// The cache uses a TTL to avoid stale embeddings. This saves redundant API calls
// when the same query is used across multiple collections.
func (cv *ChromemVectorDB) getQueryEmbedding(ctx context.Context, query string) ([]float32, error) {
	cv.queryCacheMu.RLock()
	if entry, ok := cv.queryCache[query]; ok && time.Since(entry.timestamp) < cv.queryCacheTTL {
		embedding := cloneFloat32Slice(entry.embedding)
		cv.queryCacheMu.RUnlock()
		return embedding, nil
	}
	cv.queryCacheMu.RUnlock()

	resultCh := cv.sfGroup.DoChan(query, func() (interface{}, error) {
		internalCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return cv.embeddingFunc(internalCtx, query)
	})
	var res interface{}
	var err error
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		res = result.Val
		err = result.Err
	}
	if err != nil {
		cv.sfGroup.Forget(query)
		return nil, err
	}
	embedding := res.([]float32)
	cachedEmbedding := cloneFloat32Slice(embedding)

	cv.queryCacheMu.Lock()
	cv.queryCache[query] = queryCacheEntry{embedding: cachedEmbedding, timestamp: time.Now()}
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

	return cloneFloat32Slice(cachedEmbedding), nil
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
	if docCount < 0 {
		docCount = 0
	}
	timeout := 30*time.Second + time.Duration(docCount)*2*time.Second
	if timeout > 5*time.Minute {
		return 5 * time.Minute
	}
	return timeout
}

func truncateArchiveItemContent(content string) string {
	return truncateUTF8Bytes(content, maxBatchItemBytes)
}

// IsIndexing reports whether async indexing is currently in progress.
func (cv *ChromemVectorDB) IsIndexing() bool {
	return cv.indexing.Load() > 0
}

// PreloadCache eagerly computes and caches query embeddings for the given queries.
// This avoids cold-start latency where every new search requires an embedding API call.
// Errors are logged but not returned — preloading is best-effort.
func (cv *ChromemVectorDB) PreloadCache(queries []string) {
	doneSearch, err := cv.beginTrackedOperation(&cv.searchWg)
	if err != nil {
		return
	}
	defer doneSearch()

	if !cv.ready.Load() || cv.disabled.Load() || len(queries) == 0 {
		return
	}
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		cv.queryCacheMu.RLock()
		if _, ok := cv.queryCache[query]; ok {
			cv.queryCacheMu.RUnlock()
			continue
		}
		cv.queryCacheMu.RUnlock()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		embedding, err := cv.embeddingFunc(ctx, query)
		cancel()
		if err != nil {
			cv.logger.Debug("PreloadCache: embedding failed", "query", query, "error", err)
			continue
		}

		cv.queryCacheMu.Lock()
		cv.queryCache[query] = queryCacheEntry{embedding: cloneFloat32Slice(embedding), timestamp: time.Now()}
		cv.queryCacheMu.Unlock()
	}
	cv.logger.Info("Preloaded query embedding cache", "queries", len(queries))
}

// buildContentString safely concatenates concept and content, handling empty fields.
func buildContentString(concept, content string) string {
	concept = strings.TrimSpace(concept)
	content = strings.TrimSpace(content)
	switch {
	case concept == "" && content == "":
		return ""
	case concept == "":
		return content
	case content == "":
		return concept
	default:
		return concept + "\n\n" + content
	}
}
