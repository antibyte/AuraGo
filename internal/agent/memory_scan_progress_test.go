package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"aurago/internal/memory"
)

func TestMemoryConflictScanPersistsProgressAcrossRestart(t *testing.T) {
	stm, db := newMemorySafetyStore(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ltm := &baselineConflictVectorDB{documents: make(map[string]string)}
	for i := 0; i < 501; i++ {
		id := fmt.Sprintf("doc-%03d", i)
		if err := stm.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
		ltm.documents[id] = "A stable fact."
	}
	if err := detectMemoryConflictsAcrossLTMWithContext(t.Context(), logger, stm, ltm, nil); err != nil {
		t.Fatal(err)
	}
	cursor, err := stm.GetMemoryMaintenanceState(memoryConflictScanCursorKey)
	if err != nil || cursor != "doc-249" || len(ltm.readIDs) != 250 {
		t.Fatalf("first scan: cursor=%s reads=%d err=%v", cursor, len(ltm.readIDs), err)
	}
	var seq int
	var name, path string
	if err := db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := memory.NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	for _, want := range []int{500, 501} {
		if err := detectMemoryConflictsAcrossLTMWithContext(t.Context(), logger, reopened, ltm, nil); err != nil || len(ltm.readIDs) != want {
			t.Fatalf("resumed scan: reads=%d want=%d err=%v", len(ltm.readIDs), want, err)
		}
	}
	for i, id := range ltm.readIDs {
		if id != fmt.Sprintf("doc-%03d", i) {
			t.Fatalf("scan repeated or skipped document %d: %s", i, id)
		}
	}
	if cursor, err := reopened.GetMemoryMaintenanceState(memoryConflictScanCursorKey); err != nil || cursor != "" {
		t.Fatalf("end did not reset cursor: %q %v", cursor, err)
	}
	if err := detectMemoryConflictsAcrossLTMWithContext(t.Context(), logger, reopened, ltm, nil); err != nil || len(ltm.readIDs) != 751 || ltm.readIDs[501] != "doc-000" {
		t.Fatalf("new pass did not restart: reads=%d err=%v", len(ltm.readIDs), err)
	}
}

func TestMemoryConflictScanCapsArchivedRowsAndContinues(t *testing.T) {
	stm, db := newMemorySafetyStore(t)
	if _, err := db.Exec(`WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM seq WHERE n < 50001)
		INSERT INTO memory_meta(doc_id, verification_status) SELECT printf('a-%05d', n), 'archived' FROM seq`); err != nil {
		t.Fatal(err)
	}
	if err := stm.UpsertMemoryMeta("z-active"); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ltm := &baselineConflictVectorDB{documents: map[string]string{"z-active": "A stable fact."}}
	if err := detectMemoryConflictsAcrossLTMWithContext(t.Context(), logger, stm, ltm, nil); err != nil {
		t.Fatal(err)
	}
	if cursor, err := stm.GetMemoryMaintenanceState(memoryConflictScanCursorKey); err != nil || cursor != "a-50000" || len(ltm.readIDs) != 0 {
		t.Fatalf("raw row budget: cursor=%s reads=%v err=%v", cursor, ltm.readIDs, err)
	}
	if err := detectMemoryConflictsAcrossLTMWithContext(t.Context(), logger, stm, ltm, nil); err != nil || len(ltm.readIDs) != 1 || ltm.readIDs[0] != "z-active" {
		t.Fatalf("archive prefix starved active document: reads=%v err=%v", ltm.readIDs, err)
	}
}

func TestMemoryConflictScanCancellationRetainsIncompleteDocument(t *testing.T) {
	stm, _ := newMemorySafetyStore(t)
	for _, id := range []string{"doc-001", "doc-002"} {
		if err := stm.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ltm := &cancellingConflictVector{cancel: cancel, memorySafetyVector: memorySafetyVector{stored: map[string]string{"doc-001": "A stable fact.", "doc-002": "User prefers Emacs"}}}
	if err := detectMemoryConflictsAcrossLTMWithContext(ctx, logger, stm, ltm, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("scan did not report cancellation: %v", err)
	}
	if cursor, err := stm.GetMemoryMaintenanceState(memoryConflictScanCursorKey); err != nil || cursor != "doc-001" {
		t.Fatalf("incomplete document marked complete: %s %v", cursor, err)
	}
	resumed := &baselineConflictVectorDB{documents: map[string]string{"doc-002": "User prefers Emacs"}}
	if err := detectMemoryConflictsAcrossLTMWithContext(t.Context(), logger, stm, resumed, nil); err != nil || len(resumed.readIDs) != 1 || resumed.readIDs[0] != "doc-002" {
		t.Fatalf("resume skipped incomplete check: %v %v", resumed.readIDs, err)
	}
}

func TestMemoryConflictScanRecordsAndRetriesIndividualFailures(t *testing.T) {
	stm, _ := newMemorySafetyStore(t)
	for _, id := range []string{"a-missing", "b-present"} {
		if err := stm.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ltm := &baselineConflictVectorDB{documents: map[string]string{"b-present": "A stable fact."}}
	if err := detectMemoryConflictsAcrossLTMWithContext(t.Context(), logger, stm, ltm, nil); err == nil || len(ltm.readIDs) != 2 {
		t.Fatalf("failure hid error or blocked other documents: %v %v", ltm.readIDs, err)
	}
	if count, err := stm.MemoryMaintenanceFailureCount("memory_conflict_scan", "a-missing"); err != nil || count != 1 {
		t.Fatalf("failure not recorded: %d %v", count, err)
	}
	ltm.documents["a-missing"] = "Recovered fact."
	if err := detectMemoryConflictsAcrossLTMWithContext(t.Context(), logger, stm, ltm, nil); err != nil || len(ltm.readIDs) != 4 {
		t.Fatalf("failed document not retried: %v %v", ltm.readIDs, err)
	}
	if count, err := stm.MemoryMaintenanceFailureCount("memory_conflict_scan", "a-missing"); err != nil || count != 0 {
		t.Fatalf("recovered failure not cleared: %d %v", count, err)
	}
}
