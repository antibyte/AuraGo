package memory

import (
	"io"
	"log/slog"
	"strings"
	"testing"
)

func newFTSMigrationTestMemory(t *testing.T) *SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	return stm
}

func clearFTSIndex(t *testing.T, stm *SQLiteMemory, table string) {
	t.Helper()
	if _, err := stm.db.Exec("INSERT INTO " + table + "(" + table + ") VALUES('delete-all')"); err != nil {
		t.Fatalf("clear %s: %v", table, err)
	}
}

func deleteFTSMarker(t *testing.T, stm *SQLiteMemory, key string) {
	t.Helper()
	if _, err := stm.db.Exec("DELETE FROM memory_schema_meta WHERE key = ?", key); err != nil &&
		!strings.Contains(err.Error(), "no such table") {
		t.Fatalf("delete marker %s: %v", key, err)
	}
}

func TestNotesFTSMigrationRebuildsEmptyExternalContentIndex(t *testing.T) {
	stm := newFTSMigrationTestMemory(t)
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("initial InitNotesTables: %v", err)
	}
	if _, err := stm.AddNote("ops", "Preexisting note needle", "migration content", 2, ""); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	clearFTSIndex(t, stm, "notes_fts")
	deleteFTSMarker(t, stm, "fts.notes")

	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("migration InitNotesTables: %v", err)
	}
	got, err := stm.SearchNotes("preexisting", 10)
	if err != nil {
		t.Fatalf("SearchNotes: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("SearchNotes returned %d rows, want 1 after rebuild", len(got))
	}
}

func TestJournalFTSMigrationRebuildsEmptyExternalContentIndex(t *testing.T) {
	stm := newFTSMigrationTestMemory(t)
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("initial InitJournalTables: %v", err)
	}
	if _, err := stm.InsertJournalEntry(JournalEntry{
		EntryType: "milestone",
		Title:     "Preexisting journal needle",
		Content:   "migration content",
	}); err != nil {
		t.Fatalf("InsertJournalEntry: %v", err)
	}
	clearFTSIndex(t, stm, "journal_entries_fts")
	deleteFTSMarker(t, stm, "fts.journal_entries")

	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("migration InitJournalTables: %v", err)
	}
	got, err := stm.SearchJournalEntries("preexisting", 10)
	if err != nil {
		t.Fatalf("SearchJournalEntries: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("SearchJournalEntries returned %d rows, want 1 after rebuild", len(got))
	}
}

func TestEpisodicFTSMigrationRebuildsEmptyExternalContentIndex(t *testing.T) {
	stm := newFTSMigrationTestMemory(t)
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("initial InitJournalTables: %v", err)
	}
	if err := stm.InsertEpisodicMemory(
		"2026-07-11", "Preexisting episodic needle", "migration summary", nil, 2, "test",
	); err != nil {
		t.Fatalf("InsertEpisodicMemory: %v", err)
	}
	clearFTSIndex(t, stm, "episodic_memories_fts")
	deleteFTSMarker(t, stm, "fts.episodic_memories")

	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("migration InitJournalTables: %v", err)
	}
	got, err := stm.SearchEpisodicMemoriesInRange("preexisting", "", "", 10)
	if err != nil {
		t.Fatalf("SearchEpisodicMemoriesInRange: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("SearchEpisodicMemoriesInRange returned %d rows, want 1 after rebuild", len(got))
	}
}

