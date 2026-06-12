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
	if retrieved.OutputRef == "" {
		t.Fatal("OutputRef should be generated")
	}
	if retrieved.SummaryContent != "" {
		t.Fatalf("SummaryContent = %q, want empty default", retrieved.SummaryContent)
	}

	byRef, err := stm.RetrieveCompressedOutputByRef(ctx, "sess-1", retrieved.OutputRef)
	if err != nil {
		t.Fatalf("RetrieveCompressedOutputByRef: %v", err)
	}
	if byRef.ToolCallID != "call_abc" {
		t.Fatalf("RetrieveCompressedOutputByRef ToolCallID = %q, want call_abc", byRef.ToolCallID)
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

func TestCompressedOutputStore_PreservesProvidedOutputRefAndSummary(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.db.Close()

	ctx := context.Background()
	out := &CompressedToolOutput{
		SessionID:         "sess-ref",
		ToolCallID:        "call_ref",
		OutputRef:         "toolout_custom_ref",
		ToolName:          "shell",
		OriginalContent:   "line one\nline two",
		CompressedContent: "compact view",
		SummaryContent:    "short summary",
		CompressionRatio:  0.25,
		FilterUsed:        "vault",
	}
	if err := stm.StoreCompressedOutput(ctx, out); err != nil {
		t.Fatalf("StoreCompressedOutput: %v", err)
	}

	retrieved, err := stm.RetrieveCompressedOutputByRef(ctx, "sess-ref", "toolout_custom_ref")
	if err != nil {
		t.Fatalf("RetrieveCompressedOutputByRef: %v", err)
	}
	if retrieved.OutputRef != "toolout_custom_ref" {
		t.Fatalf("OutputRef = %q, want toolout_custom_ref", retrieved.OutputRef)
	}
	if retrieved.SummaryContent != "short summary" {
		t.Fatalf("SummaryContent = %q, want short summary", retrieved.SummaryContent)
	}
}

func TestCompressedOutputStore_BackfillsOutputRefForLegacyRows(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.db.Close()

	ctx := context.Background()
	if _, err := stm.db.ExecContext(ctx, `
		INSERT INTO compressed_tool_outputs
			(session_id, tool_call_id, tool_name, original_content, compressed_content, compression_ratio, filter_used)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"sess-legacy", "call-legacy", "shell", "legacy original", "legacy compact", 0.5, "smart-crusher"); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}

	retrieved, err := stm.RetrieveCompressedOutput(ctx, "sess-legacy", "call-legacy")
	if err != nil {
		t.Fatalf("RetrieveCompressedOutput: %v", err)
	}
	wantRef := StableToolOutputRef("sess-legacy", "call-legacy")
	if retrieved.OutputRef != wantRef {
		t.Fatalf("OutputRef = %q, want %q", retrieved.OutputRef, wantRef)
	}
	byRef, err := stm.RetrieveCompressedOutputByRef(ctx, "sess-legacy", wantRef)
	if err != nil {
		t.Fatalf("RetrieveCompressedOutputByRef after backfill: %v", err)
	}
	if byRef.ToolCallID != "call-legacy" {
		t.Fatalf("ToolCallID = %q, want call-legacy", byRef.ToolCallID)
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
