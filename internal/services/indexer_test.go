package services

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

type fakeIndexerVectorDB struct {
	mu           sync.Mutex
	storeOnce    sync.Once
	nextID       int
	deleted      []string
	stored       []string
	disabled     bool
	fingerprint  string
	storeErr     error
	storeStarted chan struct{}
	releaseStore chan struct{}
}

func (f *fakeIndexerVectorDB) StoreDocument(concept, content string) ([]string, error) {
	f.mu.Lock()
	f.nextID++
	docID := fmt.Sprintf("doc-%d", f.nextID)
	if f.storeStarted != nil {
		f.storeOnce.Do(func() { close(f.storeStarted) })
	}
	f.mu.Unlock()
	if f.releaseStore != nil {
		<-f.releaseStore
	}
	if f.storeErr != nil {
		return nil, f.storeErr
	}
	f.mu.Lock()
	f.stored = append(f.stored, content)
	f.mu.Unlock()
	return []string{docID}, nil
}

func (f *fakeIndexerVectorDB) StoreDocumentInCollection(concept, content, collection string) ([]string, error) {
	return f.StoreDocument(concept, content)
}

func (f *fakeIndexerVectorDB) StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	return fmt.Sprintf("doc-%d", f.nextID), nil
}

func (f *fakeIndexerVectorDB) StoreDocumentWithEmbeddingInCollection(concept, content string, embedding []float32, collection string) (string, error) {
	return f.StoreDocumentWithEmbedding(concept, content, embedding)
}

func (f *fakeIndexerVectorDB) StoreBatch(items []memory.ArchiveItem) ([]string, error) {
	return nil, nil
}

func (f *fakeIndexerVectorDB) SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error) {
	return nil, nil, nil
}

func (f *fakeIndexerVectorDB) SearchMemoriesOnly(query string, topK int) ([]string, []string, error) {
	return nil, nil, nil
}

func (f *fakeIndexerVectorDB) GetByID(id string) (string, error) { return "", nil }
func (f *fakeIndexerVectorDB) GetByIDFromCollection(id, collection string) (string, error) {
	return "", nil
}

func (f *fakeIndexerVectorDB) DeleteDocument(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeIndexerVectorDB) DeleteDocumentFromCollection(id, collection string) error {
	return f.DeleteDocument(id)
}

func (f *fakeIndexerVectorDB) Count() int       { return 0 }
func (f *fakeIndexerVectorDB) IsDisabled() bool { return f.disabled }
func (f *fakeIndexerVectorDB) IsReady() bool    { return true }
func (f *fakeIndexerVectorDB) Close() error     { return nil }
func (f *fakeIndexerVectorDB) EmbeddingFingerprint() string {
	return f.fingerprint
}

func (f *fakeIndexerVectorDB) StoreCheatsheet(id, name, content string, attachments ...string) error {
	return nil
}

func (f *fakeIndexerVectorDB) DeleteCheatsheet(id string) error {
	return nil
}

func (f *fakeIndexerVectorDB) RegisterCollections(collections []string) {}

func TestFileIndexerReplacesTrackedEmbeddingsOnReindex(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("first version"), 0644); err != nil {
		t.Fatalf("WriteFile first version: %v", err)
	}
	firstMod := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Second)
	if err := os.Chtimes(path, firstMod, firstMod); err != nil {
		t.Fatalf("Chtimes first version: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}
	cfg.Indexing.PollIntervalSeconds = 60

	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	_, indexed, errs := fi.scanDirectory(context.Background(), dir, "file_index")
	if indexed != 1 {
		t.Fatalf("first scan indexed = %d, want 1", indexed)
	}
	if len(errs) != 0 {
		t.Fatalf("first scan errors = %v", errs)
	}

	firstIDs, err := stm.GetFileEmbeddingDocIDs(path, "file_index")
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs first: %v", err)
	}
	if !reflect.DeepEqual(firstIDs, []string{"doc-1"}) {
		t.Fatalf("first tracked IDs = %v, want [doc-1]", firstIDs)
	}

	if err := os.WriteFile(path, []byte("second version"), 0644); err != nil {
		t.Fatalf("WriteFile second version: %v", err)
	}
	secondMod := firstMod.Add(2 * time.Minute)
	if err := os.Chtimes(path, secondMod, secondMod); err != nil {
		t.Fatalf("Chtimes second version: %v", err)
	}

	_, indexed, errs = fi.scanDirectory(context.Background(), dir, "file_index")
	if indexed != 1 {
		t.Fatalf("second scan indexed = %d, want 1", indexed)
	}
	if len(errs) != 0 {
		t.Fatalf("second scan errors = %v", errs)
	}

	if !reflect.DeepEqual(vdb.deleted, []string{"doc-1"}) {
		t.Fatalf("deleted docs = %v, want [doc-1]", vdb.deleted)
	}

	secondIDs, err := stm.GetFileEmbeddingDocIDs(path, "file_index")
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs second: %v", err)
	}
	if !reflect.DeepEqual(secondIDs, []string{"doc-2"}) {
		t.Fatalf("second tracked IDs = %v, want [doc-2]", secondIDs)
	}
}

