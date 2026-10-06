package agent

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

type dockerComposeResolverCall struct {
	file        string
	allProfiles bool
}

// stubDockerComposeResolverModes replaces both Compose resolutions (the default
// validity gate and the all-profiles ownership model) and records every call.
func stubDockerComposeResolverModes(t *testing.T, resolve func(ctx context.Context, file string, opts tools.DockerComposeConfigOptions) (string, error)) *[]dockerComposeResolverCall {
	t.Helper()
	var calls []dockerComposeResolverCall
	original := resolveDockerComposeConfig
	t.Cleanup(func() { resolveDockerComposeConfig = original })
	resolveDockerComposeConfig = func(ctx context.Context, _ tools.DockerConfig, file string, opts tools.DockerComposeConfigOptions) (string, error) {
		calls = append(calls, dockerComposeResolverCall{file: file, allProfiles: opts.AllProfiles})
		return resolve(ctx, file, opts)
	}
	return &calls
}

// stubDockerComposeResolver answers both resolutions with the same model and
// records the default (validity-gate) resolutions only.
func stubDockerComposeResolver(t *testing.T, resolve func(file string) (string, error)) *[]string {
	t.Helper()
	var seen []string
	stubDockerComposeResolverModes(t, func(_ context.Context, file string, opts tools.DockerComposeConfigOptions) (string, error) {
		if !opts.AllProfiles {
			seen = append(seen, file)
		}
		return resolve(file)
	})
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

func TestDockerComposePolicyWithoutWorkspaceJailsToWorkingDirectory(t *testing.T) {
	// Before the shared preflight, an empty workspace made the Compose checks
	// jail to the process working directory: files inside it ran, files outside
	// it were blocked. The outside file exists, so only the jail can block it.
	root := t.TempDir()
	workdir := filepath.Join(root, "work")
	inside := writeComposeFixture(t, workdir, "stack/compose.yml", "services:\n  web:\n    image: alpine\n")
	outside := writeComposeFixture(t, root, "outside.yml", "services:\n  web:\n    image: alpine\n")
	t.Chdir(workdir)
	seen := stubDockerComposeResolver(t, func(string) (string, error) { return `{"services":{"web":{"image":"alpine"}}}`, nil })

	for _, file := range []string{"stack/compose.yml", inside} {
		if got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{}, dockerArgs{Operation: "compose", File: file, Command: "ps"}); got != "" {
			t.Fatalf("%s: compose file inside the working directory was blocked: %s", file, got)
		}
	}
	allowedCalls := len(*seen)
	workdirText := strings.ReplaceAll(workdir, `\`, `\\`) // as it appears inside the JSON envelope
	for _, file := range []string{filepath.Join("..", "outside.yml"), outside} {
		got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{}, dockerArgs{Operation: "compose", File: file, Command: "ps"})
		if !strings.Contains(got, `"code":"docker_compose_file_outside_workspace"`) || !strings.Contains(got, "working directory") || !strings.Contains(got, workdirText) {
			t.Fatalf("%s: compose file outside the working directory was not jailed: %s", file, got)
		}
	}
	if len(*seen) != allowedCalls {
		t.Fatalf("resolver ran for a file outside the working directory: %q", *seen)
	}
}

const profileComposeFixture = "services:\n  web:\n    image: alpine\n  hidden:\n    image: alpine\n    profiles: [later]\n    container_name: aurago-${G}\n"

func TestDockerComposePolicyChecksProfileServicesOnTheAllProfilesModel(t *testing.T) {
	// `docker compose config` omits services of inactive profiles, but
	// `up -d hidden` activates them. The all-profiles model must see them.
	workspace := t.TempDir()
	file := writeComposeFixture(t, workspace, "prof/compose.yml", profileComposeFixture)
	calls := stubDockerComposeResolverModes(t, func(_ context.Context, _ string, opts tools.DockerComposeConfigOptions) (string, error) {
		if opts.AllProfiles {
			return `{"services":{"web":{"image":"alpine"},"hidden":{"image":"alpine","profiles":["later"],"container_name":"aurago-boring-garage"}}}`, nil
		}
		return `{"services":{"web":{"image":"alpine"}}}`, nil
	})
	dockerCfg := tools.DockerConfig{WorkspaceDir: workspace}
	for _, command := range []string{"up -d hidden", "create hidden", "up -d -- hidden", "up -dt 5 hidden"} {
		got := dockerComposePolicy(context.Background(), &config.Config{}, dockerCfg, dockerArgs{Operation: "compose", File: "prof/compose.yml", Command: command})
		if !strings.Contains(got, "Boring Computers Garage") {
			t.Fatalf("%s: profile-gated garage container was not blocked: %s", command, got)
		}
	}
	// Commands that do not name the hidden service never activate its profile.
	for _, command := range []string{"ps", "up -d", "up -d web", "start hidden", "logs hidden"} {
		if got := dockerComposePolicy(context.Background(), &config.Config{}, dockerCfg, dockerArgs{Operation: "compose", File: "prof/compose.yml", Command: command}); got != "" {
			t.Fatalf("%s: an unused profile service blocked the call: %s", command, got)
		}
	}
	var gate, allProfiles int
	for _, call := range *calls {
		if call.file != file {
			t.Fatalf("resolver got %q, want the jailed file %q", call.file, file)
		}
		if call.allProfiles {
			allProfiles++
		} else {
			gate++
		}
	}
	if gate != 9 || allProfiles != 9 {
		t.Fatalf("resolver calls: %d default, %d all-profiles; want one of each per policy call", gate, allProfiles)
	}
}

func TestDockerComposePolicyIgnoresUnusedProfileServices(t *testing.T) {
	// prof4: an unused debug profile joins AuraGo's app container namespace.
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n  debug:\n    image: alpine\n    profiles: [debug]\n    network_mode: \"container:aurago\"\n")
	stubDockerComposeResolverModes(t, func(_ context.Context, _ string, opts tools.DockerComposeConfigOptions) (string, error) {
		if opts.AllProfiles {
			return `{"services":{"web":{"image":"alpine"},"debug":{"image":"alpine","profiles":["debug"],"network_mode":"container:aurago"}}}`, nil
		}
		return `{"services":{"web":{"image":"alpine"}}}`, nil
	})
	policy := func(command string) string {
		return dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: command})
	}
	for _, command := range []string{"ps", "up -d web", "up -d", "up -d --scale debug=2 web", "up -d --exit-code-from debug web", "down"} {
		if got := policy(command); got != "" {
			t.Fatalf("%s: unused profile service blocked the call: %s", command, got)
		}
	}
	for _, command := range []string{"up -d debug", "create debug", "up -d web -- debug"} {
		if got := policy(command); !strings.Contains(got, `"code":"docker_managed_aurago_resource"`) {
			t.Fatalf("%s: named profile service in the app container namespace was not blocked: %s", command, got)
		}
	}
}

func TestDockerComposePolicyFollowsDependenciesOfNamedProfileServices(t *testing.T) {
	// `up -d a` also starts a's same-profile dependency f (verified with --dry-run).
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n  a:\n    image: alpine\n    profiles: [p]\n    depends_on: [f]\n  f:\n    image: alpine\n    profiles: [p]\n    container_name: aurago-${G}\n")
	stubDockerComposeResolverModes(t, func(_ context.Context, _ string, opts tools.DockerComposeConfigOptions) (string, error) {
		if opts.AllProfiles {
			return `{"services":{"web":{"image":"alpine"},"a":{"image":"alpine","profiles":["p"],"depends_on":{"f":{"condition":"service_started","required":true}}},"f":{"image":"alpine","profiles":["p"],"container_name":"aurago-boring-garage"}}}`, nil
		}
		return `{"services":{"web":{"image":"alpine"}}}`, nil
	})
	policy := func(command string) string {
		return dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: command})
	}
	if got := policy("up -d a"); !strings.Contains(got, "Boring Computers Garage") {
		t.Fatalf("dependency of a named profile service was not checked: %s", got)
	}
	if got := policy("up -d web"); got != "" {
		t.Fatalf("unused profile services blocked up -d web: %s", got)
	}
}

func TestDockerComposeEffectiveModelHoldsWhatTheCommandRuns(t *testing.T) {
	// prof5: an unused privileged profile service must not appear in the model
	// later policies evaluate for `up -d`.
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n  tool:\n    image: alpine\n    profiles: [tools]\n    privileged: true\n")
	stubDockerComposeResolverModes(t, func(_ context.Context, _ string, opts tools.DockerComposeConfigOptions) (string, error) {
		if opts.AllProfiles {
			return `{"name":"p5","services":{
				"web":{"image":"alpine","networks":{"default":null}},
				"tool":{"image":"alpine","profiles":["tools"],"privileged":true,
					"volumes":[{"type":"volume","source":"toolvol","target":"/t"}],
					"networks":{"toolnet":null},"secrets":[{"source":"toolsecret","target":"/run/secrets/toolsecret"}]},
				"other":{"image":"alpine","profiles":["other"],"volumes":[{"type":"volume","source":"othervol","target":"/o"}]}},
				"volumes":{"toolvol":{"name":"p5_toolvol"},"othervol":{"name":"p5_othervol"}},
				"networks":{"default":{"name":"p5_default"},"toolnet":{"name":"p5_toolnet"}},
				"secrets":{"toolsecret":{"name":"p5_toolsecret","file":"/ws/secret.txt"}}}`, nil
		}
		return `{"name":"p5","services":{"web":{"image":"alpine","networks":{"default":null}}},"networks":{"default":{"name":"p5_default"}}}`, nil
	})
	preflight, err := loadDockerComposePreflight(context.Background(), tools.DockerConfig{WorkspaceDir: workspace}, "compose.yml")
	if err != nil {
		t.Fatal(err)
	}

	plain := preflight.effectiveModel("up -d")
	if len(plain.model.Services) != 1 || len(plain.profileServices) != 0 || len(plain.unverified) != 0 {
		t.Fatalf("up -d effective model = %+v", plain)
	}
	if _, ok := plain.model.Services["tool"]; ok {
		t.Fatal("unused privileged profile service is in the effective model of up -d")
	}
	if len(plain.model.Volumes) != 0 || len(plain.model.Secrets) != 0 || len(plain.model.Networks) != 1 {
		t.Fatalf("up -d top-level resources = %+v %+v %+v", plain.model.Volumes, plain.model.Networks, plain.model.Secrets)
	}

	named := preflight.effectiveModel("up -d tool")
	if tool, ok := named.model.Services["tool"]; !ok || !tool.Privileged || len(named.model.Services) != 2 {
		t.Fatalf("up -d tool services = %+v", named.model.Services)
	}
	if strings.Join(named.profileServices, ",") != "tool" {
		t.Fatalf("profileServices = %q", named.profileServices)
	}
	if _, ok := named.model.Volumes["toolvol"]; !ok {
		t.Fatalf("volume of the named service missing: %+v", named.model.Volumes)
	}
	if _, ok := named.model.Volumes["othervol"]; ok {
		t.Fatal("volume of an unused profile service is in the effective model")
	}
	if _, ok := named.model.Networks["toolnet"]; !ok || named.model.Secrets["toolsecret"].File != "/ws/secret.txt" {
		t.Fatalf("network or secret of the named service missing: %+v %+v", named.model.Networks, named.model.Secrets)
	}
}

func TestDockerComposePolicyLogsTheAllProfilesFailure(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", profileComposeFixture)
	stubDockerComposeResolverModes(t, func(_ context.Context, _ string, opts tools.DockerComposeConfigOptions) (string, error) {
		if opts.AllProfiles {
			return "", errors.New("resolve Compose config: exit status 1: unknown flag: --no-env-resolution")
		}
		return `{"services":{"web":{"image":"alpine"}}}`, nil
	})
	var logged strings.Builder
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })

	got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: "up -d hidden"})
	if !strings.Contains(got, `"code":"docker_compose_profile_service_unverified"`) || !strings.Contains(got, "2.35") {
		t.Fatalf("unverified message = %s, want the code and the minimum Compose version", got)
	}
	if strings.Contains(got, "no-env-resolution") {
		t.Fatalf("agent message carries the Compose error: %s", got)
	}
	if !strings.Contains(logged.String(), "unknown flag: --no-env-resolution") {
		t.Fatalf("server log lacks the all-profiles failure: %q", logged.String())
	}
}

func TestDockerComposePolicyBlocksProfileServiceTokensInTheAllProfilesText(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "include:\n  - inc.yml\nservices:\n  web:\n    image: alpine\n")
	stubDockerComposeResolverModes(t, func(_ context.Context, _ string, opts tools.DockerComposeConfigOptions) (string, error) {
		if opts.AllProfiles {
			return `{"services":{"web":{"image":"alpine"},"tool":{"image":"alpine","profiles":["ops"],"labels":{"com.aurago.owner":"local-llm"}}}}`, nil
		}
		return `{"services":{"web":{"image":"alpine"}}}`, nil
	})
	got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: "up -d tool"})
	if !strings.Contains(got, "managed local LLM volumes") {
		t.Fatalf("local LLM token of a profile service was not blocked: %s", got)
	}
}

func TestDockerComposePolicyAllowsHarmlessProfileServices(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n  cache:\n    image: redis:7\n    profiles: [debug]\n    container_name: dev-cache\n")
	stubDockerComposeResolverModes(t, func(_ context.Context, _ string, opts tools.DockerComposeConfigOptions) (string, error) {
		if opts.AllProfiles {
			return `{"services":{"web":{"image":"alpine"},"cache":{"image":"redis:7","profiles":["debug"],"container_name":"dev-cache"}}}`, nil
		}
		return `{"services":{"web":{"image":"alpine"}}}`, nil
	})
	for _, command := range []string{"up -d cache", "up -d", "create cache", "ps"} {
		got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: command})
		if got != "" {
			t.Fatalf("%s: harmless profile service was blocked: %s", command, got)
		}
	}
}

func TestDockerComposePolicyFallsBackWhenComposeCannotResolveAllProfiles(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", profileComposeFixture)
	defaultModel := `{"services":{"web":{"image":"alpine"}}}`
	stubDockerComposeResolverModes(t, func(_ context.Context, _ string, opts tools.DockerComposeConfigOptions) (string, error) {
		if opts.AllProfiles {
			return "", errors.New("resolve Compose config: exit status 1: unknown flag: --no-env-resolution")
		}
		return defaultModel, nil
	})
	dockerCfg := tools.DockerConfig{WorkspaceDir: workspace}
	policy := func(command string) string {
		return dockerComposePolicy(context.Background(), &config.Config{}, dockerCfg, dockerArgs{Operation: "compose", File: "compose.yml", Command: command})
	}
	for _, command := range []string{"up -d hidden", "up --timeout 10 -d hidden", "create hidden", "up -d web hidden", "up -dt 5 hidden", "up -d -- hidden"} {
		got := policy(command)
		if !strings.Contains(got, `"code":"docker_compose_profile_service_unverified"`) || !strings.Contains(got, "hidden") {
			t.Fatalf("%s: got %s, want docker_compose_profile_service_unverified naming the service", command, got)
		}
		if classifyLegacyToolResult(got) != ToolResultFailed {
			t.Fatalf("%s: classified as %v", command, classifyLegacyToolResult(got))
		}
	}
	// start and restart never create containers; -dt takes its value from the
	// next argument; --scale and --exit-code-from name services without starting them.
	for _, command := range []string{"up -d", "up -d web", "up -d --timeout 10 web", "up -dt 5 web", "up -t5 web", "up --scale web=2 -d", "up -d --scale hidden=0 web", "up --exit-code-from hidden web", "create --pull always web", "start hidden", "restart -t 5 hidden", "ps hidden", "logs hidden", "stop hidden", "down", "config"} {
		if got := policy(command); got != "" {
			t.Fatalf("%s: fallback blocked a call that names only default services: %s", command, got)
		}
	}

	defaultModel = `{"services":{"web":{"image":"alpine","container_name":"aurago"}}}`
	if got := policy("ps"); !strings.Contains(got, `"code":"docker_managed_aurago_resource"`) {
		t.Fatalf("fallback skipped the default-model ownership check: %s", got)
	}
}

func TestDockerComposePolicyRunsComposeUnderTheDispatchContext(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n")
	type dispatchKey struct{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), dispatchKey{}, "dispatch"))
	defer cancel()
	var values []any
	stubDockerComposeResolverModes(t, func(ctx context.Context, _ string, _ tools.DockerComposeConfigOptions) (string, error) {
		values = append(values, ctx.Value(dispatchKey{}))
		return `{"services":{"web":{"image":"alpine"}}}`, nil
	})
	req := dockerArgs{Operation: "compose", File: "compose.yml", Command: "ps"}
	if got := dockerComposePolicy(ctx, &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, req); got != "" {
		t.Fatalf("dockerComposePolicy() = %s", got)
	}
	if len(values) != 2 || values[0] != "dispatch" || values[1] != "dispatch" {
		t.Fatalf("resolver contexts carried %v, want the dispatch context twice", values)
	}
	cancel()
	if got := dockerComposePolicy(ctx, &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, req); !strings.Contains(got, `"code":"docker_compose_preflight_failed"`) || !strings.Contains(got, "context canceled") {
		t.Fatalf("cancelled dispatch did not stop the preflight: %s", got)
	}
}

func TestDockerComposePolicyKeepsTheEndOfLongPreflightErrors(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n")
	stubDockerComposeResolver(t, func(string) (string, error) {
		return "", errors.New("resolve Compose config: exit status 1: " + strings.Repeat("context ", 200) + "env file missing.env not found")
	})
	got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: "ps"})
	if !strings.Contains(got, `"code":"docker_compose_preflight_failed"`) || !strings.Contains(got, "env file missing.env not found") {
		t.Fatalf("long preflight error lost its final error: %s", got)
	}
	if len(got) > 1200 {
		t.Fatalf("preflight error is not bounded: %d bytes", len(got))
	}
}

func TestDockerComposePolicyReportsPreflightFailure(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n    env_file: [missing.env]\n")
	stubDockerComposeResolver(t, func(string) (string, error) {
		return "", errors.New("resolve Compose config: exit status 1: env file missing.env not found")
	})
	dockerCfg := tools.DockerConfig{WorkspaceDir: workspace}
	for _, file := range []string{"compose.yml", "missing.yml"} {
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

func TestDockerComposePolicyReportsFilesOutsideTheWorkspace(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n")
	outside := writeComposeFixture(t, root, "outside.yml", "services:\n  web:\n    image: alpine\n")
	seen := stubDockerComposeResolver(t, func(string) (string, error) { return `{"services":{"web":{"image":"alpine"}}}`, nil })
	dockerCfg := tools.DockerConfig{WorkspaceDir: workspace}
	for _, file := range []string{filepath.Join("..", "outside.yml"), outside} {
		got := dockerComposePolicy(context.Background(), &config.Config{}, dockerCfg, dockerArgs{Operation: "compose", File: file, Command: "ps"})
		if !strings.Contains(got, `"code":"docker_compose_file_outside_workspace"`) || !strings.Contains(got, "agent workspace") {
			t.Fatalf("%s: got %s, want docker_compose_file_outside_workspace", file, got)
		}
		if strings.Contains(got, "install the Docker Compose plugin") {
			t.Fatalf("%s: jail violation suggests installing Compose: %s", file, got)
		}
		if classifyLegacyToolResult(got) != ToolResultFailed {
			t.Fatalf("%s: classified as %v", file, classifyLegacyToolResult(got))
		}
	}

	if len(*seen) != 0 {
		t.Fatalf("resolver ran for a file outside the workspace: %q", *seen)
	}
}

func TestDockerComposePolicyRejectsSymlinksOutOfTheWorkspace(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := writeComposeFixture(t, root, "outside.yml", "services:\n  web:\n    image: alpine\n")
	if err := os.Symlink(outside, filepath.Join(workspace, "linked.yml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	seen := stubDockerComposeResolver(t, func(string) (string, error) { return `{"services":{"web":{"image":"alpine"}}}`, nil })
	got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "linked.yml", Command: "ps"})
	if !strings.Contains(got, `"code":"docker_compose_file_outside_workspace"`) {
		t.Fatalf("symlink to a file outside the workspace: got %s", got)
	}
	if len(*seen) != 0 {
		t.Fatalf("resolver ran for a symlink out of the workspace: %q", *seen)
	}
}

func TestDockerComposePolicyFollowsSymlinksInsideTheWorkspace(t *testing.T) {
	workspace := t.TempDir()
	target := writeComposeFixture(t, workspace, "real/compose.yml", "services:\n  web:\n    image: alpine\n")
	if err := os.Symlink(target, filepath.Join(workspace, "compose.yml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	stubDockerComposeResolver(t, func(string) (string, error) { return `{"services":{"web":{"image":"alpine"}}}`, nil })
	if got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: "ps"}); got != "" {
		t.Fatalf("symlinked compose file inside the workspace was blocked: %s", got)
	}
}

func TestDockerComposePolicyReadsOnlyBoundedRegularFiles(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "dir.yml"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeComposeFixture(t, workspace, "huge.yml", "services:\n  web:\n    image: alpine\n"+strings.Repeat("# padding\n", (4<<20)/10+1))
	seen := stubDockerComposeResolver(t, func(string) (string, error) { return `{"services":{"web":{"image":"alpine"}}}`, nil })
	dockerCfg := tools.DockerConfig{WorkspaceDir: workspace}
	for file, want := range map[string]string{"dir.yml": "not a regular file", "huge.yml": "larger than 4 MiB"} {
		got := dockerComposePolicy(context.Background(), &config.Config{}, dockerCfg, dockerArgs{Operation: "compose", File: file, Command: "ps"})
		if !strings.Contains(got, `"code":"docker_compose_preflight_failed"`) || !strings.Contains(got, want) {
			t.Fatalf("%s: got %s, want docker_compose_preflight_failed with %q", file, got, want)
		}
	}
	if len(*seen) != 0 {
		t.Fatalf("resolver ran for an unreadable compose file: %q", *seen)
	}
}

func TestDockerComposeStartedServiceNamesSkipConfigOutputTargets(t *testing.T) {
	// Every -o/--output form that tools.DockerCompose accepts must be parsed so
	// that the target is never a service name and the services after it are
	// still named, whatever the target ends with (-ologo ends in the flag
	// letter itself).
	for command, want := range map[string][]string{
		"config -o rendered/stack.yml":                 nil,
		"config -o rendered/stack.yml hidden":          {"hidden"},
		"config --output rendered/stack.yml hidden":    {"hidden"},
		"config --output=rendered/stack.yml hidden":    {"hidden"},
		"config -o=rendered/stack.yml hidden":          {"hidden"},
		"config -orendered/stack.yml hidden":           {"hidden"},
		"config -ologo hidden":                         {"hidden"},
		"config -qologo hidden":                        {"hidden"},
		"config -qo rendered/stack.yml hidden":         {"hidden"},
		"config --format json -o out.yml hidden other": {"hidden", "other"},
		"convert -o out.yml hidden":                    {"hidden"},
		"config -q -o out.yml -- hidden":               {"hidden"},
		"up -d -t5 hidden":                             {"hidden"},
		"up -dt 5 hidden":                              {"hidden"},
		"build -qm 1g hidden":                          {"hidden"},
	} {
		got := dockerComposeStartedServiceNames(command)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%q: service names = %q, want %q", command, got, want)
		}
	}
}

func TestDockerComposePolicyChecksServicesNamedAfterConfigOutputTargets(t *testing.T) {
	workspace := t.TempDir()
	file := writeComposeFixture(t, workspace, "prof/compose.yml", profileComposeFixture)
	stubDockerComposeResolverByMode(t, `{"services":{"web":{"image":"alpine"}}}`,
		`{"services":{"web":{"image":"alpine"},"hidden":{"image":"alpine","profiles":["later"],"container_name":"aurago-boring-garage"}}}`)
	dockerCfg := tools.DockerConfig{WorkspaceDir: workspace}
	for _, command := range []string{
		"config -o rendered/stack.yml hidden",
		"config --output=rendered/stack.yml hidden",
		"config -qo rendered/stack.yml hidden",
		"config -ologo hidden",
		"convert -o=rendered/stack.yml hidden",
	} {
		got := dockerComposePolicy(context.Background(), &config.Config{}, dockerCfg, dockerArgs{Operation: "compose", File: file, Command: command})
		if !strings.Contains(got, "Boring Computers Garage") {
			t.Fatalf("%s: profile-gated garage container was not checked: %s", command, got)
		}
	}
	// The output target is not a service: it must not be looked up as one.
	for _, command := range []string{"config -o rendered/stack.yml", "config --output=rendered/stack.yml web"} {
		if got := dockerComposePolicy(context.Background(), &config.Config{}, dockerCfg, dockerArgs{Operation: "compose", File: file, Command: command}); got != "" {
			t.Fatalf("%s: blocked: %s", command, got)
		}
	}
}
