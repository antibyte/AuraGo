package tools

import (
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setHomepageFoldCase(t *testing.T, fold bool) {
	t.Helper()
	previous := homepageFoldsCase
	homepageFoldsCase = fold
	t.Cleanup(func() { homepageFoldsCase = previous })
}

func identityTestEnsure(t *testing.T, db *sql.DB, cfg HomepageConfig, dir string) *HomepageProject {
	t.Helper()
	proj, err := EnsureHomepageProjectForDir(db, cfg, dir, "", "")
	if err != nil {
		t.Fatalf("ensure %s: %v", dir, err)
	}
	return proj
}

func TestHomepageEnsureRejectsAncestorAndDescendantAllowsSiblings(t *testing.T) {
	setHomepageFoldCase(t, false)
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	site := identityTestEnsure(t, db, cfg, "clients/site")
	identityTestEnsure(t, db, cfg, "archive/site")

	_, err := EnsureHomepageProjectForDir(db, cfg, "clients", "", "")
	if !errors.Is(err, errHomepageProjectOverlap) || !strings.Contains(err.Error(), `would contain the registered site "clients/site"`) || !strings.Contains(err.Error(), "managed_site_id") {
		t.Fatalf("ancestor not rejected: %v", err)
	}
	_, err = EnsureHomepageProjectForDir(db, cfg, "clients/site/docs", "", "")
	if !errors.Is(err, errHomepageProjectOverlap) || !strings.Contains(err.Error(), `lies inside the registered site "clients/site"`) {
		t.Fatalf("descendant not rejected: %v", err)
	}
	if again := identityTestEnsure(t, db, cfg, "clients/site"); again.ID != site.ID {
		t.Fatalf("exact row changed: %d vs %d", again.ID, site.ID)
	}
}

func TestHomepageExecDiscoverySkipsOverlappingCandidate(t *testing.T) {
	setHomepageFoldCase(t, false)
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	os.MkdirAll(filepath.Join(cfg.WorkspacePath, "main"), 0755)
	os.MkdirAll(filepath.Join(cfg.WorkspacePath, "source"), 0755)
	identityTestEnsure(t, db, cfg, "main")
	identityTestEnsure(t, db, cfg, "source")
	raw := ExecuteHomepageOperation(cfg, db, HomepageOperation{Name: "exec", ProjectDir: "source", Reason: "Generate docs", FilesMayChange: true}, slog.Default(), func(c HomepageConfig) string {
		os.MkdirAll(filepath.Join(c.WorkspacePath, "main", "docs"), 0755)
		os.WriteFile(filepath.Join(c.WorkspacePath, "main", "docs", "index.html"), []byte("docs"), 0644)
		return `{"status":"ok"}`
	})
	out := decodeLedgerTestJSON(t, raw)
	if out["status"] != "ok" || out["ledger_updated"] != true {
		t.Fatalf("overlap became a ledger failure: %s", raw)
	}
	warnings, _ := out["warnings"].([]interface{})
	if len(warnings) != 1 || !strings.Contains(warnings[0].(string), "main/docs") {
		t.Fatalf("missing discovery warning: %s", raw)
	}
	if _, err := GetProjectByDir(db, "main/docs"); err == nil {
		t.Fatal("overlapping candidate was registered")
	}
}

func TestHomepageRelocateRejectsOverlap(t *testing.T) {
	setHomepageFoldCase(t, false)
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	os.MkdirAll(filepath.Join(cfg.WorkspacePath, "a", "inner"), 0755)
	os.MkdirAll(filepath.Join(cfg.WorkspacePath, "b"), 0755)
	identityTestEnsure(t, db, cfg, "a")
	moving := identityTestEnsure(t, db, cfg, "b")
	raw, _ := DispatchHomepageInventory(cfg, db, "relocate", moving.ID, "a/inner", "", 0, 0, "move", slog.Default())
	if !strings.Contains(raw, `lies inside the registered site \"a\"`) {
		t.Fatalf("relocate overlap not rejected: %s", raw)
	}
}

func TestHomepageFoldCaseModes(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}

	setHomepageFoldCase(t, true)
	site := identityTestEnsure(t, db, cfg, "Site")
	if variant := identityTestEnsure(t, db, cfg, "site"); variant.ID != site.ID || variant.ProjectDir != "Site" {
		t.Fatalf("fold mode must reuse the row with registry spelling: %+v", variant)
	}
	got, err := ResolveHomepageProjectDir(db, cfg, "SITE/index.html", "", 0)
	if err != nil || got != "Site" {
		t.Fatalf("longest boundary = %q, %v", got, err)
	}
	got, err = ResolveHomepageProjectDir(db, cfg, "site/index.html", "SITE", site.ID)
	if err != nil || got != "Site" {
		t.Fatalf("managed id + case variant = %q, %v", got, err)
	}

	setHomepageFoldCase(t, false)
	other := identityTestEnsure(t, db, cfg, "site")
	if other.ID == site.ID {
		t.Fatal("non-fold mode must keep distinct rows")
	}
	if _, err := ResolveHomepageProjectDir(db, cfg, "x/index.html", "SITE", site.ID); err == nil {
		t.Fatal("non-fold mode must treat case variants as different")
	}
}

