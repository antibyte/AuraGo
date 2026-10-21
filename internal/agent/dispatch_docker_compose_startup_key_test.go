package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func TestDockerComposeProtectsRetainedStartupKeyAfterEnvironmentRemoval(t *testing.T) {
	previous := security.ProcessMasterKey()
	t.Cleanup(func() { security.RetainProcessMasterKey(previous) })
	key := strings.Repeat("a4", 32)
	security.RetainProcessMasterKey(key)
	t.Setenv("AURAGO_MASTER_KEY", "")
	if err := os.Unsetenv("AURAGO_MASTER_KEY"); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Docker.AllowHostAccess = true
	useRuntimePermissionsForTest(t, cfg)
	if dockerComposeMasterKey(cfg) != key {
		t.Fatal("Compose lost the key protected by the startup handoff")
	}
	cfg.Server.MasterKey = strings.Repeat("c5", 32)
	if dockerComposeMasterKey(cfg) != key {
		t.Fatal("stale run snapshot replaced the protected startup key")
	}
	cfg.Server.MasterKey = ""
	// Match preflight normalization of raw source, including uppercase key copies.
	rawSource := "COPIED=" + strings.ToUpper(key)
	if _, blocked := dockerComposeRawStateReference(strings.ToLower(rawSource), cfg); !blocked {
		t.Fatal("raw Compose source no longer protects the retained key")
	}
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "app/compose.yml", "services:\n  app:\n    image: alpine\n    env_file: [copy.env]\n")
	stubDockerComposeResolverByMode(t,
		`{"services":{"app":{"image":"alpine","environment":{"COPIED":"`+strings.ToUpper(key)+`"}}}}`,
		`{"services":{"app":{"image":"alpine","env_file":[{"path":`+jsonPath(t, filepath.Join(workspace, "app", "copy.env"))+`}]}}}`)
	for _, command := range []string{"up -d", "create", "config", "build", "pull"} {
		got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "app/compose.yml", Command: command})
		if !strings.Contains(got, `"code":"docker_compose_protected_path_denied"`) || strings.Contains(strings.ToLower(got), key) {
			t.Fatal("Compose allowed or echoed the retained master key")
		}
	}
	// A clean resolved model must still reject key-bearing command arguments;
	// these assertions cannot pass merely because the fixture env_file leaked.
	stubDockerComposeResolver(t, func(string) (string, error) {
		return `{"services":{"app":{"image":"alpine","build":{"context":` + jsonPath(t, filepath.Join(workspace, "app")) + `}}}}`, nil
	})
	for _, command := range []string{"build --build-arg AURAGO_MASTER_KEY", "build --build-arg COPY=" + key} {
		got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "app/compose.yml", Command: command})
		if !strings.Contains(got, `"code":"docker_compose_protected_path_denied"`) || strings.Contains(strings.ToLower(got), key) {
			t.Fatal("Compose allowed or echoed the retained key through build arguments")
		}
	}
	if _, present := os.LookupEnv("AURAGO_MASTER_KEY"); present || cfg.Server.MasterKey != "" {
		t.Fatal("Compose restored an ambient/config master key")
	}
}

func TestDockerComposeMasterKeyStandaloneFallbacks(t *testing.T) {
	previous := security.ProcessMasterKey()
	t.Cleanup(func() { security.RetainProcessMasterKey(previous) })
	security.RetainProcessMasterKey("")
	key := strings.Repeat("e6", 32)
	cfg := &config.Config{}
	cfg.Server.MasterKey = key
	if dockerComposeMasterKey(cfg) != key {
		t.Fatal("standalone config key compatibility changed")
	}
	cfg.Server.MasterKey = ""
	t.Setenv("AURAGO_MASTER_KEY", key)
	if dockerComposeMasterKey(cfg) != key {
		t.Fatal("standalone environment key compatibility changed")
	}
}
