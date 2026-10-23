package tools

import (
	"aurago/internal/dbutil"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomepageLegacyMigrationBacksUpAndPreservesTargetLinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := dbutil.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile("testdata/homepage_registry_v0.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO homepage_projects(id,name,project_dir) VALUES(1,'Site','site'); INSERT INTO homepage_deploy_targets(id,project_id,provider,provider_target_id,url) VALUES(7,1,'vercel','p','https://example.test'); INSERT INTO homepage_deployments(project_id,target_id,provider,url) VALUES(1,7,'vercel','https://example.test')")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = InitHomepageRegistryDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	backups, _ := filepath.Glob(path + ".homepage-v*-*.bak")
	if len(backups) != 1 {
		t.Fatalf("missing migration backup: %v", backups)
	}
	var id int64
	if err := db.QueryRow("SELECT target_id FROM homepage_deployments").Scan(&id); err != nil || id != 7 {
		t.Fatalf("target link lost: %d %v", id, err)
	}
	backup, err := sql.Open("sqlite", "file:"+filepath.ToSlash(backups[0])+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var version int
	if err := backup.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 0 {
		t.Fatalf("backup was not pre-migration: %d %v", version, err)
	}
}
func TestHomepageMultipleTargetsAndRestartRecovery(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	project, err := EnsureHomepageProjectForDir(db, cfg, "site", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, environment := range []string{"preview", "production", "preview"} {
		err := RecordHomepageDeployment(db, HomepageDeploymentRecord{ProjectID: project.ID, Provider: "vercel", ProviderTargetID: "p", URL: "https://example.test", Metadata: map[string]interface{}{"target": environment}})
		if err != nil {
			t.Fatal(err)
		}
	}
	targets, err := ListHomepageManagedDeployTargets(db, project.ID)
	if err != nil || len(targets) != 2 {
		t.Fatalf("targets overwritten: %+v %v", targets, err)
	}
	op := HomepageOperation{ID: "crash-fixture", Name: "write_file", Reason: "Fix menu", Actor: "agent"}
	if _, err := recordHomepageOperationEvent(db, project.ID, op, "operation_started", "pending", nil); err != nil {
		t.Fatal(err)
	}
	if err := RecoverHomepageOperations(db); err != nil {
		t.Fatal(err)
	}
	if err := RecoverHomepageOperations(db); err != nil {
		t.Fatal(err)
	}
	var terminal int
	db.QueryRow("SELECT COUNT(*) FROM homepage_site_events WHERE correlation_id='crash-fixture' AND event_type='operation_finished'").Scan(&terminal)
	if terminal != 1 {
		t.Fatalf("recovery is not idempotent: %d", terminal)
	}
}
func TestHomepageRevisionHistoryBeyondOneThousand(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for id := 1; id <= 1005; id++ {
		if _, err := tx.Exec("INSERT INTO homepage_revisions(id,project_dir) VALUES(?,'site')", id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec("INSERT INTO homepage_revision_files(revision_id,path,change_type,content_after,content_hash_after,size_after) VALUES(1,'index.html','added','v1',?,2)", hashContent([]byte("v1"))); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	latest, err := GetLatestHomepageRevision(db, "site")
	if err != nil || latest.ID != 1005 {
		t.Fatalf("unstable latest revision: %+v %v", latest, err)
	}
	state, err := reconstructProjectState(db, "site", latest.ID, slog.Default())
	if err != nil || string(state["index.html"].Data) != "v1" {
		t.Fatalf("lost old baseline: %+v %v", state, err)
	}
}

func TestHomepageExecDiscoversNewSitesAndPreservesReasons(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	os.MkdirAll(filepath.Join(cfg.WorkspacePath, "source"), 0755)
	raw := ExecuteHomepageOperation(cfg, db, HomepageOperation{Name: "exec", ProjectDir: "source", Reason: "Create campaign page", Actor: "agent", FilesMayChange: true}, slog.Default(), func(c HomepageConfig) string {
		os.MkdirAll(filepath.Join(c.WorkspacePath, "campaign"), 0755)
		os.WriteFile(filepath.Join(c.WorkspacePath, "campaign", "index.html"), []byte("campaign"), 0644)
		return `{"status":"ok"}`
	})
	if !strings.Contains(raw, "ledger_updated") {
		t.Fatal(raw)
	}
	project, err := GetProjectByDir(db, "campaign")
	if err != nil {
		t.Fatal(err)
	}
	site, err := GetHomepageManagedSite(db, project.ID)
	if err != nil || site.Origin != "agent_created" {
		t.Fatalf("new site missing: %+v %v", site, err)
	}
	found := false
	for _, event := range site.Events {
		if event.Reason == "Create campaign page" {
			found = true
		}
	}
	if !found {
		t.Fatal("new site lost the command's purpose")
	}
}

func TestHomepageNativeProviderDuplicateKeepsRemoteLocationAndReason(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	raw := ExecuteHomepageProviderOperation(cfg, db, "here_now", "original", "personal", "duplicate", HomepageOperation{Reason: "Create campaign variant", Actor: "agent", Source: "here_now_site"}, slog.Default(), func() string {
		return `{"slug":"campaign-variant","url":"https://campaign-variant.example.test"}`
	})
	if !strings.Contains(raw, "ledger_updated") {
		t.Fatal(raw)
	}
	sites, err := ListHomepageManagedSites(db)
	if err != nil || len(sites) != 2 || sites[0].LocalStatus != "remote_only" {
		t.Fatalf("remote identity missing: %+v %v", sites, err)
	}
	var createdID int64
	for _, site := range sites {
		if site.Origin == "agent_created" {
			createdID = site.ID
		}
	}
	site, err := GetHomepageManagedSite(db, createdID)
	if err != nil || len(site.Events) == 0 || site.Events[0].Reason != "Create campaign variant" || len(site.DeployTargets) != 1 || site.DeployTargets[0].URL != "https://campaign-variant.example.test" {
		t.Fatalf("remote location missing: %+v %v", site, err)
	}
}

func TestHomepageRelocateAttachesRemoteOnlySiteWithoutLosingIdentity(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	result, err := db.Exec("INSERT INTO homepage_projects(name,project_dir) VALUES('Remote only','')")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(cfg.WorkspacePath, "local-site"), 0755); err != nil {
		t.Fatal(err)
	}
	raw, handled := DispatchHomepageInventory(cfg, db, "relocate", id, "local-site", "", 20, 0, "Attach its checked out source", slog.Default())
	if !handled || !strings.Contains(raw, `"status":"success"`) {
		t.Fatal(raw)
	}
	state, err := GetHomepageProjectState(db, id)
	if err != nil || state.LocalRoot != filepath.Join(cfg.WorkspacePath, "local-site") {
		t.Fatalf("%+v %v", state, err)
	}
}
func TestHomepageAutomaticWebServerIntentUsesSavedProject(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	for _, dir := range []string{"one", "two"} {
		os.MkdirAll(filepath.Join(cfg.WorkspacePath, dir), 0755)
		os.WriteFile(filepath.Join(cfg.WorkspacePath, dir, "index.html"), []byte(dir), 0644)
	}
	if err := saveWebserverState(cfg.WorkspacePath, "two", "."); err != nil {
		t.Fatal(err)
	}
	raw := ExecuteHomepageOperation(cfg, db, HomepageOperation{Name: "webserver_start", Reason: "Restore published site"}, slog.Default(), func(c HomepageConfig) string {
		if c.ManagedProjectDir != "two" {
			t.Fatalf("intent belongs to %q", c.ManagedProjectDir)
		}
		return `{"status":"ok"}`
	})
	if !strings.Contains(raw, `"ledger_updated":true`) {
		t.Fatal(raw)
	}
	site, err := GetProjectByDir(db, "two")
	if err != nil {
		t.Fatal(err)
	}
	events, err := ListHomepageSiteEvents(db, site.ID, 20, 0)
	if err != nil || len(events) < 2 {
		t.Fatalf("%+v %v", events, err)
	}
}

func TestHomepageProviderMutationPreservesKnownAccountIdentity(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	project, err := EnsureHomepageProjectForDir(db, cfg, "site", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{ProjectID: project.ID, Provider: "here_now", ProviderTargetID: "slug", URL: "https://slug.example.test", Metadata: map[string]interface{}{"account": "account-id"}}); err != nil {
		t.Fatal(err)
	}
	raw := ExecuteHomepageProviderOperation(cfg, db, "here_now", "slug", "", "update_metadata", HomepageOperation{Reason: "Correct the title"}, slog.Default(), func() string { return `{"status":"ok"}` })
	if !strings.Contains(raw, `"ledger_updated":true`) {
		t.Fatal(raw)
	}
	targets, err := ListHomepageManagedDeployTargets(db, project.ID)
	if err != nil || len(targets) != 1 {
		t.Fatalf("provider identity split: %+v %v", targets, err)
	}
}

func TestHomepageWorkspaceCommandRecordsMissingOtherProject(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	var otherID int64
	for _, name := range []string{"main", "other"} {
		os.MkdirAll(filepath.Join(cfg.WorkspacePath, name), 0755)
		os.WriteFile(filepath.Join(cfg.WorkspacePath, name, "index.html"), []byte(name), 0644)
		site, err := EnsureHomepageProjectForDir(db, cfg, name, "", "")
		if err != nil {
			t.Fatal(err)
		}
		if name == "other" {
			otherID = site.ID
		}
	}
	raw := ExecuteHomepageOperation(cfg, db, HomepageOperation{Name: "exec", ProjectDir: "main", Reason: "Consolidate campaign folders", FilesMayChange: true}, slog.Default(), func(c HomepageConfig) string {
		if err := os.RemoveAll(filepath.Join(c.WorkspacePath, "other")); err != nil {
			t.Fatal(err)
		}
		return ErrorJSON("remaining command failed")
	})
	if !strings.Contains(raw, `"status":"partial_success"`) || !strings.Contains(raw, `"retry_safe":false`) {
		t.Fatal(raw)
	}
	site, err := GetHomepageManagedSite(db, otherID)
	if err != nil || site.LocalStatus != "missing" || len(site.Events) == 0 || site.Events[0].EventType != "project_location_missing" || site.Events[0].Reason != "Consolidate campaign folders" {
		t.Fatalf("%+v %v", site, err)
	}
}
