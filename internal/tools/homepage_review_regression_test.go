package tools

import (
	"aurago/internal/dbutil"
	"aurago/internal/security"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHomepageReviewInitPreservesPreexistingProvenance(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	dir := filepath.Join(cfg.WorkspacePath, "existing")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	raw := ExecuteHomepageOperation(cfg, db, HomepageOperation{Name: "init_project", ProjectDir: "existing", Reason: "Refresh the template", FilesMayChange: true}, slog.Default(), func(c HomepageConfig) string {
		if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("replacement"), 0644); err != nil {
			t.Fatal(err)
		}
		return `{"status":"ok"}`
	})
	if !strings.Contains(raw, `"ledger_updated":true`) {
		t.Fatal(raw)
	}
	site, err := GetProjectByDir(db, "existing")
	if err != nil {
		t.Fatal(err)
	}
	revisions, total, err := ListHomepageRevisions(db, "existing", 10, 0)
	if err != nil || total != 2 {
		t.Fatalf("original content lost from history: %d %+v %v", total, revisions, err)
	}
	before, err := GetHomepageRevisionFiles(db, revisions[1].ID)
	if err != nil || len(before) != 1 || before[0].ContentAfter != "original" || revisions[1].Author != "external" {
		t.Fatalf("bad preimage: %+v %+v %v", revisions, before, err)
	}
	managed, err := GetHomepageManagedSite(db, site.ID)
	if err != nil || managed.Origin == "agent_created" {
		t.Fatalf("preexisting website attributed to scaffold: %+v %v", managed, err)
	}
}

