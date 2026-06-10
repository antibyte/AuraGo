package agent

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestRunMaintenanceTaskPersistsLedgerOnPromptFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	cfg := &config.Config{}
	cfg.CircuitBreaker.MaintenanceTimeoutMinutes = 1
	cfg.Directories.PromptsDir = filepath.Join(t.TempDir(), "missing-prompts")
	cfg.LLM.Model = "test-model"
	cfg.Maintenance.Enabled = true

	runMaintenanceTask(context.Background(), cfg, logger, nil, nil, nil, nil, nil, nil, stm, nil, nil, nil, nil, nil, nil, nil)

	record, err := stm.GetLatestMaintenanceRun()
	if err != nil {
		t.Fatalf("GetLatestMaintenanceRun: %v", err)
	}
	if record == nil {
		t.Fatal("expected maintenance run ledger entry")
	}
	if record.Status != "failed" {
		t.Fatalf("status = %q, want failed", record.Status)
	}
	if len(record.PhaseResults.Errors) == 0 {
		t.Fatal("expected ledger errors for missing maintenance prompt")
	}
}

func TestMaintenanceRunLedgerStatus(t *testing.T) {
	ledger := newMaintenanceRunLedger()
	if ledger.status() != "completed" {
		t.Fatalf("empty ledger status = %q, want completed", ledger.status())
	}
	ledger.addError("inventory_kg_sync: timeout")
	if ledger.status() != "partial" {
		t.Fatalf("error ledger status = %q, want partial", ledger.status())
	}
	ledger.markFailed()
	if ledger.status() != "failed" {
		t.Fatalf("failed ledger status = %q, want failed", ledger.status())
	}
}

func TestComputeNextMaintenanceRun(t *testing.T) {
	cfg := &config.Config{}
	cfg.Maintenance.Time = "04:00"
	now := time.Date(2026, 6, 10, 5, 0, 0, 0, time.Local)
	next := ComputeNextMaintenanceRun(cfg, now)
	expected := time.Date(2026, 6, 11, 4, 0, 0, 0, time.Local)
	if !next.Equal(expected) {
		t.Fatalf("next = %v, want %v", next, expected)
	}
}

func TestRunMaintenanceCompressedOutputCleanupReturnsDeletedCount(t *testing.T) {
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

	deleted, err := runMaintenanceCompressedOutputCleanup(ctx, cfg, logger, stm)
	if err != nil {
		t.Fatalf("runMaintenanceCompressedOutputCleanup: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
}