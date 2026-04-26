package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSkillDocumentationFilename verifies the naming convention.
func TestSkillDocumentationFilename(t *testing.T) {
	t.Parallel()

	cases := []struct {
		executable string
		want       string
	}{
		{"my_skill.py", "my_skill.md"},
		{"tool.py", "tool.md"},
		{"no_ext", "no_ext.md"},
		{"", ""},
		{"a.b.c.py", "a.b.c.md"},
	}
	for _, tc := range cases {
		got := SkillDocumentationFilename(tc.executable)
		if got != tc.want {
			t.Errorf("SkillDocumentationFilename(%q) = %q, want %q", tc.executable, got, tc.want)
		}
	}
}

// TestSkillDocumentationRoundTrip exercises the full Get/Set/Delete cycle.
func TestSkillDocumentationRoundTrip(t *testing.T) {
	t.Parallel()

	mgr, _ := setupTestSkillManager(t)
	code := "import sys, json\nargs=json.load(sys.stdin)\njson.dump({'ok':True}, sys.stdout)\n"
	entry, err := mgr.CreateSkillEntry("doc_rt", "roundtrip skill", code, SkillTypeUser, "user", "", nil)
	if err != nil {
		t.Fatalf("CreateSkillEntry: %v", err)
	}

	// Initially no documentation
	content, err := mgr.GetSkillDocumentation(entry.ID)
	if err != nil {
		t.Fatalf("GetSkillDocumentation (initial): %v", err)
	}
	if content != "" {
		t.Fatalf("expected empty documentation, got %q", content)
	}

	// Set documentation
	manual := "## Description\nThis skill does roundtrip testing.\n\n## Parameters\nNone.\n"
	if err := mgr.SetSkillDocumentation(entry.ID, manual, "user"); err != nil {
		t.Fatalf("SetSkillDocumentation: %v", err)
	}

	// Read it back
	got, err := mgr.GetSkillDocumentation(entry.ID)
	if err != nil {
		t.Fatalf("GetSkillDocumentation (after set): %v", err)
	}
	if got != manual {
		t.Fatalf("content mismatch:\nwant: %q\n got: %q", manual, got)
	}

	// GetSkill should now reflect has_documentation
	updated, err := mgr.GetSkill(entry.ID)
	if err != nil {
		t.Fatalf("GetSkill after set: %v", err)
	}
	if !updated.HasDocumentation {
		t.Fatal("expected HasDocumentation == true after SetSkillDocumentation")
	}
	if updated.DocumentationPath == "" {
		t.Fatal("expected non-empty DocumentationPath")
	}
	if updated.DocumentationHash == "" {
		t.Fatal("expected non-empty DocumentationHash")
	}

	// Delete documentation
	if err := mgr.DeleteSkillDocumentation(entry.ID, "user"); err != nil {
		t.Fatalf("DeleteSkillDocumentation: %v", err)
	}

	// Confirm deletion
	afterDelete, err := mgr.GetSkillDocumentation(entry.ID)
	if err != nil {
		t.Fatalf("GetSkillDocumentation (after delete): %v", err)
	}
	if afterDelete != "" {
		t.Fatalf("expected empty after delete, got %q", afterDelete)
	}
	updated2, err := mgr.GetSkill(entry.ID)
	if err != nil {
		t.Fatalf("GetSkill after delete: %v", err)
	}
	if updated2.HasDocumentation {
		t.Fatal("expected HasDocumentation == false after DeleteSkillDocumentation")
	}
}

// TestSkillDocumentationSizeLimitRejected ensures the 64 KB cap is enforced.
func TestSkillDocumentationSizeLimitRejected(t *testing.T) {
	t.Parallel()

	mgr, _ := setupTestSkillManager(t)
	code := "import sys, json\njson.dump({}, sys.stdout)\n"
	entry, err := mgr.CreateSkillEntry("doc_big", "big doc skill", code, SkillTypeUser, "user", "", nil)
	if err != nil {
		t.Fatalf("CreateSkillEntry: %v", err)
	}

	// Exactly at limit should succeed
	atLimit := strings.Repeat("a", MaxSkillDocumentationBytes)
	if err := mgr.SetSkillDocumentation(entry.ID, atLimit, "user"); err != nil {
		t.Fatalf("expected success at exact limit, got: %v", err)
	}

	// One byte over limit must fail
	overLimit := strings.Repeat("a", MaxSkillDocumentationBytes+1)
	if err := mgr.SetSkillDocumentation(entry.ID, overLimit, "user"); err == nil {
		t.Fatal("expected error when documentation exceeds size limit, got nil")
	}
}

