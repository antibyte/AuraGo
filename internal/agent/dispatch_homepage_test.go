package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func homepageToolOutputMessage(t *testing.T, output string) string {
	t.Helper()
	const prefix = "Tool Output: "
	if !strings.HasPrefix(output, prefix) {
		t.Fatalf("expected Tool Output prefix, got %s", output)
	}
	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(output, prefix)), &payload); err != nil {
		t.Fatalf("parse tool output JSON: %v; output=%s", err, output)
	}
	if payload.Status != "error" {
		t.Fatalf("expected error status, got %q in %s", payload.Status, output)
	}
	return payload.Message
}

func TestDispatchHomepageDestroyRequiresForce(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowContainerManagement = true
	cfg.Homepage.WorkspacePath = t.TempDir()

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "destroy",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if !strings.Contains(output, "requires force=true") {
		t.Fatalf("expected destroy without force to be rejected, got %s", output)
	}
}

func TestDispatchHomepageInitProjectRegistersProjectDir(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowContainerManagement = true
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	cfg.Homepage.WorkspacePath = t.TempDir()
	db, err := tools.InitHomepageRegistryDB(t.TempDir() + "/homepage.db")
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "init_project",
		Framework: "html",
		Name:      "site-a",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger, HomepageRegistryDB: db})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if !strings.Contains(output, `"project_dir":"site-a"`) {
		t.Fatalf("expected tool output to include project_dir, got %s", output)
	}

	proj, err := tools.GetProjectByDir(db, "site-a")
	if err != nil {
		t.Fatalf("expected registered project by dir: %v", err)
	}
	if proj.Name != "site-a" {
		t.Fatalf("registered project name = %q, want site-a", proj.Name)
	}
}

func TestDispatchHomepagePublishLocalRequiresProjectDir(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	cfg.Homepage.AllowLocalServer = true
	db, err := tools.InitHomepageRegistryDB(t.TempDir() + "/homepage.db")
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "publish_local",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger, HomepageRegistryDB: db})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if !strings.Contains(output, `"status":"error"`) || !strings.Contains(output, "project_dir is required") {
		t.Fatalf("expected missing project_dir error, got %s", output)
	}
}

func TestDispatchHomepagePublishLocalRejectsAmbiguousProjectDir(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	cfg.Homepage.AllowLocalServer = true
	db, err := tools.InitHomepageRegistryDB(t.TempDir() + "/homepage.db")
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "publish_local",
		Params: map[string]interface{}{
			"project_dir": ".",
		},
	}, &DispatchContext{Cfg: cfg, Logger: testLogger, HomepageRegistryDB: db})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if message := homepageToolOutputMessage(t, output); !strings.Contains(message, `project_dir "." is ambiguous`) {
		t.Fatalf("expected ambiguous project_dir error, got %q", message)
	}
}

func TestDispatchHomepageDeployRequiresProjectDir(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowDeploy = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	db, err := tools.InitHomepageRegistryDB(t.TempDir() + "/homepage.db")
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "deploy",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger, HomepageRegistryDB: db})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if !strings.Contains(output, `"status":"error"`) || !strings.Contains(output, "project_dir is required") {
		t.Fatalf("expected missing project_dir error, got %s", output)
	}
}

func TestDispatchHomepageDeployRejectsAmbiguousProjectDir(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowDeploy = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	db, err := tools.InitHomepageRegistryDB(t.TempDir() + "/homepage.db")
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "deploy",
		Params: map[string]interface{}{
			"project_dir": ".",
		},
	}, &DispatchContext{Cfg: cfg, Logger: testLogger, HomepageRegistryDB: db})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if message := homepageToolOutputMessage(t, output); !strings.Contains(message, `project_dir "." is ambiguous`) {
		t.Fatalf("expected ambiguous project_dir error, got %q", message)
	}
}

func TestDispatchHomepageDeployNetlifyRejectsAmbiguousProjectDirBeforeTokenCheck(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	cfg.Netlify.Enabled = true
	cfg.Netlify.AllowDeploy = true

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "deploy_netlify",
		Params: map[string]interface{}{
			"project_dir": ".",
		},
	}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if message := homepageToolOutputMessage(t, output); !strings.Contains(message, `project_dir "." is ambiguous`) {
		t.Fatalf("expected ambiguous project_dir error before token check, got %q", message)
	}
}

func TestHomepageRecordDeploymentStrictSkipsNilRegistry(t *testing.T) {
	result := `{"status":"ok","url":"http://localhost:8080","project_dir":"site-a","build_dir":"."}`

	output := homepageRecordDeploymentStrictResult(tools.HomepageConfig{}, nil, "site-a", "local", ".", result, testLogger)

	if strings.Contains(output, `"status":"error"`) {
		t.Fatalf("successful deploy must stay successful when registry is unavailable, got %s", output)
	}
}

