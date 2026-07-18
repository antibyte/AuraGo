package memory

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/config"

	chromem "github.com/philippgille/chromem-go"
)

func markTestVectorDBReady(cv *ChromemVectorDB) {
	if cv != nil {
		cv.ready.Store(true)
	}
}

func newTestChromemVectorDB(t *testing.T, embeddingFunc chromem.EmbeddingFunc) *ChromemVectorDB {
	t.Helper()
	db := chromem.NewDB()
	collection, err := db.GetOrCreateCollection("aurago_memories", nil, embeddingFunc)
	if err != nil {
		t.Fatalf("GetOrCreateCollection: %v", err)
	}
	cv := &ChromemVectorDB{
		db:                     db,
		collection:             collection,
		logger:                 slog.New(slog.NewTextHandler(io.Discard, nil)),
		embeddingFunc:          embeddingFunc,
		embeddingFingerprint:   "test|embedding|3",
		queryCache:             make(map[string]queryCacheEntry),
		queryCacheTTL:          5 * time.Minute,
		dedupSem:               make(chan struct{}, 16),
		fileIndexerCollections: make(map[string]struct{}),
	}
	markTestVectorDBReady(cv)
	return cv
}

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

func TestTruncateArchiveItemContentPreservesUTF8(t *testing.T) {
	value := "界" + string(make([]byte, 20))
	got := truncateArchiveItemContent(value)
	if !utf8.ValidString(got) {
		t.Fatalf("invalid UTF-8: %q", got)
	}
	if len(got) > maxBatchItemBytes {
		t.Fatalf("len = %d, want <= %d", len(got), maxBatchItemBytes)
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

func TestWaitForWaitGroupTimeoutDoesNotBlockUntilWorkerFinishes(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Done()

	start := time.Now()
	if waitForWaitGroup(&wg, 20*time.Millisecond) {
		t.Fatal("waitForWaitGroup returned true for an unfinished worker")
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("waitForWaitGroup blocked for %v, want a bounded timeout", elapsed)
	}
}

func TestChromemVectorDBCloseReturnsTimeoutAndDisablesStoreWhenWorkersHang(t *testing.T) {
	oldWait := vectorDBCloseWait
	vectorDBCloseWait = 20 * time.Millisecond
	t.Cleanup(func() { vectorDBCloseWait = oldWait })

	var cv ChromemVectorDB
	cv.ready.Store(true)
	cv.indexingWg.Add(1)

	if err := cv.Close(); err == nil {
		t.Fatal("Close() error = nil, want timeout error")
	}
	if err := cv.requireReadyForStore(); !errors.Is(err, ErrVectorDBClosed) {
		t.Fatalf("requireReadyForStore() error = %v, want ErrVectorDBClosed after Close", err)
	}
	if cv.IsDisabled() {
		t.Fatal("IsDisabled() = true after Close, want embedding-disabled state unchanged")
	}

	cv.indexingWg.Done()
}

func TestValidateEmbeddingWithRetryStopsOnPermanentAuthError(t *testing.T) {
	var calls atomic.Int32
	embeddingErr := errors.New("401 unauthorized: invalid api key")
	_, err := validateEmbeddingWithRetry(context.Background(), func(_ context.Context, _ string) ([]float32, error) {
		calls.Add(1)
		return nil, embeddingErr
	}, 3, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("validateEmbeddingWithRetry error = nil, want auth error")
	}
	if calls.Load() != 1 {
		t.Fatalf("embedding calls = %d, want 1 for permanent auth error", calls.Load())
	}
}

func TestValidateEmbeddingWithRetryBackoffStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	start := time.Now()
	_, err := validateEmbeddingWithRetry(ctx, func(_ context.Context, _ string) ([]float32, error) {
		if calls.Add(1) == 1 {
			cancel()
		}
		return nil, errors.New("temporary embedding failure")
	}, 3, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("validateEmbeddingWithRetry error = %v, want context.Canceled", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("embedding calls = %d, want 1 after cancellation", calls.Load())
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("canceled validation took %v, want no retry backoff wait", elapsed)
	}
}

func TestNewChromemVectorDBDisabledDoesNotOpenPersistentDB(t *testing.T) {
	vectorDir := filepath.Join(t.TempDir(), "vectordb")
	cfg := &config.Config{}
	cfg.Directories.VectorDBDir = vectorDir
	cfg.Embeddings.Provider = "disabled"

	cv, err := NewChromemVectorDB(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewChromemVectorDB disabled: %v", err)
	}
	defer cv.Close()

	if !cv.IsDisabled() {
		t.Fatal("disabled VectorDB IsDisabled() = false")
	}
	if !cv.IsReady() {
		t.Fatal("disabled VectorDB IsReady() = false")
	}
	if cv.db != nil {
		t.Fatal("disabled VectorDB opened a persistent chromem DB")
	}
	if _, statErr := os.Stat(vectorDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("disabled VectorDB vector dir stat err = %v, want os.ErrNotExist", statErr)
	}
}

func TestBuildEmbeddingRuntimeFromConfigResolvesLegacyProviders(t *testing.T) {
	cfg := &config.Config{}
	cfg.Embeddings.Provider = "internal"
	cfg.Embeddings.Model = ""
	cfg.Embeddings.InternalModel = "internal-embedding"
	cfg.LLM.BaseURL = "https://llm.example/v1/"
	cfg.LLM.APIKey = "llm-key"

	internalRuntime := buildEmbeddingRuntimeFromConfig(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if internalRuntime.Disabled {
		t.Fatal("internal runtime disabled = true")
	}
	if internalRuntime.BaseURL != cfg.LLM.BaseURL {
		t.Fatalf("internal BaseURL = %q, want %q", internalRuntime.BaseURL, cfg.LLM.BaseURL)
	}
	if internalRuntime.APIKey != cfg.LLM.APIKey {
		t.Fatalf("internal APIKey = %q, want main LLM key", internalRuntime.APIKey)
	}
	if internalRuntime.Model != "internal-embedding" {
		t.Fatalf("internal Model = %q, want internal-embedding", internalRuntime.Model)
	}
	if internalRuntime.Fingerprint != buildEmbeddingFingerprint("internal", cfg.LLM.BaseURL, "internal-embedding") {
		t.Fatalf("internal Fingerprint = %q", internalRuntime.Fingerprint)
	}

	cfg.Embeddings.Provider = "external"
	cfg.Embeddings.BaseURL = ""
	cfg.Embeddings.ExternalURL = "http://ollama:11434/v1"
	cfg.Embeddings.Model = ""
	cfg.Embeddings.ExternalModel = "nomic-embed-text"
	cfg.Embeddings.APIKey = "external-key"
	externalRuntime := buildEmbeddingRuntimeFromConfig(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if externalRuntime.Disabled {
		t.Fatal("external runtime disabled = true")
	}
	if externalRuntime.BaseURL != cfg.Embeddings.ExternalURL {
		t.Fatalf("external BaseURL = %q, want %q", externalRuntime.BaseURL, cfg.Embeddings.ExternalURL)
	}
	if externalRuntime.Model != "nomic-embed-text" {
		t.Fatalf("external Model = %q, want nomic-embed-text", externalRuntime.Model)
	}
	if externalRuntime.Fingerprint != buildEmbeddingFingerprint("external", cfg.Embeddings.ExternalURL, "nomic-embed-text") {
		t.Fatalf("external Fingerprint = %q", externalRuntime.Fingerprint)
	}
}

func TestBuildEmbeddingRuntimeFromConfigPreservesProviderStatus(t *testing.T) {
	cfg := &config.Config{}
	cfg.Embeddings.Provider = "openrouter-embeddings"
	cfg.Embeddings.BaseURL = "https://openrouter.example/v1"
	cfg.Embeddings.APIKey = "test-key"
	cfg.Embeddings.Model = "qwen/qwen3-embedding-8b"

	runtime := buildEmbeddingRuntimeFromConfig(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if runtime.Embedder == nil {
		t.Fatal("configured provider embedder is nil")
	}
	status := runtime.Embedder.Status()
	if status.Provider != cfg.Embeddings.Provider || status.ModelID != cfg.Embeddings.Model {
		t.Fatalf("configured provider status = %#v", status)
	}
}

func TestBuildEmbeddingRuntimeFromConfigDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Embeddings.Provider = "disabled"
	runtime := buildEmbeddingRuntimeFromConfig(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if !runtime.Disabled {
		t.Fatal("disabled runtime Disabled = false")
	}
	if runtime.Provider != "disabled" {
		t.Fatalf("disabled Provider = %q, want disabled", runtime.Provider)
	}
	if runtime.EmbeddingFunc == nil {
		t.Fatal("disabled runtime EmbeddingFunc = nil")
	}
}

func TestSearchSimilarScoredContextHonorsCanceledContext(t *testing.T) {
	var calls atomic.Int32
	cv := newTestChromemVectorDB(t, func(_ context.Context, _ string) ([]float32, error) {
		calls.Add(1)
		return []float32{1, 0, 0}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := cv.SearchSimilarScoredContext(ctx, "query", 3)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SearchSimilarScoredContext error = %v, want context.Canceled", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("embedding calls = %d, want 0 for pre-canceled context", calls.Load())
	}
}

func TestStoreBatchTruncatesContentWithoutSplittingUTF8(t *testing.T) {
	value := strings.Repeat("a", maxBatchItemBytes-1) + "ä" + "tail"
	got := truncateArchiveItemContent(value)

	if len(got) > maxBatchItemBytes {
		t.Fatalf("truncated length = %d, want <= %d", len(got), maxBatchItemBytes)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("truncated content is not valid UTF-8")
	}
	if strings.HasSuffix(got, "\xc3") {
		t.Fatalf("truncated content ended with partial UTF-8 byte")
	}
}

func TestStoreDocumentWithDomainDuplicateReturnsExistingDocID(t *testing.T) {
	cv := newTestChromemVectorDB(t, func(_ context.Context, text string) ([]float32, error) {
		if strings.Contains(text, "backup") {
			return []float32{1, 0, 0}, nil
		}
		return []float32{0, 1, 0}, nil
	})

	firstIDs, err := cv.StoreDocumentWithDomain("backup policy", "Use rsync for backups", "ops")
	if err != nil {
		t.Fatalf("StoreDocumentWithDomain first: %v", err)
	}
	if len(firstIDs) != 1 {
		t.Fatalf("first ids = %v, want one id", firstIDs)
	}

	secondIDs, err := cv.StoreDocumentWithDomain("backup policy", "Use rsync for backups", "ops")
	if err != nil {
		t.Fatalf("StoreDocumentWithDomain duplicate: %v", err)
	}
	if len(secondIDs) != 1 || secondIDs[0] != firstIDs[0] {
		t.Fatalf("duplicate ids = %v, want existing id %q", secondIDs, firstIDs[0])
	}
}

func TestGetQueryEmbeddingReturnsCanceledCallerWithoutWaitingForSingleflight(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	cv := newTestChromemVectorDB(t, func(ctx context.Context, _ string) ([]float32, error) {
		calls.Add(1)
		select {
		case <-started:
		default:
			close(started)
		}
		select {
		case <-release:
			return []float32{1, 0, 0}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	firstDone := make(chan error, 1)
	go func() {
		_, err := cv.getQueryEmbedding(context.Background(), "same query")
		firstDone <- err
	}()
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	canceledDone := make(chan error, 1)
	go func() {
		_, err := cv.getQueryEmbedding(ctx, "same query")
		canceledDone <- err
	}()

	select {
	case err := <-canceledDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled caller err = %v, want context.Canceled", err)
		}
	case <-time.After(100 * time.Millisecond):
		close(release)
		t.Fatal("canceled caller waited for in-flight singleflight request")
	}

	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first getQueryEmbedding: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("embedding calls = %d, want one shared in-flight request", got)
	}
}

func TestGetQueryEmbeddingSharedRequestSurvivesFirstCallerCancel(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	cv := newTestChromemVectorDB(t, func(ctx context.Context, _ string) ([]float32, error) {
		calls.Add(1)
		select {
		case <-started:
		default:
			close(started)
		}
		select {
		case <-release:
			return []float32{1, 0, 0}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	firstCtx, firstCancel := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() {
		_, err := cv.getQueryEmbedding(firstCtx, "shared query")
		firstDone <- err
	}()
	<-started

	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		close(secondEntered)
		embedding, err := cv.getQueryEmbedding(context.Background(), "shared query")
		if err == nil && len(embedding) != 3 {
			err = errors.New("unexpected embedding length")
		}
		secondDone <- err
	}()
	<-secondEntered
	time.Sleep(25 * time.Millisecond)

	firstCancel()
	select {
	case err := <-firstDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("first caller err = %v, want context.Canceled", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("first caller did not return after cancellation")
	}

	close(release)
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatalf("second caller err = %v, want shared request success", err)
		}
	case <-time.After(time.Second):
		t.Fatal("second caller did not receive shared embedding result")
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("embedding calls = %d, want one shared in-flight request", got)
	}
	if _, err := cv.getQueryEmbedding(context.Background(), "shared query"); err != nil {
		t.Fatalf("cached getQueryEmbedding: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("embedding calls after cache hit = %d, want 1", got)
	}
}

func TestGetQueryEmbeddingReturnsCacheCopies(t *testing.T) {
	var calls atomic.Int32
	cv := newTestChromemVectorDB(t, func(_ context.Context, _ string) ([]float32, error) {
		calls.Add(1)
		return []float32{1, 2, 3}, nil
	})

	first, err := cv.getQueryEmbedding(context.Background(), "copy me")
	if err != nil {
		t.Fatalf("first getQueryEmbedding: %v", err)
	}
	first[0] = 99

	second, err := cv.getQueryEmbedding(context.Background(), "copy me")
	if err != nil {
		t.Fatalf("second getQueryEmbedding: %v", err)
	}
	if second[0] != 1 {
		t.Fatalf("cached embedding was mutated through caller slice: got %v", second)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("embedding calls = %d, want cached second lookup", got)
	}
	second[1] = 88

	third, err := cv.getQueryEmbedding(context.Background(), "copy me")
	if err != nil {
		t.Fatalf("third getQueryEmbedding: %v", err)
	}
	if third[1] != 2 {
		t.Fatalf("cached embedding was mutated by second caller slice: got %v", third)
	}
}

func TestStoreDocumentWithEmbeddingValidatesVectors(t *testing.T) {
	cv := newTestChromemVectorDB(t, func(_ context.Context, _ string) ([]float32, error) {
		return []float32{1, 0, 0}, nil
	})
	cv.embeddingDimension.Store(3)

	tests := []struct {
		name      string
		embedding []float32
		wantErr   string
	}{
		{name: "empty", embedding: nil, wantErr: "embedding vector is empty"},
		{name: "nan", embedding: []float32{1, float32(math.NaN()), 0}, wantErr: "embedding vector contains invalid value"},
		{name: "inf", embedding: []float32{1, float32(math.Inf(1)), 0}, wantErr: "embedding vector contains invalid value"},
		{name: "dimension mismatch", embedding: []float32{1, 0}, wantErr: "embedding dimension mismatch"},
	}

	for _, tt := range tests {
		t.Run("default_"+tt.name, func(t *testing.T) {
			_, err := cv.StoreDocumentWithEmbedding("concept", "content", tt.embedding)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("StoreDocumentWithEmbedding err = %v, want %q", err, tt.wantErr)
			}
		})
		t.Run("collection_"+tt.name, func(t *testing.T) {
			_, err := cv.StoreDocumentWithEmbeddingInCollection("concept", "content", tt.embedding, "files")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("StoreDocumentWithEmbeddingInCollection err = %v, want %q", err, tt.wantErr)
			}
		})
	}

	if _, err := cv.StoreDocumentWithEmbedding("concept", "content", []float32{1, 0, 0}); err != nil {
		t.Fatalf("StoreDocumentWithEmbedding valid vector: %v", err)
	}
	if _, err := cv.StoreDocumentWithEmbeddingInCollection("concept", "content", []float32{1, 0, 0}, "files"); err != nil {
		t.Fatalf("StoreDocumentWithEmbeddingInCollection valid vector: %v", err)
	}
}

func TestPreloadCacheStoresCopyForFutureQueryHits(t *testing.T) {
	source := []float32{0.2, 0.4, 0.6}
	cv := newTestChromemVectorDB(t, func(_ context.Context, _ string) ([]float32, error) {
		return source, nil
	})

	cv.PreloadCache([]string{"preloaded"})
	source[0] = 99

	first, err := cv.getQueryEmbedding(context.Background(), "preloaded")
	if err != nil {
		t.Fatalf("getQueryEmbedding first: %v", err)
	}
	first[1] = 88

	second, err := cv.getQueryEmbedding(context.Background(), "preloaded")
	if err != nil {
		t.Fatalf("getQueryEmbedding second: %v", err)
	}
	if second[0] != 0.2 || second[1] != 0.4 || second[2] != 0.6 {
		t.Fatalf("cached embedding = %v, want original preloaded values", second)
	}
}

func TestStoreBatchDoesNotSpawnOneGoroutinePerItem(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var active atomic.Int32
	cv := newTestChromemVectorDB(t, func(ctx context.Context, text string) ([]float32, error) {
		if strings.Contains(text, "concept") {
			select {
			case <-started:
			default:
				close(started)
			}
			active.Add(1)
			defer active.Add(-1)
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return []float32{1, 0, 0}, nil
	})

	items := make([]ArchiveItem, 96)
	for i := range items {
		items[i] = ArchiveItem{Concept: "concept-" + strconv.Itoa(i), Content: "content-" + strconv.Itoa(i)}
	}

	before := runtime.NumGoroutine()
	done := make(chan error, 1)
	go func() {
		_, err := cv.StoreBatch(items)
		done <- err
	}()
	<-started
	time.Sleep(50 * time.Millisecond)
	during := runtime.NumGoroutine()
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("StoreBatch: %v", err)
	}

	if delta := during - before; delta > 40 {
		t.Fatalf("StoreBatch spawned too many goroutines: delta=%d, want <= 40", delta)
	}
}

func TestIndexDirectoryReportsFileIndexReadError(t *testing.T) {
	cv := newTestChromemVectorDB(t, func(_ context.Context, _ string) ([]float32, error) {
		return []float32{0.1, 0.2, 0.3}, nil
	})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("hello index"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}

	stm, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.Close(); err != nil {
		t.Fatalf("Close stm: %v", err)
	}

	err = cv.IndexDirectory(dir, "docs", stm, false)
	if err == nil {
		t.Fatal("expected file index read error")
	}
	if !strings.Contains(err.Error(), "get file index") {
		t.Fatalf("error = %v, want get file index context", err)
	}
}

func TestIndexDirectoryStoresTrackedDocIDs(t *testing.T) {
	cv := newTestChromemVectorDB(t, func(_ context.Context, _ string) ([]float32, error) {
		return []float32{0.1, 0.2, 0.3}, nil
	})
	dir := t.TempDir()
	path := filepath.Join(dir, "guide.md")
	if err := os.WriteFile(path, []byte("hello index"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}

	stm, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	if err := cv.IndexDirectory(dir, "docs", stm, true); err != nil {
		t.Fatalf("IndexDirectory: %v", err)
	}

	docIDs, err := stm.GetFileEmbeddingDocIDs(path, "docs")
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs: %v", err)
	}
	if len(docIDs) == 0 {
		t.Fatal("expected IndexDirectory to persist generated document IDs")
	}
}

func TestIndexDirectoryClearsTrackingAfterAddFailure(t *testing.T) {
	cv := newTestChromemVectorDB(t, func(_ context.Context, _ string) ([]float32, error) {
		return nil, errors.New("embedding failed")
	})
	dir := t.TempDir()
	path := filepath.Join(dir, "guide.md")
	if err := os.WriteFile(path, []byte("hello index"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}
	modTime := time.Now().UTC().Truncate(time.Second)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	stm, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()
	if err := stm.UpdateFileIndexWithDocs(path, "docs", modTime, []string{"old-doc"}); err != nil {
		t.Fatalf("UpdateFileIndexWithDocs: %v", err)
	}

	err = cv.IndexDirectory(dir, "docs", stm, true)
	if err == nil {
		t.Fatal("expected IndexDirectory add failure")
	}

	lastIndexed, err := stm.GetFileIndex(path, "docs")
	if err != nil {
		t.Fatalf("GetFileIndex: %v", err)
	}
	if !lastIndexed.IsZero() {
		t.Fatalf("expected failed file tracking to be cleared, got %v", lastIndexed)
	}
}

func TestIndexDirectoryRemovesDeletedTrackedMarkdownFiles(t *testing.T) {
	cv := newTestChromemVectorDB(t, func(_ context.Context, _ string) ([]float32, error) {
		return []float32{0.1, 0.2, 0.3}, nil
	})
	dir := t.TempDir()
	path := filepath.Join(dir, "guide.md")
	if err := os.WriteFile(path, []byte("hello index"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}

	stm, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	if err := cv.IndexDirectory(dir, "docs", stm, true); err != nil {
		t.Fatalf("IndexDirectory initial: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if err := cv.IndexDirectory(dir, "docs", stm, false); err != nil {
		t.Fatalf("IndexDirectory cleanup: %v", err)
	}
	paths, err := stm.ListIndexedFiles("docs")
	if err != nil {
		t.Fatalf("ListIndexedFiles: %v", err)
	}
	if len(paths) != 0 {
		t.Fatalf("expected deleted file tracking to be removed, got %v", paths)
	}
}

func TestIndexDirectoryReindexesWhenMarkdownContentChangesWithSameModTime(t *testing.T) {
	var calls atomic.Int32
	cv := newTestChromemVectorDB(t, func(_ context.Context, _ string) ([]float32, error) {
		calls.Add(1)
		return []float32{0.1, 0.2, 0.3}, nil
	})
	dir := t.TempDir()
	path := filepath.Join(dir, "guide.md")
	modTime := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	if err := os.WriteFile(path, []byte("first markdown body"), 0o644); err != nil {
		t.Fatalf("write first guide: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes first guide: %v", err)
	}

	stm, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	if err := cv.IndexDirectory(dir, "docs", stm, false); err != nil {
		t.Fatalf("IndexDirectory initial: %v", err)
	}
	initialCalls := calls.Load()
	if initialCalls == 0 {
		t.Fatal("expected initial indexing to call embedding function")
	}

	if err := os.WriteFile(path, []byte("second markdown body with same timestamp"), 0o644); err != nil {
		t.Fatalf("write second guide: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes second guide: %v", err)
	}

	if err := cv.IndexDirectory(dir, "docs", stm, false); err != nil {
		t.Fatalf("IndexDirectory second: %v", err)
	}
	if got := calls.Load(); got <= initialCalls {
		t.Fatalf("embedding calls = %d after content change, want > %d", got, initialCalls)
	}
}

func TestIndexDirectorySkipsMarkdownWhenOnlyModTimeChanges(t *testing.T) {
	var calls atomic.Int32
	cv := newTestChromemVectorDB(t, func(_ context.Context, _ string) ([]float32, error) {
		calls.Add(1)
		return []float32{0.1, 0.2, 0.3}, nil
	})
	dir := t.TempDir()
	path := filepath.Join(dir, "guide.md")
	modTime := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	content := []byte("same markdown body")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes guide: %v", err)
	}

	stm, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	if err := cv.IndexDirectory(dir, "docs", stm, false); err != nil {
		t.Fatalf("IndexDirectory initial: %v", err)
	}
	initialCalls := calls.Load()
	if initialCalls == 0 {
		t.Fatal("expected initial indexing to call embedding function")
	}

	touchedModTime := modTime.Add(time.Minute)
	if err := os.Chtimes(path, touchedModTime, touchedModTime); err != nil {
		t.Fatalf("Chtimes touched guide: %v", err)
	}

	if err := cv.IndexDirectory(dir, "docs", stm, false); err != nil {
		t.Fatalf("IndexDirectory touched: %v", err)
	}
	if got := calls.Load(); got != initialCalls {
		t.Fatalf("embedding calls = %d after mtime-only change, want %d", got, initialCalls)
	}
}

func TestIndexDirectorySkipsSymlinkedMarkdownAndCleansTracking(t *testing.T) {
	var calls atomic.Int32
	cv := newTestChromemVectorDB(t, func(_ context.Context, _ string) ([]float32, error) {
		calls.Add(1)
		return []float32{0.1, 0.2, 0.3}, nil
	})
	dir := t.TempDir()
	outsideDir := t.TempDir()
	target := filepath.Join(outsideDir, "outside.md")
	if err := os.WriteFile(target, []byte("outside markdown"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	linkPath := filepath.Join(dir, "linked.md")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}

	stm, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()
	if err := stm.UpdateFileIndexWithDocsAndState(linkPath, "docs", time.Now().UTC(), "old-hash", "old-fingerprint", []string{"old-doc"}); err != nil {
		t.Fatalf("UpdateFileIndexWithDocsAndState: %v", err)
	}

	if err := cv.IndexDirectory(dir, "docs", stm, false); err != nil {
		t.Fatalf("IndexDirectory: %v", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("embedding calls = %d, want 0 for symlinked markdown", got)
	}
	paths, err := stm.ListIndexedFiles("docs")
	if err != nil {
		t.Fatalf("ListIndexedFiles: %v", err)
	}
	if len(paths) != 0 {
		t.Fatalf("tracked files after symlink cleanup = %v, want none", paths)
	}
}

func TestIndexDirectoryAfterCloseReturnsVectorDBClosed(t *testing.T) {
	cv := newTestChromemVectorDB(t, func(_ context.Context, _ string) ([]float32, error) {
		return []float32{0.1, 0.2, 0.3}, nil
	})
	dir := t.TempDir()
	if err := cv.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	err := cv.IndexDirectory(dir, "docs", nil, false)
	if !errors.Is(err, ErrVectorDBClosed) {
		t.Fatalf("IndexDirectory err = %v, want ErrVectorDBClosed", err)
	}
	if cv.IsReady() {
		t.Fatal("VectorDB should not report ready after Close")
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

func TestCountDoesNotCreateMissingCollections(t *testing.T) {
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
		fileIndexerCollections: map[string]struct{}{"custom_docs": {}},
		logger:                 slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if err := collection.AddDocument(context.Background(), chromem.Document{ID: "mem-1", Content: "test memory"}); err != nil {
		t.Fatalf("add doc: %v", err)
	}

	if got := cv.Count(); got != 1 {
		t.Fatalf("Count() = %d, want only existing default collection", got)
	}
	collections := db.ListCollections()
	for _, name := range []string{"tool_guides", "documentation", "file_index", "custom_docs"} {
		if _, ok := collections[name]; ok {
			t.Fatalf("Count() created missing collection %q", name)
		}
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
	markTestVectorDBReady(cv)

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

func TestSearchToolGuides_DisabledReturnsError(t *testing.T) {
	cv := &ChromemVectorDB{}
	markTestVectorDBReady(cv)
	cv.disabled.Store(true)

	_, err := cv.SearchToolGuides("docker", 2)
	if !errors.Is(err, ErrVectorDBDisabled) {
		t.Fatalf("SearchToolGuides err = %v, want ErrVectorDBDisabled", err)
	}
}

func TestSearchToolGuides_NotReadyReturnsError(t *testing.T) {
	cv := &ChromemVectorDB{}

	_, err := cv.SearchToolGuides("docker", 2)
	if !errors.Is(err, ErrVectorDBNotReady) {
		t.Fatalf("SearchToolGuides err = %v, want ErrVectorDBNotReady", err)
	}
}

func TestSearchToolGuides_EmptyQueryReturnsNil(t *testing.T) {
	cv := &ChromemVectorDB{}
	markTestVectorDBReady(cv)

	paths, err := cv.SearchToolGuides("", 2)
	if err != nil {
		t.Fatalf("SearchToolGuides empty query err = %v, want nil", err)
	}
	if paths != nil {
		t.Fatalf("paths = %v, want nil", paths)
	}
}

func TestSearchSimilarScored_DisabledReturnsError(t *testing.T) {
	cv := &ChromemVectorDB{}
	markTestVectorDBReady(cv)
	cv.disabled.Store(true)

	_, err := cv.SearchSimilarScored("query", 3)
	if !errors.Is(err, ErrVectorDBDisabled) {
		t.Fatalf("SearchSimilarScored err = %v, want ErrVectorDBDisabled", err)
	}

	_, err = cv.SearchMemoriesOnlyScored("query", 3)
	if !errors.Is(err, ErrVectorDBDisabled) {
		t.Fatalf("SearchMemoriesOnlyScored err = %v, want ErrVectorDBDisabled", err)
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
	markTestVectorDBReady(cv)

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
	markTestVectorDBReady(cv)

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

func TestChromemVectorDBBlocksStoreBeforeReady(t *testing.T) {
	cv := &ChromemVectorDB{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	_, err := cv.StoreDocument("concept", "content")
	if !errors.Is(err, ErrVectorDBNotReady) {
		t.Fatalf("StoreDocument err = %v, want ErrVectorDBNotReady", err)
	}
}
