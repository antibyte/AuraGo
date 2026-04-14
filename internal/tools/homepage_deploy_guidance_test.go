package tools

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

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
	result := HomepageWebServerStart(cfg, "missing-site", ".", discardLogger())
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
	result := HomepageWebServerStart(cfg, "", "", discardLogger())

	// The function should either succeed (Python fallback) or give a Docker error,
	// but NOT "Local publish source does not exist" since the restored path exists.
	if strings.Contains(result, "does not exist") {
		t.Fatalf("should not report missing path when saved state exists, got: %s", result)
	}
}

// ─── findServableProject tests ──────────────────────────────────────────

func TestFindServableProjectEmpty(t *testing.T) {
	dir := t.TempDir()
	pd, bd := findServableProject(dir)
	if pd != "" || bd != "" {
		t.Fatalf("expected empty for empty workspace, got project_dir=%q build_dir=%q", pd, bd)
	}
}

func TestFindServableProjectSingleProject(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "my-site", "out"), 0755)
	os.WriteFile(filepath.Join(dir, "my-site", "out", "index.html"), []byte("hello"), 0644)

	pd, bd := findServableProject(dir)
	if pd != "my-site" || bd != "out" {
		t.Fatalf("expected my-site/out, got project_dir=%q build_dir=%q", pd, bd)
	}
}

func TestFindServableProjectMultipleProjects(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "site-a", "out"), 0755)
	os.MkdirAll(filepath.Join(dir, "site-b", "dist"), 0755)

	pd, bd := findServableProject(dir)
	if pd != "" || bd != "" {
		t.Fatalf("expected empty when multiple candidates exist, got project_dir=%q build_dir=%q", pd, bd)
	}
}

func TestFindServableProjectRootBuildOutput(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "dist"), 0755)
	os.WriteFile(filepath.Join(dir, "dist", "index.html"), []byte("root"), 0644)

	pd, bd := findServableProject(dir)
	if pd != "" || bd != "dist" {
		t.Fatalf("expected root-level dist, got project_dir=%q build_dir=%q", pd, bd)
	}
}

func TestFindServableProjectSkipsDotDirs(t *testing.T) {
	dir := t.TempDir()
	// .hidden should be skipped
	os.MkdirAll(filepath.Join(dir, ".hidden", "out"), 0755)
	// Only one real candidate
	os.MkdirAll(filepath.Join(dir, "real-site", "build"), 0755)

	pd, bd := findServableProject(dir)
	if pd != "real-site" || bd != "build" {
		t.Fatalf("expected real-site/build, got project_dir=%q build_dir=%q", pd, bd)
	}
}

func TestFindServableProjectSkipsNextJsDotNext(t *testing.T) {
	dir := t.TempDir()
	// Only .next exists (no out/dist/build) — should not be found as servable
	os.MkdirAll(filepath.Join(dir, "phaser-demo", ".next"), 0755)
	os.WriteFile(filepath.Join(dir, "phaser-demo", "next.config.js"), []byte("module.exports = {}"), 0644)

	pd, bd := findServableProject(dir)
	if pd != "" || bd != "" {
		t.Fatalf("expected empty when only .next exists (not servable), got project_dir=%q build_dir=%q", pd, bd)
	}
}

func TestFindServableProjectEmptyWorkspace(t *testing.T) {
	pd, bd := findServableProject("")
	if pd != "" || bd != "" {
		t.Fatalf("expected empty for empty workspace path, got project_dir=%q build_dir=%q", pd, bd)
	}
}

// ─── findNextJsProjectWithoutExport tests ────────────────────────────────

func TestFindNextJsProjectWithoutExportSingle(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "phaser-demo"), 0755)
	os.WriteFile(filepath.Join(dir, "phaser-demo", "next.config.js"), []byte("module.exports = {}"), 0644)
	os.MkdirAll(filepath.Join(dir, "phaser-demo", ".next"), 0755)
	// No out/ directory

	result := findNextJsProjectWithoutExport(dir)
	if result != "phaser-demo" {
		t.Fatalf("expected phaser-demo, got %q", result)
	}
}

