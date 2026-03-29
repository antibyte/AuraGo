package server

import (
	"aurago/internal/config"
	"aurago/internal/memory"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestApplyPendingEmbeddingsResetClearsVectorState(t *testing.T) {
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

	if err := WriteEmbeddingsResetMarker(cfg, logger, "test"); err != nil {
		t.Fatalf("WriteEmbeddingsResetMarker: %v", err)
	}

	applied, err := ApplyPendingEmbeddingsReset(cfg, stm, logger)
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
	if len(entries) != 0 {
		t.Fatalf("expected recreated vector dir to be empty, got %d entries", len(entries))
	}

	lastIndexed, err := stm.GetFileIndex("knowledge/test.pdf")
	if err != nil {
		t.Fatalf("GetFileIndex: %v", err)
	}
	if !lastIndexed.IsZero() {
		t.Fatalf("expected file index to be cleared, got %v", lastIndexed)
	}

	meta, err := stm.GetAllMemoryMeta(100, 0)
	if err != nil {
		t.Fatalf("GetAllMemoryMeta: %v", err)
	}
	if len(meta) != 0 {
		t.Fatalf("expected memory meta to be cleared, got %d entries", len(meta))
	}

	info, err := os.Stat(vectorDir)
	if err != nil {
		t.Fatalf("Stat vector dir: %v", err)
	}
	if info.Mode().Type() != fs.ModeDir {
		t.Fatalf("expected vector dir to exist after reset")
	}
}
