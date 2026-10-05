package agent

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

const traefikComposeModel = `{"name":"traefik","services":{"traefik":{"image":"traefik:v3.1","volumes":[{"type":"bind","source":"/var/run/docker.sock","target":"/var/run/docker.sock","read_only":true}]}}}`

// stubDockerComposeResolverByMode answers the default and the all-profiles
// resolution with separate models; an empty all-profiles model simulates a
// Compose without `--profile * --no-env-resolution`.
func stubDockerComposeResolverByMode(t *testing.T, defaultModel, allProfilesModel string) {
	t.Helper()
	stubDockerComposeResolverModes(t, func(_ context.Context, _ string, opts tools.DockerComposeConfigOptions) (string, error) {
		if opts.AllProfiles {
			if allProfilesModel == "" {
				return "", errors.New("resolve Compose config: exit status 16: unknown flag: --no-env-resolution")
			}
			return allProfilesModel, nil
		}
		return defaultModel, nil
	})
}

func jsonPath(t *testing.T, path string) string {
	t.Helper()
	data, err := json.Marshal(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestDockerComposePolicyHostAccessAppliesOnlyToUpCreateBuild(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "traefik/compose.yml", "services:\n  traefik:\n    image: traefik:v3.1\n    volumes:\n      - /var/run/docker.sock:/var/run/docker.sock:ro\n")
	stubDockerComposeResolver(t, func(string) (string, error) { return traefikComposeModel, nil })
	cfg := &config.Config{}
	useRuntimePermissionsForTest(t, cfg)
	dockerCfg := tools.DockerConfig{WorkspaceDir: workspace}
	call := func(command string) string {
		return dockerComposePolicy(context.Background(), cfg, dockerCfg, dockerArgs{Operation: "compose", File: "traefik/compose.yml", Command: command})
	}
	for _, command := range []string{"up -d", "create"} {
		got := call(command)
		if !strings.Contains(got, `"code":"docker_compose_host_access_denied"`) || !strings.Contains(got, "/var/run/docker.sock") {
			t.Fatalf("%s: got %s, want docker_compose_host_access_denied", command, got)
		}
		if !strings.Contains(got, `"status":"policy_denied"`) || classifyLegacyToolResult(got) != ToolResultDenied {
			t.Fatalf("%s: denial classified as %v: %s", command, classifyLegacyToolResult(got), got)
		}
	}
	// Lifecycle and read-only commands are never subject to the host-access
	// policy; build checks build sections only and this stack has none; config
	// checks only AuraGo state files.
	for _, command := range []string{"down", "stop", "start", "restart", "rm -f", "kill", "pause", "unpause", "pull", "ps", "logs --tail 50", "config", "images", "top", "port traefik 80", "ls", "events", "version", "build"} {
		if got := call(command); got != "" {
			t.Fatalf("%s was blocked: %s", command, got)
		}
	}
	cfg.Docker.AllowHostAccess = true
	if got := call("up -d"); got != "" {
		t.Fatalf("up -d blocked with docker.allow_host_access: %s", got)
	}
}

func TestDockerComposePolicyHostAccessFollowsRunPermissions(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "traefik/compose.yml", "services:\n  traefik:\n    image: traefik:v3.1\n")
	stubDockerComposeResolver(t, func(string) (string, error) { return traefikComposeModel, nil })
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.AllowHostAccess = true
	useRuntimePermissionsForTest(t, cfg)
	req := dockerArgs{Operation: "compose", File: "traefik/compose.yml", Command: "up -d"}
	narrowed := tools.WithRuntimePermissions(context.Background(), tools.RuntimePermissions{DockerEnabled: true})
	if got := dockerComposePolicy(narrowed, cfg, tools.DockerConfig{WorkspaceDir: workspace}, req); !strings.Contains(got, "docker_compose_host_access_denied") {
		t.Fatalf("run without the runtime grant was allowed: %s", got)
	}
	granted := tools.WithRuntimePermissions(context.Background(), tools.RuntimePermissions{DockerEnabled: true, AllowDockerHostAccess: true})
	if got := dockerComposePolicy(granted, cfg, tools.DockerConfig{WorkspaceDir: workspace}, req); got != "" {
		t.Fatalf("granted run was blocked: %s", got)
	}
	// The run's config can narrow but never widen the server grant.
	cfg.Docker.AllowHostAccess = false
	if got := dockerComposePolicy(granted, cfg, tools.DockerConfig{WorkspaceDir: workspace}, req); !strings.Contains(got, "docker_compose_host_access_denied") {
		t.Fatalf("server snapshot without host access was widened by the run: %s", got)
	}
}

