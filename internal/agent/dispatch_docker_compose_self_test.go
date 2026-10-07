package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
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

// Without a proven identity the shipped names can match another stack's
// volume, so both denials say how to get out of the way.
func TestAuraGoDataVolumeDenialsSuggestRenamingAnotherStacksVolume(t *testing.T) {
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Runtime.IsDocker = true
	useRuntimePermissionsForTest(t, cfg)
	output, _ := dispatchServices(context.Background(), ToolCall{Action: "docker", Operation: "remove_volume", Name: "media_aurago_data"}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !strings.Contains(output, "give it another name") {
		t.Fatalf("create/run/volume denial lacks the rename hint: %s", output)
	}
	workspace := cfg.Directories.WorkspaceDir
	writeComposeFixture(t, workspace, "media/compose.yml", "services:\n  app:\n    image: alpine\n")
	stubDockerComposeResolver(t, func(string) (string, error) {
		return `{"services":{"app":{"image":"alpine","volumes":[{"type":"volume","source":"aurago_data","target":"/d"}]}},"volumes":{"aurago_data":{"name":"media_aurago_data"}}}`, nil
	})
	cfg.Docker.AllowHostAccess = true
	got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "media/compose.yml", Command: "up -d"})
	if !strings.Contains(got, `"code":"docker_managed_aurago_resource"`) || !strings.Contains(got, "give it another name") {
		t.Fatalf("Compose denial lacks the rename hint: %s", got)
	}
}

func TestDispatchDockerCreateRefusesManagedSidecarNamesWithoutHostAccess(t *testing.T) {
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.ReadOnly = true // a call that passes the name check stops at the read-only gate, before Docker
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	cfg.Directories.WorkspaceDir = t.TempDir()
	useRuntimePermissionsForTest(t, cfg)
	create := func(operation, name string) string {
		output, _ := dispatchServices(context.Background(), ToolCall{Action: "docker", Operation: operation, Name: name, Image: "alpine:latest"}, &DispatchContext{Cfg: cfg, Logger: testLogger})
		return output
	}
	for _, name := range []string{"aurago_gotenberg", "AURAGO_ANSIBLE", "/aurago-piper-tts", "aurago_browser_automation", "aurago_dograh_redis"} {
		for _, operation := range []string{"run", "create"} {
			if got := create(operation, name); !strings.Contains(got, `"code":"docker_managed_sidecar_name"`) {
				t.Fatalf("%s %s without host access: %s", operation, name, got)
			}
		}
	}
	for _, name := range []string{"worker", "aurago-code-studio", "aurago-openscad", "my-gotenberg"} {
		if got := create("run", name); strings.Contains(got, "docker_managed_sidecar_name") {
			t.Fatalf("%s refused: %s", name, got)
		}
	}
	// Grandfathered installs (docker.allow_host_access true) are unchanged.
	cfg.Docker.AllowHostAccess = true
	cfg.Tools.DocumentCreator.Enabled = true
	cfg.Tools.DocumentCreator.Backend = "gotenberg"
	if got := create("run", "aurago_gotenberg"); strings.Contains(got, "docker_managed_sidecar_name") {
		t.Fatalf("grandfathered install refused a sidecar name: %s", got)
	}
	// The security proxy name stays reserved for everyone, as before.
	if got := create("run", "aurago-security-proxy"); !strings.Contains(got, `"code":"docker_managed_security_proxy_resource"`) {
		t.Fatalf("security proxy name with host access: %s", got)
	}
}

func TestDockerComposePolicyRefusesManagedSidecarContainerNamesWithoutHostAccess(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "pdf/compose.yml", "services:\n  pdf:\n    image: gotenberg/gotenberg:8\n")
	stubDockerComposeResolver(t, func(string) (string, error) {
		return `{"name":"pdf","services":{"pdf":{"image":"gotenberg/gotenberg:8","container_name":"aurago_gotenberg"}}}`, nil
	})
	cfg := &config.Config{}
	useRuntimePermissionsForTest(t, cfg)
	policy := func(command string) string {
		return dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "pdf/compose.yml", Command: command})
	}
	for _, command := range []string{"up -d", "create"} {
		if got := policy(command); !strings.Contains(got, `"code":"docker_managed_sidecar_name"`) {
			t.Fatalf("%s: %s", command, got)
		}
	}
	for _, command := range []string{"ps", "down", "logs", "config", "pull"} {
		if got := policy(command); got != "" {
			t.Fatalf("%s refused: %s", command, got)
		}
	}
	cfg.Docker.AllowHostAccess = true
	if got := policy("up -d"); got != "" {
		t.Fatalf("grandfathered install refused a sidecar name in a stack: %s", got)
	}
}