func TestActivityFTSMigrationRebuildsEmptyExternalContentIndex(t *testing.T) {
	stm := newFTSMigrationTestMemory(t)
	if _, err := stm.InsertActivityTurn(ActivityTurn{
		Date:         "2026-07-11",
		Intent:       "Preexisting activity needle",
		UserRequest:  "migrate activity",
		UserRelevant: true,
	}); err != nil {
		t.Fatalf("InsertActivityTurn: %v", err)
	}
	clearFTSIndex(t, stm, "activity_turns_fts")
	deleteFTSMarker(t, stm, "fts.activity_turns")

	if err := stm.InitActivityTables(); err != nil {
		t.Fatalf("migration InitActivityTables: %v", err)
	}
	got, err := stm.SearchActivityTurnsInRange("preexisting", "", "", 10)
	if err != nil {
		t.Fatalf("SearchActivityTurnsInRange: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("SearchActivityTurnsInRange returned %d rows, want 1 after rebuild", len(got))
	}
	var marker string
	if err := stm.db.QueryRow(
		"SELECT value FROM memory_schema_meta WHERE key = 'fts.activity_turns'",
	).Scan(&marker); err != nil {
		t.Fatalf("read activity FTS marker: %v", err)
	}
	if marker != "1" {
		t.Fatalf("activity FTS marker = %q, want 1", marker)
	}
}

func TestFTSMigrationMarkersPreventRepeatedRebuildsAndRefreshOutdatedVersions(t *testing.T) {
	stm := newFTSMigrationTestMemory(t)
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}

	if _, err := stm.AddNote("ops", "marker note needle", "", 2, ""); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	if _, err := stm.InsertJournalEntry(JournalEntry{EntryType: "test", Title: "marker journal needle"}); err != nil {
		t.Fatalf("InsertJournalEntry: %v", err)
	}
	if err := stm.InsertEpisodicMemory("2026-07-11", "marker episodic needle", "marker summary", nil, 2, "test"); err != nil {
		t.Fatalf("InsertEpisodicMemory: %v", err)
	}
	if _, err := stm.InsertActivityTurn(ActivityTurn{Date: "2026-07-11", Intent: "marker activity needle"}); err != nil {
		t.Fatalf("InsertActivityTurn: %v", err)
	}

	markers := map[string]string{
		"fts.notes":             "notes_fts",
		"fts.journal_entries":   "journal_entries_fts",
		"fts.episodic_memories": "episodic_memories_fts",
		"fts.activity_turns":    "activity_turns_fts",
	}
	for key, table := range markers {
		var value string
		if err := stm.db.QueryRow("SELECT value FROM memory_schema_meta WHERE key = ?", key).Scan(&value); err != nil {
			t.Fatalf("read marker %s: %v", key, err)
		}
		if value != "1" {
			t.Fatalf("marker %s = %q, want 1", key, value)
		}
		clearFTSIndex(t, stm, table)
	}

	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("repeated InitNotesTables: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("repeated InitJournalTables: %v", err)
	}
	if err := stm.InitActivityTables(); err != nil {
		t.Fatalf("repeated InitActivityTables: %v", err)
	}
	for _, table := range markers {
		var count int
		if err := stm.db.QueryRow("SELECT count(*) FROM " + table + " WHERE " + table + " MATCH 'marker'").Scan(&count); err != nil {
			t.Fatalf("search cleared %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s was rebuilt despite a current marker", table)
		}
	}

	if _, err := stm.db.Exec("UPDATE memory_schema_meta SET value = '0'"); err != nil {
		t.Fatalf("outdate markers: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("refresh InitNotesTables: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("refresh InitJournalTables: %v", err)
	}
	if err := stm.InitActivityTables(); err != nil {
		t.Fatalf("refresh InitActivityTables: %v", err)
	}
	for key, table := range markers {
		var value string
		if err := stm.db.QueryRow("SELECT value FROM memory_schema_meta WHERE key = ?", key).Scan(&value); err != nil {
			t.Fatalf("read refreshed marker %s: %v", key, err)
		}
		if value != "1" {
			t.Fatalf("refreshed marker %s = %q, want 1", key, value)
		}
		var count int
		if err := stm.db.QueryRow("SELECT count(*) FROM " + table + " WHERE " + table + " MATCH 'marker'").Scan(&count); err != nil {
			t.Fatalf("search rebuilt %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("%s search count = %d, want 1 after outdated marker rebuild", table, count)
		}
	}
}

func TestInitActivityTablesPropagatesFTSRebuildErrors(t *testing.T) {
	stm := newFTSMigrationTestMemory(t)
	if _, err := stm.db.Exec(`
		DROP TABLE activity_turns_fts;
		CREATE VIEW activity_turns_fts AS SELECT 'invalid' AS dummy;
	`); err != nil {
		t.Fatalf("replace activity FTS table with view: %v", err)
	}
	deleteFTSMarker(t, stm, "fts.activity_turns")

	if err := stm.InitActivityTables(); err == nil {
		t.Fatal("InitActivityTables returned nil for an invalid FTS table")
	}
}