func TestDockerComposePolicyAlwaysRejectsAuraGoStateWithHostAccess(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "agent_workspace", "workdir")
	writeComposeFixture(t, workspace, "leak/compose.yml", "services:\n  leak:\n    image: alpine\n    volumes:\n      - ../../../data:/state:ro\n")
	dataDir := filepath.Join(root, "data")
	resolved, _ := json.Marshal(map[string]any{"services": map[string]any{"leak": map[string]any{"image": "alpine",
		"volumes": []map[string]any{{"type": "bind", "source": dataDir, "target": "/state", "read_only": true}}}}})
	stubDockerComposeResolver(t, func(string) (string, error) { return string(resolved), nil })
	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	cfg.ConfigPath = filepath.Join(root, "config.yaml")
	cfg.Docker.AllowHostAccess = true
	useRuntimePermissionsForTest(t, cfg)
	for _, command := range []string{"up -d", "create"} {
		got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "leak/compose.yml", Command: command})
		if !strings.Contains(got, `"code":"docker_compose_protected_path_denied"`) {
			t.Fatalf("%s: AuraGo data bind allowed with host access: %s", command, got)
		}
		if classifyLegacyToolResult(got) != ToolResultDenied {
			t.Fatalf("%s: denial classified as %v", command, classifyLegacyToolResult(got))
		}
	}
	// Binds are irrelevant to config and lifecycle commands.
	for _, command := range []string{"config", "down", "ps"} {
		if got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "leak/compose.yml", Command: command}); got != "" {
			t.Fatalf("%s: blocked by a bind: %s", command, got)
		}
	}
}

func TestDockerComposePolicyConfigRejectsAuraGoEnvFiles(t *testing.T) {
	// `docker compose config` prints env_file content inlined into environment.
	root := t.TempDir()
	workspace := filepath.Join(root, "agent_workspace", "workdir")
	writeComposeFixture(t, workspace, "leak/compose.yml", "services:\n  leak:\n    image: alpine\n    env_file: [../../../.env]\n")
	envFile := filepath.Join(root, ".env")
	stubDockerComposeResolverByMode(t,
		`{"services":{"leak":{"image":"alpine","environment":{"OTHER":"x"}}}}`,
		`{"services":{"leak":{"image":"alpine","env_file":[{"path":`+jsonPath(t, envFile)+`}]}}}`)
	for _, allow := range []bool{false, true} {
		cfg := &config.Config{}
		cfg.Directories.DataDir = filepath.Join(root, "data")
		cfg.ConfigPath = filepath.Join(root, "config.yaml")
		cfg.Docker.AllowHostAccess = allow
		useRuntimePermissionsForTest(t, cfg)
		for _, command := range []string{"config", "convert", "config --format json", "up -d", "create"} {
			got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "leak/compose.yml", Command: command})
			if !strings.Contains(got, `"code":"docker_compose_protected_path_denied"`) || !strings.Contains(got, "env_file") {
				t.Fatalf("allow=%v %s: AuraGo .env as env_file allowed: %s", allow, command, got)
			}
		}
		for _, command := range []string{"ps", "down", "logs", "restart", "build"} {
			if got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "leak/compose.yml", Command: command}); got != "" {
				t.Fatalf("allow=%v %s: blocked by an env_file: %s", allow, command, got)
			}
		}
	}
}

func TestDockerComposePolicyConfigAllowsHostAttributes(t *testing.T) {
	// config/convert never apply the host-access tier: a new install can still
	// inspect a Traefik stack it may not start.
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "traefik/compose.yml", "services:\n  traefik:\n    image: traefik:v3.1\n    env_file: [traefik.env]\n")
	stubDockerComposeResolverByMode(t, traefikComposeModel,
		`{"services":{"traefik":{"image":"traefik:v3.1","env_file":[{"path":"/srv/env/traefik.env"}],"volumes":[{"type":"bind","source":"/var/run/docker.sock","target":"/var/run/docker.sock"}]}}}`)
	cfg := &config.Config{}
	useRuntimePermissionsForTest(t, cfg)
	for _, command := range []string{"config", "convert", "config --services"} {
		if got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "traefik/compose.yml", Command: command}); got != "" {
			t.Fatalf("%s blocked a host stack without host access: %s", command, got)
		}
	}
}