// internal/proxy/AGENTS.md: agent create/run already refuses the security
// proxy name for everyone; a Compose container_name of it is refused for
// everyone too. Only container_name counts, so stacks that join the proxy's
// namespaces stay as they are.
func TestDockerComposePolicyRefusesTheSecurityProxyContainerName(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "edge/compose.yml", "services:\n  edge:\n    image: caddy:2\n")
	model := `{"name":"edge","services":{"edge":{"image":"caddy:2","container_name":"Aurago-Security-Proxy"}}}`
	stubDockerComposeResolver(t, func(string) (string, error) { return model, nil })
	cfg := &config.Config{}
	cfg.Docker.AllowHostAccess = true
	useRuntimePermissionsForTest(t, cfg)
	policy := func(command string) string {
		return dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "edge/compose.yml", Command: command})
	}
	for _, command := range []string{"up -d", "create"} {
		if got := policy(command); !strings.Contains(got, `"code":"docker_managed_security_proxy_resource"`) {
			t.Fatalf("%s with host access: %s", command, got)
		}
	}
	if got := policy("ps"); got != "" {
		t.Fatalf("ps refused: %s", got)
	}
	model = `{"name":"edge","services":{"edge":{"image":"caddy:2","network_mode":"container:aurago-security-proxy"}}}`
	if got := policy("up -d"); got != "" {
		t.Fatalf("a stack joining the proxy's namespace was newly refused: %s", got)
	}
}

// F-C20: without a workspace, validateDockerBindMount confines no create/run
// bind, so while docker.allow_host_access is off AuraGo's own state (the
// Compose always tier, plus the host paths of a proven container's data) is
// refused there too. With the flag on, or with a workspace, nothing changes.
func TestDockerCreateStateBindDenialWithoutAWorkspace(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.DataDir = filepath.Join(root, "data")
	cfg.ConfigPath = filepath.Join(root, "config.yaml")
	slash := func(path string) string { return dockerutil.NormalizeHostPathForBind(path) }
	stateBinds := []string{slash(filepath.Join(root, "data", "vault.bin")) + ":/v:ro", slash(filepath.Join(root, "config.yaml")) + ":/c", slash(filepath.Join(root, ".env")) + ":/e"}
	denial := func(operation string, volumes ...string) string {
		return dockerCreateStateBindDenial(context.Background(), cfg, tools.DockerConfig{}, dockerArgs{Operation: operation, Name: "worker", Image: "alpine", Volumes: volumes})
	}
	for _, bind := range stateBinds {
		for _, operation := range []string{"run", "create"} {
			if got := denial(operation, slash(filepath.Join(root, "media"))+":/m", bind); !strings.Contains(got, `"code":"docker_protected_path_denied"`) {
				t.Fatalf("%s %s without a workspace and host access: %s", operation, bind, got)
			}
		}
	}
	for _, volumes := range [][]string{{slash(filepath.Join(root, "media")) + ":/m"}, {slash(root) + ":/parent"}, {"named:/data"}, nil} {
		if got := denial("run", volumes...); got != "" {
			t.Fatalf("%q refused: %s", volumes, got)
		}
	}
	if got := denial("start", stateBinds[0]); got != "" {
		t.Fatalf("an operation that creates nothing was refused: %s", got)
	}
	// A proven container also protects the host paths behind its data.
	cfg.Runtime.IsDocker = true
	stubDockerSelfIdentity(t, tools.DockerSelfIdentity{Proven: true, StateHostPaths: []string{"/var/lib/docker/volumes/aurago_aurago_data/_data"}})
	if got := denial("run", "/var/lib/docker/volumes/aurago_aurago_data/_data:/d"); !strings.Contains(got, "docker_protected_path_denied") {
		t.Fatalf("host path of the proven data volume allowed: %s", got)
	}
	// Grandfathered installs and configured workspaces are unchanged.
	cfg.Docker.AllowHostAccess = true
	if got := denial("run", stateBinds[0]); got != "" {
		t.Fatalf("with host access a state bind was newly refused: %s", got)
	}
	cfg.Docker.AllowHostAccess = false
	cfg.Directories.WorkspaceDir = filepath.Join(root, "ws")
	if got := denial("run", stateBinds[0]); got != "" {
		t.Fatalf("with a workspace (validateDockerBindMount confines binds) the new check ran: %s", got)
	}
}

func TestDispatchDockerRunRefusesAuraGoStateBindsWithoutAWorkspace(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	cfg.Directories.DataDir = filepath.Join(root, "data")
	useRuntimePermissionsForTest(t, cfg)
	bind := dockerutil.NormalizeHostPathForBind(filepath.Join(root, "data")) + ":/loot"
	output, _ := dispatchServices(context.Background(), ToolCall{Action: "docker", Operation: "run", Name: "thief", Image: "alpine:latest", Volumes: []string{bind}}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !strings.Contains(output, `"code":"docker_protected_path_denied"`) {
		t.Fatalf("dispatch output = %s, want the state-bind denial before Docker", output)
	}
}
