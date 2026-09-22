package memory

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func newTestNotesDB(t *testing.T) *SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	return stm
}

// TestAddNoteUTF8Truncation verifies that truncation of long content does not
// split multi-byte UTF-8 sequences (e.g. emoji, Chinese characters).
func TestAddNoteUTF8Truncation(t *testing.T) {
	stm := newTestNotesDB(t)

	// Build a string that is slightly over maxNoteContentLen runes,
	// using multi-byte characters (日 is 3 bytes in UTF-8).
	runeCount := maxNoteContentLen + 10
	var b strings.Builder
	for i := 0; i < runeCount; i++ {
		b.WriteRune('日')
	}
	longContent := b.String()

	id, err := stm.AddNote("test", "title", longContent, 2, "")
	if err != nil {
		t.Fatalf("AddNote: %v", err)
	}

	notes, err := stm.ListNotes("test", -1)
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 1 || notes[0].ID != id {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}

	content := notes[0].Content
	// Content must be valid UTF-8
	if !utf8.ValidString(content) {
		t.Error("stored content is not valid UTF-8 after truncation")
	}
	// Rune count must not exceed the limit
	rc := utf8.RuneCountInString(content)
	if rc > maxNoteContentLen {
		t.Errorf("content rune count %d exceeds maxNoteContentLen %d", rc, maxNoteContentLen)
	}
}

// TestAddNoteTitleUTF8Truncation verifies that title truncation is also rune-safe.
func TestAddNoteTitleUTF8Truncation(t *testing.T) {
	stm := newTestNotesDB(t)

	runeCount := maxNoteTitleLen + 5
	var b strings.Builder
	for i := 0; i < runeCount; i++ {
		b.WriteRune('ä') // 2 bytes in UTF-8
	}
	longTitle := b.String()

	_, err := stm.AddNote("test", longTitle, "content", 2, "")
	if err != nil {
		t.Fatalf("AddNote with long title: %v", err)
	}

	notes, err := stm.ListNotes("test", -1)
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	title := notes[0].Title
	if !utf8.ValidString(title) {
		t.Error("stored title is not valid UTF-8 after truncation")
	}
	if utf8.RuneCountInString(title) > maxNoteTitleLen {
		t.Errorf("title rune count exceeds maxNoteTitleLen")
	}
}

func TestNotesCurationArchivesOnlySafeLowPriorityNotes(t *testing.T) {
	stm := newTestNotesDB(t)
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -120).Format(time.RFC3339)

	lowID, err := stm.AddNote("ops", "Old low priority cleanup", "", 1, "")
	if err != nil {
		t.Fatalf("AddNote low: %v", err)
	}
	highID, err := stm.AddNote("ops", "Old high priority task", "", 3, "")
	if err != nil {
		t.Fatalf("AddNote high: %v", err)
	}
	if _, err := stm.db.Exec(`UPDATE notes SET updated_at = ?, created_at = ? WHERE id IN (?, ?)`, old, old, lowID, highID); err != nil {
		t.Fatalf("backdate notes: %v", err)
	}

	plan, err := stm.BuildNotesCurationPlan(NotesCurationOptions{Now: now, MaxActions: 10})
	if err != nil {
		t.Fatalf("BuildNotesCurationPlan: %v", err)
	}
	if plan.AutoArchiveCount != 1 || plan.AutoArchive[0].NoteID != lowID {
		t.Fatalf("auto archive = %+v, want only low priority note", plan.AutoArchive)
	}
	if plan.ReviewRequiredCount != 1 || plan.ReviewRequired[0].NoteID != highID {
		t.Fatalf("review required = %+v, want high priority note", plan.ReviewRequired)
	}

	if err := stm.ApplyNoteCurationAction(plan.AutoArchive[0], "test", false); err != nil {
		t.Fatalf("ApplyNoteCurationAction: %v", err)
	}
	active, err := stm.ListNotes("ops", -1)
	if err != nil {
		t.Fatalf("ListNotes active: %v", err)
	}
	if len(active) != 1 || active[0].ID != highID {
		t.Fatalf("active notes = %+v, want only high priority note", active)
	}
	all, err := stm.ListNotesWithOptions(NotesListOptions{Category: "ops", DoneFilter: -1, IncludeArchived: true})
	if err != nil {
		t.Fatalf("ListNotesWithOptions include archived: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("all notes = %d, want 2", len(all))
	}
	var archived Note
	for _, note := range all {
		if note.ID == lowID {
			archived = note
		}
	}
	if !archived.Archived || archived.ArchivedReason == "" || archived.ArchivedAt == "" {
		t.Fatalf("archived note metadata = %+v, want archived metadata", archived)
	}
}