func TestDockerComposePolicyRejectsTheMasterKeyValue(t *testing.T) {
	workspace := t.TempDir()
	masterKey := strings.Repeat("c4", 32)
	writeComposeFixture(t, workspace, "mk/compose.yml", "services:\n  app:\n    image: alpine\n    env_file: [copy.env]\n")
	// The env file holds a copy of the key: only the default resolution, which
	// inlines env_file into environment, carries the value.
	stubDockerComposeResolverByMode(t,
		`{"services":{"app":{"image":"alpine","environment":{"K":"`+strings.ToUpper(masterKey)+`"}}}}`,
		`{"services":{"app":{"image":"alpine","env_file":[{"path":`+jsonPath(t, filepath.Join(workspace, "mk", "copy.env"))+`}]}}}`)
	cfg := &config.Config{}
	cfg.Server.MasterKey = masterKey
	cfg.Docker.AllowHostAccess = true
	useRuntimePermissionsForTest(t, cfg)
	for _, command := range []string{"up -d", "create", "config", "convert"} {
		got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "mk/compose.yml", Command: command})
		if !strings.Contains(got, `"code":"docker_compose_protected_path_denied"`) || strings.Contains(strings.ToLower(got), masterKey) {
			t.Fatalf("%s: master key value allowed or echoed: %s", command, got)
		}
	}
	// build reads only build sections; environment never reaches an image.
	for _, command := range []string{"ps", "down", "logs", "build"} {
		if got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "mk/compose.yml", Command: command}); got != "" {
			t.Fatalf("%s: blocked by the master key check: %s", command, got)
		}
	}
}

func TestDockerComposePolicyBuildArgumentsFromTheEnvironment(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "app/compose.yml", "services:\n  app:\n    build: .\n")
	model := `{"services":{"app":{"build":{"context":` + jsonPath(t, filepath.Join(workspace, "app")) + `,"dockerfile":"Dockerfile"}}}}`
	stubDockerComposeResolver(t, func(string) (string, error) { return model, nil })
	for _, allow := range []bool{false, true} {
		cfg := &config.Config{}
		cfg.Docker.AllowHostAccess = allow
		useRuntimePermissionsForTest(t, cfg)
		policy := func(command string) string {
			return dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "app/compose.yml", Command: command})
		}
		// --build-arg NAME without a value copies NAME from AuraGo's environment.
		for _, command := range []string{"build --build-arg AURAGO_MASTER_KEY", "build --build-arg=aurago_master_key app"} {
			if got := policy(command); !strings.Contains(got, `"code":"docker_compose_protected_path_denied"`) {
				t.Fatalf("allow=%v %s: master key build arg allowed: %s", allow, command, got)
			}
		}
		if got := policy("build --build-arg VERSION=1 --pull app"); got != "" {
			t.Fatalf("allow=%v: plain build arg blocked: %s", allow, got)
		}
		got := policy("build --ssh default app")
		if allow && got != "" {
			t.Fatalf("build --ssh blocked with host access: %s", got)
		}
		if !allow && !strings.Contains(got, `"code":"docker_compose_host_access_denied"`) {
			t.Fatalf("build --ssh allowed without host access: %s", got)
		}
	}
}

func TestDockerComposePolicyOwnedResourcesStayBlockedWithHostAccess(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "stack/compose.yml", "services:\n  web:\n    image: alpine\n    container_name: aurago-${G}\n")
	stubDockerComposeResolver(t, func(string) (string, error) {
		return `{"services":{"web":{"image":"alpine","container_name":"aurago-boring-garage"}}}`, nil
	})
	cfg := &config.Config{}
	cfg.Docker.AllowHostAccess = true
	useRuntimePermissionsForTest(t, cfg)
	got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "stack/compose.yml", Command: "up -d"})
	if !strings.Contains(got, "Boring Computers Garage") {
		t.Fatalf("Garage name from .env allowed with host access: %s", got)
	}
}