func TestFileIndexerReindexesWhenContentChangesWithSameModTime(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	modTime := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Second)
	if err := os.WriteFile(path, []byte("first version"), 0644); err != nil {
		t.Fatalf("WriteFile first version: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes first version: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}

	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	_, indexed, errs := fi.scanDirectory(context.Background(), dir, "file_index")
	if indexed != 1 {
		t.Fatalf("first scan indexed = %d, want 1", indexed)
	}
	if len(errs) != 0 {
		t.Fatalf("first scan errors = %v", errs)
	}

	if err := os.WriteFile(path, []byte("second version with same timestamp"), 0644); err != nil {
		t.Fatalf("WriteFile second version: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes second version: %v", err)
	}

	_, indexed, errs = fi.scanDirectory(context.Background(), dir, "file_index")
	if indexed != 1 {
		t.Fatalf("second scan indexed = %d, want 1", indexed)
	}
	if len(errs) != 0 {
		t.Fatalf("second scan errors = %v", errs)
	}
	if !reflect.DeepEqual(vdb.deleted, []string{"doc-1"}) {
		t.Fatalf("deleted docs = %v, want [doc-1]", vdb.deleted)
	}
}

func TestFileIndexerReindexesWhenEmbeddingFingerprintChanges(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	modTime := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Second)
	if err := os.WriteFile(path, []byte("same content"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}

	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{fingerprint: "provider|old-model"}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	_, indexed, errs := fi.scanDirectory(context.Background(), dir, "file_index")
	if indexed != 1 || len(errs) != 0 {
		t.Fatalf("first scan indexed=%d errors=%v", indexed, errs)
	}

	vdb.fingerprint = "provider|new-model"
	_, indexed, errs = fi.scanDirectory(context.Background(), dir, "file_index")
	if indexed != 1 || len(errs) != 0 {
		t.Fatalf("second scan indexed=%d errors=%v", indexed, errs)
	}
	if !reflect.DeepEqual(vdb.deleted, []string{"doc-1"}) {
		t.Fatalf("deleted docs = %v, want [doc-1]", vdb.deleted)
	}
}

func TestFileIndexerMultimodalEmbedderCacheKeyIncludesAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{}
	cfg.Embeddings.BaseURL = "https://example.test/v1"
	cfg.Embeddings.Model = "embed-model"
	cfg.Embeddings.MultimodalFormat = "image"
	cfg.Embeddings.ProviderType = "openai"
	cfg.Embeddings.APIKey = "first-key"
	cfgMu := &sync.RWMutex{}
	fi := NewFileIndexer(cfg, cfgMu, &fakeIndexerVectorDB{}, nil, logger)

	first := fi.getMultimodalEmbedder()
	if first == nil {
		t.Fatal("first embedder is nil")
	}

	cfgMu.Lock()
	cfg.Embeddings.APIKey = "second-key"
	cfgMu.Unlock()

	second := fi.getMultimodalEmbedder()
	if second == nil {
		t.Fatal("second embedder is nil")
	}
	if first == second {
		t.Fatal("expected API key rotation to create a new multimodal embedder")
	}
}