func TestNotesCurationSkipsProtectedAndKeepForeverNotes(t *testing.T) {
	stm := newTestNotesDB(t)
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -120).Format(time.RFC3339)

	protectedID, err := stm.AddNote("ops", "Protected old task", "", 1, "")
	if err != nil {
		t.Fatalf("AddNote protected: %v", err)
	}
	keepForeverID, err := stm.AddNote("ops", "Permanent old task", "", 1, "")
	if err != nil {
		t.Fatalf("AddNote keep forever: %v", err)
	}
	archiveID, err := stm.AddNote("ops", "Archive candidate", "", 1, "")
	if err != nil {
		t.Fatalf("AddNote archive candidate: %v", err)
	}
	if _, err := stm.db.Exec(`
		UPDATE notes
		SET updated_at = ?, created_at = ?,
		    protected = CASE WHEN id = ? THEN 1 ELSE 0 END,
		    keep_forever = CASE WHEN id = ? THEN 1 ELSE 0 END
		WHERE id IN (?, ?, ?)`,
		old, old, protectedID, keepForeverID, protectedID, keepForeverID, archiveID); err != nil {
		t.Fatalf("backdate and protect notes: %v", err)
	}

	plan, err := stm.BuildNotesCurationPlan(NotesCurationOptions{Now: now, MaxActions: 10})
	if err != nil {
		t.Fatalf("BuildNotesCurationPlan: %v", err)
	}
	if plan.AutoArchiveCount != 1 || plan.AutoArchive[0].NoteID != archiveID {
		t.Fatalf("auto archive = %+v, want only unprotected candidate %d", plan.AutoArchive, archiveID)
	}

	all, err := stm.ListNotesWithOptions(NotesListOptions{Category: "ops", DoneFilter: -1, IncludeArchived: true})
	if err != nil {
		t.Fatalf("ListNotesWithOptions: %v", err)
	}
	flagsByID := map[int64]Note{}
	for _, note := range all {
		flagsByID[note.ID] = note
	}
	if !flagsByID[protectedID].Protected {
		t.Fatalf("protected note = %+v, want Protected=true", flagsByID[protectedID])
	}
	if !flagsByID[keepForeverID].KeepForever {
		t.Fatalf("keep forever note = %+v, want KeepForever=true", flagsByID[keepForeverID])
	}
}

func TestNotesCurationLimitsReviewRequiredNotes(t *testing.T) {
	stm := newTestNotesDB(t)
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -120).Format(time.RFC3339)

	for i := 0; i < 5; i++ {
		id, err := stm.AddNote("ops", fmt.Sprintf("High priority stale %d", i), "", 3, "")
		if err != nil {
			t.Fatalf("AddNote high %d: %v", i, err)
		}
		if _, err := stm.db.Exec(`UPDATE notes SET updated_at = ?, created_at = ? WHERE id = ?`, old, old, id); err != nil {
			t.Fatalf("backdate high note %d: %v", i, err)
		}
	}

	plan, err := stm.BuildNotesCurationPlan(NotesCurationOptions{Now: now, MaxActions: 2})
	if err != nil {
		t.Fatalf("BuildNotesCurationPlan: %v", err)
	}
	if plan.ReviewRequiredCount != 2 || len(plan.ReviewRequired) != 2 {
		t.Fatalf("review required = count %d len %d, want limit 2", plan.ReviewRequiredCount, len(plan.ReviewRequired))
	}
}

func TestNormalizeCanonicalMemoryNamesUsesExactAliases(t *testing.T) {
	got := NormalizeCanonicalMemoryNames("AuroraGo links to AuroraGopher and aurorago.")
	want := "AuraGo links to AuroraGopher and aurorago."
	if got != want {
		t.Fatalf("NormalizeCanonicalMemoryNames = %q, want %q", got, want)
	}
}