func TestDockerComposePolicyUpAllowsDevAuragoName(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "dev/compose.yml", "services:\n  web:\n    image: alpine\n    container_name: dev-aurago\n")
	stubDockerComposeResolver(t, func(string) (string, error) {
		return `{"services":{"web":{"image":"alpine","container_name":"dev-aurago"}}}`, nil
	})
	for _, allow := range []bool{false, true} {
		cfg := &config.Config{}
		cfg.Docker.AllowHostAccess = allow
		useRuntimePermissionsForTest(t, cfg)
		got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "dev/compose.yml", Command: "up -d"})
		if got != "" {
			t.Fatalf("allow_host_access=%v: container_name dev-aurago was blocked: %s", allow, got)
		}
	}
}

func TestDockerComposePolicyEnvFilesUnknownOnOldCompose(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "app/compose.yml", "services:\n  app:\n    image: alpine\n    env_file: [app.env]\n")
	writeComposeFixture(t, workspace, "plain/compose.yml", "services:\n  app:\n    image: alpine\n")
	stubDockerComposeResolverByMode(t, `{"services":{"app":{"image":"alpine","environment":{"FOO":"bar"}}}}`, "")
	cfg := &config.Config{}
	useRuntimePermissionsForTest(t, cfg)
	dockerCfg := tools.DockerConfig{WorkspaceDir: workspace}
	policy := func(file, command string) string {
		return dockerComposePolicy(context.Background(), cfg, dockerCfg, dockerArgs{Operation: "compose", File: file, Command: command})
	}
	for _, command := range []string{"up -d", "create"} {
		got := policy("app/compose.yml", command)
		if !strings.Contains(got, "cannot list env_file paths") || !strings.Contains(got, `"code":"docker_compose_host_access_denied"`) {
			t.Fatalf("%s: unknown env_file paths accepted without host access: %s", command, got)
		}
		if strings.Contains(got, "no-env-resolution") {
			t.Fatalf("%s: agent message carries the Compose error: %s", command, got)
		}
	}
	// build, config and lifecycle commands do not read env_file paths here.
	for _, command := range []string{"build", "config", "down", "ps"} {
		if got := policy("app/compose.yml", command); got != "" {
			t.Fatalf("%s: blocked on old Compose: %s", command, got)
		}
	}
	if got := policy("plain/compose.yml", "up -d"); got != "" {
		t.Fatalf("stack without env_file blocked: %s", got)
	}
	cfg.Docker.AllowHostAccess = true
	if got := policy("app/compose.yml", "up -d"); got != "" {
		t.Fatalf("env_file stack blocked with host access: %s", got)
	}
}

func TestDockerComposePolicyProfileServiceUnverifiedComesFirst(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n    env_file: [web.env]\n  hidden:\n    image: alpine\n    profiles: [later]\n")
	stubDockerComposeResolverByMode(t, `{"services":{"web":{"image":"alpine","environment":{"A":"b"}}}}`, "")
	cfg := &config.Config{}
	useRuntimePermissionsForTest(t, cfg)
	for _, command := range []string{"up -d hidden", "create hidden", "build hidden", "pull hidden"} {
		got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: command})
		if !strings.Contains(got, `"code":"docker_compose_profile_service_unverified"`) {
			t.Fatalf("%s: got %s, want docker_compose_profile_service_unverified", command, got)
		}
	}
	// config/convert create nothing; old Compose keeps printing profile services.
	for _, command := range []string{"config hidden", "convert hidden"} {
		if got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: command}); got != "" {
			t.Fatalf("%s: blocked on old Compose: %s", command, got)
		}
	}
}

func TestDockerComposePolicyIgnoresUnusedPrivilegedProfiles(t *testing.T) {
	// prof5: an unused privileged profile service never blocks up -d.
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n  tool:\n    image: alpine\n    profiles: [tools]\n    privileged: true\n")
	stubDockerComposeResolverByMode(t, `{"services":{"web":{"image":"alpine"}}}`,
		`{"services":{"web":{"image":"alpine"},"tool":{"image":"alpine","profiles":["tools"],"privileged":true}}}`)
	cfg := &config.Config{}
	useRuntimePermissionsForTest(t, cfg)
	policy := func(command string) string {
		return dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: command})
	}
	for _, command := range []string{"up -d", "up -d web", "create", "build", "pull tool", "config tool", "start tool"} {
		if got := policy(command); got != "" {
			t.Fatalf("%s: unused privileged profile blocked the call: %s", command, got)
		}
	}
	for _, command := range []string{"up -d tool", "create tool", "up -d web -- tool"} {
		if got := policy(command); !strings.Contains(got, `"code":"docker_compose_host_access_denied"`) || !strings.Contains(got, "privileged") {
			t.Fatalf("%s: named privileged profile service allowed without host access: %s", command, got)
		}
	}
	cfg.Docker.AllowHostAccess = true
	if got := policy("up -d tool"); got != "" {
		t.Fatalf("up -d tool blocked with host access: %s", got)
	}
}