func TestFileIndexerRemovesEmbeddingsForDeletedFiles(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("keep me indexed"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	modTime := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}
	cfg.Indexing.PollIntervalSeconds = 60

	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	_, indexed, errs := fi.scanDirectory(context.Background(), dir, "file_index")
	if indexed != 1 {
		t.Fatalf("initial scan indexed = %d, want 1", indexed)
	}
	if len(errs) != 0 {
		t.Fatalf("initial scan errors = %v", errs)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove indexed file: %v", err)
	}

	total, indexed, errs := fi.scanDirectory(context.Background(), dir, "file_index")
	if total != 0 {
		t.Fatalf("deleted-file scan total = %d, want 0", total)
	}
	if indexed != 0 {
		t.Fatalf("deleted-file scan indexed = %d, want 0", indexed)
	}
	if len(errs) != 0 {
		t.Fatalf("deleted-file scan errors = %v", errs)
	}

	if !reflect.DeepEqual(vdb.deleted, []string{"doc-1"}) {
		t.Fatalf("deleted docs = %v, want [doc-1]", vdb.deleted)
	}

	docIDs, err := stm.GetFileEmbeddingDocIDs(path, "file_index")
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs after delete: %v", err)
	}
	if len(docIDs) != 0 {
		t.Fatalf("expected tracked IDs removed after file delete, got %v", docIDs)
	}

	lastIndexed, err := stm.GetFileIndex(path, "file_index")
	if err != nil {
		t.Fatalf("GetFileIndex after delete: %v", err)
	}
	if !lastIndexed.IsZero() {
		t.Fatalf("expected file index removed after file delete, got %v", lastIndexed)
	}
}

func TestFileIndexerSkipsSymlinkFilesAndCleansTrackedPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	outsideDir := t.TempDir()
	target := filepath.Join(outsideDir, "outside.txt")
	if err := os.WriteFile(target, []byte("do not index via symlink"), 0o644); err != nil {
		t.Fatalf("WriteFile target: %v", err)
	}
	linkPath := filepath.Join(dir, "linked.txt")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := stm.UpdateFileIndexWithDocsAndState(linkPath, IndexerCollection, time.Now().UTC(), "old-hash", "old-fingerprint", []string{"old-doc"}); err != nil {
		t.Fatalf("UpdateFileIndexWithDocsAndState: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}
	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	counted, countErrs := fi.countIndexableFiles(context.Background(), dir)
	if counted != 0 || len(countErrs) != 0 {
		t.Fatalf("countIndexableFiles counted=%d errors=%v, want counted=0 errors=[]", counted, countErrs)
	}

	total, indexed, errs := fi.scanDirectory(context.Background(), dir, IndexerCollection)
	if total != 0 || indexed != 0 || len(errs) != 0 {
		t.Fatalf("scan total=%d indexed=%d errors=%v, want all zero", total, indexed, errs)
	}
	if !reflect.DeepEqual(vdb.deleted, []string{"old-doc"}) {
		t.Fatalf("deleted docs = %v, want [old-doc]", vdb.deleted)
	}
	paths, err := stm.ListIndexedFiles(IndexerCollection)
	if err != nil {
		t.Fatalf("ListIndexedFiles: %v", err)
	}
	if len(paths) != 0 {
		t.Fatalf("tracked files after symlink cleanup = %v, want none", paths)
	}
}

func TestFileIndexerRecordsDisabledVectorDBScanStatus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("index me"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}
	cfgMu := &sync.RWMutex{}
	fi := NewFileIndexer(cfg, cfgMu, &fakeIndexerVectorDB{disabled: true}, stm, logger)

	fi.scan(context.Background())

	status := fi.Status()
	if status.LastScanAt.IsZero() {
		t.Fatal("expected disabled VectorDB scan to update LastScanAt")
	}
	if status.TotalFiles != 1 {
		t.Fatalf("TotalFiles = %d, want 1", status.TotalFiles)
	}
	if status.IndexedFiles != 0 {
		t.Fatalf("IndexedFiles = %d, want 0", status.IndexedFiles)
	}
	if len(status.Errors) == 0 || !strings.Contains(strings.ToLower(status.Errors[0]), "embedding") {
		t.Fatalf("Errors = %v, want embedding pipeline explanation", status.Errors)
	}
}

