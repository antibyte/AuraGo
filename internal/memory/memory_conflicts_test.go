package memory

import (
	"io"
	"log/slog"
	"testing"
)

func TestRegisterMemoryConflictMarksBothDocsContradicted(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertMemoryMeta("doc-a"); err != nil {
		t.Fatalf("UpsertMemoryMeta doc-a: %v", err)
	}
	if err := stm.UpsertMemoryMeta("doc-b"); err != nil {
		t.Fatalf("UpsertMemoryMeta doc-b: %v", err)
	}
	if err := stm.RegisterMemoryConflict("doc-b", "doc-a", "user|language", "german", "english", "conflicting language preference"); err != nil {
		t.Fatalf("RegisterMemoryConflict: %v", err)
	}

	conflicts, err := stm.GetOpenMemoryConflicts(10)
	if err != nil {
		t.Fatalf("GetOpenMemoryConflicts: %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("len(conflicts) = %d, want 1", len(conflicts))
	}
	metas, err := stm.GetAllMemoryMeta(10, 0)
	if err != nil {
		t.Fatalf("GetAllMemoryMeta: %v", err)
	}
	statuses := map[string]string{}
	for _, meta := range metas {
		statuses[meta.DocID] = meta.VerificationStatus
	}
	if statuses["doc-a"] != "contradicted" || statuses["doc-b"] != "contradicted" {
		t.Fatalf("unexpected verification statuses: %+v", statuses)
	}
}