func TestDockerComposePolicyBuildAndPullActivateNamedProfileServices(t *testing.T) {
	workspace := t.TempDir()
	file := writeComposeFixture(t, workspace, "prof/compose.yml", profileComposeFixture)
	stubDockerComposeResolverByMode(t, `{"services":{"web":{"image":"alpine"}}}`,
		`{"services":{"web":{"image":"alpine"},"hidden":{"image":"alpine","profiles":["later"],"container_name":"aurago-boring-garage"}}}`)
	dockerCfg := tools.DockerConfig{WorkspaceDir: workspace}
	for _, command := range []string{"build hidden", "build --pull hidden", "build --build-arg A=1 -m 1g hidden", "pull hidden", "pull --policy always hidden", "pull -q -- hidden", "config hidden", "config --format json hidden"} {
		got := dockerComposePolicy(context.Background(), &config.Config{}, dockerCfg, dockerArgs{Operation: "compose", File: file, Command: command})
		if !strings.Contains(got, "Boring Computers Garage") {
			t.Fatalf("%s: profile-gated garage container was not checked: %s", command, got)
		}
	}
	for _, command := range []string{"build", "build web", "pull", "pull --policy missing web", "config", "build --builder hidden web"} {
		if got := dockerComposePolicy(context.Background(), &config.Config{}, dockerCfg, dockerArgs{Operation: "compose", File: file, Command: command}); got != "" {
			t.Fatalf("%s: an unused profile service blocked the call: %s", command, got)
		}
	}
}

func TestDockerComposePolicyFollowsServiceBuildContexts(t *testing.T) {
	// prof14: `up -d a` and `build a` also build b, which a names as the
	// additional build context "service:b"; b is in another inactive profile.
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n  a:\n    profiles: [p1]\n    build:\n      context: ./ctx\n      additional_contexts:\n        base: \"service:b\"\n  b:\n    profiles: [p2]\n    image: ${R}:latest\n    build:\n      context: ./bctx\n")
	ctx := jsonPath(t, filepath.Join(workspace, "ctx"))
	bctx := jsonPath(t, filepath.Join(workspace, "bctx"))
	stubDockerComposeResolverByMode(t, `{"services":{"web":{"image":"alpine"}}}`,
		`{"services":{"web":{"image":"alpine"},"a":{"profiles":["p1"],"build":{"context":`+ctx+`,"dockerfile":"Dockerfile","additional_contexts":{"base":"service:b"}}},"b":{"profiles":["p2"],"image":"aurago-homepage:latest","build":{"context":`+bctx+`,"dockerfile":"Dockerfile"}}}}`)
	for _, command := range []string{"up -d a", "build a", "create a"} {
		got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: command})
		if !strings.Contains(got, `"code":"docker_managed_homepage_resource"`) {
			t.Fatalf("%s: service build context b was not checked: %s", command, got)
		}
	}
	if got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: "up -d"}); got != "" {
		t.Fatalf("up -d blocked by unused profile services: %s", got)
	}
}

