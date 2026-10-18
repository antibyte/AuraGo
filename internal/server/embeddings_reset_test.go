package server

import (
	"aurago/internal/config"
	"aurago/internal/kgquality"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/services"
	"context"
	"database/sql"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func TestEmbeddingsConfigChangedDetectsRelevantChanges(t *testing.T) {
	t.Parallel()

	var oldCfg config.Config
	oldCfg.Embeddings.Provider = "emb"
	oldCfg.Embeddings.ProviderType = "openai"
	oldCfg.Embeddings.BaseURL = "https://example.com/v1"
	oldCfg.Embeddings.Model = "text-embed-3-small"

	newCfg := oldCfg
	if embeddingsConfigChanged(oldCfg, newCfg) {
		t.Fatalf("expected identical embeddings config to be unchanged")
	}

	newCfg.Embeddings.Model = "text-embed-3-large"
	if !embeddingsConfigChanged(oldCfg, newCfg) {
		t.Fatalf("expected model change to trigger embeddings reset warning")
	}
}

func TestApplyPendingEmbeddingsResetPreservesSourceState(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	stmPath := filepath.Join(tmpDir, "short_term.db")
	stm, err := memory.NewSQLiteMemory(stmPath, logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	if err := stm.UpdateFileIndex("knowledge/test.pdf", "file_index", time.Now().UTC()); err != nil {
		t.Fatalf("UpdateFileIndex: %v", err)
	}
	if err := stm.UpsertMemoryMeta("aurago_memories_test_chunk_1"); err != nil {
		t.Fatalf("UpsertMemoryMeta: %v", err)
	}
	kg, err := memory.NewKnowledgeGraph(filepath.Join(tmpDir, "kg.db"), "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	defer kg.Close()
	if err := kg.BulkMergeExtractedEntities([]memory.Node{
		{ID: "file-node", Label: "File Node", Properties: map[string]string{"source": "file_sync", "source_file": "knowledge/test.pdf"}},
		{ID: "manual-node", Label: "Manual Node", Properties: map[string]string{"source": "manual"}},
	}, []memory.Edge{
		{Source: "file-node", Target: "manual-node", Relation: "mentions", Properties: map[string]string{"source": "file_sync", "source_file": "knowledge/test.pdf"}},
	}); err != nil {
		t.Fatalf("BulkMergeExtractedEntities: %v", err)
	}

	vectorDir := filepath.Join(tmpDir, "vectordb")
	if err := os.MkdirAll(vectorDir, 0750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vectorDir, "stale.vec"), []byte("old"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = tmpDir
	cfg.Directories.VectorDBDir = vectorDir

	createVerifiedResetCorpus(t, vectorDir)
	if err := WriteEmbeddingsResetMarker(cfg, logger, "test"); err != nil {
		t.Fatalf("WriteEmbeddingsResetMarker: %v", err)
	}

	applied, err := ApplyPendingEmbeddingsReset(cfg, stm, kg, logger)
	if err != nil {
		t.Fatalf("ApplyPendingEmbeddingsReset: %v", err)
	}
	if !applied {
		t.Fatalf("expected pending reset to be applied")
	}

	if _, err := os.Stat(embeddingsResetMarkerPath(cfg)); !os.IsNotExist(err) {
		t.Fatalf("expected marker file to be removed, got err=%v", err)
	}

	entries, err := os.ReadDir(vectorDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("expected original vector source files to remain, got %d entries", len(entries))
	}

	lastIndexed, err := stm.GetFileIndex("knowledge/test.pdf", "file_index")
	if err != nil {
		t.Fatalf("GetFileIndex: %v", err)
	}
	if lastIndexed.IsZero() {
		t.Fatalf("expected file index to remain, got %v", lastIndexed)
	}

	meta, err := stm.GetAllMemoryMeta(100, 0)
	if err != nil {
		t.Fatalf("GetAllMemoryMeta: %v", err)
	}
	if len(meta) != 1 {
		t.Fatalf("expected memory metadata to remain, got %d entries", len(meta))
	}
	nodes, err := kg.GetAllNodes(10)
	if err != nil {
		t.Fatalf("GetAllNodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("KG facts removed: %d", len(nodes))
	}
	edges, err := kg.GetAllEdges(10)
	if err != nil {
		t.Fatalf("GetAllEdges: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected file-sync KG edge to remain during embeddings reset, got %d", len(edges))
	}

	info, err := os.Stat(vectorDir)
	if err != nil {
		t.Fatalf("Stat vector dir: %v", err)
	}
	if info.Mode().Type() != fs.ModeDir {
		t.Fatalf("expected vector dir to exist after reset")
	}
}

func TestFileKGLifecycleUpdateRenameDeleteReindexReset(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	stm, err := memory.NewSQLiteMemory(filepath.Join(tmpDir, "short_term.db"), logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()
	kg, err := memory.NewKnowledgeGraph(filepath.Join(tmpDir, "kg.db"), "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	defer kg.Close()

	cfg := &config.Config{}
	cfg.LLM.Model = "test-kg-model"
	cfg.Directories.DataDir = tmpDir
	cfg.Directories.VectorDBDir = filepath.Join(tmpDir, "vectordb")

	originalPath := "knowledge/original.md"
	renamedPath := "knowledge/renamed.md"
	vectorDB := &lifecycleVectorDB{docs: map[string]string{
		"doc-v1": "Original Proxmox host notes with enough details for extraction. It runs old backup services and local monitoring.",
		"doc-v2": "Updated NAS host notes with enough details for extraction. It now runs fresh storage services and snapshots.",
		"doc-v3": "Renamed file notes with enough details for extraction. It describes renamed services and network links.",
		"doc-v4": "Reindexed file notes with enough details for extraction. It describes rebuilt services after deletion.",
	}}
	client := &lifecycleKGClient{responses: []string{
		`{"nodes":[{"id":"old_service","label":"Old Service","properties":{"type":"service"}}],"edges":[]}`,
		`{"nodes":[{"id":"updated_service","label":"Updated Service","properties":{"type":"service"}}],"edges":[]}`,
		`{"nodes":[{"id":"renamed_service","label":"Renamed Service","properties":{"type":"service"}}],"edges":[]}`,
		`{"nodes":[{"id":"reindexed_service","label":"Reindexed Service","properties":{"type":"service"}}],"edges":[]}`,
	}}
	syncer := services.NewFileKGSyncer(cfg, logger, client, vectorDB, stm, kg)

	if err := stm.UpdateFileIndexWithDocs(originalPath, services.IndexerCollection, time.Now().UTC(), []string{"doc-v1"}); err != nil {
		t.Fatalf("track original v1: %v", err)
	}
	if result := syncer.SyncFile(originalPath, services.IndexerCollection, services.FileKGSyncOptions{}); len(result.Errors) != 0 {
		t.Fatalf("sync original v1 errors: %v", result.Errors)
	}
	assertSourceFileNodes(t, kg, originalPath, []string{"old_service"})

	if err := stm.UpdateFileIndexWithDocs(originalPath, services.IndexerCollection, time.Now().UTC().Add(time.Minute), []string{"doc-v2"}); err != nil {
		t.Fatalf("track original v2: %v", err)
	}
	if result := syncer.SyncFile(originalPath, services.IndexerCollection, services.FileKGSyncOptions{}); len(result.Errors) != 0 {
		t.Fatalf("sync original v2 errors: %v", result.Errors)
	}
	assertSourceFileNodes(t, kg, originalPath, []string{"updated_service"})

	if err := stm.DeleteFileIndex(originalPath, services.IndexerCollection); err != nil {
		t.Fatalf("delete original file index for rename: %v", err)
	}
	if err := stm.UpdateFileIndexWithDocs(renamedPath, services.IndexerCollection, time.Now().UTC().Add(2*time.Minute), []string{"doc-v3"}); err != nil {
		t.Fatalf("track renamed file: %v", err)
	}
	if result := syncer.CleanupOrphans(false); len(result.Errors) != 0 {
		t.Fatalf("cleanup renamed orphans errors: %v", result.Errors)
	}
	assertSourceFileNodes(t, kg, originalPath, nil)
	if result := syncer.SyncFile(renamedPath, services.IndexerCollection, services.FileKGSyncOptions{}); len(result.Errors) != 0 {
		t.Fatalf("sync renamed file errors: %v", result.Errors)
	}
	assertSourceFileNodes(t, kg, renamedPath, []string{"renamed_service"})

	if err := stm.DeleteFileIndex(renamedPath, services.IndexerCollection); err != nil {
		t.Fatalf("delete renamed file index: %v", err)
	}
	if result := syncer.CleanupOrphans(false); len(result.Errors) != 0 {
		t.Fatalf("cleanup deleted file errors: %v", result.Errors)
	}
	assertSourceFileNodes(t, kg, renamedPath, nil)

	if err := stm.UpdateFileIndexWithDocs(originalPath, services.IndexerCollection, time.Now().UTC().Add(3*time.Minute), []string{"doc-v4"}); err != nil {
		t.Fatalf("track reindexed file: %v", err)
	}
	if result := syncer.SyncFile(originalPath, services.IndexerCollection, services.FileKGSyncOptions{}); len(result.Errors) != 0 {
		t.Fatalf("sync reindexed file errors: %v", result.Errors)
	}
	assertSourceFileNodes(t, kg, originalPath, []string{"reindexed_service"})

	if err := os.MkdirAll(cfg.Directories.VectorDBDir, 0750); err != nil {
		t.Fatalf("create vector dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Directories.VectorDBDir, "stale.vec"), []byte("old"), 0644); err != nil {
		t.Fatalf("write stale vector: %v", err)
	}
	createVerifiedResetCorpus(t, cfg.Directories.VectorDBDir)
	if err := WriteEmbeddingsResetMarker(cfg, logger, "lifecycle_test"); err != nil {
		t.Fatalf("WriteEmbeddingsResetMarker: %v", err)
	}
	applied, err := ApplyPendingEmbeddingsReset(cfg, stm, kg, logger)
	if err != nil {
		t.Fatalf("ApplyPendingEmbeddingsReset: %v", err)
	}
	if !applied {
		t.Fatalf("expected embeddings reset marker to be applied")
	}
	assertSourceFileNodes(t, kg, originalPath, []string{"reindexed_service"})
	paths, err := stm.ListIndexedFiles(services.IndexerCollection)
	if err != nil {
		t.Fatalf("ListIndexedFiles after reset: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected file index to remain after reset, got %v", paths)
	}
}

func assertSourceFileNodes(t *testing.T, kg *memory.KnowledgeGraph, sourceFile string, wantIDs []string) {
	t.Helper()

	if wantIDs != nil {
		wantIDs = append([]string{kgquality.FileNodeID(sourceFile)}, wantIDs...)
	}
	nodes, err := kg.GetNodesBySourceFile(sourceFile, 20)
	if err != nil {
		t.Fatalf("GetNodesBySourceFile(%s): %v", sourceFile, err)
	}
	got := make([]string, 0, len(nodes))
	for _, node := range nodes {
		got = append(got, node.ID)
	}
	if strings.Join(got, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("nodes for %s = %v, want %v", sourceFile, got, wantIDs)
	}
}

type lifecycleKGClient struct {
	responses []string
	calls     int
}

func (c *lifecycleKGClient) CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if c.calls >= len(c.responses) {
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: `{"nodes":[],"edges":[]}`}}}}, nil
	}
	content := c.responses[c.calls]
	c.calls++
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: content}}}}, nil
}

