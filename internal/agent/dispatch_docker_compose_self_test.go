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

func TestDispatchDockerRefusesAuraGosDataVolumeInContainers(t *testing.T) {
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	cfg.Directories.WorkspaceDir = t.TempDir()
	useRuntimePermissionsForTest(t, cfg)
	calls := []ToolCall{
		{Action: "docker", Operation: "run", Name: "thief", Image: "alpine:latest", Volumes: []string{"aurago_aurago_data:/loot:ro"}},
		{Action: "docker", Operation: "create", Name: "thief", Image: "alpine:latest", Volumes: []string{"stack_aurago_data:/loot"}},
		{Action: "docker", Operation: "remove_volume", Name: "aurago_aurago_data"},
	}
	run := func(call ToolCall) string {
		output, ok := dispatchServices(context.Background(), call, &DispatchContext{Cfg: cfg, Logger: testLogger})
		if !ok {
			t.Fatal("expected docker operation to be handled")
		}
		return output
	}
	// Native installs: there is no AuraGo volume, so these calls go on to Docker.
	for _, call := range calls {
		if got := run(call); strings.Contains(got, "AuraGo's own data") {
			t.Fatalf("%s on a native install refused: %s", call.Operation, got)
		}
	}
	cfg.Runtime.IsDocker = true
	for _, call := range calls {
		if got := run(call); !strings.Contains(got, `"code":"docker_managed_aurago_resource"`) || !strings.Contains(got, "AuraGo's own data") {
			t.Fatalf("%s in a container: %s", call.Operation, got)
		}
	}
	if got := run(ToolCall{Action: "docker", Operation: "run", Name: "ffmpeg", Image: "alpine:latest", Volumes: []string{"aurago_aurago_workdir:/work"}}); strings.Contains(got, "AuraGo's own data") {
		t.Fatalf("the workdir volume (the agent workspace) was refused: %s", got)
	}
}

func TestDockerComposePolicyRefusesAuraGosDataVolumeInContainers(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "steal/compose.yml", "services:\n  thief:\n    image: alpine\n")
	model := `{"name":"steal","services":{"thief":{"image":"alpine","volumes":[{"type":"volume","source":"loot","target":"/loot"}]}},"volumes":{"loot":{"name":"aurago_aurago_data","external":true}}}`
	stubDockerComposeResolver(t, func(string) (string, error) { return model, nil })
	cfg := &config.Config{}
	cfg.Docker.AllowHostAccess = true
	useRuntimePermissionsForTest(t, cfg)
	policy := func(command string) string {
		return dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "steal/compose.yml", Command: command})
	}
	if got := policy("up -d"); got != "" {
		t.Fatalf("native install refused: %s", got)
	}
	cfg.Runtime.IsDocker = true
	for _, command := range []string{"up -d", "down -v", "ps"} {
		if got := policy(command); !strings.Contains(got, `"code":"docker_managed_aurago_resource"`) || !strings.Contains(got, "aurago_aurago_data") {
			t.Fatalf("%s: %s", command, got)
		}
	}
	// A proven identity names the exact volume; another stack's aurago_data stays usable.
	stubDockerSelfIdentity(t, tools.DockerSelfIdentity{Proven: true, StateVolumes: []string{"prod_aurago_data"}})
	if got := policy("up -d"); got != "" {
		t.Fatalf("another stack's volume refused with a proven identity: %s", got)
	}
	model = strings.ReplaceAll(model, "aurago_aurago_data", "prod_aurago_data")
	if got := policy("up -d"); !strings.Contains(got, "prod_aurago_data") {
		t.Fatalf("AuraGo's proven data volume allowed: %s", got)
	}
}

func TestDockerComposePolicyProtectsTheHostDirectoryOfAuraGosData(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "c/compose.yml", "services:\n  app:\n    image: alpine\n")
	stubDockerComposeResolver(t, func(string) (string, error) {
		return `{"services":{"app":{"image":"alpine","volumes":[{"type":"bind","source":"/srv/aurago/data","target":"/d"}]}}}`, nil
	})
	cfg := &config.Config{}
	cfg.Runtime.IsDocker = true
	cfg.Docker.AllowHostAccess = true
	useRuntimePermissionsForTest(t, cfg)
	stubDockerSelfIdentity(t, tools.DockerSelfIdentity{StateBindSources: []string{"/srv/aurago/data"}})
	got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "c/compose.yml", Command: "up -d"})
	if !strings.Contains(got, `"code":"docker_compose_protected_path_denied"`) {
		t.Fatalf("bind of the host directory behind /app/data allowed with host access: %s", got)
	}
}

// The host path behind AuraGo's data volume (/var/lib/docker/volumes/<name>/_data)
// is AuraGo state as well; host access never lifts that tier.
func TestDockerComposePolicyProtectsTheHostPathOfAuraGosDataVolume(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "c/compose.yml", "services:\n  app:\n    image: alpine\n")
	stubDockerComposeResolver(t, func(string) (string, error) {
		return `{"services":{"app":{"image":"alpine","volumes":[{"type":"bind","source":"/var/lib/docker/volumes/aurago_aurago_data/_data/vault.bin","target":"/v"}]}}}`, nil
	})
	cfg := &config.Config{}
	cfg.Runtime.IsDocker = true
	cfg.Docker.AllowHostAccess = true
	useRuntimePermissionsForTest(t, cfg)
	policy := func() string {
		return dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "c/compose.yml", Command: "up -d"})
	}
	if got := policy(); got != "" {
		t.Fatalf("without a proven identity the host path is not known, so it must stay allowed with host access: %s", got)
	}
	stubDockerSelfIdentity(t, tools.DockerSelfIdentity{Proven: true, StateVolumes: []string{"aurago_aurago_data"}, StateHostPaths: []string{"/var/lib/docker/volumes/aurago_aurago_data/_data"}})
	if got := policy(); !strings.Contains(got, `"code":"docker_compose_protected_path_denied"`) {
		t.Fatalf("bind of the host path behind AuraGo's data volume allowed with host access: %s", got)
	}
}