func TestDockerComposeEffectiveModelAddsBuildSecretsOfNamedServices(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n")
	stubDockerComposeResolverByMode(t, `{"services":{"web":{"image":"alpine"}}}`,
		`{"services":{"web":{"image":"alpine"},"tool":{"profiles":["t"],"build":{"context":"/ws/tool","secrets":[{"source":"npmrc"}]}}},"secrets":{"npmrc":{"name":"x_npmrc","file":"/srv/secrets/npmrc"},"other":{"name":"x_other","file":"/srv/secrets/other"}}}`)
	preflight, err := loadDockerComposePreflight(context.Background(), tools.DockerConfig{WorkspaceDir: workspace}, "compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	named := preflight.effectiveModel("build tool")
	if !named.fromAllProfiles || named.model.Secrets["npmrc"].File != "/srv/secrets/npmrc" {
		t.Fatalf("build secret of the named service missing: %+v", named)
	}
	if _, ok := named.model.Secrets["other"]; ok {
		t.Fatal("unreferenced secret is in the effective model")
	}
	if plain := preflight.effectiveModel("build"); len(plain.model.Secrets) != 0 || len(plain.model.Services) != 1 {
		t.Fatalf("build effective model = %+v", plain.model)
	}
}

func TestDockerComposePolicyFlagOffAdditions(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  app:\n    image: alpine\n")
	inside := jsonPath(t, filepath.Join(workspace, "src"))
	cases := map[string]struct {
		service string
		extra   string
		field   string
	}{
		"provider":          {service: `{"provider":{"type":"awesomecloud","options":{"type":["mysql"]}}}`, field: "provider"},
		"watch outside":     {service: `{"image":"alpine","develop":{"watch":[{"path":"/srv/src","action":"sync","target":"/app"}]}}`, field: "develop.watch"},
		"build ssh":         {service: `{"image":"alpine","build":{"context":` + inside + `,"ssh":["default"]}}`, field: "build.ssh"},
		"build secret":      {service: `{"image":"alpine","build":{"context":` + inside + `,"secrets":[{"source":"tok"}]}}`, extra: `,"secrets":{"tok":{"name":"x_tok","file":"/srv/secrets/tok"}}`, field: "secrets.tok.file"},
		"build privileged":  {service: `{"image":"alpine","build":{"context":` + inside + `,"privileged":true}}`, field: "build.privileged"},
		"build entitlement": {service: `{"image":"alpine","build":{"context":` + inside + `,"entitlements":["network.host"]}}`, field: "build.entitlements"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			model := `{"services":{"app":` + tc.service + `}` + tc.extra + `}`
			stubDockerComposeResolver(t, func(string) (string, error) { return model, nil })
			cfg := &config.Config{}
			useRuntimePermissionsForTest(t, cfg)
			got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: "up -d"})
			if !strings.Contains(got, `"code":"docker_compose_host_access_denied"`) || !strings.Contains(got, `"field":"`+tc.field+`"`) {
				t.Fatalf("flag off: got %s, want a %s host-access denial", got, tc.field)
			}
			cfg.Docker.AllowHostAccess = true
			if got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: "up -d"}); got != "" {
				t.Fatalf("flag on: %s blocked: %s", tc.field, got)
			}
		})
	}
}

func TestDockerComposePolicyWithoutWorkspaceJailsHostPathsToWorkingDirectory(t *testing.T) {
	// Without a configured workspace the compose file is jailed to the process
	// working directory; the bind check must use that same root, never an empty
	// workspace that would disable it.
	dir := t.TempDir()
	writeComposeFixture(t, dir, "stack/compose.yml", "services:\n  web:\n    image: alpine\n")
	t.Chdir(dir)
	insideModel := `{"services":{"web":{"image":"alpine","volumes":[{"type":"bind","source":` + jsonPath(t, filepath.Join(dir, "stack", "data")) + `,"target":"/data"}]}}}`
	outsideModel := `{"services":{"web":{"image":"alpine","volumes":[{"type":"bind","source":"/srv/media","target":"/media"}]}}}`
	model := insideModel
	stubDockerComposeResolver(t, func(string) (string, error) { return model, nil })
	cfg := &config.Config{}
	useRuntimePermissionsForTest(t, cfg)
	req := dockerArgs{Operation: "compose", File: "stack/compose.yml", Command: "up -d"}
	if got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{}, req); got != "" {
		t.Fatalf("bind inside the working directory blocked: %s", got)
	}
	model = outsideModel
	if got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{}, req); !strings.Contains(got, `"code":"docker_compose_host_access_denied"`) {
		t.Fatalf("bind outside the working directory allowed without a workspace: %s", got)
	}
}

func TestDispatchDockerComposeUpDeniedBeforeDockerCLI(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "traefik/compose.yml", "services:\n  traefik:\n    image: traefik:v3.1\n")
	stubDockerComposeResolver(t, func(string) (string, error) { return traefikComposeModel, nil })
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	cfg.Directories.WorkspaceDir = workspace
	useRuntimePermissionsForTest(t, cfg)
	output, ok := dispatchServices(context.Background(), ToolCall{Action: "docker", Operation: "compose", File: "traefik/compose.yml", Command: "up -d"}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok {
		t.Fatal("expected docker operation to be handled")
	}
	if !strings.Contains(output, `"code":"docker_compose_host_access_denied"`) {
		t.Fatalf("dispatch output = %s, want host-access denial before the CLI", output)
	}
}
