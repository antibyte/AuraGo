package tools

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomepageLedgerSchemaCreated(t *testing.T) {
	db := newHomepageLedgerTestDB(t)

	for _, table := range []string{
		"homepage_project_state",
		"homepage_site_file_state",
		"homepage_site_events",
		"homepage_deploy_targets",
		"homepage_deployments",
		"homepage_remote_observations",
	} {
		t.Run(table, func(t *testing.T) {
			var name string
			if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&name); err != nil {
				t.Fatalf("expected table %s to exist: %v", table, err)
			}
		})
	}
}

func TestEnsureHomepageProjectForDirCanonicalizesAndCreatesState(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	projectPath := filepath.Join(workspace, "site-a")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	proj, err := EnsureHomepageProjectForDir(db, HomepageConfig{WorkspacePath: workspace}, projectPath, "Site A", "html")
	if err != nil {
		t.Fatalf("EnsureHomepageProjectForDir failed: %v", err)
	}
	if proj.ProjectDir != "site-a" {
		t.Fatalf("ProjectDir = %q, want canonical relative site-a", proj.ProjectDir)
	}

	var localRoot, driftStatus string
	if err := db.QueryRow("SELECT local_root, drift_status FROM homepage_project_state WHERE project_id = ?", proj.ID).Scan(&localRoot, &driftStatus); err != nil {
		t.Fatalf("expected project state row: %v", err)
	}
	if localRoot != projectPath {
		t.Fatalf("local_root = %q, want %q", localRoot, projectPath)
	}
	if driftStatus != "not_deployed" {
		t.Fatalf("drift_status = %q, want not_deployed", driftStatus)
	}
}

func TestEnsureHomepageProjectForDirRejectsAmbiguousRoot(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	_, err := EnsureHomepageProjectForDir(db, HomepageConfig{WorkspacePath: t.TempDir()}, "", "Root", "html")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous root error, got %v", err)
	}
}

func TestRecordHomepageDeploymentRequiresTargetLocation(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	proj, err := EnsureHomepageProjectForDir(db, HomepageConfig{WorkspacePath: workspace}, "site-a", "site-a", "html")
	if err != nil {
		t.Fatalf("ensure project: %v", err)
	}

	err = RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID: proj.ID,
		Provider:  "local",
		Status:    "ok",
	})
	if err == nil || !strings.Contains(err.Error(), "deploy URL or remote_path") {
		t.Fatalf("expected target location error, got %v", err)
	}
}

func TestHomepageSaveRevisionUsesLatestBaselineAndUpdatesFileState(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	projectDir := "site-a"
	projectPath := filepath.Join(workspace, projectDir)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("<h1>one</h1>"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, _, err := RegisterProject(db, HomepageProject{Name: "site-a", ProjectDir: projectDir, Framework: "html"}); err != nil {
		t.Fatalf("register project: %v", err)
	}
	cfg := HomepageConfig{WorkspacePath: workspace}

	first := decodeLedgerTestJSON(t, HomepageSaveRevision(cfg, db, projectDir, "initial", "test", slog.Default()))
	if first["status"] != "ok" || int(first["added"].(float64)) != 1 {
		t.Fatalf("first revision = %#v, want one added file", first)
	}

	second := decodeLedgerTestJSON(t, HomepageSaveRevision(cfg, db, projectDir, "repeat", "test", slog.Default()))
	if second["revision_id"] != nil {
		t.Fatalf("second revision_id = %#v, want nil for unchanged content", second["revision_id"])
	}

	if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("<h1>two</h1>"), 0644); err != nil {
		t.Fatalf("modify file: %v", err)
	}
	third := decodeLedgerTestJSON(t, SaveHomepageRevisionAndState(cfg, db, projectDir, "edit", "test", "homepage_file", nil, slog.Default()).JSON)
	if int(third["added"].(float64)) != 0 || int(third["modified"].(float64)) != 1 {
		t.Fatalf("third revision = %#v, want modified=1 and added=0", third)
	}

	var fileHash string
	if err := db.QueryRow("SELECT content_hash FROM homepage_site_file_state WHERE rel_path = 'index.html'").Scan(&fileHash); err != nil {
		t.Fatalf("expected file state row: %v", err)
	}
	if fileHash == "" {
		t.Fatal("content_hash should be populated")
	}
}