func TestHomepageResolveRemoteOnlyAndUnknownSite(t *testing.T) {
	setHomepageFoldCase(t, false)
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	res, err := db.Exec("INSERT INTO homepage_projects (name, project_dir, status) VALUES ('remote', '', 'active')")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	if _, err := ResolveHomepageProjectDir(db, cfg, "index.html", "", id); err == nil || !strings.Contains(err.Error(), "remote-only") {
		t.Fatalf("remote-only + path: %v", err)
	}
	if dir, err := ResolveHomepageProjectDir(db, cfg, "", "", id); err != nil || dir != "" {
		t.Fatalf("remote-only without path changed: %q %v", dir, err)
	}
	if _, err := ResolveHomepageProjectDir(db, cfg, "a/b.html", "", 9999); err == nil || !strings.Contains(err.Error(), "managed site 9999 not found; list sites with homepage_registry overview") {
		t.Fatalf("unknown id: %v", err)
	}
	raw, _ := DispatchHomepageInventory(cfg, db, "get_site", 9999, "", "", 0, 0, "", slog.Default())
	if !strings.Contains(raw, "managed site 9999 not found") {
		t.Fatalf("get_site: %s", raw)
	}
}

func TestHomepageEnsureDerivedNameCollisionGetsSuffix(t *testing.T) {
	setHomepageFoldCase(t, false)
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	identityTestEnsure(t, db, cfg, "a/b")
	second := identityTestEnsure(t, db, cfg, "a-b")
	if second.Name != "a-b-2" {
		t.Fatalf("derived name = %q, want a-b-2", second.Name)
	}
	if _, err := EnsureHomepageProjectForDir(db, cfg, "c", "a-b", ""); err == nil {
		t.Fatal("explicit duplicate name must still fail")
	}
}

func TestHomepageWriteOperationReportsSiteCreated(t *testing.T) {
	setHomepageFoldCase(t, false)
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	run := func() map[string]interface{} {
		raw := ExecuteHomepageOperation(cfg, db, HomepageOperation{Name: "write_file", ProjectDir: "my-stie", Reason: "Write page", Source: "homepage_file", FilesMayChange: true}, slog.Default(), func(c HomepageConfig) string {
			os.MkdirAll(filepath.Join(c.WorkspacePath, "my-stie"), 0755)
			os.WriteFile(filepath.Join(c.WorkspacePath, "my-stie", "index.html"), []byte("x"), 0644)
			return `{"status":"ok"}`
		})
		return decodeLedgerTestJSON(t, raw)
	}
	first := run()
	warnings, _ := first["warnings"].([]interface{})
	if first["site_created"] != true || len(warnings) != 1 || !strings.Contains(warnings[0].(string), `registered new managed site "my-stie"`) {
		t.Fatalf("missing notice: %v", first)
	}
	if second := run(); second["site_created"] != nil || second["warnings"] != nil {
		t.Fatalf("notice repeated for an existing site: %v", second)
	}
}