// TestSkillDocumentationEmptyTriggersDelete ensures an empty/whitespace-only
// content triggers DeleteSkillDocumentation rather than writing a blank file.
func TestSkillDocumentationEmptyTriggersDelete(t *testing.T) {
	t.Parallel()

	mgr, skillsDir := setupTestSkillManager(t)
	code := "import sys, json\njson.dump({}, sys.stdout)\n"
	entry, err := mgr.CreateSkillEntry("doc_empty", "empty doc skill", code, SkillTypeUser, "user", "", nil)
	if err != nil {
		t.Fatalf("CreateSkillEntry: %v", err)
	}

	// Write a manual first
	if err := mgr.SetSkillDocumentation(entry.ID, "# Manual\nsome content\n", "user"); err != nil {
		t.Fatalf("SetSkillDocumentation: %v", err)
	}
	docFile := filepath.Join(skillsDir, SkillDocumentationFilename(entry.Executable))
	if _, statErr := os.Stat(docFile); os.IsNotExist(statErr) {
		t.Fatal("expected documentation file to exist after Set")
	}

	// Setting empty content should delete the file
	if err := mgr.SetSkillDocumentation(entry.ID, "   \n  ", "user"); err != nil {
		t.Fatalf("SetSkillDocumentation (empty): %v", err)
	}
	if _, statErr := os.Stat(docFile); !os.IsNotExist(statErr) {
		t.Fatal("expected documentation file to be removed when content is whitespace-only")
	}
}

// TestSkillDocumentationHashConsistency verifies the stored hash matches the content.
func TestSkillDocumentationHashConsistency(t *testing.T) {
	t.Parallel()

	mgr, _ := setupTestSkillManager(t)
	code := "import sys, json\njson.dump({}, sys.stdout)\n"
	entry, err := mgr.CreateSkillEntry("doc_hash", "hash skill", code, SkillTypeUser, "user", "", nil)
	if err != nil {
		t.Fatalf("CreateSkillEntry: %v", err)
	}

	manual := "# Manual\nHash test content.\n"
	if err := mgr.SetSkillDocumentation(entry.ID, manual, "user"); err != nil {
		t.Fatalf("SetSkillDocumentation: %v", err)
	}
	skill, err := mgr.GetSkill(entry.ID)
	if err != nil {
		t.Fatalf("GetSkill: %v", err)
	}
	want := hashDocumentation(manual)
	if skill.DocumentationHash != want {
		t.Fatalf("hash mismatch: DB has %q, computed %q", skill.DocumentationHash, want)
	}
}

// TestSkillDocumentationDeleteIdempotent verifies that deleting when no manual
// exists is a no-op (no error).
func TestSkillDocumentationDeleteIdempotent(t *testing.T) {
	t.Parallel()

	mgr, _ := setupTestSkillManager(t)
	code := "import sys, json\njson.dump({}, sys.stdout)\n"
	entry, err := mgr.CreateSkillEntry("doc_idempotent", "idempotent skill", code, SkillTypeUser, "user", "", nil)
	if err != nil {
		t.Fatalf("CreateSkillEntry: %v", err)
	}

	// First delete — no manual exists
	if err := mgr.DeleteSkillDocumentation(entry.ID, "user"); err != nil {
		t.Fatalf("DeleteSkillDocumentation (first): %v", err)
	}
	// Second delete — also a no-op
	if err := mgr.DeleteSkillDocumentation(entry.ID, "user"); err != nil {
		t.Fatalf("DeleteSkillDocumentation (second): %v", err)
	}
}

// TestSkillDocumentationSyncFromDisk verifies that SyncFromDisk auto-detects
// a pre-existing .md file and populates has_documentation.
func TestSkillDocumentationSyncFromDisk(t *testing.T) {
	t.Parallel()

	mgr, skillsDir := setupTestSkillManager(t)
	code := "import sys, json\njson.dump({}, sys.stdout)\n"
	entry, err := mgr.CreateSkillEntry("doc_sync", "sync skill", code, SkillTypeUser, "user", "", nil)
	if err != nil {
		t.Fatalf("CreateSkillEntry: %v", err)
	}

	// Manually place a .md file next to the code without going through
	// SetSkillDocumentation (simulates external edit / migration).
	mdName := SkillDocumentationFilename(entry.Executable)
	mdPath := filepath.Join(skillsDir, mdName)
	manualContent := "# Manual\nExternal content.\n"
	if err := os.WriteFile(mdPath, []byte(manualContent), 0o640); err != nil {
		t.Fatalf("write md file: %v", err)
	}

	// SyncFromDisk should detect the .md file and update the DB columns.
	if err := mgr.SyncFromDisk(); err != nil {
		t.Fatalf("SyncFromDisk: %v", err)
	}

	synced, err := mgr.GetSkill(entry.ID)
	if err != nil {
		t.Fatalf("GetSkill after sync: %v", err)
	}
	if !synced.HasDocumentation {
		t.Error("expected HasDocumentation after SyncFromDisk detected .md file")
	}
	// Content must match what was written
	got, err := mgr.GetSkillDocumentation(entry.ID)
	if err != nil {
		t.Fatalf("GetSkillDocumentation after sync: %v", err)
	}
	if got != manualContent {
		t.Fatalf("content after sync: want %q, got %q", manualContent, got)
	}
}