func TestRecordHomepageDeploymentPersistsTargetDeploymentAndManifest(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	projectDir := "site-a"
	buildDir := "dist"
	buildPath := filepath.Join(workspace, projectDir, buildDir)
	if err := os.MkdirAll(buildPath, 0755); err != nil {
		t.Fatalf("mkdir build: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildPath, "index.html"), []byte("<h1>deploy</h1>"), 0644); err != nil {
		t.Fatalf("write build: %v", err)
	}
	proj, err := EnsureHomepageProjectForDir(db, HomepageConfig{WorkspacePath: workspace}, projectDir, "site-a", "html")
	if err != nil {
		t.Fatalf("ensure project: %v", err)
	}
	revID, err := CreateHomepageRevision(db, proj.ID, projectDir, "deployable", "test", "agent", 1, true, `{"added":1}`)
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}

	manifest, err := BuildHomepageArtifactManifest(HomepageConfig{WorkspacePath: workspace}, proj.ID, projectDir, revID, "abc123", buildDir)
	if err != nil {
		t.Fatalf("BuildHomepageArtifactManifest failed: %v", err)
	}
	if manifest.ArtifactHash == "" || len(manifest.Files) != 1 {
		t.Fatalf("manifest = %#v, want artifact hash and one file", manifest)
	}
	if err := WriteHomepageArtifactManifest(HomepageConfig{WorkspacePath: workspace}, manifest); err != nil {
		t.Fatalf("WriteHomepageArtifactManifest failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(buildPath, ".aurago-site-manifest.json")); err != nil {
		t.Fatalf("manifest file missing: %v", err)
	}

	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID:        proj.ID,
		RevisionID:       revID,
		GitSHA:           "abc123",
		Provider:         "netlify",
		ProviderTargetID: "site-123",
		ProviderDeployID: "deploy-123",
		URL:              "https://example.netlify.app",
		BuildDir:         buildDir,
		ArtifactHash:     manifest.ArtifactHash,
		Status:           "ok",
	}); err != nil {
		t.Fatalf("RecordHomepageDeployment failed: %v", err)
	}

	var targetURL, deployID string
	if err := db.QueryRow("SELECT url FROM homepage_deploy_targets WHERE project_id = ? AND provider = 'netlify'", proj.ID).Scan(&targetURL); err != nil {
		t.Fatalf("expected deploy target: %v", err)
	}
	if targetURL != "https://example.netlify.app" {
		t.Fatalf("target url = %q", targetURL)
	}
	if err := db.QueryRow("SELECT provider_deploy_id FROM homepage_deployments WHERE project_id = ? AND provider = 'netlify'", proj.ID).Scan(&deployID); err != nil {
		t.Fatalf("expected deployment row: %v", err)
	}
	if deployID != "deploy-123" {
		t.Fatalf("deploy id = %q", deployID)
	}
}

