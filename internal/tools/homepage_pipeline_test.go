package tools

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomepagePrepareDependenciesInstallsMissingNodeModules(t *testing.T) {
	dir := t.TempDir()
	projectRoot := filepath.Join(dir, "vite-site")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "package.json"), []byte(`{"scripts":{"build":"vite"},"dependencies":{"@vitejs/plugin-react":"latest"}}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0o644); err != nil {
		t.Fatalf("write package-lock.json: %v", err)
	}

	oldExec := homepageDockerExecFunc
	defer func() { homepageDockerExecFunc = oldExec }()
	var commands []string
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		commands = append(commands, command)
		return `{"status":"ok","exit_code":0,"output":"installed"}`
	}

	project, err := homepageResolveProject(HomepageConfig{WorkspacePath: dir}, "vite-site")
	if err != nil {
		t.Fatalf("resolve project: %v", err)
	}
	result := homepagePrepareDependencies(HomepageConfig{WorkspacePath: dir}, project, slogDiscard())
	if result.Status != "ok" {
		t.Fatalf("expected prepare ok, got %+v", result)
	}
	if !result.InstallRan {
		t.Fatalf("expected install to run, got %+v", result)
	}
	if result.PackageManager != "npm" {
		t.Fatalf("expected npm package manager, got %q", result.PackageManager)
	}
	if len(commands) == 0 || !strings.Contains(commands[0], "npm ci") {
		t.Fatalf("expected npm ci command, got %#v", commands)
	}
}

func TestHomepageDetectDeployCandidateRejectsPublicWithoutIndex(t *testing.T) {
	dir := t.TempDir()
	projectRoot := filepath.Join(dir, "app")
	if err := os.MkdirAll(filepath.Join(projectRoot, "public"), 0o755); err != nil {
		t.Fatalf("mkdir public: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "package.json"), []byte(`{"scripts":{"build":"vite"}}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "public", "logo.svg"), []byte(`<svg></svg>`), 0o644); err != nil {
		t.Fatalf("write logo: %v", err)
	}

	_, err := homepageDetectDeployCandidate(HomepageConfig{WorkspacePath: dir}, "app", "", "vite")
	if err == nil {
		t.Fatal("expected public without index.html to be rejected")
	}
	if !strings.Contains(err.Error(), "index.html") {
		t.Fatalf("expected index.html guidance, got %v", err)
	}
}

func TestHomepageDetectDeployCandidateAcceptsDistWithIndex(t *testing.T) {
	dir := t.TempDir()
	dist := filepath.Join(dir, "app", "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte(`<script src="/assets/app.js"></script>`), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	candidate, err := homepageDetectDeployCandidate(HomepageConfig{WorkspacePath: dir}, "app", "", "vite")
	if err != nil {
		t.Fatalf("detect candidate: %v", err)
	}
	if candidate.BuildDir != "dist" || candidate.Kind != "spa" {
		t.Fatalf("unexpected candidate: %+v", candidate)
	}
}

func TestHomepageDeployNetlifyFallsBackToStaticSiblingAfterBuildFailure(t *testing.T) {
	dir := t.TempDir()
	appRoot := filepath.Join(dir, "ki-news")
	staticRoot := filepath.Join(dir, "ki-news-static")
	if err := os.MkdirAll(appRoot, 0o755); err != nil {
		t.Fatalf("mkdir app: %v", err)
	}
	if err := os.MkdirAll(staticRoot, 0o755); err != nil {
		t.Fatalf("mkdir static: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appRoot, "package.json"), []byte(`{"scripts":{"build":"vite --host 0.0.0.0"}}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticRoot, "index.html"), []byte(`<main>Static KI News</main>`), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	oldExec := homepageDockerExecFunc
	defer func() { homepageDockerExecFunc = oldExec }()
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		return `{"status":"error","exit_code":1,"output":"Cannot find package '@vitejs/plugin-react'"}`
	}

	oldBaseURL := netlifyBaseURL
	oldAttempts := netlifyDeployPollAttempts
	defer func() {
		netlifyBaseURL = oldBaseURL
		netlifyDeployPollAttempts = oldAttempts
	}()
	netlifyDeployPollAttempts = 0

	var sawStaticIndex bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/sites/site-123/deploys" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read deploy body: %v", err)
		}
		zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			t.Fatalf("read deploy zip: %v", err)
		}
		for _, f := range zr.File {
			if f.Name != "index.html" {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open zipped index: %v", err)
			}
			data, _ := io.ReadAll(rc)
			_ = rc.Close()
			sawStaticIndex = strings.Contains(string(data), "Static KI News")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"","state":"ready"}`))
	}))
	defer server.Close()
	netlifyBaseURL = server.URL

	result := HomepageDeployNetlify(
		HomepageConfig{WorkspacePath: dir},
		NetlifyConfig{Token: "token", DefaultSiteID: "site-123", AllowDeploy: true},
		"ki-news", "", "", "", false, slogDiscard(),
	)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("decode result %q: %v", result, err)
	}
	if parsed["status"] == "error" {
		t.Fatalf("deploy should fall back to static sibling, got %s", result)
	}
	if !sawStaticIndex {
		t.Fatalf("Netlify ZIP did not contain the static sibling index.html; result=%s", result)
	}
	if parsed["fallback_project_dir"] != "ki-news-static" {
		t.Fatalf("fallback_project_dir = %v, want ki-news-static; result=%s", parsed["fallback_project_dir"], result)
	}
}

func TestEnsureNextJsStaticExportWritesValidMJS(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "next.config.mjs")
	if err := os.WriteFile(configPath, []byte("export default createConfig()\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if !ensureNextJsStaticExport(dir, slogDiscard()) {
		t.Fatal("expected config to be patched")
	}
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(contents)
	if strings.Contains(text, "import type") || strings.Contains(text, ": NextConfig") {
		t.Fatalf("next.config.mjs must not contain TypeScript syntax:\n%s", text)
	}
	if !strings.Contains(text, "export default nextConfig") {
		t.Fatalf("expected ESM default export, got:\n%s", text)
	}
}