func (c *lifecycleKGClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, nil
}

type lifecycleVectorDB struct {
	docs map[string]string
}

func (v *lifecycleVectorDB) StoreDocument(string, string) ([]string, error) { return nil, nil }
func (v *lifecycleVectorDB) StoreDocumentWithEmbedding(string, string, []float32) (string, error) {
	return "", nil
}
func (v *lifecycleVectorDB) StoreDocumentInCollection(string, string, string) ([]string, error) {
	return nil, nil
}
func (v *lifecycleVectorDB) StoreDocumentWithEmbeddingInCollection(string, string, []float32, string) (string, error) {
	return "", nil
}
func (v *lifecycleVectorDB) StoreBatch([]memory.ArchiveItem) ([]string, error) { return nil, nil }
func (v *lifecycleVectorDB) SearchSimilar(string, int, ...string) ([]string, []string, error) {
	return nil, nil, nil
}
func (v *lifecycleVectorDB) SearchMemoriesOnly(string, int) ([]string, []string, error) {
	return nil, nil, nil
}
func (v *lifecycleVectorDB) GetByIDFromCollection(id string, _ string) (string, error) {
	if doc, ok := v.docs[id]; ok {
		return doc, nil
	}
	return "", os.ErrNotExist
}
func (v *lifecycleVectorDB) GetByID(id string) (string, error) {
	return v.GetByIDFromCollection(id, "")
}
func (v *lifecycleVectorDB) DeleteDocument(string) error { return nil }
func (v *lifecycleVectorDB) DeleteDocumentFromCollection(string, string) error {
	return nil
}
func (v *lifecycleVectorDB) Count() int       { return len(v.docs) }
func (v *lifecycleVectorDB) IsDisabled() bool { return false }
func (v *lifecycleVectorDB) IsReady() bool    { return true }
func (v *lifecycleVectorDB) Close() error     { return nil }
func (v *lifecycleVectorDB) StoreCheatsheet(string, string, string, ...string) error {
	return nil
}
func (v *lifecycleVectorDB) DeleteCheatsheet(string) error { return nil }
func (v *lifecycleVectorDB) RegisterCollections([]string)  {}

