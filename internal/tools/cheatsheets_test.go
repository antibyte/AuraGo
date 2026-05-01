package tools

import (
	"strings"
	"testing"
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
	_, err = CheatsheetUpdate(db, sheet.ID, nil, &overContent, nil)
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
	if _, err := CheatsheetUpdate(db, agentSheet.ID, nil, nil, &inactive); err != nil {
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