func TestRepairCanonicalMemoryNamesRewritesTrackedMemoryMeta(t *testing.T) {
	stm := newTestNotesDB(t)
	if err := stm.UpsertMemoryMetaWithDetails("old-doc", MemoryMetaUpdate{
		ExtractionConfidence: 0.88,
		VerificationStatus:   "unverified",
		SourceType:           "system",
		SourceReliability:    0.77,
	}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails: %v", err)
	}
	fake := &fakeRepairVectorDB{docs: map[string]string{"old-doc": "Project AuroraGo deployment note"}}

	dry, err := stm.RepairCanonicalMemoryNames(fake, CanonicalRepairOptions{DryRun: true, Limit: 10})
	if err != nil {
		t.Fatalf("RepairCanonicalMemoryNames dry-run: %v", err)
	}
	if dry.RepairedCount != 1 || len(fake.stored) != 0 || len(fake.deleted) != 0 {
		t.Fatalf("dry repair = %+v stored=%v deleted=%v, want preview only", dry, fake.stored, fake.deleted)
	}

	applied, err := stm.RepairCanonicalMemoryNames(fake, CanonicalRepairOptions{Limit: 10})
	if err != nil {
		t.Fatalf("RepairCanonicalMemoryNames apply: %v", err)
	}
	if applied.RepairedCount != 1 || len(applied.Items) != 1 || len(applied.Items[0].NewDocIDs) != 1 {
		t.Fatalf("applied repair = %+v, want one repaired item with new doc id", applied)
	}
	if len(fake.deleted) != 1 || fake.deleted[0] != "old-doc" {
		t.Fatalf("deleted = %+v, want old-doc deleted", fake.deleted)
	}
	newID := applied.Items[0].NewDocIDs[0]
	if fake.docs[newID] != "AuraGo deployment note" && !strings.Contains(fake.docs[newID], "AuraGo") {
		t.Fatalf("new doc content = %q, want canonical AuraGo", fake.docs[newID])
	}
	metas, err := stm.GetAllMemoryMeta(10, 0)
	if err != nil {
		t.Fatalf("GetAllMemoryMeta: %v", err)
	}
	statusByID := map[string]string{}
	for _, meta := range metas {
		statusByID[meta.DocID] = meta.VerificationStatus
	}
	if statusByID["old-doc"] != MemoryVerificationArchived {
		t.Fatalf("old-doc status = %q, want archived", statusByID["old-doc"])
	}
	if statusByID[newID] == "" {
		t.Fatalf("new memory meta for %q missing", newID)
	}
}

func TestRepairCanonicalMemoryNamesPropagatesSourceReadError(t *testing.T) {
	stm := newTestNotesDB(t)
	if err := stm.UpsertMemoryMeta("missing-doc"); err != nil {
		t.Fatalf("UpsertMemoryMeta: %v", err)
	}
	fake := &fakeRepairVectorDB{docs: map[string]string{}}
	report, err := stm.RepairCanonicalMemoryNames(fake, CanonicalRepairOptions{Limit: 10})
	if err == nil || !strings.Contains(err.Error(), "read canonical repair source missing-doc") {
		t.Fatalf("error = %v, want source read error", err)
	}
	if report.SkippedCount != 1 || report.RepairedCount != 0 {
		t.Fatalf("report = %+v, want one skipped source error", report)
	}
}

func TestRepairCanonicalMemoryNamesPersistsKeysetCursorAcrossRestart(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dbPath := fmt.Sprintf("%s%cstm.db", t.TempDir(), os.PathSeparator)
	stm, err := NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}
	fake := &fakeRepairVectorDB{docs: make(map[string]string)}
	for i := 0; i < 105; i++ {
		docID := fmt.Sprintf("doc-%03d", i)
		if err := stm.UpsertMemoryMeta(docID); err != nil {
			t.Fatalf("UpsertMemoryMeta(%s): %v", docID, err)
		}
		fake.docs[docID] = "stable memory content"
	}
	first, err := stm.RepairCanonicalMemoryNames(fake, CanonicalRepairOptions{Limit: 100})
	if err != nil {
		t.Fatalf("first canonical repair: %v", err)
	}
	if first.RepairedCount != 0 || first.SkippedCount != 0 {
		t.Fatalf("first report = %+v, want no changes", first)
	}
	cursor, err := stm.GetMemoryMaintenanceState(canonicalRepairCursorKey)
	if err != nil || cursor != "doc-099" {
		t.Fatalf("first cursor = %q, err=%v, want doc-099", cursor, err)
	}
	if err := stm.Close(); err != nil {
		t.Fatalf("close first memory DB: %v", err)
	}

	reopened, err := NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatalf("reopen memory DB: %v", err)
	}
	if err := reopened.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables reopened: %v", err)
	}
	second, err := reopened.RepairCanonicalMemoryNames(fake, CanonicalRepairOptions{Limit: 100})
	if err != nil {
		t.Fatalf("second canonical repair: %v", err)
	}
	if second.RepairedCount != 0 || second.SkippedCount != 0 {
		t.Fatalf("second report = %+v, want no changes", second)
	}
	cursor, err = reopened.GetMemoryMaintenanceState(canonicalRepairCursorKey)
	if err != nil || cursor != "" {
		t.Fatalf("second cursor = %q, err=%v, want cleared cursor", cursor, err)
	}
	_ = reopened.Close()
}

