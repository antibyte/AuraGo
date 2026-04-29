package memory

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"

	chromem "github.com/philippgille/chromem-go"
)

func TestExtractSimilarityScore(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"standard format", "[Similarity: 0.95] some content here", 0.95},
		{"low similarity", "[Similarity: 0.32] other text", 0.32},
		{"perfect match", "[Similarity: 1.00] exact", 1.0},
		{"with domain tag", "[Similarity: 0.87] [tool_guides] docker help", 0.87},
		{"no similarity prefix", "just plain text", 0},
		{"malformed bracket", "[Similarity: bad] text", 0},
		{"empty string", "", 0},
		{"missing closing bracket", "[Similarity: 0.50 text", 0},
		{"negative clamped to 0", "[Similarity: -0.50] text", 0},
		{"above 1 clamped to 1", "[Similarity: 1.50] text", 1.0},
		{"exactly 0", "[Similarity: 0.00] text", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSimilarityScore(tt.input)
			if got != tt.expected {
				t.Errorf("ExtractSimilarityScore(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCalculateBatchTimeout(t *testing.T) {
	tests := []struct {
		name     string
		docCount int
		minSecs  int
		maxSecs  int
	}{
		{"single doc", 1, 30, 35},
		{"10 docs", 10, 49, 51},
		{"50 docs", 50, 129, 131},
		{"100 docs", 100, 229, 231},
		{"1000 docs - capped", 1000, 299, 301},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateBatchTimeout(tt.docCount)
			gotSecs := int(got.Seconds())
			if gotSecs < tt.minSecs || gotSecs > tt.maxSecs {
				t.Errorf("calculateBatchTimeout(%d) = %v (%ds), want between %ds and %ds",
					tt.docCount, got, gotSecs, tt.minSecs, tt.maxSecs)
			}
		})
	}

	// Cap at 5 minutes
	got := calculateBatchTimeout(10000)
	if got != 5*time.Minute {
		t.Errorf("calculateBatchTimeout(10000) = %v, want 5m0s (capped)", got)
	}
}

func TestQueryCacheEntry(t *testing.T) {
	entry := queryCacheEntry{
		embedding: []float32{0.1, 0.2, 0.3},
		timestamp: time.Now(),
	}

	if len(entry.embedding) != 3 {
		t.Errorf("expected 3 dimensions, got %d", len(entry.embedding))
	}

	if time.Since(entry.timestamp) > time.Second {
		t.Error("timestamp should be recent")
	}
}

