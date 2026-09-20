package tools

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestHomepageVercelProjectRootCreatesArtifactManifest(t *testing.T) {
	for _, test := range []struct{ name, deployPath, relative string }{
		{"root", "site-a", "."}, {"output", "site-a/dist", "dist"},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := newHomepageLedgerTestDB(t)
			cfg := HomepageConfig{WorkspacePath: t.TempDir()}
			root := filepath.Join(cfg.WorkspacePath, "site-a", test.relative)
			if err := os.MkdirAll(root, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<html>site</html>"), 0644); err != nil {
				t.Fatal(err)
			}
			warnings, err := RecordHomepageDeploymentFromResultStrict(cfg, db, "site-a", "vercel", "", `{"status":"ok","project_id":"project-1","deployment_id":"deploy-1","deployment_url":"https://example.test","deploy_path":"`+test.deployPath+`"}`, slog.Default())
			if err != nil || len(warnings) > 0 {
				t.Fatalf("deployment ledger: %v %v", err, warnings)
			}
			var hash string
			if err := db.QueryRow("SELECT artifact_hash FROM homepage_deployments WHERE provider='vercel'").Scan(&hash); err != nil {
				t.Fatal(err)
			}
			if hash == "" {
				t.Fatal("deployment lacks artifact hash")
			}
		})
	}
}
