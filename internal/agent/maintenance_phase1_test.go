package agent

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/tools"
)

func TestRunPostConsolidationMemoryMaintenanceCuratesWithoutNewFacts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertMemoryMetaWithDetails("doc-weak", memory.MemoryMetaUpdate{
		ExtractionConfidence: 0.50,
		VerificationStatus:   "unverified",
		SourceReliability:    0.50,
	}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails: %v", err)
	}
	if err := stm.SetMemoryMetaLastAccessed("doc-weak", time.Now().UTC().Add(-60*24*time.Hour)); err != nil {
		t.Fatalf("SetMemoryMetaLastAccessed: %v", err)
	}

	cfg := &config.Config{}
	cfg.Consolidation.AutoOptimize = true
	cfg.MemoryAnalysis.AutoConfirm = 0.92

	runPostConsolidationMemoryMaintenance(cfg, logger, nil, stm, nil, nil, 0)

	metas, err := stm.GetAllMemoryMeta(10, 0)
	if err != nil {
		t.Fatalf("GetAllMemoryMeta: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("meta count = %d, want 1", len(metas))
	}
	if metas[0].VerificationStatus != "archived" {
		t.Fatalf("VerificationStatus = %q, want archived", metas[0].VerificationStatus)
	}
}

func TestRunMaintenanceCompressedOutputCleanupRemovesExpiredOutputs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	ctx := context.Background()
	if err := stm.StoreCompressedOutput(ctx, &memory.CompressedToolOutput{
		SessionID:         "sess-1",
		ToolCallID:        "call_old",
		ToolName:          "shell",
		OriginalContent:   "old output",
		CompressedContent: "old",
	}); err != nil {
		t.Fatalf("StoreCompressedOutput: %v", err)
	}
	if err := stm.SetCompressedOutputCreatedAt(ctx, "sess-1", "call_old", time.Now().UTC().Add(-48*time.Hour)); err != nil {
		t.Fatalf("SetCompressedOutputCreatedAt: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.OutputCompression.Reversible.Enabled = true
	cfg.Agent.OutputCompression.Reversible.MaxAgeHours = 24

	runMaintenanceCompressedOutputCleanup(ctx, cfg, logger, stm)

	has, err := stm.HasCompressedOutputsForSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("HasCompressedOutputsForSession: %v", err)
	}
	if has {
		t.Fatal("expected expired compressed output to be cleaned up")
	}
}

func TestRunMaintenanceTaskClearsMaintenanceLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "maintenance.lock")
	tools.SetBusyFilePath(lockPath)
	tools.SetBusy(false)
	t.Cleanup(func() {
		tools.SetBusy(false)
		_ = os.Remove(lockPath)
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{}
	cfg.CircuitBreaker.MaintenanceTimeoutMinutes = 1
	cfg.Directories.PromptsDir = filepath.Join(t.TempDir(), "missing-prompts")
	cfg.LLM.Model = "test-model"

	runMaintenanceTask(context.Background(), cfg, logger, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	if tools.IsBusy() {
		t.Fatal("maintenance busy flag still set after runMaintenanceTask")
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("maintenance lock file still present: %v", err)
	}
}

func TestConsolidateSTMtoLTMReturnsZeroWhenNoArchivedMessages(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	cfg.Consolidation.MaxBatchMessages = 200
	cfg.Consolidation.ArchiveRetainDays = 30

	totalStored, messagesConsolidated := consolidateSTMtoLTM(cfg, logger, nil, stm, &dedupConsolidationVectorDB{}, nil)
	if totalStored != 0 || messagesConsolidated != 0 {
		t.Fatalf("stored=%d messages=%d, want 0/0", totalStored, messagesConsolidated)
	}
}