func TestRepairCanonicalMemoryNamesCleansNewVectorsWhenMetaUpsertFails(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	dbPath := fmt.Sprintf("%s%cstm.db", t.TempDir(), os.PathSeparator)
	stm, err := NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}
	if err := stm.UpsertMemoryMetaWithDetails("old-doc", MemoryMetaUpdate{
		ExtractionConfidence: 0.88,
		VerificationStatus:   "unverified",
		SourceType:           "system",
		SourceReliability:    0.77,
	}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails: %v", err)
	}
	fake := &fakeRepairVectorDB{docs: map[string]string{"old-doc": "Project AuroraGo deployment note"}}
	fake.afterStore = func() {
		_ = stm.Close()
	}

	report, err := stm.RepairCanonicalMemoryNames(fake, CanonicalRepairOptions{Limit: 10})
	if err == nil {
		t.Fatal("RepairCanonicalMemoryNames returned nil error after replacement failure and closed database")
	}
	if report.RepairedCount != 0 || report.SkippedCount != 1 {
		t.Fatalf("repair report = %+v, want skipped failed repair", report)
	}
	if len(fake.deleted) != 1 || fake.deleted[0] != "new-doc-1" {
		t.Fatalf("deleted docs = %+v, want cleanup of new-doc-1 only", fake.deleted)
	}
	if _, ok := fake.docs["old-doc"]; !ok {
		t.Fatal("old vector doc was deleted despite failed meta upsert")
	}

	reopened, err := NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatalf("reopen stm: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	metas, err := reopened.GetAllMemoryMeta(10, 0)
	if err != nil {
		t.Fatalf("GetAllMemoryMeta: %v", err)
	}
	statusByID := map[string]string{}
	for _, meta := range metas {
		statusByID[meta.DocID] = meta.VerificationStatus
	}
	if statusByID["old-doc"] == MemoryVerificationArchived {
		t.Fatalf("old-doc status = %q, want original meta preserved", statusByID["old-doc"])
	}
	if _, ok := statusByID["new-doc-1"]; ok {
		t.Fatal("new memory meta exists despite failed upsert")
	}
}

func TestRepairCanonicalMemoryNamesReportsRollbackFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	dbPath := fmt.Sprintf("%s%cstm.db", t.TempDir(), os.PathSeparator)
	stm, err := NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}
	if err := stm.UpsertMemoryMetaWithDetails("old-doc", MemoryMetaUpdate{VerificationStatus: "unverified"}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails: %v", err)
	}
	fake := &fakeRepairVectorDB{docs: map[string]string{"old-doc": "Project AuroraGo deployment note"}}
	fake.afterStore = func() {
		fake.deleteErr = map[string]error{"new-doc-1": fmt.Errorf("delete failed")}
		_ = stm.Close()
	}

	report, err := stm.RepairCanonicalMemoryNames(fake, CanonicalRepairOptions{Limit: 10})
	if err == nil {
		t.Fatal("RepairCanonicalMemoryNames returned nil error after rollback failure")
	}
	if len(report.Items) != 1 || !strings.Contains(report.Items[0].Error, "rollback canonical repair artifacts") {
		t.Fatalf("report item = %+v, want rollback failure context", report.Items)
	}
}