func TestRecordHomepageDeploymentKeepsDeploymentTargetLinkAfterUpsert(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	cfg := HomepageConfig{WorkspacePath: workspace}
	projectA := "site-a"
	projectB := "site-b"
	for _, projectDir := range []string{projectA, projectB} {
		buildPath := filepath.Join(workspace, projectDir)
		if err := os.MkdirAll(buildPath, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", projectDir, err)
		}
		if err := os.WriteFile(filepath.Join(buildPath, "index.html"), []byte(projectDir), 0644); err != nil {
			t.Fatalf("write %s: %v", projectDir, err)
		}
	}
	projA, err := EnsureHomepageProjectForDir(db, cfg, projectA, "site-a", "html")
	if err != nil {
		t.Fatalf("ensure site-a: %v", err)
	}
	projB, err := EnsureHomepageProjectForDir(db, cfg, projectB, "site-b", "html")
	if err != nil {
		t.Fatalf("ensure site-b: %v", err)
	}
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID:        projA.ID,
		Provider:         "netlify",
		ProviderTargetID: "site-a-target",
		ProviderDeployID: "deploy-a-1",
		URL:              "https://a-one.example",
		Status:           "ok",
	}); err != nil {
		t.Fatalf("record site-a first deployment: %v", err)
	}
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID:        projB.ID,
		Provider:         "netlify",
		ProviderTargetID: "site-b-target",
		ProviderDeployID: "deploy-b-1",
		URL:              "https://b.example",
		Status:           "ok",
	}); err != nil {
		t.Fatalf("record site-b deployment: %v", err)
	}
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID:        projA.ID,
		Provider:         "netlify",
		ProviderTargetID: "site-a-target",
		ProviderDeployID: "deploy-a-2",
		URL:              "https://a-two.example",
		Status:           "ok",
	}); err != nil {
		t.Fatalf("record site-a second deployment: %v", err)
	}

	var linkedProjectID int64
	err = db.QueryRow(`SELECT t.project_id
		FROM homepage_deployments d
		JOIN homepage_deploy_targets t ON t.id = d.target_id
		WHERE d.provider_deploy_id = 'deploy-a-2'`).Scan(&linkedProjectID)
	if err != nil {
		t.Fatalf("read linked target project: %v", err)
	}
	if linkedProjectID != projA.ID {
		t.Fatalf("deploy-a-2 linked to project %d, want %d", linkedProjectID, projA.ID)
	}
}

