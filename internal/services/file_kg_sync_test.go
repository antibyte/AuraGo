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