func createVerifiedResetCorpus(t *testing.T, dir string) {
	t.Helper()
	if err := memory.PrepareDocumentCorpus(dir); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "documents.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO collections(name) VALUES('aurago_memories'); INSERT INTO documents(collection,id,content,metadata,vector,dimension,fingerprint,pending) VALUES('aurago_memories','source','retained source','{}','[1,0,0]',3,'old',0);`); err != nil {
		t.Fatal(err)
	}
}
func TestApplyPendingEmbeddingsResetRetainsUnverifiedLegacyStore(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.DataDir = root
	cfg.Directories.VectorDBDir = filepath.Join(root, "vectors")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(filepath.Join(root, "memory.db"), logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	if err := os.MkdirAll(cfg.Directories.VectorDBDir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfg.Directories.VectorDBDir, "legacy.gob")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteEmbeddingsResetMarker(cfg, logger, "test"); err != nil {
		t.Fatal(err)
	}
	if applied, err := ApplyPendingEmbeddingsReset(cfg, stm, nil, logger); err == nil || applied {
		t.Fatalf("unsafe reset: %v %v", applied, err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "original" {
		t.Fatalf("legacy changed: %v", err)
	}
	if _, err := os.Stat(embeddingsResetMarkerPath(cfg)); err != nil {
		t.Fatalf("marker removed: %v", err)
	}
}