func TestFileIndexerSkipsUnchangedImageBeforeVisionCall(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	var calls atomic.Int32
	visionSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"same image description"}}],"usage":{"prompt_tokens":1,"completion_tokens":2}}`))
	}))
	defer visionSrv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(path, []byte("not really a jpeg, but enough bytes for the provider payload"), 0o644); err != nil {
		t.Fatalf("WriteFile image: %v", err)
	}
	modTime := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes image: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = dir
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".jpg"}
	cfg.Indexing.IndexImages = true
	cfg.Vision.BaseURL = visionSrv.URL
	cfg.Vision.APIKey = "vision-key"
	cfg.Vision.Model = "vision-model"
	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	_, indexed, errs := fi.scanDirectory(context.Background(), dir, IndexerCollection)
	if indexed != 1 || len(errs) != 0 {
		t.Fatalf("first scan indexed=%d errors=%v, want indexed=1 errors=[]", indexed, errs)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("vision calls after first scan = %d, want 1", got)
	}

	touchedModTime := modTime.Add(time.Minute)
	if err := os.Chtimes(path, touchedModTime, touchedModTime); err != nil {
		t.Fatalf("Chtimes touched image: %v", err)
	}

	_, indexed, errs = fi.scanDirectory(context.Background(), dir, IndexerCollection)
	if indexed != 0 || len(errs) != 0 {
		t.Fatalf("second scan indexed=%d errors=%v, want indexed=0 errors=[]", indexed, errs)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("vision calls after unchanged second scan = %d, want 1", got)
	}
}

func TestFileIndexerSkipsOverlappingScans(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("index me once"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}
	cfg.Indexing.PollIntervalSeconds = 60
	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{storeStarted: make(chan struct{}), releaseStore: make(chan struct{})}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	done := make(chan struct{})
	go func() {
		fi.scan(context.Background())
		close(done)
	}()

	select {
	case <-vdb.storeStarted:
	case <-time.After(time.Second):
		t.Fatal("first scan did not start storing")
	}

	fi.scan(context.Background())
	vdb.mu.Lock()
	storedWhileBlocked := vdb.nextID
	vdb.mu.Unlock()
	if storedWhileBlocked != 1 {
		t.Fatalf("stored docs while first scan blocked = %d, want 1", storedWhileBlocked)
	}

	close(vdb.releaseStore)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("first scan did not finish")
	}
}

func TestFileIndexerStartReturnsBeforeInitialScanCompletes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("index me slowly"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}
	cfg.Indexing.PollIntervalSeconds = 60
	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{storeStarted: make(chan struct{}), releaseStore: make(chan struct{})}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	startReturned := make(chan struct{})
	go func() {
		fi.Start(context.Background())
		close(startReturned)
	}()

	select {
	case <-startReturned:
	case <-time.After(100 * time.Millisecond):
		blocked := true
		close(vdb.releaseStore)
		select {
		case <-startReturned:
		case <-time.After(time.Second):
			t.Fatal("Start did not return after releasing the blocked scan")
		}
		fi.Stop()
		if blocked {
			t.Fatal("Start blocked on the initial scan")
		}
	}

	select {
	case <-vdb.storeStarted:
	case <-time.After(time.Second):
		close(vdb.releaseStore)
		t.Fatal("initial scan did not start asynchronously")
	}
	close(vdb.releaseStore)
	fi.Stop()
}

func TestFileIndexerStopWaitsForInFlightScan(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("index me slowly"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}
	cfg.Indexing.PollIntervalSeconds = 60
	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{storeStarted: make(chan struct{}), releaseStore: make(chan struct{})}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)
	startReturned := make(chan struct{})
	go func() {
		fi.Start(context.Background())
		close(startReturned)
	}()

	select {
	case <-vdb.storeStarted:
	case <-time.After(time.Second):
		close(vdb.releaseStore)
		t.Fatal("initial scan did not start")
	}

	select {
	case <-startReturned:
	case <-time.After(100 * time.Millisecond):
		close(vdb.releaseStore)
		select {
		case <-startReturned:
		case <-time.After(time.Second):
			t.Fatal("Start did not return after releasing the blocked scan")
		}
		t.Fatal("Start blocked on the initial scan")
	}

	stopReturned := make(chan struct{})
	go func() {
		fi.Stop()
		close(stopReturned)
	}()

	select {
	case <-stopReturned:
		close(vdb.releaseStore)
		t.Fatal("Stop returned before the in-flight scan completed")
	case <-time.After(50 * time.Millisecond):
	}

	close(vdb.releaseStore)
	select {
	case <-stopReturned:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after the scan was released")
	}
}

func TestFileIndexerStartIsIdempotent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("index once"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}
	cfg.Indexing.PollIntervalSeconds = 60
	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{storeStarted: make(chan struct{}), releaseStore: make(chan struct{})}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fi.Start(ctx)
	fi.Start(ctx)

	select {
	case <-vdb.storeStarted:
	case <-time.After(time.Second):
		close(vdb.releaseStore)
		t.Fatal("initial scan did not start")
	}
	close(vdb.releaseStore)

	if !fi.Stop() {
		t.Fatal("Stop returned false after a single active runner")
	}
	vdb.mu.Lock()
	stored := vdb.nextID
	vdb.mu.Unlock()
	if stored != 1 {
		t.Fatalf("stored docs = %d, want 1 with idempotent Start", stored)
	}
}

func TestFileIndexerStopCancelsRetryBackoff(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("retry then stop"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}
	cfg.Indexing.PollIntervalSeconds = 60
	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{
		storeStarted: make(chan struct{}),
		storeErr:     fmt.Errorf("rate limit exceeded"),
	}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	fi.Start(context.Background())
	select {
	case <-vdb.storeStarted:
	case <-time.After(time.Second):
		t.Fatal("initial store attempt did not start")
	}

	start := time.Now()
	if !fi.Stop() {
		t.Fatal("Stop returned false, want true after canceling retry backoff")
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("Stop took %v, want retry backoff canceled promptly", elapsed)
	}
}

func TestFileIndexerCleanupDirectoryRemovesTrackedFiles(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("remove me from the index"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}
	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	_, indexed, errs := fi.scanDirectory(context.Background(), dir, IndexerCollection)
	if indexed != 1 || len(errs) != 0 {
		t.Fatalf("scan indexed=%d errors=%v, want indexed=1 errors=[]", indexed, errs)
	}

	cleanupErrors := fi.CleanupDirectory(config.IndexingDirectory{Path: dir})
	if len(cleanupErrors) != 0 {
		t.Fatalf("CleanupDirectory errors = %v", cleanupErrors)
	}
	if !reflect.DeepEqual(vdb.deleted, []string{"doc-1"}) {
		t.Fatalf("deleted docs = %v, want [doc-1]", vdb.deleted)
	}
	ids, err := stm.GetFileEmbeddingDocIDs(path, IndexerCollection)
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("tracked IDs after cleanup = %v, want none", ids)
	}
}

func TestFileIndexerReindexesWhenFullContentChangesBeyondIndexedLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")
	prefix := strings.Repeat("a", maxIndexedContentBytes+1024)
	modTime := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	if err := os.WriteFile(path, []byte(prefix+"first-tail"), 0o644); err != nil {
		t.Fatalf("WriteFile first version: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes first version: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}
	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	_, indexed, errs := fi.scanDirectory(context.Background(), dir, IndexerCollection)
	if indexed != 1 || len(errs) != 0 {
		t.Fatalf("first scan indexed=%d errors=%v, want indexed=1 errors=[]", indexed, errs)
	}

	if err := os.WriteFile(path, []byte(prefix+"second-tail"), 0o644); err != nil {
		t.Fatalf("WriteFile second version: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes second version: %v", err)
	}

	_, indexed, errs = fi.scanDirectory(context.Background(), dir, IndexerCollection)
	if indexed != 1 || len(errs) != 0 {
		t.Fatalf("second scan indexed=%d errors=%v, want indexed=1 errors=[] for full-content hash change", indexed, errs)
	}
}

func TestFileIndexerPassesFullTextContentToVectorDB(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")
	tail := "needle-tail-beyond-old-limit"
	content := strings.Repeat("a", maxIndexedContentBytes+1024) + tail
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}
	cfg.Indexing.Chunking.Strategy = "recursive"
	cfg.Indexing.Chunking.MaxChars = 3500
	cfg.Indexing.Chunking.OverlapChars = 200
	cfg.Indexing.Chunking.MaxChunksPerFile = 200

	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	_, indexed, errs := fi.scanDirectory(context.Background(), dir, IndexerCollection)
	if indexed != 1 || len(errs) != 0 {
		t.Fatalf("scan indexed=%d errors=%v, want indexed=1 errors=[]", indexed, errs)
	}

	vdb.mu.Lock()
	defer vdb.mu.Unlock()
	if len(vdb.stored) != 1 {
		t.Fatalf("stored contents = %d, want 1", len(vdb.stored))
	}
	if !strings.Contains(vdb.stored[0], tail) {
		t.Fatalf("stored content missing tail beyond old 500KB limit")
	}
}

func TestFileIndexerReindexesWhenChunkingFingerprintChanges(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("same content, different chunking config"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	modTime := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}
	cfg.Indexing.Chunking.Strategy = "legacy"
	cfg.Indexing.Chunking.MaxChars = 3500
	cfg.Indexing.Chunking.OverlapChars = 200
	cfg.Indexing.Chunking.MaxChunksPerFile = 200

	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	_, indexed, errs := fi.scanDirectory(context.Background(), dir, IndexerCollection)
	if indexed != 1 || len(errs) != 0 {
		t.Fatalf("first scan indexed=%d errors=%v, want indexed=1 errors=[]", indexed, errs)
	}

	cfgMu.Lock()
	cfg.Indexing.Chunking.Strategy = "recursive"
	cfgMu.Unlock()

	_, indexed, errs = fi.scanDirectory(context.Background(), dir, IndexerCollection)
	if indexed != 1 || len(errs) != 0 {
		t.Fatalf("second scan indexed=%d errors=%v, want indexed=1 errors=[] after chunking fingerprint change", indexed, errs)
	}
}

func TestFileIndexerSkipsDocumentWhenExtractionFails(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "broken.pdf")
	if err := os.WriteFile(path, []byte("not a valid PDF but definitely raw bytes"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".pdf"}
	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	total, indexed, errs := fi.scanDirectory(context.Background(), dir, IndexerCollection)
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if indexed != 0 {
		t.Fatalf("indexed = %d, want 0", indexed)
	}
	if len(errs) == 0 || !strings.Contains(errs[0], "text extraction error") {
		t.Fatalf("errors = %v, want text extraction error", errs)
	}
	vdb.mu.Lock()
	stored := vdb.nextID
	vdb.mu.Unlock()
	if stored != 0 {
		t.Fatalf("stored docs = %d, want 0", stored)
	}
}

func TestFileIndexerTriggersKGSyncForIndexedFiles(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("Proxmox runs the home lab services."), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}
	cfg.Tools.KnowledgeGraph.Enabled = true
	cfg.Tools.KnowledgeGraph.AutoExtraction = true

	cfgMu := &sync.RWMutex{}
	fi := NewFileIndexer(cfg, cfgMu, &fakeIndexerVectorDB{}, stm, logger)

	synced := make(chan string, 1)
	syncer := NewFileKGSyncer(cfg, logger, nil, nil, stm, nil)
	syncer.syncFile = func(path, collection string, opts FileKGSyncOptions) FileKGSyncResult {
		synced <- path + "|" + collection
		return FileKGSyncResult{FilesProcessed: 1}
	}
	fi.SetKGSyncer(syncer)

	_, indexed, errs := fi.scanDirectory(context.Background(), dir, IndexerCollection)
	if indexed != 1 {
		t.Fatalf("scan indexed = %d, want 1", indexed)
	}
	if len(errs) != 0 {
		t.Fatalf("scan errors = %v", errs)
	}

	select {
	case got := <-synced:
		want := path + "|" + IndexerCollection
		if got != want {
			t.Fatalf("synced = %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("expected file indexer to trigger KG sync for indexed file")
	}
}

func TestShouldRetryIndexingErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"rate limit error", fmt.Errorf("rate limit exceeded"), true},
		{"too many requests", fmt.Errorf("too many requests"), true},
		{"429 in message", fmt.Errorf("request failed with 429"), true},
		{"5xx http error", fmt.Errorf("http 500 internal server error"), true},
		{"context deadline", context.DeadlineExceeded, true},
		{"permanent error", fmt.Errorf("file not found"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRetryIndexingErr(tt.err)
			if got != tt.want {
				t.Errorf("shouldRetryIndexingErr(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