func TestHomepageRecordDeploymentStrictSkipsCaddyRootProjectDir(t *testing.T) {
	db, err := tools.InitHomepageRegistryDB(t.TempDir() + "/homepage.db")
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	result := `{"status":"ok","url":"http://localhost:8080","build_dir":"."}`
	projectDir := homepageProjectDirFromResult(result, "")
	output := homepageRecordDeploymentStrictResult(tools.HomepageConfig{WorkspacePath: t.TempDir()}, db, projectDir, "caddy", ".", result, testLogger)

	if strings.Contains(output, `"status":"error"`) {
		t.Fatalf("successful root webserver_start must stay successful, got %s", output)
	}
	var deployments int
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_deployments").Scan(&deployments); err != nil {
		t.Fatalf("count deployments: %v", err)
	}
	if deployments != 0 {
		t.Fatalf("root caddy start should not record a deployment, got %d", deployments)
	}
}

func TestHomepageProjectDirFromResultPreservesNestedProjectIdentities(t *testing.T) {
	for name, tc := range map[string]struct {
		result   string
		fallback string
		want     string
	}{
		"project_dir identity": {
			result: `{"status":"ok","project_dir":"sites/landing-page","path":"site-a/index.html"}`,
			want:   "sites/landing-page",
		},
		"fallback_project_dir identity": {
			result: `{"status":"ok","fallback_project_dir":"sites/landing-page"}`,
			want:   "sites/landing-page",
		},
		"path remains file path": {
			result: `{"status":"ok","path":"sites/landing-page/index.html"}`,
			want:   "sites",
		},
		"fallback identity": {
			result:   `{"status":"ok"}`,
			fallback: "sites/landing-page",
			want:     "sites/landing-page",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := homepageProjectDirFromResult(tc.result, tc.fallback); got != tc.want {
				t.Fatalf("homepageProjectDirFromResult() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHomepageRecordDeploymentStrictRecordsCaddyNestedProjectDir(t *testing.T) {
	db, err := tools.InitHomepageRegistryDB(t.TempDir() + "/homepage.db")
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	result := `{"status":"ok","url":"http://localhost:8080","project_dir":"sites/landing-page","build_dir":"."}`
	projectDir := homepageProjectDirFromResult(result, "")
	if projectDir != "sites/landing-page" {
		t.Fatalf("webserver project_dir = %q, want sites/landing-page", projectDir)
	}
	output := homepageRecordDeploymentStrictResult(tools.HomepageConfig{WorkspacePath: t.TempDir()}, db, projectDir, "caddy", ".", result, testLogger)

	if strings.Contains(output, `"status":"error"`) {
		t.Fatalf("successful nested caddy start must stay successful, got %s", output)
	}
	var recordedProjectDir string
	if err := db.QueryRow(`
		SELECT p.project_dir
		FROM homepage_deployments d
		JOIN homepage_projects p ON p.id = d.project_id
		WHERE d.provider = 'caddy'
	`).Scan(&recordedProjectDir); err != nil {
		t.Fatalf("read recorded deployment project_dir: %v", err)
	}
	if recordedProjectDir != "sites/landing-page" {
		t.Fatalf("recorded caddy project_dir = %q, want sites/landing-page", recordedProjectDir)
	}
}

func TestDispatchHomepageWriteFileRecordsLedgerRevision(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	cfg.Homepage.WorkspacePath = t.TempDir()
	db, err := tools.InitHomepageRegistryDB(t.TempDir() + "/homepage.db")
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := tools.EnsureHomepageProjectForDir(db, tools.HomepageConfig{WorkspacePath: cfg.Homepage.WorkspacePath}, "site-a", "site-a", "html"); err != nil {
		t.Fatalf("EnsureHomepageProjectForDir failed: %v", err)
	}

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "write_file",
		Path:      "site-a/index.html",
		Content:   "<h1>Hello</h1>",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger, HomepageRegistryDB: db})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if !strings.Contains(output, `"status":"ok"`) {
		t.Fatalf("expected successful write, got %s", output)
	}

	var revisionCount, eventCount, fileStateCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_revisions WHERE project_dir = 'site-a'").Scan(&revisionCount); err != nil {
		t.Fatalf("count revisions: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_site_events WHERE event_type = 'file_written'").Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_site_file_state WHERE rel_path = 'index.html'").Scan(&fileStateCount); err != nil {
		t.Fatalf("count file state: %v", err)
	}
	if revisionCount != 1 || eventCount != 1 || fileStateCount != 1 {
		t.Fatalf("ledger counts revision=%d event=%d file_state=%d, want all 1", revisionCount, eventCount, fileStateCount)
	}
}

func TestHomepageNetlifyResultVerifiedRequiresVerifiedFlagAndURL(t *testing.T) {
	if !homepageNetlifyResultVerified(map[string]interface{}{
		"status":       "ok",
		"verified":     true,
		"verified_url": "https://site.example",
	}) {
		t.Fatal("verified Netlify result with URL should be accepted")
	}

	for name, parsed := range map[string]map[string]interface{}{
		"missing flag": {
			"status":       "ok",
			"verified_url": "https://site.example",
		},
		"false flag": {
			"status":       "ok",
			"verified":     false,
			"verified_url": "https://site.example",
		},
		"missing url": {
			"status":   "ok",
			"verified": true,
		},
		"empty url": {
			"status":       "ok",
			"verified":     true,
			"verified_url": " ",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if homepageNetlifyResultVerified(parsed) {
				t.Fatalf("unverified Netlify result should not be accepted: %#v", parsed)
			}
		})
	}
}

