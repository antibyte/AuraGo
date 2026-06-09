package memory

import (
	"fmt"
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
	if err != nil {
		t.Fatalf("RepairCanonicalMemoryNames: %v", err)
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

type fakeRepairVectorDB struct {
	docs       map[string]string
	stored     []string
	deleted    []string
	counter    int
	afterStore func()
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
	delete(f.docs, id)
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
