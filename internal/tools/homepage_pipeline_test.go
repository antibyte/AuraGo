package tools

import (
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
