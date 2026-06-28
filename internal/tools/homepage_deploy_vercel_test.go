package tools

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildVercelDeployCommandLinksProjectBeforeDeploy(t *testing.T) {
	cmd := buildVercelDeployCommand("testseite-aurago", "prj_123", "production", VercelConfig{
		TeamID: "team_abc",
	})

	if !strings.Contains(cmd, "vercel link ") {
		t.Fatalf("expected deploy command to link the Vercel project first, got: %s", cmd)
	}
	if !strings.Contains(cmd, "--project 'prj_123'") {
		t.Fatalf("expected project reference to be used for vercel link, got: %s", cmd)
	}
	if strings.Count(cmd, "--scope 'team_abc'") != 2 {
		t.Fatalf("expected scope to be applied to link and deploy commands, got: %s", cmd)
	}

	parts := strings.SplitN(cmd, "vercel deploy ", 2)
	if len(parts) != 2 {
		t.Fatalf("expected command to include vercel deploy, got: %s", cmd)
	}
	if strings.Contains(parts[1], "--project") {
		t.Fatalf("vercel deploy must not receive the unsupported --project flag, got: %s", cmd)
	}
}

func TestBuildVercelDeployCommandSkipsLinkWithoutProjectRef(t *testing.T) {
	cmd := buildVercelDeployCommand("testseite-aurago", "", "preview", VercelConfig{})

	if strings.Contains(cmd, "vercel link ") {
		t.Fatalf("expected no link step without a project reference, got: %s", cmd)
	}
	if strings.Contains(cmd, "--project") {
		t.Fatalf("expected no --project flag without a project reference, got: %s", cmd)
	}
	if !strings.Contains(cmd, "vercel deploy --yes") {
		t.Fatalf("expected deploy command, got: %s", cmd)
	}
}

func TestHomepageVercelStrategyUsesProjectRootForFrameworks(t *testing.T) {
	candidate := homepageDeployCandidate{BuildDir: "dist", ContainerSubdir: "vite-site/dist", Kind: "spa"}
	deploySubdir, useCandidate := homepageVercelDeploySubdir("vite-site", "vite", "", candidate)
	if deploySubdir != "vite-site" {
		t.Fatalf("framework deploys should run from project root, got %q", deploySubdir)
	}
	if useCandidate {
		t.Fatal("framework deploys should not deploy the static output subdirectory by default")
	}
}

func TestHomepageVercelStrategyIgnoresExplicitBuildDirForPackageProjects(t *testing.T) {
	candidate := homepageDeployCandidate{BuildDir: "dist", ContainerSubdir: "vite-site/dist", Kind: "spa"}
	deploySubdir, useCandidate := homepageVercelDeploySubdir("vite-site", "vite", "dist", candidate, true)
	if deploySubdir != "vite-site" {
		t.Fatalf("package projects should deploy from source root even with explicit build_dir, got %q", deploySubdir)
	}
	if useCandidate {
		t.Fatal("package projects should not deploy the static output subdirectory")
	}
}

func TestHomepageVercelStrategyAllowsExplicitStaticBuildDirForPlainStatic(t *testing.T) {
	candidate := homepageDeployCandidate{BuildDir: "dist", ContainerSubdir: "static-site/dist", Kind: "static"}
	deploySubdir, useCandidate := homepageVercelDeploySubdir("static-site", "static", "dist", candidate, false)
	if deploySubdir != "static-site/dist" {
		t.Fatalf("plain static explicit build_dir should deploy static output, got %q", deploySubdir)
	}
	if !useCandidate {
		t.Fatal("plain static explicit build_dir should use the validated static candidate")
	}
}

func TestHomepageDeployVercelAllowsFrameworkSourceBuildWithoutStaticIndex(t *testing.T) {
	dir := t.TempDir()
	projectRoot := filepath.Join(dir, "next-site")
	if err := os.MkdirAll(filepath.Join(projectRoot, "node_modules"), 0o755); err != nil {
		t.Fatalf("mkdir project node_modules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "package.json"), []byte(`{"scripts":{"build":"next build"},"dependencies":{"next":"latest"}}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "next.config.mjs"), []byte(`export default {}`), 0o644); err != nil {
		t.Fatalf("write next config: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v9/projects/") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"prj_123","name":"next-site","framework":"nextjs"}`))
			return
		}
		t.Fatalf("unexpected Vercel API request %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	oldBaseURL := vercelBaseURL
	oldHTTPClient := vercelHTTPClient
	oldExec := homepageDockerExecFunc
	oldInternal := homepageDockerExecInternalFunc
	defer func() {
		vercelBaseURL = oldBaseURL
		vercelHTTPClient = oldHTTPClient
		homepageDockerExecFunc = oldExec
		homepageDockerExecInternalFunc = oldInternal
	}()
	vercelBaseURL = server.URL
	vercelHTTPClient = server.Client()

	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		switch {
		case strings.Contains(command, "command -v node") && strings.Contains(command, "command -v npm"):
			return `{"status":"ok","exit_code":0,"output":"/usr/local/bin/node\n/usr/local/bin/npm"}`
		case strings.Contains(command, "npm run build"):
			return `{"status":"ok","exit_code":0,"output":"next build completed without static export"}`
		default:
			return `{"status":"ok","exit_code":0,"output":""}`
		}
	}

	deployCalled := false
	homepageDockerExecInternalFunc = func(cfg DockerConfig, containerName, command, user string, env []string) string {
		if strings.Contains(command, "vercel deploy") {
			deployCalled = true
			return `{"status":"ok","exit_code":0,"output":"deployment finished but output omitted the URL"}`
		}
		return `{"status":"ok","exit_code":0,"output":""}`
	}

	result := HomepageDeployVercel(
		HomepageConfig{WorkspacePath: dir},
		VercelConfig{Token: "token", DefaultProjectID: "next-site"},
		"next-site", "", "", "preview", "", "", false, false, slogDiscard(),
	)
	if !deployCalled {
		t.Fatalf("expected Vercel deploy to proceed after framework build without static index, got: %s", result)
	}
	if strings.Contains(result, "index.html") {
		t.Fatalf("Vercel framework-source deploy must not fail on missing static index.html, got: %s", result)
	}
	if !strings.Contains(result, "no deployment URL") {
		t.Fatalf("expected test to reach mocked deploy result, got: %s", result)
	}
}
