package services

import (
	"log/slog"
	"os"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestFileKGSyncer_SyncAll_NilDependencies(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{}

	// Should not panic when KG or STM are nil.
	syncer := NewFileKGSyncer(cfg, logger, nil, nil, nil, nil)
	result := syncer.SyncAll(FileKGSyncOptions{})

	if result.FilesProcessed != 0 {
		t.Errorf("expected 0 files processed, got %d", result.FilesProcessed)
	}
	if result.NodesExtracted != 0 {
		t.Errorf("expected 0 nodes extracted, got %d", result.NodesExtracted)
	}
}

func TestFileKGSyncer_SyncFile_VectorDBDisabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{}

	// Create a disabled VectorDB stub.
	vectorDB := &memory.ChromemVectorDB{}

	// STM and KG are nil — SyncAll should bail early.
	syncer := NewFileKGSyncer(cfg, logger, nil, vectorDB, nil, nil)
	result := syncer.SyncAll(FileKGSyncOptions{})

	if result.FilesProcessed != 0 {
		t.Errorf("expected 0 files processed with nil STM/KG, got %d", result.FilesProcessed)
	}
}

func TestFileKGSyncer_CleanupFile_NilKG(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{}

	syncer := NewFileKGSyncer(cfg, logger, nil, nil, nil, nil)
	result := syncer.CleanupFile("/docs/a.txt", "file_index", false)

	if len(result.Errors) != 0 {
		t.Errorf("expected no errors with nil KG, got %v", result.Errors)
	}
}

func TestFileKGSyncer_CleanupFile_DryRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{}

	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	defer kg.Close()

	syncer := NewFileKGSyncer(cfg, logger, nil, nil, nil, kg)
	result := syncer.CleanupFile("/docs/a.txt", "file_index", true)

	if len(result.Errors) != 0 {
		t.Errorf("expected no errors in dry-run, got %v", result.Errors)
	}
}
