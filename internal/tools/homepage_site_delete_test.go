package tools

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func deleteTestSeed(t *testing.T, db *sql.DB, cfg HomepageConfig, dir string) *HomepageProject {
	t.Helper()
	full := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(dir))
	if err := os.MkdirAll(filepath.Join(full, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, "sub", "index.html"), []byte("<p>x</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	proj := identityTestEnsure(t, db, cfg, dir)
	seedHomepageProjectRows(t, db, proj.ID, dir)
	return proj
}

func seedHomepageProjectRows(t *testing.T, db *sql.DB, id int64, dir string) {
	t.Helper()
	if _, err := RecordHomepageEvent(db, id, "note", "test", "seed", nil); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"INSERT INTO homepage_history(project_id,content) VALUES(?, 'h')",
		"INSERT INTO homepage_deploy_targets(project_id,provider,provider_target_id,target_key) VALUES(?, 'netlify','n1','k1')",
		"INSERT INTO homepage_deployments(project_id,provider) VALUES(?, 'netlify')",
		"INSERT INTO homepage_remote_observations(project_id,provider) VALUES(?, 'netlify')",
		"INSERT INTO homepage_site_file_state(project_id,rel_path,content_hash) VALUES(?, 'index.html','abc')",
		"INSERT OR REPLACE INTO homepage_project_state(project_id) VALUES(?)",
	} {
		if _, err := db.Exec(q, id); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	res, err := db.Exec("INSERT INTO homepage_revisions(project_id,project_dir) VALUES(?,?)", id, dir)
	if err != nil {
		t.Fatal(err)
	}
	rev, _ := res.LastInsertId()
	if _, err := db.Exec("INSERT INTO homepage_revision_files(revision_id,path,change_type) VALUES(?, 'index.html','add')", rev); err != nil {
		t.Fatal(err)
	}
}

func countHomepageProjectRows(t *testing.T, db *sql.DB, id int64) int {
	t.Helper()
	total := 0
	for _, q := range []string{
		"SELECT COUNT(*) FROM homepage_projects WHERE id=?",
		"SELECT COUNT(*) FROM homepage_site_events WHERE project_id=?",
		"SELECT COUNT(*) FROM homepage_history WHERE project_id=?",
		"SELECT COUNT(*) FROM homepage_deploy_targets WHERE project_id=?",
		"SELECT COUNT(*) FROM homepage_deployments WHERE project_id=?",
		"SELECT COUNT(*) FROM homepage_remote_observations WHERE project_id=?",
		"SELECT COUNT(*) FROM homepage_site_file_state WHERE project_id=?",
		"SELECT COUNT(*) FROM homepage_project_state WHERE project_id=?",
		"SELECT COUNT(*) FROM homepage_revisions WHERE project_id=?",
		"SELECT COUNT(*) FROM homepage_revision_files WHERE revision_id IN (SELECT id FROM homepage_revisions WHERE project_id=?)",
	} {
		var n int
		if err := db.QueryRow(q, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		total += n
	}
	return total
}

func TestHomepageDeleteSiteRemovesFolderAndRowsOnly(t *testing.T) {
	setHomepageFoldCase(t, false)
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	victim := deleteTestSeed(t, db, cfg, "victim")
	keeper := deleteTestSeed(t, db, cfg, "keeper")
	keeperRows := countHomepageProjectRows(t, db, keeper.ID)

	got, err := DeleteHomepageSite(cfg, db, victim.ID, "user", nil)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !got.FolderRemoved || got.ID != victim.ID || got.TotalRows < 10 {
		t.Fatalf("unexpected result %+v", got)
	}
	if _, err := os.Stat(filepath.Join(cfg.WorkspacePath, "victim")); !os.IsNotExist(err) {
		t.Fatalf("folder still there: %v", err)
	}
	if n := countHomepageProjectRows(t, db, victim.ID); n != 0 {
		t.Fatalf("%d rows left", n)
	}
	if _, err := os.Stat(filepath.Join(cfg.WorkspacePath, "keeper", "sub", "index.html")); err != nil {
		t.Fatalf("other folder damaged: %v", err)
	}
	if n := countHomepageProjectRows(t, db, keeper.ID); n != keeperRows {
		t.Fatalf("other site rows changed: %d -> %d", keeperRows, n)
	}
	if _, err := DeleteHomepageSite(cfg, db, victim.ID, "user", nil); !errors.Is(err, ErrHomepageSiteNotFound) {
		t.Fatalf("second delete = %v", err)
	}
}

func TestHomepageDeleteSiteRefusesNestedSites(t *testing.T) {
	setHomepageFoldCase(t, false)
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	outer := deleteTestSeed(t, db, cfg, "outer")
	if err := ArchiveHomepageSite(db, outer.ID, "user", "test"); err != nil {
		t.Fatal(err)
	}
	inner := deleteTestSeed(t, db, cfg, "outer/inner") // allowed: the outer site is archived
	for _, id := range []int64{outer.ID, inner.ID} {
		_, err := DeleteHomepageSite(cfg, db, id, "user", nil)
		if !errors.Is(err, ErrHomepageSiteNested) || !strings.Contains(err.Error(), "managed_site_id") {
			t.Fatalf("delete %d = %v", id, err)
		}
	}
	if _, err := os.Stat(filepath.Join(cfg.WorkspacePath, "outer", "inner", "sub", "index.html")); err != nil {
		t.Fatalf("files removed despite refusal: %v", err)
	}
	if countHomepageProjectRows(t, db, outer.ID) == 0 || countHomepageProjectRows(t, db, inner.ID) == 0 {
		t.Fatal("rows removed despite refusal")
	}
}

func TestHomepageDeleteSiteRefusesUnsafeFolders(t *testing.T) {
	setHomepageFoldCase(t, false)
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, dir := range map[string]string{"escape": "../" + filepath.Base(outside), "root": ".", "abs": outside} {
		res, err := db.Exec("INSERT INTO homepage_projects(name,project_dir,origin) VALUES(?,?,'unknown')", name, dir)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		if _, err := DeleteHomepageSite(cfg, db, id, "user", nil); !errors.Is(err, ErrHomepageSiteFolderUnsafe) {
			t.Fatalf("%s: %v", name, err)
		}
		if countHomepageProjectRows(t, db, id) == 0 {
			t.Fatalf("%s: registry row removed", name)
		}
	}
	if _, err := os.Stat(filepath.Join(outside, "keep.txt")); err != nil {
		t.Fatalf("outside file removed: %v", err)
	}
	// A site bound to another workspace is refused as well.
	other := deleteTestSeed(t, db, HomepageConfig{WorkspacePath: t.TempDir()}, "foreign")
	if _, err := DeleteHomepageSite(cfg, db, other.ID, "user", nil); !errors.Is(err, ErrHomepageSiteFolderUnsafe) {
		t.Fatalf("foreign workspace: %v", err)
	}
}

func TestHomepageDeleteSiteSymlinkEscape(t *testing.T) {
	setHomepageFoldCase(t, false)
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(cfg.WorkspacePath, "link")); err != nil {
		t.Skipf("symlinks unavailable here (needs privileges on Windows): %v", err)
	}
	res, err := db.Exec("INSERT INTO homepage_projects(name,project_dir,origin,workspace_root) VALUES('viasym','link/site','unknown',?)", homepageWorkspaceRoot(cfg))
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	if _, err := DeleteHomepageSite(cfg, db, id, "user", nil); err == nil {
		// Missing folder below the link is acceptable only if nothing outside was touched.
		t.Log("delete reported success")
	}
	if _, err := os.Stat(filepath.Join(outside, "keep.txt")); err != nil {
		t.Fatalf("file outside the workspace was removed: %v", err)
	}
}

func TestHomepageDeleteRemoteOnlyAndMissingFolder(t *testing.T) {
	setHomepageFoldCase(t, false)
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	res, err := db.Exec("INSERT INTO homepage_projects(name,project_dir,origin) VALUES('remote','','unknown')")
	if err != nil {
		t.Fatal(err)
	}
	remote, _ := res.LastInsertId()
	seedHomepageProjectRows(t, db, remote, "")
	got, err := DeleteHomepageSite(cfg, db, remote, "user", nil)
	if err != nil || got.FolderRemoved || countHomepageProjectRows(t, db, remote) != 0 {
		t.Fatalf("remote-only: %+v %v", got, err)
	}

	site := deleteTestSeed(t, db, cfg, "gone")
	if err := os.RemoveAll(filepath.Join(cfg.WorkspacePath, "gone")); err != nil {
		t.Fatal(err)
	}
	got, err = DeleteHomepageSite(cfg, db, site.ID, "user", nil)
	if err != nil || got.FolderRemoved || countHomepageProjectRows(t, db, site.ID) != 0 {
		t.Fatalf("missing folder: %+v %v", got, err)
	}
}

func insertLegacySite(t *testing.T, db *sql.DB, name, dir, workspaceRoot string) int64 {
	t.Helper()
	res, err := db.Exec("INSERT INTO homepage_projects(name,project_dir,origin,workspace_root) VALUES(?,?,'unknown',?)", name, dir, workspaceRoot)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestHomepageFoldDirStripsTrailingDots(t *testing.T) {
	setHomepageFoldCase(t, true)
	prev := homepageStripsTrailingDots
	homepageStripsTrailingDots = true
	t.Cleanup(func() { homepageStripsTrailingDots = prev })
	if homepageFoldDir("Clients./Site. ") != "clients/site" {
		t.Fatalf("got %q", homepageFoldDir("Clients./Site. "))
	}
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	identityTestEnsure(t, db, cfg, "site")
	if _, err := EnsureHomepageProjectForDir(db, cfg, "site.", "", ""); !errors.Is(err, errHomepageProjectOverlap) && err == nil {
		// Same identity may also resolve to the existing row; both are acceptable, a second row is not.
		var n int
		db.QueryRow("SELECT COUNT(*) FROM homepage_projects").Scan(&n)
		if n != 1 {
			t.Fatalf("site. registered as a second site (%d rows)", n)
		}
	}
}

func TestHomepageDeleteRefusesTrailingDotAliasAndLegacyForms(t *testing.T) {
	setHomepageFoldCase(t, true)
	prev := homepageStripsTrailingDots
	homepageStripsTrailingDots = true
	t.Cleanup(func() { homepageStripsTrailingDots = prev })
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	site := deleteTestSeed(t, db, cfg, "site")
	root := homepageWorkspaceRoot(cfg)

	// F1: legacy row "site." next to "site".
	alias := insertLegacySite(t, db, "alias", "site.", root)
	if _, err := DeleteHomepageSite(cfg, db, site.ID, "user", nil); !errors.Is(err, ErrHomepageSiteNested) {
		t.Fatalf("trailing-dot alias: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.WorkspacePath, "site", "sub", "index.html")); err != nil {
		t.Fatalf("files removed: %v", err)
	}
	db.Exec("DELETE FROM homepage_projects WHERE id=?", alias)

	// F2: a legacy "./site/inner" row is normalized and found inside.
	insertLegacySite(t, db, "legacy", "./site/inner", root)
	if _, err := DeleteHomepageSite(cfg, db, site.ID, "user", nil); !errors.Is(err, ErrHomepageSiteNested) {
		t.Fatalf("legacy ./ form: %v", err)
	}
	// An unreadable legacy dir fails closed, naming the site id.
	db.Exec("DELETE FROM homepage_projects WHERE name='legacy'")
	bad := insertLegacySite(t, db, "bad", "C:/abs/path", root)
	if _, err := DeleteHomepageSite(cfg, db, site.ID, "user", nil); !errors.Is(err, ErrHomepageSiteFolderUnsafe) || !strings.Contains(err.Error(), "managed_site_id "+strings.TrimSpace(strconvI(bad))) {
		t.Fatalf("unreadable legacy dir: %v", err)
	}
	db.Exec("DELETE FROM homepage_projects WHERE id=?", bad)

	// F3: workspace_root differing only by case still counts.
	insertLegacySite(t, db, "caseroot", "site/inner", strings.ToUpper(root))
	if _, err := DeleteHomepageSite(cfg, db, site.ID, "user", nil); !errors.Is(err, ErrHomepageSiteNested) {
		t.Fatalf("case-variant workspace root: %v", err)
	}
}

func strconvI(n int64) string { return fmt.Sprintf("%d", n) }