func TestRepairCanonicalMemoryNamesReportsCleanupFailureAfterOldVectorDelete(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	dbPath := fmt.Sprintf("%s%cstm.db", t.TempDir(), os.PathSeparator)
	stm, err := NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}
	if err := stm.UpsertMemoryMetaWithDetails("old-doc", MemoryMetaUpdate{VerificationStatus: "unverified"}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails: %v", err)
	}
	fake := &fakeRepairVectorDB{docs: map[string]string{"old-doc": "Project AuroraGo deployment note"}}
	fake.afterDelete = func() {
		if fake.deleted[len(fake.deleted)-1] == "old-doc" {
			_ = stm.Close()
		}
	}

	report, err := stm.RepairCanonicalMemoryNames(fake, CanonicalRepairOptions{Limit: 10})
	if err == nil {
		t.Fatal("expected cleanup failure to be returned")
	}
	if !strings.Contains(err.Error(), "cleanup old vector doc references") {
		t.Fatalf("error = %v, want cleanup context", err)
	}
	if report.RepairedCount != 0 {
		t.Fatalf("repaired count = %d, want 0 when cleanup fails", report.RepairedCount)
	}
	if len(report.Items) != 1 || !strings.Contains(report.Items[0].Error, "cleanup old vector doc references") {
		t.Fatalf("report item = %+v, want cleanup error", report.Items)
	}
}

type fakeRepairVectorDB struct {
	docs        map[string]string
	stored      []string
	deleted     []string
	counter     int
	afterStore  func()
	afterDelete func()
	deleteErr   map[string]error
	storeOwned  func(string, string, VectorStoreMode) (VectorStoreResult, error)
}

func (f *fakeRepairVectorDB) StoreDocument(concept, content string) ([]string, error) {
	f.counter++
	id := fmt.Sprintf("new-doc-%d", f.counter)
	f.docs[id] = content
	f.stored = append(f.stored, id)
	if f.afterStore != nil {
		f.afterStore()
	}
	return []string{id}, nil
}

func (f *fakeRepairVectorDB) StoreDocumentOwned(concept, content string, mode VectorStoreMode) (VectorStoreResult, error) {
	if f.storeOwned != nil {
		return f.storeOwned(concept, content, mode)
	}
	ids, err := f.StoreDocument(concept, content)
	return VectorStoreResult{CreatedIDs: ids}, err
}

func (f *fakeRepairVectorDB) DeleteDocumentIfContentMatches(id, expectedSHA256 string) (bool, error) {
	content, err := f.GetByID(id)
	if err != nil {
		return false, err
	}
	if contentSHA256(content) != expectedSHA256 {
		return false, nil
	}
	if err := f.DeleteDocument(id); err != nil {
		return false, err
	}
	return true, nil
}

func (f *fakeRepairVectorDB) StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error) {
	return "", nil
}

func (f *fakeRepairVectorDB) StoreDocumentInCollection(concept, content, collection string) ([]string, error) {
	return nil, nil
}

func (f *fakeRepairVectorDB) StoreDocumentWithEmbeddingInCollection(concept, content string, embedding []float32, collection string) (string, error) {
	return "", nil
}

func (f *fakeRepairVectorDB) StoreBatch(items []ArchiveItem) ([]string, error) { return nil, nil }
func (f *fakeRepairVectorDB) SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error) {
	return nil, nil, nil
}
func (f *fakeRepairVectorDB) SearchMemoriesOnly(query string, topK int) ([]string, []string, error) {
	return nil, nil, nil
}
func (f *fakeRepairVectorDB) GetByIDFromCollection(id, collection string) (string, error) {
	return "", nil
}
func (f *fakeRepairVectorDB) GetByID(id string) (string, error) {
	content, ok := f.docs[id]
	if !ok {
		return "", fmt.Errorf("missing doc %s", id)
	}
	return content, nil
}
func (f *fakeRepairVectorDB) DeleteDocument(id string) error {
	f.deleted = append(f.deleted, id)
	if err := f.deleteErr[id]; err != nil {
		return err
	}
	delete(f.docs, id)
	if f.afterDelete != nil {
		f.afterDelete()
	}
	return nil
}
func (f *fakeRepairVectorDB) DeleteDocumentFromCollection(id, collection string) error { return nil }
func (f *fakeRepairVectorDB) Count() int                                               { return len(f.docs) }
func (f *fakeRepairVectorDB) IsDisabled() bool                                         { return false }
func (f *fakeRepairVectorDB) IsReady() bool                                            { return true }
func (f *fakeRepairVectorDB) Close() error                                             { return nil }
func (f *fakeRepairVectorDB) StoreCheatsheet(id, name, content string, attachments ...string) error {
	return nil
}
func (f *fakeRepairVectorDB) DeleteCheatsheet(id string) error         { return nil }
func (f *fakeRepairVectorDB) RegisterCollections(collections []string) {}
