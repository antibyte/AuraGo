package tools

import (
	"aurago/internal/memory"
	"strings"
	"testing"
	"time"
)

func TestCheatsheetCreateAndGet(t *testing.T) {
	t.Parallel()
	db, err := InitCheatsheetDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()

	sheet, err := CheatsheetCreate(db, "My Sheet", "content here", "user")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sheet.Name != "My Sheet" || sheet.Content != "content here" || sheet.CreatedBy != "user" {
		t.Fatalf("unexpected: %+v", sheet)
	}

	got, err := CheatsheetGet(db, sheet.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != sheet.ID || got.Name != sheet.Name {
		t.Fatalf("get mismatch: %+v", got)
	}

	byName, err := CheatsheetGetByName(db, "my sheet") // case-insensitive
	if err != nil {
		t.Fatalf("getByName: %v", err)
	}
	if byName.ID != sheet.ID {
		t.Fatalf("getByName returned wrong sheet")
	}
}

func TestCheatsheetTagsPersistAndClear(t *testing.T) {
	t.Parallel()
	db, err := InitCheatsheetDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()

	sheet, err := CheatsheetCreateWithTags(db, "Tagged", "content", "user", []string{"ops", " deploy ", "ops", ""})
	if err != nil {
		t.Fatalf("create with tags: %v", err)
	}
	if got := strings.Join(sheet.Tags, ","); got != "deploy,ops" {
		t.Fatalf("created tags = %q, want deploy,ops", got)
	}

	got, err := CheatsheetGet(db, sheet.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if gotTags := strings.Join(got.Tags, ","); gotTags != "deploy,ops" {
		t.Fatalf("persisted tags = %q, want deploy,ops", gotTags)
	}

	emptyTags := []string{}
	updated, err := CheatsheetUpdate(db, sheet.ID, nil, nil, nil, nil, nil, &emptyTags)
	if err != nil {
		t.Fatalf("clear tags: %v", err)
	}
	if len(updated.Tags) != 0 {
		t.Fatalf("cleared tags = %#v, want empty", updated.Tags)
	}
}

func TestCheatsheetCreateNameRequired(t *testing.T) {
	t.Parallel()
	db, err := InitCheatsheetDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()

	_, err = CheatsheetCreate(db, "", "content", "user")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

type cheatsheetVectorDBRecorder struct {
	storedIDs  []string
	deletedIDs []string
}

func (v *cheatsheetVectorDBRecorder) StoreDocument(string, string) ([]string, error) {
	return nil, nil
}
func (v *cheatsheetVectorDBRecorder) StoreDocumentWithEmbedding(string, string, []float32) (string, error) {
	return "", nil
}
func (v *cheatsheetVectorDBRecorder) StoreBatch([]memory.ArchiveItem) ([]string, error) {
	return nil, nil
}
func (v *cheatsheetVectorDBRecorder) SearchSimilar(string, int, ...string) ([]string, []string, error) {
	return nil, nil, nil
}
func (v *cheatsheetVectorDBRecorder) SearchMemoriesOnly(string, int) ([]string, []string, error) {
	return nil, nil, nil
}
func (v *cheatsheetVectorDBRecorder) GetByID(string) (string, error) { return "", nil }
func (v *cheatsheetVectorDBRecorder) GetByIDFromCollection(string, string) (string, error) {
	return "", nil
}
func (v *cheatsheetVectorDBRecorder) DeleteDocument(string) error { return nil }
func (v *cheatsheetVectorDBRecorder) DeleteDocumentFromCollection(string, string) error {
	return nil
}
func (v *cheatsheetVectorDBRecorder) Count() int       { return 0 }
func (v *cheatsheetVectorDBRecorder) IsDisabled() bool { return false }
func (v *cheatsheetVectorDBRecorder) IsReady() bool    { return true }
func (v *cheatsheetVectorDBRecorder) Close() error     { return nil }
func (v *cheatsheetVectorDBRecorder) StoreDocumentInCollection(string, string, string) ([]string, error) {
	return nil, nil
}
func (v *cheatsheetVectorDBRecorder) StoreDocumentWithEmbeddingInCollection(string, string, []float32, string) (string, error) {
	return "", nil
}
func (v *cheatsheetVectorDBRecorder) StoreCheatsheet(id, name, content string, attachments ...string) error {
	v.storedIDs = append(v.storedIDs, id)
	return nil
}
func (v *cheatsheetVectorDBRecorder) DeleteCheatsheet(id string) error {
	v.deletedIDs = append(v.deletedIDs, id)
	return nil
}
func (v *cheatsheetVectorDBRecorder) RegisterCollections([]string) {}

func TestReindexCheatsheetDeletesInactiveFromVectorDB(t *testing.T) {
	t.Parallel()
	db, err := InitCheatsheetDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()

	sheet, err := CheatsheetCreate(db, "Inactive", "content", "user")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	inactive := false
	if _, err := CheatsheetUpdate(db, sheet.ID, nil, nil, nil, &inactive, nil, nil); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	vdb := &cheatsheetVectorDBRecorder{}
	if err := ReindexCheatsheetInVectorDB(db, vdb, sheet.ID); err != nil {
		t.Fatalf("reindex inactive: %v", err)
	}
	if len(vdb.storedIDs) != 0 {
		t.Fatalf("stored inactive IDs = %#v, want none", vdb.storedIDs)
	}
	if len(vdb.deletedIDs) != 1 || vdb.deletedIDs[0] != sheet.ID {
		t.Fatalf("deleted IDs = %#v, want %q", vdb.deletedIDs, sheet.ID)
	}
}

func TestCheatsheetContentLimit(t *testing.T) {
	t.Parallel()
	db, err := InitCheatsheetDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()

	bigContent := strings.Repeat("x", MaxContentChars+1)

	_, err = CheatsheetCreate(db, "Big", bigContent, "user")
	if err == nil {
		t.Fatal("expected error for content exceeding limit")
	}
	if !strings.Contains(err.Error(), "character limit") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Exact limit should succeed
	exactContent := strings.Repeat("x", MaxContentChars)
	sheet, err := CheatsheetCreate(db, "Exact", exactContent, "user")
	if err != nil {
		t.Fatalf("exact limit should succeed: %v", err)
	}

	// Update over limit should fail
	overContent := strings.Repeat("y", MaxContentChars+1)
	_, err = CheatsheetUpdate(db, sheet.ID, nil, &overContent, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for update exceeding limit")
	}
}

func TestCheatsheetCount(t *testing.T) {
	t.Parallel()
	db, err := InitCheatsheetDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()

	total, active, agentCreated, err := CheatsheetCount(db)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 0 || active != 0 || agentCreated != 0 {
		t.Fatalf("expected all zeros, got %d/%d/%d", total, active, agentCreated)
	}

	CheatsheetCreate(db, "Sheet1", "c", "user")
	CheatsheetCreate(db, "Sheet2", "c", "agent")

	total, active, agentCreated, err = CheatsheetCount(db)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 2 || active != 2 || agentCreated != 1 {
		t.Fatalf("expected 2/2/1, got %d/%d/%d", total, active, agentCreated)
	}
}

func TestCheatsheetListByCreatedByFiltersUserSheets(t *testing.T) {
	t.Parallel()
	db, err := InitCheatsheetDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()

	userSheet, err := CheatsheetCreate(db, "User Sheet", "user content", "user")
	if err != nil {
		t.Fatalf("create user sheet: %v", err)
	}
	agentSheet, err := CheatsheetCreate(db, "Agent Sheet", "agent content", "agent")
	if err != nil {
		t.Fatalf("create agent sheet: %v", err)
	}
	inactive := false
	if _, err := CheatsheetUpdate(db, agentSheet.ID, nil, nil, nil, &inactive, nil); err != nil {
		t.Fatalf("deactivate agent sheet: %v", err)
	}

	sheets, err := CheatsheetListByCreatedBy(db, true, "user")
	if err != nil {
		t.Fatalf("list user sheets: %v", err)
	}
	if len(sheets) != 1 || sheets[0].ID != userSheet.ID {
		t.Fatalf("user active sheets = %+v, want only %q", sheets, userSheet.ID)
	}

	allSheets, err := CheatsheetListByCreatedBy(db, false, "")
	if err != nil {
		t.Fatalf("list all sheets: %v", err)
	}
	if len(allSheets) != 2 {
		t.Fatalf("all sheets = %+v, want 2", allSheets)
	}
}

func TestCheatsheetDeleteNotFound(t *testing.T) {
	t.Parallel()
	db, err := InitCheatsheetDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()

	err = CheatsheetDelete(db, "nonexistent")
	if err == nil {
		t.Fatal("expected error for deleting nonexistent sheet")
	}
}

func TestCheatsheetMetadataUsageAndDeleteLock(t *testing.T) {
	t.Parallel()
	db, err := InitCheatsheetDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()

	sheet, err := CheatsheetCreate(db, "Locked", "content", "agent")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sheet.ExpiresAt == "" {
		t.Fatal("agent-created sheet should get an expiration")
	}

	abstract := "Useful for testing metadata."
	locked := true
	sheet, err = CheatsheetUpdate(db, sheet.ID, nil, nil, &abstract, nil, &locked)
	if err != nil {
		t.Fatalf("update metadata: %v", err)
	}
	if sheet.Abstract != abstract || !sheet.DeleteLocked {
		t.Fatalf("metadata not updated: %+v", sheet)
	}

	if err := CheatsheetRecordUsage(db, sheet.ID); err != nil {
		t.Fatalf("record usage: %v", err)
	}
	used, err := CheatsheetGet(db, sheet.ID)
	if err != nil {
		t.Fatalf("get used: %v", err)
	}
	if used.UsageCount != 1 || used.LastUsedAt == "" {
		t.Fatalf("usage not recorded: %+v", used)
	}

	if err := CheatsheetDelete(db, sheet.ID); err == nil || !strings.Contains(err.Error(), "delete-locked") {
		t.Fatalf("expected delete lock error, got %v", err)
	}
}

func TestCheatsheetExpiredUnusedQueryAndMark(t *testing.T) {
	t.Parallel()
	db, err := InitCheatsheetDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()

	sheet, err := CheatsheetCreate(db, "Expired", "content", "agent")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	old := time.Now().UTC().Add(-8 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := db.Exec("UPDATE cheatsheets SET expires_at = ? WHERE id = ?", old, sheet.ID); err != nil {
		t.Fatalf("force expiration: %v", err)
	}

	expired, err := CheatsheetGetExpiredUnused(db)
	if err != nil {
		t.Fatalf("expired query: %v", err)
	}
	if len(expired) != 1 || expired[0].ID != sheet.ID {
		t.Fatalf("expired = %+v, want %q", expired, sheet.ID)
	}
	if err := CheatsheetMarkUnused(db, sheet.ID); err != nil {
		t.Fatalf("mark unused: %v", err)
	}
	marked, err := CheatsheetGet(db, sheet.ID)
	if err != nil {
		t.Fatalf("get marked: %v", err)
	}
	if marked.Active {
		t.Fatalf("expected inactive after mark unused: %+v", marked)
	}
}

func TestCheatsheetAttachmentAdd(t *testing.T) {
	t.Parallel()
	db, err := InitCheatsheetDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()

	sheet, err := CheatsheetCreate(db, "Sheet", "content", "user")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	att, err := CheatsheetAttachmentAdd(db, sheet.ID, "readme.md", "upload", "# Hello")
	if err != nil {
		t.Fatalf("add attachment: %v", err)
	}
	if att.Filename != "readme.md" || att.CharCount != 7 {
		t.Fatalf("unexpected attachment: %+v", att)
	}

	// Verify it appears in the attachment list
	attachments, err := CheatsheetAttachmentList(db, sheet.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}

	// Remove it
	err = CheatsheetAttachmentRemove(db, sheet.ID, att.ID)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
}

func TestCheatsheetAttachmentLimitEnforced(t *testing.T) {
	t.Parallel()
	db, err := InitCheatsheetDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()

	sheet, err := CheatsheetCreate(db, "Sheet", "content", "user")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Fill up to 24000 chars
	content24k := strings.Repeat("a", 24000)
	_, err = CheatsheetAttachmentAdd(db, sheet.ID, "big.txt", "upload", content24k)
	if err != nil {
		t.Fatalf("add 24k: %v", err)
	}

	// Adding 1001 more should fail (24000 + 1001 = 25001 > 25000)
	content1001 := strings.Repeat("b", 1001)
	_, err = CheatsheetAttachmentAdd(db, sheet.ID, "over.txt", "upload", content1001)
	if err == nil {
		t.Fatal("expected error for exceeding attachment char limit")
	}
	if !strings.Contains(err.Error(), "character limit") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Adding exactly 1000 should succeed (24000 + 1000 = 25000)
	content1000 := strings.Repeat("c", 1000)
	_, err = CheatsheetAttachmentAdd(db, sheet.ID, "exact.txt", "upload", content1000)
	if err != nil {
		t.Fatalf("add exact limit should succeed: %v", err)
	}
}

func TestCheatsheetAttachmentInvalidExtension(t *testing.T) {
	t.Parallel()
	db, err := InitCheatsheetDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()

	sheet, err := CheatsheetCreate(db, "Sheet", "content", "user")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = CheatsheetAttachmentAdd(db, sheet.ID, "image.png", "upload", "data")
	if err == nil {
		t.Fatal("expected error for .png extension")
	}
}

func TestCheatsheetGetMultipleNoDuplicateQueries(t *testing.T) {
	t.Parallel()
	db, err := InitCheatsheetDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()

	sheet, err := CheatsheetCreate(db, "Sheet", "body", "user")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = CheatsheetAttachmentAdd(db, sheet.ID, "file.txt", "upload", "attachment content")
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	result := CheatsheetGetMultiple(db, []string{sheet.ID})
	if !strings.Contains(result, "body") {
		t.Fatal("expected content in result")
	}
	if !strings.Contains(result, "attachment content") {
		t.Fatal("expected attachment content in result")
	}
}