func TestRecordHomepageDeploymentFromResultParsesProviderFields(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	projectDir := "site-a"
	buildPath := filepath.Join(workspace, projectDir)
	if err := os.MkdirAll(buildPath, 0755); err != nil {
		t.Fatalf("mkdir build: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildPath, "index.html"), []byte("<h1>deploy</h1>"), 0644); err != nil {
		t.Fatalf("write build: %v", err)
	}
	cfg := HomepageConfig{WorkspacePath: workspace}
	if _, err := EnsureHomepageProjectForDir(db, cfg, projectDir, "site-a", "html"); err != nil {
		t.Fatalf("ensure project: %v", err)
	}
	if got := SaveHomepageRevisionAndState(cfg, db, projectDir, "initial", "test", "test", nil, slog.Default()); len(got.Warnings) > 0 {
		t.Fatalf("initial save warnings: %v", got.Warnings)
	}

	warnings := RecordHomepageDeploymentFromResult(cfg, db, projectDir, "netlify", "", `{"status":"ok","id":"deploy-1","site_id":"site-1","verified_url":"https://site.example"}`, slog.Default())
	if len(warnings) > 0 {
		t.Fatalf("RecordHomepageDeploymentFromResult warnings: %v", warnings)
	}

	var targetID, deployURL string
	if err := db.QueryRow("SELECT provider_target_id FROM homepage_deploy_targets WHERE provider = 'netlify'").Scan(&targetID); err != nil {
		t.Fatalf("expected target: %v", err)
	}
	if err := db.QueryRow("SELECT url FROM homepage_deployments WHERE provider = 'netlify'").Scan(&deployURL); err != nil {
		t.Fatalf("expected deployment: %v", err)
	}
	if targetID != "site-1" || deployURL != "https://site.example" {
		t.Fatalf("targetID=%q deployURL=%q", targetID, deployURL)
	}
}

func TestRecordHomepageDeploymentFromResultUsesNetlifyFallbackProjectAndBuildDir(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	cfg := HomepageConfig{WorkspacePath: workspace}
	originalDir := "next-site"
	fallbackDir := "next-site-static"
	buildDir := "public"
	buildPath := filepath.Join(workspace, fallbackDir, buildDir)
	if err := os.MkdirAll(buildPath, 0755); err != nil {
		t.Fatalf("mkdir fallback build: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildPath, "index.html"), []byte("<h1>fallback</h1>"), 0644); err != nil {
		t.Fatalf("write fallback build: %v", err)
	}
	if _, err := EnsureHomepageProjectForDir(db, cfg, originalDir, "next-site", "next"); err != nil {
		t.Fatalf("ensure original project: %v", err)
	}
	result := `{"status":"ok","deploy_id":"deploy-1","site_id":"site-1","verified_url":"https://fallback.example","fallback_project_dir":"next-site-static","fallback_build_dir":"public"}`

	warnings := RecordHomepageDeploymentFromResult(cfg, db, originalDir, "netlify", "", result, slog.Default())
	if len(warnings) > 0 {
		t.Fatalf("warnings: %v", warnings)
	}
	var projectDir, build string
	err := db.QueryRow(`SELECT p.project_dir, d.build_dir
		FROM homepage_deployments d
		JOIN homepage_projects p ON p.id = d.project_id
		WHERE d.provider_deploy_id = 'deploy-1'`).Scan(&projectDir, &build)
	if err != nil {
		t.Fatalf("read deployment: %v", err)
	}
	if projectDir != fallbackDir || build != buildDir {
		t.Fatalf("projectDir=%q buildDir=%q, want %q/%q", projectDir, build, fallbackDir, buildDir)
	}
}

func TestReconcileHomepageProjectDetectsLocalChanges(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	projectDir := "site-a"
	projectPath := filepath.Join(workspace, projectDir)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("one"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	cfg := HomepageConfig{WorkspacePath: workspace}
	proj, err := EnsureHomepageProjectForDir(db, cfg, projectDir, "site-a", "html")
	if err != nil {
		t.Fatalf("ensure project: %v", err)
	}
	if got := SaveHomepageRevisionAndState(cfg, db, projectDir, "initial", "test", "test", nil, slog.Default()); len(got.Warnings) > 0 {
		t.Fatalf("initial save warnings: %v", got.Warnings)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("two"), 0644); err != nil {
		t.Fatalf("modify file: %v", err)
	}

	state, err := ReconcileHomepageProject(cfg, db, projectDir, slog.Default())
	if err != nil {
		t.Fatalf("ReconcileHomepageProject failed: %v", err)
	}
	if state.ProjectID != proj.ID {
		t.Fatalf("ProjectID = %d, want %d", state.ProjectID, proj.ID)
	}
	if state.DriftStatus != "local_changed" {
		t.Fatalf("DriftStatus = %q, want local_changed", state.DriftStatus)
	}
}

func TestReconcileHomepageProjectDetectsUndeployedLatestRevision(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	projectDir := "site-a"
	projectPath := filepath.Join(workspace, projectDir)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	cfg := HomepageConfig{WorkspacePath: workspace}
	if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("one"), 0644); err != nil {
		t.Fatalf("write initial file: %v", err)
	}
	proj, err := EnsureHomepageProjectForDir(db, cfg, projectDir, "site-a", "html")
	if err != nil {
		t.Fatalf("ensure project: %v", err)
	}
	initial := SaveHomepageRevisionAndState(cfg, db, projectDir, "initial", "test", "test", nil, slog.Default())
	if len(initial.Warnings) > 0 || initial.RevisionID == 0 {
		t.Fatalf("initial revision = %#v", initial)
	}
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID:  proj.ID,
		RevisionID: initial.RevisionID,
		Provider:   "local",
		RemotePath: filepath.Join(workspace, projectDir),
		BuildDir:   ".",
		Status:     "ok",
	}); err != nil {
		t.Fatalf("record initial deploy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("two"), 0644); err != nil {
		t.Fatalf("write changed file: %v", err)
	}
	changed := SaveHomepageRevisionAndState(cfg, db, projectDir, "changed", "test", "test", nil, slog.Default())
	if len(changed.Warnings) > 0 || changed.RevisionID == 0 {
		t.Fatalf("changed revision = %#v", changed)
	}

	state, err := ReconcileHomepageProject(cfg, db, projectDir, slog.Default())
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if state.DriftStatus != "not_deployed" {
		t.Fatalf("DriftStatus = %q, want not_deployed; message=%q", state.DriftStatus, state.DriftMessage)
	}
}

func TestReconcileHomepageProjectDetectsRemoteChanges(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	projectDir := "site-a"
	projectPath := filepath.Join(workspace, projectDir)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("one"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	cfg := HomepageConfig{WorkspacePath: workspace}
	proj, err := EnsureHomepageProjectForDir(db, cfg, projectDir, "site-a", "html")
	if err != nil {
		t.Fatalf("ensure project: %v", err)
	}
	save := SaveHomepageRevisionAndState(cfg, db, projectDir, "initial", "test", "test", nil, slog.Default())
	if len(save.Warnings) > 0 {
		t.Fatalf("save revision warnings: %v", save.Warnings)
	}

	body := "remote-one"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID:    proj.ID,
		RevisionID:   save.RevisionID,
		Provider:     "local",
		URL:          server.URL,
		BuildDir:     ".",
		ArtifactHash: "artifact-1",
		Status:       "ok",
	}); err != nil {
		t.Fatalf("record deployment: %v", err)
	}

	state, err := ReconcileHomepageProject(cfg, db, projectDir, slog.Default())
	if err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if state.DriftStatus != "clean" {
		t.Fatalf("first drift = %q, want clean", state.DriftStatus)
	}

	body = "remote-two"
	state, err = ReconcileHomepageProject(cfg, db, projectDir, slog.Default())
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if state.DriftStatus != "remote_changed" {
		t.Fatalf("second drift = %q, want remote_changed", state.DriftStatus)
	}
	var observations int
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_remote_observations WHERE project_id = ?", proj.ID).Scan(&observations); err != nil {
		t.Fatalf("count observations: %v", err)
	}
	if observations < 2 {
		t.Fatalf("observations = %d, want at least 2", observations)
	}
}

