package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestHomepageRevisionSkipsSymlinkEntries proves a project that contains a
// symlink produces a revision that omits the symlink instead of failing the
// whole revision, and that the skipped entry is recorded in the metadata.
func TestHomepageRevisionSkipsSymlinkEntries(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_registry.db")
	db, err := InitHomepageRegistryDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init DB: %v", err)
	}
	defer db.Close()

	projectDir := "linked-site"
	projectPath := filepath.Join(tmpDir, projectDir)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("<h1>Hello</h1>"), 0644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}

	outside := filepath.Join(tmpDir, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatalf("failed to write outside file: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(projectPath, "link.txt")); err != nil {
		t.Skipf("symlinks are not available on this host: %v", err)
	}

	entries, skipped, err := enumerateProjectFiles(projectPath, nil)
	if err != nil {
		t.Fatalf("enumerateProjectFiles failed: %v", err)
	}
	for _, entry := range entries {
		if entry.Path == "link.txt" {
			t.Fatalf("the symlink must not enter the revision inventory")
		}
	}
	if len(entries) != 1 || entries[0].Path != "index.html" {
		t.Fatalf("expected only index.html, got %+v", entries)
	}
	found := false
	for _, path := range skipped {
		if path == "link.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the skipped symlink to be recorded, got %v", skipped)
	}
}

// TestHomepageSaveRevisionRecordsSkippedSymlinks proves the skipped entries
// reach the revision metadata.
func TestHomepageSaveRevisionRecordsSkippedSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_registry.db")
	db, err := InitHomepageRegistryDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init DB: %v", err)
	}
	defer db.Close()

	projectDir := "linked-site"
	projectPath := filepath.Join(tmpDir, projectDir)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("<h1>Hello</h1>"), 0644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}
	outside := filepath.Join(tmpDir, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatalf("failed to write outside file: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(projectPath, "link.txt")); err != nil {
		t.Skipf("symlinks are not available on this host: %v", err)
	}

	if _, _, err := RegisterProject(db, HomepageProject{Name: projectDir, ProjectDir: projectDir, Framework: "html", Status: "active"}); err != nil {
		t.Fatalf("RegisterProject failed: %v", err)
	}

	cfg := HomepageConfig{WorkspacePath: tmpDir}
	result := HomepageSaveRevision(cfg, db, projectDir, "test reason", "testing", slogDiscard())
	if strings.Contains(result, `"status":"error"`) {
		t.Fatalf("expected the revision to succeed, got %s", result)
	}

	revisions, _, err := ListHomepageRevisions(db, projectDir, 20, 0)
	if err != nil {
		t.Fatalf("ListHomepageRevisions failed: %v", err)
	}
	if len(revisions) == 0 {
		t.Fatalf("expected at least one revision")
	}
	var metadata struct {
		Added   int      `json:"added"`
		Skipped []string `json:"skipped"`
	}
	if err := json.Unmarshal([]byte(revisions[0].MetadataJSON), &metadata); err != nil {
		t.Fatalf("revision metadata is not valid JSON: %v", err)
	}
	if len(metadata.Skipped) != 1 || metadata.Skipped[0] != "link.txt" {
		t.Fatalf("expected the skipped symlink in the revision metadata, got %v", metadata.Skipped)
	}
}

func TestHomepageRevisionLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_registry.db")
	db, err := InitHomepageRegistryDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init DB: %v", err)
	}
	defer db.Close()

	projectDir := "test-site"
	projectPath := filepath.Join(tmpDir, projectDir)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("<h1>Hello</h1>"), 0644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}

	RegisterProject(db, HomepageProject{Name: "test-site", ProjectDir: projectDir, Framework: "html", Status: "active"})

	revID, err := CreateHomepageRevision(db, 1, projectDir, "initial commit", "testing", "agent", 1, true, `{"added":1}`)
	if err != nil {
		t.Fatalf("CreateHomepageRevision failed: %v", err)
	}
	if revID <= 0 {
		t.Errorf("expected positive revision id, got %d", revID)
	}

	_, err = CreateHomepageRevisionFile(db, revID, "index.html", "added", "", "<h1>Hello</h1>", "", "abc123", 0, 14)
	if err != nil {
		t.Fatalf("CreateHomepageRevisionFile failed: %v", err)
	}

	_, err = CreateHomepageRevisionFile(db, revID, "README.md", "added", "", "# Test Project", "", "def456", 0, 13)
	if err != nil {
		t.Fatalf("CreateHomepageRevisionFile failed: %v", err)
	}

	revisions, total, err := ListHomepageRevisions(db, projectDir, 20, 0)
	if err != nil {
		t.Fatalf("ListHomepageRevisions failed: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if len(revisions) != 1 {
		t.Errorf("expected 1 revision, got %d", len(revisions))
	}

	rev, err := GetHomepageRevision(db, revID)
	if err != nil {
		t.Fatalf("GetHomepageRevision failed: %v", err)
	}
	if rev.Message != "initial commit" {
		t.Errorf("expected message 'initial commit', got %q", rev.Message)
	}
	if rev.FileCount != 1 {
		t.Errorf("expected file_count=1, got %d", rev.FileCount)
	}

	files, err := GetHomepageRevisionFiles(db, revID)
	if err != nil {
		t.Fatalf("GetHomepageRevisionFiles failed: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	if err := DeleteHomepageRevision(db, revID); err != nil {
		t.Fatalf("DeleteHomepageRevision failed: %v", err)
	}

	revisions, total, err = ListHomepageRevisions(db, projectDir, 20, 0)
	if err != nil {
		t.Fatalf("ListHomepageRevisions after delete failed: %v", err)
	}
	if total != 0 {
		t.Errorf("expected total=0 after delete, got %d", total)
	}
}

func TestRevisionExclusions(t *testing.T) {
	if shouldExcludePath("node_modules/foo/bar") {
		t.Log("node_modules correctly excluded")
	}
	if shouldExcludePath(".git/config") {
		t.Log(".git correctly excluded")
	}
	if shouldExcludePath("dist/bundle.js") {
		t.Log("dist correctly excluded")
	}
	if shouldExcludePath("src/app/page.tsx") {
		t.Error("src/app/page.tsx should NOT be excluded")
	}
}

func TestHashContent(t *testing.T) {
	h1 := hashContent([]byte("hello"))
	h2 := hashContent([]byte("hello"))
	h3 := hashContent([]byte("world"))
	if h1 != h2 {
		t.Errorf("same content should produce same hash")
	}
	if h1 == h3 {
		t.Errorf("different content should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("expected sha256 hex length 64, got %d", len(h1))
	}
}

func TestIsBinaryContent(t *testing.T) {
	if isBinaryContent([]byte("hello world")) {
		t.Error("text should not be detected as binary")
	}
	if !isBinaryContent([]byte("hello\x00world")) {
		t.Error("text with null byte should be detected as binary")
	}
}

func TestComputeDelta(t *testing.T) {
	baseline := map[string]fileEntry{
		"a.txt": {Path: "a.txt", Data: []byte("old"), Hash: "hash_a"},
		"b.txt": {Path: "b.txt", Data: []byte("same"), Hash: "hash_b"},
	}
	current := []fileEntry{
		{Path: "a.txt", Data: []byte("new"), Hash: "hash_a2"},
		{Path: "b.txt", Data: []byte("same"), Hash: "hash_b"},
		{Path: "c.txt", Data: []byte("new"), Hash: "hash_c"},
	}
	delta := computeDelta(baseline, current)
	if len(delta.Modified) != 1 || delta.Modified[0].Path != "a.txt" {
		t.Errorf("expected modified=[a.txt], got %v", delta.Modified)
	}
	if len(delta.Added) != 1 || delta.Added[0].Path != "c.txt" {
		t.Errorf("expected added=[c.txt], got %v", delta.Added)
	}
	if len(delta.Deleted) != 0 {
		t.Errorf("expected no deleted, got %v", delta.Deleted)
	}
}

func TestSQLNullInt64Scans(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_null.db")
	db, err := InitHomepageRegistryDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init DB: %v", err)
	}
	defer db.Close()

	// Insert NULL for project_id directly via raw SQL since
	// CreateHomepageRevision cannot handle NULL with FK constraints enforced
	// (passing projectID=0 fails FK check: no project with id=0 exists).
	res, err := db.Exec(`INSERT INTO homepage_revisions (project_id, project_dir, message, reason, author, file_count, restorable, metadata_json) VALUES (NULL, ?, ?, ?, ?, ?, ?, ?)`,
		"test-dir", "no project", "", "agent", 0, 1, "")
	if err != nil {
		t.Fatalf("failed to insert revision with NULL project_id: %v", err)
	}
	revID, _ := res.LastInsertId()
	rev, err := GetHomepageRevision(db, revID)
	if err != nil {
		t.Fatalf("GetHomepageRevision failed: %v", err)
	}
	if rev.ProjectID != 0 {
		t.Errorf("expected project_id=0 for null, got %d", rev.ProjectID)
	}
}