func TestHomepageReviewMetadataUpdateCannotRelocate(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	project, err := EnsureHomepageProjectForDir(db, cfg, "original", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateProject(db, project.ID, map[string]interface{}{"project_dir": "moved"}); err == nil {
		t.Fatal("metadata update bypassed the identity-preserving relocate operation")
	}
	after, err := GetProject(db, project.ID)
	if err != nil || after.ProjectDir != "original" {
		t.Fatalf("identity changed: %+v %v", after, err)
	}
}

func TestHomepageReviewRelocateWaitsForWorkspaceMutation(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	for _, name := range []string{"original", "moved"} {
		if err := os.MkdirAll(filepath.Join(cfg.WorkspacePath, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	project, err := EnsureHomepageProjectForDir(db, cfg, "original", "", "")
	if err != nil {
		t.Fatal(err)
	}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan string, 1)
	go func() {
		done <- ExecuteHomepageOperation(cfg, db, HomepageOperation{Name: "write_file", ProjectDir: "original", Reason: "Update the page"}, slog.Default(), func(c HomepageConfig) string {
			close(entered)
			<-release
			return `{"status":"ok"}`
		})
	}()
	<-entered
	// A canceled waiter must fail instead of changing identity under the writer.
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	cfg.Context = ctx
	raw, handled := DispatchHomepageInventory(cfg, db, "relocate", project.ID, "moved", "", 20, 0, "Record the new folder", slog.Default())
	close(release)
	<-done
	if !handled || !strings.Contains(raw, `"status":"error"`) {
		t.Fatalf("relocate raced an in-flight mutation: %s", raw)
	}
	after, err := GetProject(db, project.ID)
	if err != nil || after.ProjectDir != "original" {
		t.Fatalf("identity changed under writer: %+v %v", after, err)
	}
}

func TestHomepageReviewRestorePreflightsDirectoryConflict(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	dir := filepath.Join(cfg.WorkspacePath, "site")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.html", "z"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	first := SaveHomepageRevisionAndState(cfg, db, "site", "first", "Create", "test", nil, slog.Default())
	if len(first.Warnings) > 0 {
		t.Fatal(first.Warnings)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.html"), []byte("current"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "z")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "z"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "z", "child.html"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	SaveHomepageRevisionAndState(cfg, db, "site", "second", "Restructure", "test", nil, slog.Default())
	raw := HomepageRestoreRevision(cfg, db, first.RevisionID, "", slog.Default())
	content, err := os.ReadFile(filepath.Join(dir, "a.html"))
	if err != nil || string(content) != "current" || !strings.Contains(raw, `"status":"error"`) {
		t.Fatalf("predictable conflict partially restored files: %q %s %v", content, raw, err)
	}
}

func TestHomepageReviewCanceledRestoreDoesNotChangeFiles(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	dir := filepath.Join(cfg.WorkspacePath, "site")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "index.html")
	os.WriteFile(path, []byte("old"), 0644)
	first := SaveHomepageRevisionAndState(cfg, db, "site", "first", "Create", "test", nil, slog.Default())
	os.WriteFile(path, []byte("current"), 0644)
	ctx, cancel := context.WithCancel(context.Background())
	cfg.Context = ctx
	raw := ExecuteHomepageOperation(cfg, db, HomepageOperation{Name: "restore_revision", ProjectDir: "site", Reason: "Restore the previous page", FilesMayChange: true}, slog.Default(), func(c HomepageConfig) string {
		cancel()
		return HomepageRestoreRevision(c, db, first.RevisionID, "", slog.Default())
	})
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "current" {
		t.Fatalf("canceled restore still wrote files: %q %s %v", content, raw, err)
	}
}

func TestHomepageReviewRevisionTracksModeChanges(t *testing.T) {
	hash := hashContent([]byte("script"))
	baseline := map[string]fileEntry{"run.sh": {Path: "run.sh", Hash: hash, Mode: 0644}}
	delta := computeDelta(baseline, []fileEntry{{Path: "run.sh", Hash: hash, Mode: 0755}})
	if len(delta.Modified) != 1 {
		t.Fatal("permission-only mutation was omitted from revision")
	}
}

func TestHomepageReviewRestoreRestoresFileMode(t *testing.T) {
	for _, mode := range []os.FileMode{0444, 0666} {
		t.Run(fmt.Sprintf("mode=%o", mode), func(t *testing.T) {
			db := newHomepageLedgerTestDB(t)
			cfg := HomepageConfig{WorkspacePath: t.TempDir()}
			dir := filepath.Join(cfg.WorkspacePath, "site")
			os.MkdirAll(dir, 0755)
			path := filepath.Join(dir, "index.html")
			os.WriteFile(path, []byte("unchanged"), 0644)
			defer os.Chmod(path, 0666)
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
			first := SaveHomepageRevisionAndState(cfg, db, "site", "first", "Read-only page", "test", nil, slog.Default())
			if len(first.Warnings) > 0 {
				t.Fatal(first.Warnings)
			}
			if err := os.Chmod(path, mode^0222); err != nil {
				t.Fatal(err)
			}
			raw := HomepageRestoreRevision(cfg, db, first.RevisionID, "", slog.Default())
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &result); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != mode || result["status"] != "ok" {
				t.Fatalf("file mode not restored: %v %s %v", info, raw, err)
			}
		})
	}
}

func TestHomepageReviewLegacySecretIsNotCopiedIntoNewRevision(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		t.Run(fmt.Sprintf("deleted=%t", deleted), func(t *testing.T) {
			db := newHomepageLedgerTestDB(t)
			cfg := HomepageConfig{WorkspacePath: t.TempDir()}
			dir := filepath.Join(cfg.WorkspacePath, "site")
			os.MkdirAll(dir, 0755)
			os.WriteFile(filepath.Join(dir, "index.js"), []byte("public"), 0644)
			project, err := EnsureHomepageProjectForDir(db, cfg, "site", "", "")
			if err != nil {
				t.Fatal(err)
			}
			secret := "homepage-review-only-synthetic-sensitive-value"
			security.RegisterSensitive(secret)
			result, err := db.Exec("INSERT INTO homepage_revisions(project_id,project_dir) VALUES(?,'site')", project.ID)
			if err != nil {
				t.Fatal(err)
			}
			id, _ := result.LastInsertId()
			if _, err := db.Exec("INSERT INTO homepage_revision_files(revision_id,path,change_type,content_after,content_hash_after,size_after) VALUES(?,'index.js','added',?,?,?)", id, secret, hashContent([]byte(secret)), len(secret)); err != nil {
				t.Fatal(err)
			}
			if deleted {
				if err := os.Remove(filepath.Join(dir, "index.js")); err != nil {
					t.Fatal(err)
				}
			}
			saved := SaveHomepageRevisionAndState(cfg, db, "site", "Remove embedded secret", "Clean up credentials", "test", nil, slog.Default())
			if len(saved.Warnings) > 0 {
				t.Fatal(saved.Warnings)
			}
			files, err := GetHomepageRevisionFiles(db, saved.RevisionID)
			if err != nil || len(files) != 1 {
				t.Fatal("new revision missing", err)
			}
			if strings.Contains(files[0].ContentBefore, secret) {
				t.Fatal("legacy credential was copied into the new revision")
			}
		})
	}
}

func TestHomepageReviewLegacySecretCannotBeRestored(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.js"), []byte("public"), 0644)
	secret := "homepage-review-only-restored-sensitive-value"
	security.RegisterSensitive(secret)
	restored, warnings := RestoreRevisionFiles(dir, []HomepageRevisionFile{{Path: "index.js", ChangeType: "modified", ContentAfter: secret, ContentHashAfter: hashContent([]byte(secret)), SizeAfter: int64(len(secret))}}, slog.Default())
	content, err := os.ReadFile(filepath.Join(dir, "index.js"))
	if err != nil || len(restored) != 0 || len(warnings) == 0 || string(content) != "public" {
		t.Fatal("legacy credential was restored to a public file")
	}
}

func TestHomepageReviewURIUpgradeBacksUpTheUnderlyingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := dbutil.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile("testdata/homepage_registry_v0.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = InitHomepageRegistryDB("file:" + filepath.ToSlash(path) + "?mode=rw")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	backups, err := filepath.Glob(path + ".homepage-v1-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("underlying database backup missing: %v %v", backups, err)
	}
}