func TestFindNextJsProjectWithoutExportAlreadyHasOut(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "my-app"), 0755)
	os.WriteFile(filepath.Join(dir, "my-app", "next.config.js"), []byte("module.exports = {}"), 0644)
	os.MkdirAll(filepath.Join(dir, "my-app", "out"), 0755)

	result := findNextJsProjectWithoutExport(dir)
	if result != "" {
		t.Fatalf("expected empty when out/ exists, got %q", result)
	}
}

func TestFindNextJsProjectWithoutExportMultiple(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app-a"), 0755)
	os.MkdirAll(filepath.Join(dir, "app-b"), 0755)
	os.WriteFile(filepath.Join(dir, "app-a", "next.config.js"), []byte("module.exports = {}"), 0644)
	os.WriteFile(filepath.Join(dir, "app-b", "next.config.js"), []byte("module.exports = {}"), 0644)

	result := findNextJsProjectWithoutExport(dir)
	if result != "" {
		t.Fatalf("expected empty when multiple Next.js projects, got %q", result)
	}
}

func TestFindNextJsProjectWithoutExportEmpty(t *testing.T) {
	result := findNextJsProjectWithoutExport("")
	if result != "" {
		t.Fatalf("expected empty for empty path, got %q", result)
	}
}

// ─── detectBuildDir .next skip tests ─────────────────────────────────────

func TestDetectBuildDirSkipsDotNextWithoutIndex(t *testing.T) {
	dir := t.TempDir()
	// Next.js project with .next/ but no index.html inside it, and no out/
	projectDir := filepath.Join(dir, "phaser-demo")
	os.MkdirAll(filepath.Join(projectDir, ".next"), 0755)
	os.WriteFile(filepath.Join(projectDir, "next.config.js"), []byte("module.exports = {}"), 0644)
	// Write something in .next that is NOT index.html
	os.WriteFile(filepath.Join(projectDir, ".next", "build-manifest.json"), []byte("{}"), 0644)

	cfg := HomepageConfig{WorkspacePath: dir}
	result := detectBuildDir(cfg, "phaser-demo")
	// Should NOT return ".next" — should fall through to "."
	if result == ".next" {
		t.Fatal("detectBuildDir should not return .next when it has no index.html")
	}
	if result != "." {
		t.Fatalf("expected '.' fallback, got %q", result)
	}
}

func TestDetectBuildDirAllowsDotNextWithIndex(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "my-site")
	os.MkdirAll(filepath.Join(projectDir, ".next"), 0755)
	os.WriteFile(filepath.Join(projectDir, "next.config.js"), []byte("module.exports = {}"), 0644)
	os.WriteFile(filepath.Join(projectDir, ".next", "index.html"), []byte("<h1>Hello</h1>"), 0644)

	cfg := HomepageConfig{WorkspacePath: dir}
	result := detectBuildDir(cfg, "my-site")
	if result != ".next" {
		t.Fatalf("expected .next when it contains index.html, got %q", result)
	}
}

func TestDetectBuildDirPrefersOutOverDotNext(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "phaser-demo")
	os.MkdirAll(filepath.Join(projectDir, "out"), 0755)
	os.MkdirAll(filepath.Join(projectDir, ".next"), 0755)
	os.WriteFile(filepath.Join(projectDir, "next.config.js"), []byte("module.exports = {}"), 0644)
	os.WriteFile(filepath.Join(projectDir, "out", "index.html"), []byte("<h1>Exported</h1>"), 0644)

	cfg := HomepageConfig{WorkspacePath: dir}
	result := detectBuildDir(cfg, "phaser-demo")
	if result != "out" {
		t.Fatalf("expected out to be preferred over .next, got %q", result)
	}
}

func TestFindServableProjectAutoDetectedByWebServerStart(t *testing.T) {
	dir := t.TempDir()
	// Create a project with servable out/ directory
	os.MkdirAll(filepath.Join(dir, "phaser-demo", "out"), 0755)
	os.WriteFile(filepath.Join(dir, "phaser-demo", "out", "index.html"), []byte("<h1>Game</h1>"), 0644)

	// No saved state — should auto-detect phaser-demo/out
	cfg := HomepageConfig{WorkspacePath: dir, WebServerPort: 0}
	result := HomepageWebServerStart(cfg, "", "", discardLogger())

	// Should not report "does not exist" because auto-detection finds phaser-demo/out
	if strings.Contains(result, "does not exist") {
		t.Fatalf("auto-detect should find phaser-demo/out, got: %s", result)
	}
}
