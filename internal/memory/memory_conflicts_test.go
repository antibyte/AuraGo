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

func TestMemoryConflictResolutionArchivesLoserAndConfirmsWinner(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	for _, docID := range []string{"doc-a", "doc-b"} {
		if err := stm.UpsertMemoryMeta(docID); err != nil {
			t.Fatalf("UpsertMemoryMeta(%s): %v", docID, err)
		}
	}
	if err := stm.RegisterMemoryConflict("doc-a", "doc-b", "user|language", "english", "german", "conflicting language preference"); err != nil {
		t.Fatalf("RegisterMemoryConflict: %v", err)
	}
	conflicts, err := stm.GetOpenMemoryConflicts(10)
	if err != nil {
		t.Fatalf("GetOpenMemoryConflicts: %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("conflicts len = %d, want 1", len(conflicts))
	}

	if err := stm.ResolveMemoryConflict(conflicts[0].ID, "doc-a", "doc-a is newer"); err != nil {
		t.Fatalf("ResolveMemoryConflict: %v", err)
	}

	conflicts, err = stm.GetOpenMemoryConflicts(10)
	if err != nil {
		t.Fatalf("GetOpenMemoryConflicts after resolve: %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("open conflicts after resolve = %#v, want none", conflicts)
	}
	statuses := memoryStatusesByDocID(t, stm)
	if statuses["doc-a"] != MemoryVerificationConfirmed {
		t.Fatalf("winner status = %q, want confirmed; statuses=%+v", statuses["doc-a"], statuses)
	}
	if statuses["doc-b"] != MemoryVerificationArchived {
		t.Fatalf("loser status = %q, want archived; statuses=%+v", statuses["doc-b"], statuses)
	}

	var winningDocID, supersededDocID string
	if err := stm.db.QueryRow(`SELECT COALESCE(winning_doc_id, ''), COALESCE(superseded_doc_id, '') FROM memory_conflicts WHERE id = ?`, conflictsID(t, stm)).Scan(&winningDocID, &supersededDocID); err != nil {
		t.Fatalf("query resolved conflict winners: %v", err)
	}
	if winningDocID != "doc-a" || supersededDocID != "doc-b" {
		t.Fatalf("resolved conflict winner/loser = %q/%q, want doc-a/doc-b", winningDocID, supersededDocID)
	}
}

func TestMemoryConflictResolutionKeepsWinnerContradictedWhenOtherOpenConflictsRemain(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	for _, docID := range []string{"doc-a", "doc-b", "doc-c"} {
		if err := stm.UpsertMemoryMeta(docID); err != nil {
			t.Fatalf("UpsertMemoryMeta(%s): %v", docID, err)
		}
	}
	if err := stm.RegisterMemoryConflict("doc-a", "doc-b", "user|language", "english", "german", "conflicting language preference"); err != nil {
		t.Fatalf("RegisterMemoryConflict doc-b: %v", err)
	}
	if err := stm.RegisterMemoryConflict("doc-a", "doc-c", "user|timezone", "cet", "utc", "conflicting timezone preference"); err != nil {
		t.Fatalf("RegisterMemoryConflict doc-c: %v", err)
	}
	conflicts, err := stm.GetOpenMemoryConflicts(10)
	if err != nil {
		t.Fatalf("GetOpenMemoryConflicts: %v", err)
	}
	var targetConflictID int64
	for _, conflict := range conflicts {
		if conflict.ConflictKey == "user|language" {
			targetConflictID = conflict.ID
			break
		}
	}
	if targetConflictID == 0 {
		t.Fatalf("language conflict not found: %#v", conflicts)
	}

	if err := stm.ResolveMemoryConflict(targetConflictID, "doc-a", "doc-a wins language only"); err != nil {
		t.Fatalf("ResolveMemoryConflict: %v", err)
	}

	statuses := memoryStatusesByDocID(t, stm)
	if statuses["doc-a"] != MemoryVerificationContradicted {
		t.Fatalf("winner should remain contradicted while another open conflict remains, statuses=%+v", statuses)
	}
	if statuses["doc-b"] != MemoryVerificationArchived {
		t.Fatalf("loser status = %q, want archived; statuses=%+v", statuses["doc-b"], statuses)
	}
	openConflicts, err := stm.GetOpenMemoryConflicts(10)
	if err != nil {
		t.Fatalf("GetOpenMemoryConflicts after resolve: %v", err)
	}
	if len(openConflicts) != 1 || openConflicts[0].ConflictKey != "user|timezone" {
		t.Fatalf("remaining open conflicts = %#v, want only timezone", openConflicts)
	}
}

func memoryStatusesByDocID(t *testing.T, stm *SQLiteMemory) map[string]string {
	t.Helper()
	metas, err := stm.GetAllMemoryMeta(100, 0)
	if err != nil {
		t.Fatalf("GetAllMemoryMeta: %v", err)
	}
	statuses := map[string]string{}
	for _, meta := range metas {
		statuses[meta.DocID] = meta.VerificationStatus
	}
	return statuses
}

func conflictsID(t *testing.T, stm *SQLiteMemory) int64 {
	t.Helper()
	var id int64
	if err := stm.db.QueryRow(`SELECT id FROM memory_conflicts LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("query conflict id: %v", err)
	}
	return id
}
