package memory

import (
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ── Core Memory CRUD ──────────────────────────────────────────────────────────

func TestCoreMemory_AddAndGet(t *testing.T) {
	stm := newTestProfileDB(t)

	id, err := stm.AddCoreMemoryFact("The sky is blue")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	facts, err := stm.GetCoreMemoryFacts()
	if err != nil {
		t.Fatalf("GetCoreMemoryFacts: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}
	if facts[0].Fact != "The sky is blue" {
		t.Errorf("unexpected fact text: %q", facts[0].Fact)
	}
	if facts[0].ID != id {
		t.Errorf("expected ID %d, got %d", id, facts[0].ID)
	}
}

func TestCoreMemory_Update(t *testing.T) {
	stm := newTestProfileDB(t)

	id, err := stm.AddCoreMemoryFact("original fact")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}

	if err := stm.UpdateCoreMemoryFact(id, "updated fact"); err != nil {
		t.Fatalf("UpdateCoreMemoryFact: %v", err)
	}

	facts, err := stm.GetCoreMemoryFacts()
	if err != nil {
		t.Fatalf("GetCoreMemoryFacts: %v", err)
	}
	if len(facts) != 1 || facts[0].Fact != "updated fact" {
		t.Errorf("expected 'updated fact', got %v", facts)
	}
}

func TestCoreMemory_UpdateNonExistent(t *testing.T) {
	stm := newTestProfileDB(t)

	err := stm.UpdateCoreMemoryFact(99999, "does not matter")
	if err == nil {
		t.Error("expected error when updating non-existent fact")
	}
}

func TestCoreMemory_Delete(t *testing.T) {
	stm := newTestProfileDB(t)

	id, err := stm.AddCoreMemoryFact("to be deleted")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}

	if err := stm.DeleteCoreMemoryFact(id); err != nil {
		t.Fatalf("DeleteCoreMemoryFact: %v", err)
	}

	facts, err := stm.GetCoreMemoryFacts()
	if err != nil {
		t.Fatalf("GetCoreMemoryFacts: %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("expected 0 facts after delete, got %d", len(facts))
	}
}

func TestCoreMemory_DeleteNonExistent(t *testing.T) {
	stm := newTestProfileDB(t)

	err := stm.DeleteCoreMemoryFact(99999)
	if err == nil {
		t.Error("expected error when deleting non-existent fact")
	}
}

func TestCoreMemory_DeleteAll(t *testing.T) {
	stm := newTestProfileDB(t)

	if _, err := stm.AddCoreMemoryFact("first fact"); err != nil {
		t.Fatalf("AddCoreMemoryFact first: %v", err)
	}
	if _, err := stm.AddCoreMemoryFact("second fact"); err != nil {
		t.Fatalf("AddCoreMemoryFact second: %v", err)
	}

	deleted, err := stm.DeleteAllCoreMemoryFacts()
	if err != nil {
		t.Fatalf("DeleteAllCoreMemoryFacts: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}

	count, err := stm.GetCoreMemoryCount()
	if err != nil {
		t.Fatalf("GetCoreMemoryCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("count after delete all = %d, want 0", count)
	}
}

func TestCoreMemory_FactExists(t *testing.T) {
	stm := newTestProfileDB(t)

	if stm.CoreMemoryFactExists("missing") {
		t.Error("expected false for non-existent fact")
	}

	if _, err := stm.AddCoreMemoryFact("present"); err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}

	if !stm.CoreMemoryFactExists("present") {
		t.Error("expected true for existing fact")
	}

	// Exact match only — a substring should not match.
	if stm.CoreMemoryFactExists("pres") {
		t.Error("CoreMemoryFactExists should do exact match, not substring")
	}
}

func TestCoreMemory_NormalizedDuplicateFactsReuseExistingEntry(t *testing.T) {
	stm := newTestProfileDB(t)

	firstID, err := stm.AddCoreMemoryFact("User prefers German responses.")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact first: %v", err)
	}
	secondID, err := stm.AddCoreMemoryFact(" user   PREFERS german responses. ")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact duplicate: %v", err)
	}
	if secondID != firstID {
		t.Fatalf("normalized duplicate id = %d, want existing id %d", secondID, firstID)
	}

	facts, err := stm.GetCoreMemoryFacts()
	if err != nil {
		t.Fatalf("GetCoreMemoryFacts: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("facts len = %d, want 1: %+v", len(facts), facts)
	}
	if !stm.CoreMemoryFactExists("USER prefers german responses.") {
		t.Fatal("CoreMemoryFactExists should match normalized case/spacing duplicates")
	}
	if stm.CoreMemoryFactExists("prefers German") {
		t.Fatal("CoreMemoryFactExists should not match substrings")
	}
}

func TestCoreMemoryMigrationBackfillsNormalizedFactAndDeduplicates(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stm.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE core_memory (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			fact TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO core_memory (fact) VALUES
			('User prefers German responses.'),
			(' user   PREFERS german responses. '),
			('Different fact');
	`)
	if err != nil {
		_ = db.Close()
		t.Fatalf("seed core_memory: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	stm, err := NewSQLiteMemory(dbPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	facts, err := stm.GetCoreMemoryFacts()
	if err != nil {
		t.Fatalf("GetCoreMemoryFacts: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("facts len = %d, want 2 after normalized duplicate cleanup: %+v", len(facts), facts)
	}

	var normalized string
	if err := stm.db.QueryRow(`
		SELECT normalized_fact FROM core_memory
		WHERE normalized_fact = ?
	`, normalizeCoreMemoryFactForDedupe("USER prefers german responses.")).Scan(&normalized); err != nil {
		t.Fatalf("normalized_fact should be backfilled and queryable: %v", err)
	}
	if normalized != "user prefers german responses." {
		t.Fatalf("normalized_fact = %q", normalized)
	}

	duplicateID, err := stm.AddCoreMemoryFact("USER   PREFERS German responses.")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact duplicate: %v", err)
	}
	if duplicateID == 0 {
		t.Fatal("duplicate AddCoreMemoryFact returned zero id")
	}
	facts, err = stm.GetCoreMemoryFacts()
	if err != nil {
		t.Fatalf("GetCoreMemoryFacts after duplicate: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("facts len after duplicate add = %d, want 2: %+v", len(facts), facts)
	}
}

func TestCoreMemoryUpdateRefreshesNormalizedFact(t *testing.T) {
	stm := newTestProfileDB(t)

	id, err := stm.AddCoreMemoryFact("original fact")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}
	if err := stm.UpdateCoreMemoryFact(id, "User Prefers Go"); err != nil {
		t.Fatalf("UpdateCoreMemoryFact: %v", err)
	}

	var normalized string
	if err := stm.db.QueryRow("SELECT normalized_fact FROM core_memory WHERE id = ?", id).Scan(&normalized); err != nil {
		t.Fatalf("query normalized_fact: %v", err)
	}
	if normalized != "user prefers go" {
		t.Fatalf("normalized_fact = %q, want %q", normalized, "user prefers go")
	}
	if !stm.CoreMemoryFactExists(" user   PREFERS go ") {
		t.Fatal("CoreMemoryFactExists should use updated normalized_fact")
	}
	if stm.CoreMemoryFactExists("original fact") {
		t.Fatal("old normalized fact should not remain after update")
	}
}

func TestNewSQLiteMemoryFailsWhenCoreMemoryDuplicateCleanupFails(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stm.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE core_memory (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			fact TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO core_memory (fact) VALUES ('duplicate'), ('duplicate');
		CREATE TRIGGER block_core_memory_delete
		BEFORE DELETE ON core_memory
		BEGIN
			SELECT RAISE(ABORT, 'delete blocked');
		END;
	`)
	if err != nil {
		_ = db.Close()
		t.Fatalf("seed duplicate core memory: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	stm, err := NewSQLiteMemory(dbPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		_ = stm.Close()
		t.Fatal("NewSQLiteMemory should fail when core_memory duplicate cleanup fails")
	}
	if !strings.Contains(err.Error(), "core_memory duplicate cleanup") {
		t.Fatalf("error = %v, want core_memory duplicate cleanup context", err)
	}
}

func TestCoreMemory_FindByFact(t *testing.T) {
	stm := newTestProfileDB(t)

	id, err := stm.AddCoreMemoryFact("findable fact")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}

	found, err := stm.FindCoreMemoryIDByFact("findable fact")
	if err != nil {
		t.Fatalf("FindCoreMemoryIDByFact: %v", err)
	}
	if found != id {
		t.Errorf("expected ID %d, got %d", id, found)
	}

	// Non-existent fact → error.
	if _, err := stm.FindCoreMemoryIDByFact("ghost"); err == nil {
		t.Error("expected error for non-existent fact")
	}
}

func TestCoreMemory_Count(t *testing.T) {
	stm := newTestProfileDB(t)

	count, err := stm.GetCoreMemoryCount()
	if err != nil {
		t.Fatalf("GetCoreMemoryCount: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	for i := 0; i < 3; i++ {
		if _, err := stm.AddCoreMemoryFact(strings.Repeat("fact", i+1)); err != nil {
			t.Fatalf("AddCoreMemoryFact #%d: %v", i, err)
		}
	}

	count, err = stm.GetCoreMemoryCount()
	if err != nil {
		t.Fatalf("GetCoreMemoryCount after inserts: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

func TestCoreMemory_TruncatesLongFact(t *testing.T) {
	stm := newTestProfileDB(t)

	longFact := strings.Repeat("x", maxCoreMemoryFactLen+100)
	id, err := stm.AddCoreMemoryFact(longFact)
	if err != nil {
		t.Fatalf("AddCoreMemoryFact (long): %v", err)
	}

	facts, err := stm.GetCoreMemoryFacts()
	if err != nil {
		t.Fatalf("GetCoreMemoryFacts: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}
	if len(facts[0].Fact) > maxCoreMemoryFactLen {
		t.Errorf("stored fact exceeds maxCoreMemoryFactLen: len=%d", len(facts[0].Fact))
	}
	_ = id
}

func TestCoreMemory_GetEmptyReturnsSlice(t *testing.T) {
	stm := newTestProfileDB(t)

	facts, err := stm.GetCoreMemoryFacts()
	if err != nil {
		t.Fatalf("GetCoreMemoryFacts on empty DB: %v", err)
	}
	// Must return non-nil empty slice (not nil) so callers can range safely.
	if facts == nil {
		t.Error("GetCoreMemoryFacts should return non-nil slice even when empty")
	}
}

func TestGetCoreMemoryUpdatedAt_ParsesAsUTC(t *testing.T) {
	stm := newTestProfileDB(t)

	_, err := stm.db.Exec(
		"INSERT INTO core_memory (fact, updated_at) VALUES (?, ?)",
		"utc fact", "2026-01-15 10:30:00",
	)
	if err != nil {
		t.Fatalf("insert core_memory: %v", err)
	}

	got, err := stm.GetCoreMemoryUpdatedAt()
	if err != nil {
		t.Fatalf("GetCoreMemoryUpdatedAt: %v", err)
	}
	want := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("parsed time = %v, want %v", got, want)
	}
	if got.Location() != time.UTC {
		t.Fatalf("location = %v, want UTC", got.Location())
	}
}

func TestCoreMemoryUpdatedAt_ChangesOnAdd(t *testing.T) {
	stm := newTestProfileDB(t)

	before, err := stm.GetCoreMemoryUpdatedAt()
	if err != nil {
		t.Fatalf("GetCoreMemoryUpdatedAt on empty DB: %v", err)
	}
	if !before.IsZero() {
		t.Errorf("expected zero time on empty DB, got %v", before)
	}

	_, err = stm.AddCoreMemoryFact("first fact")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}

	afterAdd, err := stm.GetCoreMemoryUpdatedAt()
	if err != nil {
		t.Fatalf("GetCoreMemoryUpdatedAt after add: %v", err)
	}
	if afterAdd.IsZero() {
		t.Error("expected non-zero time after adding fact")
	}
	if !afterAdd.After(before) && !afterAdd.Equal(before) {
		t.Errorf("afterAdd=%v should be after or equal before=%v", afterAdd, before)
	}
}

func TestCoreMemoryUpdatedAt_ChangesOnUpdate(t *testing.T) {
	stm := newTestProfileDB(t)

	id, err := stm.AddCoreMemoryFact("original")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}

	before, err := stm.GetCoreMemoryUpdatedAt()
	if err != nil {
		t.Fatalf("GetCoreMemoryUpdatedAt: %v", err)
	}

	err = stm.UpdateCoreMemoryFact(id, "updated")
	if err != nil {
		t.Fatalf("UpdateCoreMemoryFact: %v", err)
	}

	afterUpdate, err := stm.GetCoreMemoryUpdatedAt()
	if err != nil {
		t.Fatalf("GetCoreMemoryUpdatedAt after update: %v", err)
	}
	if !afterUpdate.After(before) && !afterUpdate.Equal(before) {
		t.Errorf("afterUpdate=%v should be after or equal before=%v", afterUpdate, before)
	}
}

func TestCoreMemoryUpdatedAt_ChangesOnDelete(t *testing.T) {
	stm := newTestProfileDB(t)

	_, err := stm.AddCoreMemoryFact("first")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}
	id2, err := stm.AddCoreMemoryFact("second")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}

	before, err := stm.GetCoreMemoryUpdatedAt()
	if err != nil {
		t.Fatalf("GetCoreMemoryUpdatedAt: %v", err)
	}

	err = stm.DeleteCoreMemoryFact(id2)
	if err != nil {
		t.Fatalf("DeleteCoreMemoryFact: %v", err)
	}

	afterDelete, err := stm.GetCoreMemoryUpdatedAt()
	if err != nil {
		t.Fatalf("GetCoreMemoryUpdatedAt after delete: %v", err)
	}
	// After deleting the most recently added fact, MAX(updated_at) should still reflect the remaining fact's timestamp
	if afterDelete.IsZero() {
		t.Error("expected non-zero time after deleting one of two facts")
	}
	// The remaining fact (id1) should still have its original timestamp
	if afterDelete.Before(before) && !afterDelete.Equal(before) {
		t.Errorf("afterDelete=%v should not be before before=%v", afterDelete, before)
	}
}

func TestCoreMemoryUpdatedAt_DeleteOnlyFactReturnsZero(t *testing.T) {
	stm := newTestProfileDB(t)

	id, err := stm.AddCoreMemoryFact("only fact")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}
	_ = id

	err = stm.DeleteCoreMemoryFact(id)
	if err != nil {
		t.Fatalf("DeleteCoreMemoryFact: %v", err)
	}

	afterDelete, err := stm.GetCoreMemoryUpdatedAt()
	if err != nil {
		t.Fatalf("GetCoreMemoryUpdatedAt after deleting only fact: %v", err)
	}
	if !afterDelete.IsZero() {
		t.Errorf("expected zero time after deleting only fact, got %v", afterDelete)
	}
}

func TestMigrateCoreMemoryFromMarkdownDoesNotImportFacts(t *testing.T) {
	stm := newTestProfileDB(t)
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "core_memory.md"), []byte("- Username is Andi\n"), 0o644); err != nil {
		t.Fatalf("write legacy core memory: %v", err)
	}

	if firstStart := stm.MigrateCoreMemoryFromMarkdown(dataDir, nil); firstStart {
		t.Fatal("legacy core_memory.md must not be treated as first start")
	}

	count, err := stm.GetCoreMemoryCount()
	if err != nil {
		t.Fatalf("GetCoreMemoryCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("core memory count = %d, want 0; migration must not write facts", count)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "core_memory.md.migrated")); err != nil {
		t.Fatalf("legacy sentinel not renamed: %v", err)
	}
}