func TestHomepageReviewRestoreRejectsNonlocalPathBeforeWalkingParents(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := validateHomepageRestoreFiles(root, []HomepageRevisionFile{{Path: filepath.Join(t.TempDir(), "index.html"), ChangeType: "deleted"}}); err == nil {
		t.Fatal("absolute restore path accepted")
	}
}

func TestHomepageReviewRestoreRecreatesDeletedParentFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "assets"), []byte("old file"), 0644); err != nil {
		t.Fatal(err)
	}
	content := "restored child"
	restored, warnings := RestoreRevisionFiles(dir, []HomepageRevisionFile{
		{Path: "assets", ChangeType: "deleted"},
		{Path: "assets/nested/page.html", ChangeType: "added", ContentAfter: content, ContentHashAfter: hashContent([]byte(content)), SizeAfter: int64(len(content))},
	}, slog.Default())
	actual, err := os.ReadFile(filepath.Join(dir, "assets", "nested", "page.html"))
	if err != nil || len(warnings) > 0 || len(restored) != 2 || string(actual) != content {
		t.Fatalf("valid file-to-directory restore failed: %v %v", warnings, err)
	}
}

func TestHomepageReviewRegisteredSecretSnapshotKeepsOnlyFingerprint(t *testing.T) {
	dir := t.TempDir()
	secret := "homepage-review-only-snapshot-sensitive-value"
	security.RegisterSensitive(secret)
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(secret), 0644); err != nil {
		t.Fatal(err)
	}
	files, err := enumerateProjectFiles(dir, slog.Default())
	if err != nil || len(files) != 1 || len(files[0].Data) != 0 || files[0].Hash != hashContent([]byte(secret)) {
		t.Fatalf("registered secret persisted in revision payload: count=%d error=%v", len(files), err)
	}
}

func TestHomepageReviewRemoteIdentityCannotBeMovedViaMetadata(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	id, _, err := RegisterProject(db, HomepageProject{Name: "Remote-only site", ProjectDir: "remote-site"})
	if err != nil {
		t.Fatal(err)
	}
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{ProjectID: id, Provider: "netlify", ProviderTargetID: "remote-only", URL: "https://example.com", Status: "ok"}); err != nil {
		t.Fatal(err)
	}
	if err := UpdateProject(db, id, map[string]interface{}{"project_dir": "different"}); err == nil {
		t.Fatal("metadata update moved a site with a remote deployment")
	}
}