func TestGetHomepageManagedSiteIncludesTargetsAndRemoteObservations(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	projectDir := "site-a"
	projectPath := filepath.Join(workspace, projectDir)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("one"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	cfg := HomepageConfig{WorkspacePath: workspace}
	proj, err := EnsureHomepageProjectForDir(db, cfg, projectDir, "site-a", "html")
	if err != nil {
		t.Fatalf("ensure project: %v", err)
	}
	save := SaveHomepageRevisionAndState(cfg, db, projectDir, "initial", "test", "test", nil, slog.Default())
	if len(save.Warnings) > 0 {
		t.Fatalf("save revision warnings: %v", save.Warnings)
	}

	body := "remote-one"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID:        proj.ID,
		RevisionID:       save.RevisionID,
		Provider:         "netlify",
		ProviderTargetID: "site-1",
		ProviderDeployID: "deploy-1",
		URL:              server.URL,
		BuildDir:         ".",
		ArtifactHash:     "artifact-1",
		Status:           "ok",
	}); err != nil {
		t.Fatalf("record deployment: %v", err)
	}
	if _, err := ReconcileHomepageProject(cfg, db, projectDir, slog.Default()); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	body = "remote-two"
	if _, err := ReconcileHomepageProject(cfg, db, projectDir, slog.Default()); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	site, err := GetHomepageManagedSite(db, proj.ID)
	if err != nil {
		t.Fatalf("managed site: %v", err)
	}
	if len(site.DeployTargets) != 1 {
		t.Fatalf("DeployTargets len = %d, want 1", len(site.DeployTargets))
	}
	if site.DeployTargets[0].Provider != "netlify" || site.DeployTargets[0].ProviderTargetID != "site-1" {
		t.Fatalf("unexpected deploy target: %+v", site.DeployTargets[0])
	}
	if len(site.RemoteObservations) == 0 {
		t.Fatal("RemoteObservations should be populated")
	}
	if site.RemoteObservations[0].Status == "" || site.RemoteObservations[0].URL != server.URL {
		t.Fatalf("unexpected remote observation: %+v", site.RemoteObservations[0])
	}
}

func newHomepageLedgerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "homepage.db"))
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func decodeLedgerTestJSON(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("invalid JSON %q: %v", raw, err)
	}
	return out
}