func TestIsLocalEmbeddingProvider(t *testing.T) {
	cloudCfg := config.Config{}
	cloudCfg.Embeddings.ProviderType = "openrouter"
	cloudCfg.Embeddings.BaseURL = "https://openrouter.ai/api/v1"

	ollamaCfg := config.Config{}
	ollamaCfg.Embeddings.ProviderType = "ollama"
	ollamaCfg.Embeddings.BaseURL = "http://127.0.0.1:11435/v1"

	tests := []struct {
		name string
		cfg  config.Config
		want bool
	}{
		{name: "openrouter cloud provider is not local", cfg: cloudCfg, want: false},
		{name: "ollama provider type is local", cfg: ollamaCfg, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLocalEmbeddingProvider(&tt.cfg); got != tt.want {
				t.Fatalf("isLocalEmbeddingProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestExtractSimilarityScoreOnRawContent is a regression test for BUG-02.
// SearchMemoriesOnly and SearchSimilar return raw document content (without a
// "[Similarity: x.xx]" prefix), so passing their output to ExtractSimilarityScore
// always returned 0 — effectively bypassing deduplication.
// The fix replaces that pattern with searchTopSimilarityScore which returns the
// raw float32 similarity directly.  This test documents the incorrect old behaviour
// so a future refactor cannot silently re-introduce it.
func TestExtractSimilarityScoreOnRawContent(t *testing.T) {
	rawContents := []string{
		"This is a document about Go memory management.",
		"Docker container started successfully.",
		"The user asked about filesystem operations.",
	}
	for _, raw := range rawContents {
		if got := ExtractSimilarityScore(raw); got != 0 {
			t.Errorf("ExtractSimilarityScore on raw content %q = %v, want 0 (no prefix)", raw, got)
		}
	}
}

// TestCountIncludesFileIndexerCollections verifies that Count() includes
// documents from the default collection, tool_guides, documentation,
// and registered FileIndexer collections.
func TestCountIncludesFileIndexerCollections(t *testing.T) {
	embeddingFunc := func(_ context.Context, _ string) ([]float32, error) {
		return []float32{0.1, 0.2, 0.3}, nil
	}

	db := chromem.NewDB()
	collection, err := db.GetOrCreateCollection("aurago_memories", nil, embeddingFunc)
	if err != nil {
		t.Fatalf("GetOrCreateCollection: %v", err)
	}

	cv := &ChromemVectorDB{
		db:                     db,
		collection:             collection,
		embeddingFunc:          embeddingFunc,
		fileIndexerCollections: make(map[string]struct{}),
	}

	// Add 1 doc to aurago_memories
	if err := collection.AddDocument(context.Background(), chromem.Document{
		ID:      "mem-1",
		Content: "test memory",
	}); err != nil {
		t.Fatalf("add doc to aurago_memories: %v", err)
	}

	// Add 1 doc to tool_guides
	tg, _ := db.GetOrCreateCollection("tool_guides", nil, embeddingFunc)
	_ = tg.AddDocument(context.Background(), chromem.Document{ID: "tg-1", Content: "tool guide"})

	// Add 1 doc to documentation
	doc, _ := db.GetOrCreateCollection("documentation", nil, embeddingFunc)
	_ = doc.AddDocument(context.Background(), chromem.Document{ID: "doc-1", Content: "documentation"})

	// Add 1 doc to file_index (default FileIndexer collection)
	fi, _ := db.GetOrCreateCollection("file_index", nil, embeddingFunc)
	_ = fi.AddDocument(context.Background(), chromem.Document{ID: "file-1", Content: "file index"})

	// Add 1 doc to a custom FileIndexer collection
	cv.registerFileIndexerCollection("custom_docs")
	ci, _ := db.GetOrCreateCollection("custom_docs", nil, embeddingFunc)
	_ = ci.AddDocument(context.Background(), chromem.Document{ID: "custom-1", Content: "custom doc"})

	got := cv.Count()
	want := 5
	if got != want {
		t.Errorf("Count() = %d, want %d", got, want)
	}
}

func TestSearchSimilarIncludesFileIndexerCollections(t *testing.T) {
	embeddingFunc := func(_ context.Context, text string) ([]float32, error) {
		if strings.Contains(strings.ToLower(text), "krankenkasse") {
			return []float32{1, 0, 0}, nil
		}
		return []float32{0, 1, 0}, nil
	}

	db := chromem.NewDB()
	collection, err := db.GetOrCreateCollection("aurago_memories", nil, embeddingFunc)
	if err != nil {
		t.Fatalf("GetOrCreateCollection: %v", err)
	}

	cv := &ChromemVectorDB{
		db:                     db,
		collection:             collection,
		embeddingFunc:          embeddingFunc,
		fileIndexerCollections: make(map[string]struct{}),
		logger:                 slog.New(slog.NewTextHandler(io.Discard, nil)),
		queryCache:             make(map[string]queryCacheEntry),
		queryCacheTTL:          time.Minute,
	}

	fileCol, err := db.GetOrCreateCollection("file_index", nil, embeddingFunc)
	if err != nil {
		t.Fatalf("GetOrCreateCollection file_index: %v", err)
	}
	if err := fileCol.AddDocument(context.Background(), chromem.Document{
		ID:      "file-krankenkasse",
		Content: "Krankenkasse PDF: Beitragserstattung und Versicherungsnummer",
		Metadata: map[string]string{
			"timestamp": "0",
		},
	}); err != nil {
		t.Fatalf("AddDocument file_index: %v", err)
	}

	results, ids, err := cv.SearchSimilar("krankenkasse beitrag", 5, "tool_guides", "documentation")
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != 1 || ids[0] != "file-krankenkasse" {
		t.Fatalf("results=%v ids=%v, want file_index hit", results, ids)
	}
	if !strings.Contains(results[0], "[file_index]") {
		t.Fatalf("result = %q, want file_index source hint", results[0])
	}
}

// TestSearchTopSimilarityScoreDisabled verifies that searchTopSimilarityScore
// returns 0 safely when the VectorDB is disabled (e.g. no embedding model configured).
func TestSearchTopSimilarityScoreDisabled(t *testing.T) {
	cv := &ChromemVectorDB{}
	cv.disabled.Store(true)
	if got := cv.searchTopSimilarityScore("any concept"); got != 0 {
		t.Errorf("searchTopSimilarityScore on disabled VectorDB = %v, want 0", got)
	}
}

// TestStoreDocumentInCollectionPersistsCollectionMetadata verifies that
// collection-aware storage persists the collection name in document metadata.
func TestStoreDocumentInCollectionPersistsCollectionMetadata(t *testing.T) {
	embeddingFunc := func(_ context.Context, _ string) ([]float32, error) {
		return []float32{0.1, 0.2, 0.3}, nil
	}

	db := chromem.NewDB()
	collection, err := db.GetOrCreateCollection("aurago_memories", nil, embeddingFunc)
	if err != nil {
		t.Fatalf("GetOrCreateCollection: %v", err)
	}

	cv := &ChromemVectorDB{
		db:                     db,
		collection:             collection,
		embeddingFunc:          embeddingFunc,
		embeddingFingerprint:   "provider|model|dim",
		fileIndexerCollections: make(map[string]struct{}),
		logger:                 slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Store a small document in a custom collection
	ids, err := cv.StoreDocumentInCollection("test concept", "test content", "custom_collection")
	if err != nil {
		t.Fatalf("StoreDocumentInCollection: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 doc ID, got %d", len(ids))
	}

	// Verify the document in the custom collection has the collection metadata
	col, err := db.GetOrCreateCollection("custom_collection", nil, embeddingFunc)
	if err != nil {
		t.Fatalf("GetOrCreateCollection custom_collection: %v", err)
	}
	doc, err := col.GetByID(context.Background(), ids[0])
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got, want := doc.Metadata["collection"], "custom_collection"; got != want {
		t.Errorf("metadata collection = %q, want %q", got, want)
	}
	if got, want := doc.Metadata["source_type"], "file_indexer"; got != want {
		t.Errorf("metadata source_type = %q, want %q", got, want)
	}
	if got, want := doc.Metadata["embedding_fingerprint"], "provider|model|dim"; got != want {
		t.Errorf("metadata embedding_fingerprint = %q, want %q", got, want)
	}
}

// TestStoreDocumentWithEmbeddingInCollectionPersistsCollectionMetadata verifies that
// collection-aware multimodal storage persists the collection name in document metadata.
func TestStoreDocumentWithEmbeddingInCollectionPersistsCollectionMetadata(t *testing.T) {
	embeddingFunc := func(_ context.Context, _ string) ([]float32, error) {
		return []float32{0.1, 0.2, 0.3}, nil
	}

	db := chromem.NewDB()
	collection, err := db.GetOrCreateCollection("aurago_memories", nil, embeddingFunc)
	if err != nil {
		t.Fatalf("GetOrCreateCollection: %v", err)
	}

	cv := &ChromemVectorDB{
		db:                     db,
		collection:             collection,
		embeddingFunc:          embeddingFunc,
		fileIndexerCollections: make(map[string]struct{}),
		logger:                 slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	id, err := cv.StoreDocumentWithEmbeddingInCollection("test concept", "test content", []float32{0.4, 0.5, 0.6}, "mm_custom_collection")
	if err != nil {
		t.Fatalf("StoreDocumentWithEmbeddingInCollection: %v", err)
	}

	col, err := db.GetOrCreateCollection("mm_custom_collection", nil, embeddingFunc)
	if err != nil {
		t.Fatalf("GetOrCreateCollection mm_custom_collection: %v", err)
	}
	doc, err := col.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got, want := doc.Metadata["collection"], "mm_custom_collection"; got != want {
		t.Errorf("metadata collection = %q, want %q", got, want)
	}
	if got, want := doc.Metadata["source_type"], "file_indexer"; got != want {
		t.Errorf("metadata source_type = %q, want %q", got, want)
	}
	if got, want := doc.Metadata["multimodal"], "true"; got != want {
		t.Errorf("metadata multimodal = %q, want %q", got, want)
	}
}
