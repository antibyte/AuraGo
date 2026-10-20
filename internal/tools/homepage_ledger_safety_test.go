package tools

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomepageOperationRecordsReasonsAndPartialFiles(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	os.MkdirAll(filepath.Join(cfg.WorkspacePath, "site"), 0755)
	raw := ExecuteHomepageOperation(cfg, db, HomepageOperation{Name: "write_file", ProjectDir: "site", Reason: "Add contact details", Source: "homepage_file", FilesMayChange: true}, slog.Default(), func(c HomepageConfig) string {
		os.WriteFile(filepath.Join(c.WorkspacePath, "site", "index.html"), []byte("contact"), 0644)
		return `{"status":"error","message":"failed after writing"}`
	})
	if !strings.Contains(raw, "ledger_updated") {
		t.Fatal(raw)
	}
	site, err := GetProjectByDir(db, "site")
	if err != nil {
		t.Fatal(err)
	}
	events, err := ListHomepageSiteEvents(db, site.ID, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	terminal := false
	for _, e := range events {
		if e.EventType == "operation_finished" {
			terminal = e.Reason == "Add contact details" && e.Outcome == "partial"
		}
	}
	if !terminal {
		t.Fatalf("missing failed operation and reason: %#v", events)
	}
	state, err := GetHomepageProjectState(db, site.ID)
	if err != nil || state.CurrentRevisionID == 0 {
		t.Fatalf("partial file change missing: %+v %v", state, err)
	}
}

func TestHomepageSafeRestoreAndSensitiveRevisionExclusion(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	dir := filepath.Join(cfg.WorkspacePath, "site")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("v1"), 0755)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=test-only"), 0600)
	EnsureHomepageProjectForDir(db, cfg, "site", "", "")
	first := SaveHomepageRevisionAndState(cfg, db, "site", "first", "create", "test", nil, slog.Default())
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("v2"), 0755)
	os.WriteFile(filepath.Join(dir, "later.html"), []byte("later"), 0644)
	SaveHomepageRevisionAndState(cfg, db, "site", "second", "edit", "test", nil, slog.Default())
	raw := HomepageRestoreRevision(cfg, db, first.RevisionID, "", slog.Default())
	var result map[string]interface{}
	json.Unmarshal([]byte(raw), &result)
	if result["status"] != "ok" {
		t.Fatal(raw)
	}
	content, _ := os.ReadFile(filepath.Join(dir, "index.html"))
	if string(content) != "v1" {
		t.Fatal(string(content))
	}
	if _, err := os.Stat(filepath.Join(dir, "later.html")); !os.IsNotExist(err) {
		t.Fatal("later tracked file survived full restore")
	}
	var secrets int
	db.QueryRow("SELECT COUNT(*) FROM homepage_revision_files WHERE path='.env'").Scan(&secrets)
	if secrets != 0 {
		t.Fatal("credential file persisted")
	}
	latest, _ := GetLatestHomepageRevision(db, "site")
	if latest.ID <= first.RevisionID {
		t.Fatal("restore did not create a new revision")
	}
}

func TestHomepageLargeRevisionRestoreRefusesBeforeChangingFiles(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	dir := filepath.Join(cfg.WorkspacePath, "site")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "large.txt"), []byte(strings.Repeat("a", 600*1024)), 0644)
	EnsureHomepageProjectForDir(db, cfg, "site", "", "")
	first := SaveHomepageRevisionAndState(cfg, db, "site", "first", "create", "test", nil, slog.Default())
	os.WriteFile(filepath.Join(dir, "large.txt"), []byte("keep me"), 0644)
	raw := HomepageRestoreRevision(cfg, db, first.RevisionID, "", slog.Default())
	if !strings.Contains(raw, "incomplete") {
		t.Fatal(raw)
	}
	content, _ := os.ReadFile(filepath.Join(dir, "large.txt"))
	if string(content) != "keep me" {
		t.Fatal("unsafe restore changed current file")
	}
}

func TestHomepageWorkspacePinAndNestedIdentity(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	EnsureHomepageProjectForDir(db, cfg, "sites/one", "", "")
	dir, err := ResolveHomepageProjectDir(db, cfg, "sites/one/src/index.html", "", 0)
	if err != nil || dir != "sites/one" {
		t.Fatalf("%q %v", dir, err)
	}
	cfg.WorkspacePath = t.TempDir()
	if _, err := EnsureHomepageProjectForDir(db, cfg, "sites/one", "", ""); err == nil {
		t.Fatal("workspace switch silently rebound project")
	}
}

func TestHomepageOperationPreservesNestedLedgerFailure(t *testing.T) {
	op := HomepageOperation{ID: "fixture", Reason: "Publish website"}
	result, outcome := homepageOperationOutcome(`{"status":"partial_success","ledger_updated":false,"retry_safe":false}`)
	if outcome != "partial" {
		t.Fatal(outcome)
	}
	raw := homepageOperationResult(result, op, 1, 0)
	if !strings.Contains(raw, `"ledger_updated":false`) || !strings.Contains(raw, `"retry_safe":false`) {
		t.Fatal(raw)
	}
}

func TestHomepageNullOperationResultRemainsUnknown(t *testing.T) {
	result, outcome := homepageOperationOutcome("null")
	raw := homepageOperationResult(result, HomepageOperation{ID: "fixture", Reason: "Update website"}, 1, 0)
	if outcome != "unknown" || !strings.Contains(raw, `"retry_safe":false`) {
		t.Fatal(raw)
	}
}
