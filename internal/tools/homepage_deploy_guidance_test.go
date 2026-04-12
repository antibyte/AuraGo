package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecorateHomepageBuildFailureMissingBuildScript(t *testing.T) {
	raw := `{"status":"error","output":"npm error Missing script: \"build\""}`
	result := decorateHomepageBuildFailure(raw, "ki-news")
	if !strings.Contains(result, `no 'build' script`) {
		t.Fatalf("expected missing-build guidance, got: %s", result)
	}
	if !strings.Contains(result, "ki-news") {
		t.Fatalf("expected project_dir to appear in guidance, got: %s", result)
	}
}

func TestHomepageWebServerStartMissingSourcePathExplainsSrvRoot(t *testing.T) {
	cfg := HomepageConfig{WorkspacePath: t.TempDir(), WebServerPort: 8080}
	result := HomepageWebServerStart(cfg, "missing-site", ".", nil)
	if !strings.Contains(result, "aurago-homepage-web") {
		t.Fatalf("expected Caddy container guidance, got: %s", result)
	}
	if !strings.Contains(result, "/srv") {
		t.Fatalf("expected /srv document root guidance, got: %s", result)
	}
	if !strings.Contains(result, "/var/www/html") {
		t.Fatalf("expected /var/www/html correction hint, got: %s", result)
	}
}

func TestSaveAndLoadWebserverState(t *testing.T) {
	dir := t.TempDir()

	// No state file initially
	pd, bd := loadWebserverState(dir)
	if pd != "" || bd != "" {
		t.Fatalf("expected empty state when no file exists, got project_dir=%q build_dir=%q", pd, bd)
	}

	// Save state
	if err := saveWebserverState(dir, "phaser-demo", "out"); err != nil {
		t.Fatalf("saveWebserverState failed: %v", err)
	}

	// Load state
	pd, bd = loadWebserverState(dir)
	if pd != "phaser-demo" || bd != "out" {
		t.Fatalf("expected phaser-demo/out, got project_dir=%q build_dir=%q", pd, bd)
	}
}

func TestLoadWebserverStateEmptyWorkspace(t *testing.T) {
	pd, bd := loadWebserverState("")
	if pd != "" || bd != "" {
		t.Fatalf("expected empty state for empty workspace, got project_dir=%q build_dir=%q", pd, bd)
	}
}

func TestSaveWebserverStateEmptyWorkspace(t *testing.T) {
	err := saveWebserverState("", "phaser-demo", "out")
	if err != nil {
		t.Fatalf("expected nil error for empty workspace, got: %v", err)
	}
}

func TestLoadWebserverStateInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".aurago-webserver-state.json"), []byte("not json"), 0644)
	pd, bd := loadWebserverState(dir)
	if pd != "" || bd != "" {
		t.Fatalf("expected empty state for invalid JSON, got project_dir=%q build_dir=%q", pd, bd)
	}
}

func TestHomepageWebServerStartRestoresSavedState(t *testing.T) {
	dir := t.TempDir()
	// Create a project with build output
	projectDir := filepath.Join(dir, "my-site", "out")
	os.MkdirAll(projectDir, 0755)
	os.WriteFile(filepath.Join(projectDir, "index.html"), []byte("<h1>Hello</h1>"), 0644)

	// Save state as if publish_local had been called
	saveWebserverState(dir, "my-site", "out")

	// Start with empty projectDir/buildDir — should restore saved state
	cfg := HomepageConfig{WorkspacePath: dir, WebServerPort: 0}
	// Since Docker is unavailable in tests, this should hit the error path
	// but the important thing is it tries to use the restored state.
	result := HomepageWebServerStart(cfg, "", "", nil)

	// The function should either succeed (Python fallback) or give a Docker error,
	// but NOT "Local publish source does not exist" since the restored path exists.
	if strings.Contains(result, "does not exist") {
		t.Fatalf("should not report missing path when saved state exists, got: %s", result)
	}
}
