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
	if len(commands) < 2 || !strings.Contains(commands[0], "command -v node") || !strings.Contains(commands[0], "command -v npm") {
		t.Fatalf("expected node/npm precheck before install, got %#v", commands)
	}
	foundInstall := false
	for _, command := range commands {
		if strings.Contains(command, "npm ci") {
			foundInstall = true
		}
	}
	if !foundInstall {
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

func TestHomepageDeployNetlifyZipExcludesSensitiveAndNonDeployableFiles(t *testing.T) {
	dir := t.TempDir()
	projectRoot := filepath.Join(dir, "site-a")
	for _, subdir := range []string{".git", "node_modules/pkg", ".cache", "data", "assets"} {
		if err := os.MkdirAll(filepath.Join(projectRoot, filepath.FromSlash(subdir)), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", subdir, err)
		}
	}
	files := map[string]string{
		"index.html":                 `<script src="/assets/app.js"></script>`,
		"assets/app.js":              `console.log("ok")`,
		".env":                       "NETLIFY_AUTH_TOKEN=secret",
		".env.local":                 "API_TOKEN=secret",
		".git/config":                "[remote]\nurl=https://token@example.com",
		"node_modules/pkg/index.js":  "module.exports = 'large'",
		".cache/tool-state.json":     `{"token":"secret"}`,
		"data/vault.bin":             "vault",
		"data/homepage_registry.db":  "sqlite",
		"debug.log":                  "stack with secret",
		"npm-debug.log":              "stack with secret",
		".npmrc":                     "//registry.npmjs.org/:_authToken=secret",
		"config.yaml":                "providers:\n- api_key: secret",
		".aurago-site-manifest.json": "old manifest",
	}
	for rel, content := range files {
		path := filepath.Join(projectRoot, filepath.FromSlash(rel))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	oldBaseURL := netlifyBaseURL
	oldAttempts := netlifyDeployPollAttempts
	defer func() {
		netlifyBaseURL = oldBaseURL
		netlifyDeployPollAttempts = oldAttempts
	}()
	netlifyDeployPollAttempts = 0

	zipped := map[string]bool{}
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
			zipped[f.Name] = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"","state":"ready"}`))
	}))
	defer server.Close()
	netlifyBaseURL = server.URL

	result := HomepageDeployNetlify(
		HomepageConfig{WorkspacePath: dir},
		NetlifyConfig{Token: "token", DefaultSiteID: "site-123", AllowDeploy: true},
		"site-a", ".", "", "", false, slogDiscard(),
	)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("decode result %q: %v", result, err)
	}
	if parsed["status"] == "error" {
		t.Fatalf("deploy should succeed while excluding sensitive files, got %s", result)
	}
	if !zipped["index.html"] || !zipped["assets/app.js"] {
		t.Fatalf("expected deployable static files in ZIP, got %#v", zipped)
	}
	for _, blocked := range []string{
		".env",
		".env.local",
		".git/config",
		"node_modules/pkg/index.js",
		".cache/tool-state.json",
		"data/vault.bin",
		"data/homepage_registry.db",
		"debug.log",
		"npm-debug.log",
		".npmrc",
		"config.yaml",
		".aurago-site-manifest.json",
	} {
		if zipped[blocked] {
			t.Fatalf("Netlify ZIP included sensitive or non-deployable file %q; names=%#v", blocked, zipped)
		}
	}
}

func TestHomepageDeployNetlifyPollsDeployIDAndReturnsProviderError(t *testing.T) {
	dir := t.TempDir()
	projectRoot := filepath.Join(dir, "site-a")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "index.html"), []byte(`<h1>Site</h1>`), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	oldBaseURL := netlifyBaseURL
	oldAttempts := netlifyDeployPollAttempts
	oldInterval := netlifyDeployPollInterval
	defer func() {
		netlifyBaseURL = oldBaseURL
		netlifyDeployPollAttempts = oldAttempts
		netlifyDeployPollInterval = oldInterval
	}()
	netlifyDeployPollAttempts = 2
	netlifyDeployPollInterval = 0

	var polls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sites/site-123/deploys":
			_, _ = w.Write([]byte(`{"id":"deploy-123","site_id":"site-123","state":"uploaded"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/deploys/deploy-123":
			polls++
			_, _ = w.Write([]byte(`{"id":"deploy-123","state":"error","error_message":"provider build failed"}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	netlifyBaseURL = server.URL

	result := HomepageDeployNetlify(
		HomepageConfig{WorkspacePath: dir},
		NetlifyConfig{Token: "token", DefaultSiteID: "site-123", AllowDeploy: true},
		"site-a", ".", "", "", false, slogDiscard(),
	)
	if polls == 0 {
		t.Fatalf("expected Netlify deploy status polling after upload, got result %s", result)
	}
	if !strings.Contains(result, `"status":"error"`) || !strings.Contains(result, "provider build failed") {
		t.Fatalf("expected provider deploy error after upload, got %s", result)
	}
}

func TestHomepageDeployNetlifyReturnsSiteAndDeployIDs(t *testing.T) {
	dir := t.TempDir()
	projectRoot := filepath.Join(dir, "site-a")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "index.html"), []byte(`<h1>Site</h1>`), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	oldBaseURL := netlifyBaseURL
	oldAttempts := netlifyDeployPollAttempts
	oldInterval := netlifyDeployPollInterval
	defer func() {
		netlifyBaseURL = oldBaseURL
		netlifyDeployPollAttempts = oldAttempts
		netlifyDeployPollInterval = oldInterval
	}()
	netlifyDeployPollAttempts = 1
	netlifyDeployPollInterval = 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sites/site-123/deploys":
			_, _ = w.Write([]byte(`{"id":"deploy-123","site_id":"site-123","state":"uploaded"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/deploys/deploy-123":
			_, _ = w.Write([]byte(`{"id":"deploy-123","state":"ready"}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	netlifyBaseURL = server.URL

	result := HomepageDeployNetlify(
		HomepageConfig{WorkspacePath: dir},
		NetlifyConfig{Token: "token", DefaultSiteID: "site-123", AllowDeploy: true},
		"site-a", ".", "", "", false, slogDiscard(),
	)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("decode result %q: %v", result, err)
	}
	if parsed["status"] == "error" {
		t.Fatalf("expected successful deploy, got %s", result)
	}
	if parsed["site_id"] != "site-123" {
		t.Fatalf("site_id = %v, want site-123; result=%s", parsed["site_id"], result)
	}
	if parsed["deploy_id"] != "deploy-123" {
		t.Fatalf("deploy_id = %v, want deploy-123; result=%s", parsed["deploy_id"], result)
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

func TestHomepageDeployNetlifyBundlesLegacyRootGeneratedImageRefs(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	projectRoot := filepath.Join(dir, "ki-news")
	imageName := "img_20260530_223436_3106625df7fa.jpeg"
	if err := os.MkdirAll(filepath.Join(dataDir, "generated_images"), 0o755); err != nil {
		t.Fatalf("mkdir data images: %v", err)
	}
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "generated_images", imageName), []byte("jpeg-bytes"), 0o644); err != nil {
		t.Fatalf("write generated image: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "index.html"), []byte(`<div id="app"></div><script src="/assets/main.js"></script>`), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "assets", "main.js"), []byte(`const hero="/`+imageName+`";`), 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	oldBaseURL := netlifyBaseURL
	oldAttempts := netlifyDeployPollAttempts
	defer func() {
		netlifyBaseURL = oldBaseURL
		netlifyDeployPollAttempts = oldAttempts
	}()
	netlifyDeployPollAttempts = 0

	var sawRootImage bool
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
			if f.Name == imageName {
				sawRootImage = true
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"","state":"ready"}`))
	}))
	defer server.Close()
	netlifyBaseURL = server.URL

	result := HomepageDeployNetlify(
		HomepageConfig{WorkspacePath: dir, DataDir: dataDir},
		NetlifyConfig{Token: "token", DefaultSiteID: "site-123", AllowDeploy: true},
		"ki-news", ".", "", "", false, slogDiscard(),
	)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("decode result %q: %v", result, err)
	}
	if parsed["status"] == "error" {
		t.Fatalf("deploy should bundle legacy root image reference, got %s", result)
	}
	if !sawRootImage {
		t.Fatalf("Netlify ZIP did not contain root legacy generated image %q; result=%s", imageName, result)
	}
}

func TestCopyAssetsToBuildDirCopiesGeneratedImageAssetRefs(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	buildDir := filepath.Join(dir, "site", "dist")
	imageName := "img_20260621_111628_homepage.webp"
	if err := os.MkdirAll(filepath.Join(dataDir, "generated_images"), 0o755); err != nil {
		t.Fatalf("mkdir data images: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(buildDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir build assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "generated_images", imageName), []byte("webp-bytes"), 0o644); err != nil {
		t.Fatalf("write generated image: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "assets", "main-Dcz0NIgT.js"), []byte(`const hero="/assets/`+imageName+`";`), 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	copyAssetsToBuildDir(buildDir, dataDir, slogDiscard())

	copied, err := os.ReadFile(filepath.Join(buildDir, "assets", imageName))
	if err != nil {
		t.Fatalf("expected generated image to be copied into dist assets: %v", err)
	}
	if string(copied) != "webp-bytes" {
		t.Fatalf("copied image contents = %q, want webp-bytes", string(copied))
	}
}

func TestHomepageDeployNetlifyBundlesGeneratedImageAssetRefs(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	projectRoot := filepath.Join(dir, "ki-news")
	imageName := "img_20260621_111628_homepage.webp"
	if err := os.MkdirAll(filepath.Join(dataDir, "generated_images"), 0o755); err != nil {
		t.Fatalf("mkdir data images: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir project assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "generated_images", imageName), []byte("webp-bytes"), 0o644); err != nil {
		t.Fatalf("write generated image: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "index.html"), []byte(`<div id="app"></div><script src="/assets/main.js"></script>`), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "assets", "main.js"), []byte(`const hero="/assets/`+imageName+`";`), 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	oldBaseURL := netlifyBaseURL
	oldAttempts := netlifyDeployPollAttempts
	defer func() {
		netlifyBaseURL = oldBaseURL
		netlifyDeployPollAttempts = oldAttempts
	}()
	netlifyDeployPollAttempts = 0

	var sawAssetImage bool
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
			if f.Name == "assets/"+imageName {
				sawAssetImage = true
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"","state":"ready"}`))
	}))
	defer server.Close()
	netlifyBaseURL = server.URL

	result := HomepageDeployNetlify(
		HomepageConfig{WorkspacePath: dir, DataDir: dataDir},
		NetlifyConfig{Token: "token", DefaultSiteID: "site-123", AllowDeploy: true},
		"ki-news", ".", "", "", false, slogDiscard(),
	)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("decode result %q: %v", result, err)
	}
	if parsed["status"] == "error" {
		t.Fatalf("deploy should bundle generated image asset reference, got %s", result)
	}
	if !sawAssetImage {
		t.Fatalf("Netlify ZIP did not contain generated image asset %q; result=%s", imageName, result)
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
