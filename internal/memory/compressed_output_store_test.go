package memory

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestCompressedOutputStore(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.db.Close()

	ctx := context.Background()

	// Store
	out := &CompressedToolOutput{
		SessionID:         "sess-1",
		ToolCallID:        "call_abc",
		ToolName:          "docker",
		OriginalContent:   "original content here",
		CompressedContent: "compressed",
		CompressionRatio:  0.5,
		FilterUsed:        "smart-crusher",
	}
	if err := stm.StoreCompressedOutput(ctx, out); err != nil {
		t.Fatalf("StoreCompressedOutput: %v", err)
	}

	// Retrieve
	retrieved, err := stm.RetrieveCompressedOutput(ctx, "sess-1", "call_abc")
	if err != nil {
		t.Fatalf("RetrieveCompressedOutput: %v", err)
	}
	if retrieved.OriginalContent != "original content here" {
		t.Errorf("OriginalContent = %q, want %q", retrieved.OriginalContent, "original content here")
	}
	if retrieved.CompressionRatio != 0.5 {
		t.Errorf("CompressionRatio = %f, want 0.5", retrieved.CompressionRatio)
	}

	// Mark accessed
	if err := stm.MarkCompressedOutputAccessed(ctx, retrieved.ID); err != nil {
		t.Fatalf("MarkCompressedOutputAccessed: %v", err)
	}
	retrieved2, _ := stm.RetrieveCompressedOutput(ctx, "sess-1", "call_abc")
	if retrieved2.AccessCount != 1 {
		t.Errorf("AccessCount = %d, want 1", retrieved2.AccessCount)
	}
	if retrieved2.AccessedAt == nil {
		t.Error("AccessedAt should be set")
	}

	// Has compressed outputs
	has, err := stm.HasCompressedOutputsForSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("HasCompressedOutputsForSession: %v", err)
	}
	if !has {
		t.Error("expected HasCompressedOutputsForSession to be true")
	}
	has2, _ := stm.HasCompressedOutputsForSession(ctx, "sess-2")
	if has2 {
		t.Error("expected HasCompressedOutputsForSession to be false for empty session")
	}

	// Cleanup
	deleted, err := stm.CleanupCompressedOutputs(ctx, 0)
	if err != nil {
		t.Fatalf("CleanupCompressedOutputs: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	_, err = stm.RetrieveCompressedOutput(ctx, "sess-1", "call_abc")
	if err == nil {
		t.Error("expected error after cleanup")
	}
}

func TestCompressedOutputStore_Scrubbing(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.db.Close()

	ctx := context.Background()

	out := &CompressedToolOutput{
		SessionID:         "sess-1",
		ToolCallID:        "call_1",
		ToolName:          "shell",
		OriginalContent:   "password: secret12345",
		CompressedContent: "compressed",
		CompressionRatio:  0.5,
	}
	if err := stm.StoreCompressedOutput(ctx, out); err != nil {
		t.Fatalf("StoreCompressedOutput: %v", err)
	}

	retrieved, _ := stm.RetrieveCompressedOutput(ctx, "sess-1", "call_1")
	if retrieved.OriginalContent == "password: secret12345" {
		t.Error("expected secrets to be scrubbed from archived output")
	}
}

func TestCompressedOutputStore_CleanupOlderThan(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.db.Close()

	ctx := context.Background()

	// Insert with past timestamp
	_, err = stm.db.ExecContext(ctx, `
		INSERT INTO compressed_tool_outputs (session_id, tool_call_id, tool_name, original_content, compressed_content, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"sess", "call", "tool", "orig", "comp", time.Now().UTC().Add(-48*time.Hour))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	deleted, err := stm.CleanupCompressedOutputs(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("CleanupCompressedOutputs: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}
}
