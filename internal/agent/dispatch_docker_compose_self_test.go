package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// stubDockerSelfIdentity binds a fixed identity of AuraGo's own container.
func stubDockerSelfIdentity(t *testing.T, identity tools.DockerSelfIdentity) {
	t.Helper()
	tools.SetDockerSelfIdentityResolver(func(context.Context, tools.DockerConfig) tools.DockerSelfIdentity { return identity })
	t.Cleanup(func() { tools.SetDockerSelfIdentityResolver(nil) })
}

func TestDockerComposePolicyRefusesAuraGosOwnComposeProject(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "aurago/compose.yml", "services:\n  web:\n    image: alpine\n")
	model := `{"name":"aurago","services":{"web":{"image":"alpine"}}}`
	stubDockerComposeResolver(t, func(string) (string, error) { return model, nil })
	cfg := &config.Config{}
	cfg.Docker.AllowHostAccess = true
	useRuntimePermissionsForTest(t, cfg)
	policy := func(command string) string {
		return dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "aurago/compose.yml", Command: command})
	}
	// Without a self identity (native installs, no resolver) nothing changes.
	for _, command := range []string{"up -d", "down", "logs"} {
		if got := policy(command); got != "" {
			t.Fatalf("%s without a self identity: %s", command, got)
		}
	}
	stubDockerSelfIdentity(t, tools.DockerSelfIdentity{ComposeProject: "AuraGo"})
	for _, command := range []string{"up -d", "create", "start", "stop", "restart", "down", "down --remove-orphans", "rm -f", "kill", "pause", "unpause", "logs --tail 20", "top"} {
		got := policy(command)
		if !strings.Contains(got, `"code":"docker_managed_aurago_resource"`) || !strings.Contains(got, "Compose project AuraGo itself runs in") {
			t.Fatalf("%s on AuraGo's own project: %s", command, got)
		}
	}
	for _, command := range []string{"config", "convert", "ps", "images", "port web 80", "ls", "version", "events", "build", "pull"} {
		if got := policy(command); got != "" {
			t.Fatalf("%s does not act on containers but was refused: %s", command, got)
		}
	}
	model = `{"name":"aurago-dev","services":{"web":{"image":"alpine"}}}`
	if got := policy("down"); got != "" {
		t.Fatalf("another project was refused: %s", got)
	}
}
