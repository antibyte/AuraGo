package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func stubDockerComposeResolver(t *testing.T, resolve func(file string) (string, error)) *[]string {
	t.Helper()
	var seen []string
	original := resolveDockerComposeConfig
	t.Cleanup(func() { resolveDockerComposeConfig = original })
	resolveDockerComposeConfig = func(_ tools.DockerConfig, file string) (string, error) {
		seen = append(seen, file)
		return resolve(file)
	}
	return &seen
}

func writeComposeFixture(t *testing.T, workspace, name, body string) string {
	t.Helper()
	path := filepath.Join(workspace, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDockerComposePolicyBlocksOwnedResourcesHiddenFromRawText(t *testing.T) {
	workspace := t.TempDir()
	// The raw file names no protected token. The names come from an included
	// file and from .env interpolation, as `docker compose config` resolves them.
	writeComposeFixture(t, workspace, "stack/compose.yml", "include:\n  - inc.yml\nservices:\n  web:\n    image: alpine\n    container_name: aurago-${G}\n")
	dockerCfg := tools.DockerConfig{WorkspaceDir: workspace}
	cases := []struct{ name, resolved, want string }{
		{"garage via .env", `{"services":{"web":{"image":"alpine","container_name":"aurago-boring-garage"}}}`, "Boring Computers Garage"},
		{"homepage via include", `{"services":{"squat":{"image":"alpine","container_name":"aurago-homepage-web"}}}`, `"code":"docker_managed_homepage_resource"`},
		{"homepage label", `{"services":{"web":{"image":"alpine","labels":{"aurago.managed":"homepage"}}}}`, `"code":"docker_managed_homepage_resource"`},
		{"homepage image", `{"services":{"web":{"image":"registry.test/team/aurago-homepage:v1"}}}`, `"code":"docker_managed_homepage_resource"`},
		{"garage data bind", `{"services":{"web":{"image":"alpine","volumes":[{"type":"bind","source":"/opt/aurago/data/sidecars/garage","target":"/data"}]}}}`, "Boring Computers Garage"},
		{"local llm named volume", `{"services":{"web":{"image":"alpine","volumes":[{"type":"volume","source":"models","target":"/m"}]}},"volumes":{"models":{"name":"aurago_models"}}}`, "managed local LLM volumes"},
		{"app container name", `{"services":{"web":{"image":"alpine","container_name":"aurago"}}}`, `"code":"docker_managed_aurago_resource"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubDockerComposeResolver(t, func(string) (string, error) { return tc.resolved, nil })
			got := dockerComposePolicy(context.Background(), &config.Config{}, dockerCfg, dockerArgs{Operation: "compose", File: "stack/compose.yml", Command: "up -d"})
			if !strings.Contains(got, tc.want) {
				t.Fatalf("dockerComposePolicy() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestDockerComposePolicyKeepsRawTokenMatches(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "# proxies aurago-homepage\nservices:\n  web:\n    image: alpine\n")
	stubDockerComposeResolver(t, func(string) (string, error) { return `{"services":{"web":{"image":"alpine"}}}`, nil })
	got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: "ps"})
	if !strings.Contains(got, `"code":"docker_managed_homepage_resource"`) {
		t.Fatalf("raw-text homepage match no longer blocks: %s", got)
	}
}

func TestDockerComposePolicyIgnoresProtectedWordsInResolvedProjectPaths(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "aurago-homepage-tests/compose.yml", "services:\n  web:\n    image: alpine\n    volumes:\n      - ./data:/data\n")
	stubDockerComposeResolver(t, func(string) (string, error) {
		return `{"name":"aurago-homepage-tests","services":{"web":{"image":"alpine","volumes":[{"type":"bind","source":"/ws/aurago-homepage-tests/data","target":"/data"}]}}}`, nil
	})
	got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "aurago-homepage-tests/compose.yml", Command: "ps"})
	if got != "" {
		t.Fatalf("project path name blocked an unrelated stack: %s", got)
	}
}

func TestDockerComposePolicyAllowsUserContainerNamesEndingInAurago(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "dev/compose.yml", "services:\n  web:\n    image: alpine\n    container_name: dev-aurago\n")
	stubDockerComposeResolver(t, func(string) (string, error) {
		return `{"services":{"web":{"image":"alpine","container_name":"dev-aurago"}}}`, nil
	})
	// up -d is covered in K6 (TestDockerComposePolicyUpAllowsDevAuragoName), where
	// the env_file resolver can be stubbed.
	for _, command := range []string{"ps", "down", "config"} {
		got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "dev/compose.yml", Command: command})
		if got != "" {
			t.Fatalf("%s: container_name dev-aurago was blocked: %s", command, got)
		}
	}
}

func TestDockerComposePolicyResolvesAbsoluteWorkspaceFileOnce(t *testing.T) {
	workspace := t.TempDir()
	file := writeComposeFixture(t, workspace, "stack/compose.yml", "services:\n  web:\n    image: alpine\n")
	seen := stubDockerComposeResolver(t, func(string) (string, error) { return `{"services":{"web":{"image":"alpine"}}}`, nil })
	got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: file, Command: "ps"})
	if got != "" {
		t.Fatalf("absolute in-workspace compose file blocked: %s", got)
	}
	if len(*seen) != 1 || (*seen)[0] != file {
		t.Fatalf("resolver calls = %q, want exactly [%q]", *seen, file)
	}
}

func TestDockerComposePolicyWithoutWorkspaceResolvesAgainstWorkingDirectory(t *testing.T) {
	// An empty directories.workspace_dir stays empty after config loading. Before
	// the shared preflight such a config resolved the compose file against the
	// process working directory and ran; it must keep doing so.
	dir := t.TempDir()
	fixture := writeComposeFixture(t, dir, "stack/compose.yml", "services:\n  web:\n    image: alpine\n")
	t.Chdir(dir)
	resolved := `{"services":{"web":{"image":"alpine"}}}`
	seen := stubDockerComposeResolver(t, func(string) (string, error) { return resolved, nil })
	req := dockerArgs{Operation: "compose", File: "stack/compose.yml", Command: "ps"}

	if got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{}, req); got != "" {
		t.Fatalf("relative compose file without a workspace was blocked: %s", got)
	}
	if len(*seen) != 1 || !filepath.IsAbs((*seen)[0]) {
		t.Fatalf("resolver calls = %q, want one absolute path", *seen)
	}
	resolvedInfo, err := os.Stat((*seen)[0])
	if err != nil {
		t.Fatalf("resolver got a path that does not exist: %v", err)
	}
	fixtureInfo, err := os.Stat(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(resolvedInfo, fixtureInfo) {
		t.Fatalf("resolver got %q, want the working-directory file %q", (*seen)[0], fixture)
	}

	resolved = `{"services":{"web":{"image":"alpine","container_name":"aurago"}}}`
	if got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{}, req); !strings.Contains(got, `"code":"docker_managed_aurago_resource"`) {
		t.Fatalf("AuraGo-owned container without a workspace was not blocked: %s", got)
	}
}

func TestDockerComposePolicyReportsPreflightFailure(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n    env_file: [missing.env]\n")
	stubDockerComposeResolver(t, func(string) (string, error) {
		return "", errors.New("resolve Compose config: exit status 1: env file missing.env not found")
	})
	dockerCfg := tools.DockerConfig{WorkspaceDir: workspace}
	for _, file := range []string{"compose.yml", "missing.yml", filepath.Join("..", "outside.yml")} {
		got := dockerComposePolicy(context.Background(), &config.Config{}, dockerCfg, dockerArgs{Operation: "compose", File: file, Command: "ps"})
		if !strings.Contains(got, `"code":"docker_compose_preflight_failed"`) {
			t.Fatalf("%s: got %s, want docker_compose_preflight_failed", file, got)
		}
		if classifyLegacyToolResult(got) != ToolResultFailed {
			t.Fatalf("%s: preflight failure classified as %v", file, classifyLegacyToolResult(got))
		}
	}
	got := dockerComposePolicy(context.Background(), &config.Config{}, dockerCfg, dockerArgs{Operation: "compose", File: "compose.yml", Command: "ps"})
	if !strings.Contains(got, "env file missing.env not found") {
		t.Fatalf("Compose error detail missing: %s", got)
	}
}
