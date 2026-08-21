package tools

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestHereNowDeploymentLedgerRequiresVerifiedFinalResult(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	projectDir := "site-a"
	buildDir := "dist"
	buildPath := filepath.Join(workspace, projectDir, buildDir)
	if err := os.MkdirAll(buildPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildPath, "index.html"), []byte("<h1>here.now</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := HomepageConfig{WorkspacePath: workspace}
	if _, err := EnsureHomepageProjectForDir(db, cfg, projectDir, "site-a", "html"); err != nil {
		t.Fatal(err)
	}

	for _, raw := range []string{
		`{"status":"error","message":"upload failed"}`,
		`{"status":"ok","slug":"not-finalized","current_version_id":"pending","site_url":"https://not-finalized.here.now","verified":false}`,
	} {
		warnings, err := RecordHomepageDeploymentFromResultStrict(cfg, db, projectDir, "here_now", buildDir, raw, slog.Default())
		if err != nil || len(warnings) != 0 {
			t.Fatalf("ignored failed result warnings=%v err=%v", warnings, err)
		}
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_deployments WHERE provider = 'here_now'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed/unverified results wrote %d ledger rows", count)
	}

	raw := `{"status":"ok","slug":"live-site","current_version_id":"version-live","account":"account-1","site_url":"https://live-site.here.now","verified":true,"verified_url":"https://live-site.here.now","project_dir":"site-a","build_dir":"dist"}`
	warnings, err := RecordHomepageDeploymentFromResultStrict(cfg, db, projectDir, "here_now", buildDir, raw, slog.Default())
	if err != nil || len(warnings) != 0 {
		t.Fatalf("record verified result warnings=%v err=%v", warnings, err)
	}
	var targetID, deployID, deployURL, metadata string
	if err := db.QueryRow(`SELECT t.provider_target_id, d.provider_deploy_id, d.url, d.metadata_json
		FROM homepage_deployments d JOIN homepage_deploy_targets t ON t.id = d.target_id
		WHERE d.provider = 'here_now'`).Scan(&targetID, &deployID, &deployURL, &metadata); err != nil {
		t.Fatal(err)
	}
	if targetID != "live-site" || deployID != "version-live" || deployURL != "https://live-site.here.now" {
		t.Fatalf("target=%q deploy=%q url=%q", targetID, deployID, deployURL)
	}
	if metadata == "" {
		t.Fatal("expected account and provider metadata in generic ledger payload")
	}
}

func TestHereNowLedgerAliasesDoNotAffectOtherProviders(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	buildPath := filepath.Join(workspace, "site-a", "dist")
	if err := os.MkdirAll(buildPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildPath, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := HomepageConfig{WorkspacePath: workspace}
	if _, err := EnsureHomepageProjectForDir(db, cfg, "site-a", "site-a", "html"); err != nil {
		t.Fatal(err)
	}
	raw := `{"status":"ok","slug":"here-now-only-target","current_version_id":"here-now-only-version","url":"https://example.test","build_dir":"dist"}`
	warnings, err := RecordHomepageDeploymentFromResultStrict(cfg, db, "site-a", "netlify", "dist", raw, slog.Default())
	if err != nil || len(warnings) != 0 {
		t.Fatalf("record non-here.now result warnings=%v err=%v", warnings, err)
	}
	var targetID, deployID string
	if err := db.QueryRow(`SELECT t.provider_target_id, d.provider_deploy_id
		FROM homepage_deployments d JOIN homepage_deploy_targets t ON t.id = d.target_id
		WHERE d.provider = 'netlify'`).Scan(&targetID, &deployID); err != nil {
		t.Fatal(err)
	}
	if targetID != "" || deployID != "" {
		t.Fatalf("here.now aliases leaked to netlify: target=%q deploy=%q", targetID, deployID)
	}
}