func TestDispatchHomepageDeployNetlifyDoesNotLogFailedDeploy(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowDeploy = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	cfg.Netlify.Enabled = true
	cfg.Netlify.AllowDeploy = true
	cfg.Netlify.DefaultSiteID = "site-123"

	db, err := tools.InitHomepageRegistryDB(t.TempDir() + "/homepage.db")
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := tools.EnsureHomepageProjectForDir(db, tools.HomepageConfig{WorkspacePath: cfg.Homepage.WorkspacePath}, "site-a", "site-a", "html"); err != nil {
		t.Fatalf("EnsureHomepageProjectForDir failed: %v", err)
	}
	vault := newDispatchTestVault(t, map[string]string{"netlify_token": "nf-secret"})

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "deploy_netlify",
		Params: map[string]interface{}{
			"project_dir": "site-a",
			"build_dir":   ".",
		},
	}, &DispatchContext{Cfg: cfg, Logger: testLogger, Vault: vault, HomepageRegistryDB: db})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if !strings.Contains(output, `"status":"error"`) {
		t.Fatalf("expected deploy failure for missing project files, got %s", output)
	}

	proj, err := tools.GetProjectByDir(db, "site-a")
	if err != nil {
		t.Fatalf("GetProjectByDir failed: %v", err)
	}
	if proj.LastDeployURL != "" || proj.LastDeployedAt != "" {
		t.Fatalf("failed Netlify deploy must not update last deployment fields, got url=%q at=%q", proj.LastDeployURL, proj.LastDeployedAt)
	}
}

func TestDispatchHomepageDeployVercelDoesNotLogFailedDeploy(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowDeploy = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	cfg.Vercel.Enabled = true
	cfg.Vercel.AllowDeploy = true
	cfg.Vercel.DefaultProjectID = "site-a"

	db, err := tools.InitHomepageRegistryDB(t.TempDir() + "/homepage.db")
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := tools.EnsureHomepageProjectForDir(db, tools.HomepageConfig{WorkspacePath: cfg.Homepage.WorkspacePath}, "site-a", "site-a", "html"); err != nil {
		t.Fatalf("EnsureHomepageProjectForDir failed: %v", err)
	}
	vault := newDispatchTestVault(t, map[string]string{"vercel_token": "vc-token-fixture"})

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "deploy_vercel",
		Params: map[string]interface{}{
			"project_dir": "site-a",
			"build_dir":   ".",
		},
	}, &DispatchContext{Cfg: cfg, Logger: testLogger, Vault: vault, HomepageRegistryDB: db})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if !strings.Contains(output, `"status":"error"`) {
		t.Fatalf("expected deploy failure for missing project files, got %s", output)
	}

	proj, err := tools.GetProjectByDir(db, "site-a")
	if err != nil {
		t.Fatalf("GetProjectByDir failed: %v", err)
	}
	if proj.LastDeployURL != "" || proj.LastDeployedAt != "" || proj.DeployHost != "" || proj.URL != "" {
		t.Fatalf("failed Vercel deploy must not update deployment fields, got last_url=%q last_at=%q host=%q url=%q", proj.LastDeployURL, proj.LastDeployedAt, proj.DeployHost, proj.URL)
	}
}

func TestDispatchHomepageDeploySFTPDoesNotLogFailedDeploy(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowDeploy = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	cfg.Homepage.DeployHost = "sftp.example"
	cfg.Homepage.DeployUser = "deploy"
	cfg.Homepage.DeployPath = "/var/www/site-a"

	db, err := tools.InitHomepageRegistryDB(t.TempDir() + "/homepage.db")
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := tools.EnsureHomepageProjectForDir(db, tools.HomepageConfig{WorkspacePath: cfg.Homepage.WorkspacePath}, "site-a", "site-a", "html"); err != nil {
		t.Fatalf("EnsureHomepageProjectForDir failed: %v", err)
	}

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "deploy",
		Params: map[string]interface{}{
			"project_dir": "site-a",
			"build_dir":   ".",
		},
	}, &DispatchContext{Cfg: cfg, Logger: testLogger, HomepageRegistryDB: db})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if !strings.Contains(output, `"status":"error"`) {
		t.Fatalf("expected deploy failure for missing SFTP credential, got %s", output)
	}

	proj, err := tools.GetProjectByDir(db, "site-a")
	if err != nil {
		t.Fatalf("GetProjectByDir failed: %v", err)
	}
	if proj.LastDeployURL != "" || proj.LastDeployedAt != "" {
		t.Fatalf("failed SFTP deploy must not update last deployment fields, got url=%q at=%q", proj.LastDeployURL, proj.LastDeployedAt)
	}
}
