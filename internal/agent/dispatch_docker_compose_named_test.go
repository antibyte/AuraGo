package agent

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// stubDockerComposeResolverNamed answers the default, the all-profiles (""
// simulates Compose < v2.35) and the named resolution separately, and records
// the services of every named resolution.
func stubDockerComposeResolverNamed(t *testing.T, defaultModel, allProfilesModel string, named func(services []string) (string, error)) *[][]string {
	t.Helper()
	var calls [][]string
	stubDockerComposeResolverModes(t, func(_ context.Context, _ string, opts tools.DockerComposeConfigOptions) (string, error) {
		switch {
		case len(opts.Services) > 0:
			calls = append(calls, append([]string(nil), opts.Services...))
			return named(opts.Services)
		case opts.AllProfiles:
			if allProfilesModel == "" {
				return "", errors.New("resolve Compose config: exit status 16: unknown flag: --no-env-resolution")
			}
			return allProfilesModel, nil
		}
		return defaultModel, nil
	})
	return &calls
}

const p1ComposeFile = "services:\n  web:\n    image: alpine\n  hidden:\n    image: alpine\n    profiles: [later]\n    privileged: true\n    depends_on: [helper]\n  helper:\n    image: alpine\n    profiles: [later]\n    volumes:\n      - /srv/helper:/data\n  other:\n    image: alpine\n    profiles: [unrelated]\n"

func TestDockerComposePolicyOldComposeResolvesNamedProfileServices(t *testing.T) {
	// k6rev/p1 on Compose < v2.35: `config -- hidden` returns hidden and its
	// dependency helper; the policy checks them instead of denying.
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", p1ComposeFile)
	hidden := `{"services":{"hidden":{"image":"alpine","profiles":["later"],"privileged":true,"depends_on":{"helper":{"condition":"service_started","required":true}}},"helper":{"image":"alpine","profiles":["later"],"volumes":[{"type":"bind","source":"/srv/helper","target":"/data"}]}}}`
	named := func(services []string) (string, error) {
		switch strings.Join(services, ",") {
		case "hidden":
			return hidden, nil
		case "other":
			return `{"services":{"other":{"image":"alpine","profiles":["unrelated"]}}}`, nil
		}
		return "", errors.New("resolve Compose config: exit status 1: no such service: " + strings.Join(services, ","))
	}
	calls := stubDockerComposeResolverNamed(t, `{"services":{"web":{"image":"alpine"}}}`, "", named)
	cfg := &config.Config{}
	useRuntimePermissionsForTest(t, cfg)
	policy := func(command string) string {
		return dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: command})
	}
	got := policy("up -d hidden")
	if !strings.Contains(got, `"code":"docker_compose_host_access_denied"`) || !strings.Contains(got, `"field":"privileged"`) || !strings.Contains(got, "/srv/helper") {
		t.Fatalf("up -d hidden: got %s, want the host-access check of hidden and helper", got)
	}
	if len(*calls) != 1 || strings.Join((*calls)[0], ",") != "hidden" {
		t.Fatalf("named resolutions = %q", *calls)
	}
	for _, command := range []string{"up -d other", "create other", "build hidden", "pull hidden", "config hidden", "up -d"} {
		if got := policy(command); got != "" {
			t.Fatalf("%s: resolvable profile service blocked: %s", command, got)
		}
	}
	cfg.Docker.AllowHostAccess = true
	if got := policy("up -d hidden"); got != "" {
		t.Fatalf("up -d hidden blocked with host access: %s", got)
	}
	// Only a failed resolution denies: always for up/create; for build and
	// pull (which never read env files) only without host access.
	for _, command := range []string{"up -d missing", "create missing"} {
		if got := policy(command); !strings.Contains(got, `"code":"docker_compose_profile_service_unverified"`) || !strings.Contains(got, "missing") {
			t.Fatalf("%s: got %s, want docker_compose_profile_service_unverified", command, got)
		}
	}
	for _, command := range []string{"build missing", "pull missing", "config missing"} {
		if got := policy(command); got != "" {
			t.Fatalf("%s: unresolvable profile service blocked with host access: %s", command, got)
		}
	}
	cfg.Docker.AllowHostAccess = false
	for _, command := range []string{"up -d missing", "build missing", "pull missing"} {
		if got := policy(command); !strings.Contains(got, `"code":"docker_compose_profile_service_unverified"`) {
			t.Fatalf("%s without host access: got %s, want docker_compose_profile_service_unverified", command, got)
		}
	}
	if got := policy("config missing"); got != "" {
		t.Fatalf("config of an unresolvable service blocked: %s", got)
	}
}

func TestDockerComposePolicyOldComposeChecksOwnershipOfNamedServices(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "prof/compose.yml", profileComposeFixture)
	stubDockerComposeResolverNamed(t, `{"services":{"web":{"image":"alpine"}}}`, "", func([]string) (string, error) {
		return `{"services":{"hidden":{"image":"alpine","profiles":["later"],"container_name":"aurago-boring-garage"}}}`, nil
	})
	cfg := &config.Config{}
	cfg.Docker.AllowHostAccess = true
	useRuntimePermissionsForTest(t, cfg)
	for _, command := range []string{"up -d hidden", "build hidden", "pull hidden", "config hidden"} {
		if got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "prof/compose.yml", Command: command}); !strings.Contains(got, "Boring Computers Garage") {
			t.Fatalf("%s: garage container of a named profile service allowed on old Compose: %s", command, got)
		}
	}
}

func TestDockerComposePolicyOldComposeFollowsServiceBuildContexts(t *testing.T) {
	// k6rev/p3: `config -- a` does not return b, which a builds from; the
	// resolution is repeated with b.
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n  a:\n    profiles: [p1]\n    build:\n      context: ./ctx\n      additional_contexts:\n        base: \"service:b\"\n  b:\n    profiles: [p2]\n    image: ${R}:latest\n    build:\n      context: ./bctx\n")
	ctx := jsonPath(t, filepath.Join(workspace, "ctx"))
	bctx := jsonPath(t, filepath.Join(workspace, "bctx"))
	a := `"a":{"profiles":["p1"],"build":{"context":` + ctx + `,"dockerfile":"Dockerfile","additional_contexts":{"base":"service:b"}}}`
	b := `"b":{"profiles":["p2"],"image":"aurago-homepage:latest","build":{"context":` + bctx + `,"dockerfile":"Dockerfile"}}`
	calls := stubDockerComposeResolverNamed(t, `{"services":{"web":{"image":"alpine"}}}`, "", func(services []string) (string, error) {
		if strings.Join(services, ",") == "a" {
			return `{"services":{` + a + `}}`, nil
		}
		return `{"services":{` + a + `,` + b + `}}`, nil
	})
	got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: "build a"})
	if !strings.Contains(got, `"code":"docker_managed_homepage_resource"`) {
		t.Fatalf("service build context b was not checked on old Compose: %s", got)
	}
	if len(*calls) != 2 || strings.Join((*calls)[1], ",") != "a,b" {
		t.Fatalf("named resolutions = %q, want [a] then [a b]", *calls)
	}
}

func TestDockerComposePolicyChecksTheEnvResolvedTextOfProfileServices(t *testing.T) {
	// k6rev/p4: the all-profiles model keeps env_file as a path; only the
	// named resolution shows the value it brings in.
	workspace := t.TempDir()
	masterKey := strings.Repeat("e7", 32)
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n  hidden:\n    image: alpine\n    profiles: [later]\n    env_file: [copy.env]\n")
	all := `{"services":{"web":{"image":"alpine"},"hidden":{"image":"alpine","profiles":["later"],"env_file":[{"path":` + jsonPath(t, filepath.Join(workspace, "copy.env")) + `}]}}}`
	resolvedHidden := `{"services":{"hidden":{"image":"alpine","profiles":["later"],"environment":{"FROM_FILE":"` + masterKey + `"}}}}`
	var namedErr error
	namedModel := resolvedHidden
	calls := stubDockerComposeResolverNamed(t, `{"services":{"web":{"image":"alpine"}}}`, all, func([]string) (string, error) { return namedModel, namedErr })
	cfg := &config.Config{}
	cfg.Server.MasterKey = masterKey
	cfg.Docker.AllowHostAccess = true
	useRuntimePermissionsForTest(t, cfg)
	policy := func(command string) string {
		return dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: command})
	}
	for _, command := range []string{"up -d hidden", "create hidden", "config hidden"} {
		if got := policy(command); !strings.Contains(got, `"code":"docker_compose_protected_path_denied"`) || strings.Contains(got, masterKey) {
			t.Fatalf("%s: master key from a profile service env file allowed: %s", command, got)
		}
	}
	// build and pull never read env files, so they skip the named resolution.
	before := len(*calls)
	for _, command := range []string{"up -d", "build hidden", "pull hidden"} {
		if got := policy(command); got != "" {
			t.Fatalf("%s: blocked by a profile service env file: %s", command, got)
		}
	}
	if len(*calls) != before {
		t.Fatal("up -d, build or pull ran a named resolution")
	}
	namedModel = `{"services":{"hidden":{"image":"alpine","labels":{"com.aurago.owner":"local-llm"}}}}`
	if got := policy("up -d hidden"); !strings.Contains(got, "managed local LLM volumes") {
		t.Fatalf("local LLM token in the env-resolved text allowed: %s", got)
	}
	namedErr = errors.New("resolve Compose config: exit status 1: env file copy.env not found")
	if got := policy("up -d hidden"); !strings.Contains(got, `"code":"docker_compose_profile_service_unverified"`) {
		t.Fatalf("failed named resolution allowed: %s", got)
	}
}

const p6ComposeFile = "services:\n  web:\n    image: alpine\n  tool:\n    profiles: [tools]\n    image: example.invalid/tool\n    build:\n      context: .\n    env_file: [runtime-secrets.env]\n"

func TestDockerComposePolicyBuildAndPullDoNotNeedProfileEnvFiles(t *testing.T) {
	// k6rev/p6: `build tool` and `pull tool` work without the env file
	// (Compose does not read it), while `config -- tool` fails.
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", p6ComposeFile)
	all := `{"services":{"web":{"image":"alpine"},"tool":{"profiles":["tools"],"image":"example.invalid/tool","build":{"context":` +
		jsonPath(t, workspace) + `,"dockerfile":"Dockerfile"},"env_file":[{"path":` + jsonPath(t, filepath.Join(workspace, "runtime-secrets.env")) + `}]}}}`
	missing := func([]string) (string, error) {
		return "", errors.New("resolve Compose config: exit status 1: env file runtime-secrets.env not found")
	}
	for _, allow := range []bool{false, true} {
		calls := stubDockerComposeResolverNamed(t, `{"services":{"web":{"image":"alpine"}}}`, all, missing)
		cfg := &config.Config{}
		cfg.Docker.AllowHostAccess = allow
		useRuntimePermissionsForTest(t, cfg)
		policy := func(command string) string {
			return dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: command})
		}
		for _, command := range []string{"build tool", "pull tool", "build --pull tool", "config tool"} {
			if got := policy(command); got != "" {
				t.Fatalf("allow=%v %s: blocked by a missing env file Compose does not read: %s", allow, command, got)
			}
		}
		if len(*calls) != 1 {
			t.Fatalf("allow=%v: named resolutions = %q, want only the one for config", allow, *calls)
		}
		for _, command := range []string{"up -d tool", "create tool"} {
			if got := policy(command); !strings.Contains(got, `"code":"docker_compose_profile_service_unverified"`) {
				t.Fatalf("allow=%v %s: got %s, want docker_compose_profile_service_unverified", allow, command, got)
			}
		}
	}
}

func TestDockerComposePolicyChecksHostProgramsOfNamedLifecycleServices(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n")
	all := `{"services":{"web":{"image":"alpine"},` +
		`"hidden":{"image":"alpine","profiles":["later"],"pre_stop":[{"command":["true"],"privileged":true}]},` +
		`"prov":{"profiles":["cloud"],"provider":{"type":"awesomecloud"}},` +
		`"clean":{"image":"alpine","profiles":["later"],"privileged":true}}}`
	calls := stubDockerComposeResolverNamed(t, `{"services":{"web":{"image":"alpine"}}}`, all, func([]string) (string, error) {
		return "", errors.New("named resolution must not run for lifecycle commands")
	})
	cfg := &config.Config{}
	useRuntimePermissionsForTest(t, cfg)
	policy := func(command string) string {
		return dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: command})
	}
	for _, command := range []string{"restart hidden", "down hidden", "rm -s hidden", "stop -t 5 hidden", "start prov", "down --rmi local prov"} {
		if got := policy(command); !strings.Contains(got, `"code":"docker_compose_host_access_denied"`) {
			t.Fatalf("%s: host program of a named profile service allowed without host access: %s", command, got)
		}
	}
	// A privileged container is not a host program; lifecycle commands only
	// check providers and privileged hooks.
	for _, command := range []string{"down clean", "stop clean", "down", "restart web", "kill hidden", "logs hidden"} {
		if got := policy(command); got != "" {
			t.Fatalf("%s: blocked: %s", command, got)
		}
	}
	cfg.Docker.AllowHostAccess = true
	for _, command := range []string{"restart hidden", "down hidden", "rm -s hidden", "start prov"} {
		if got := policy(command); got != "" {
			t.Fatalf("%s: blocked with host access: %s", command, got)
		}
	}
	if len(*calls) != 0 {
		t.Fatalf("lifecycle commands ran a named resolution: %q", *calls)
	}
}

func TestDockerComposePolicyLifecycleNamesNeverDenyOnOldCompose(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "prof/compose.yml", profileComposeFixture)
	stubDockerComposeResolverNamed(t, `{"services":{"web":{"image":"alpine"}}}`, "", func([]string) (string, error) {
		return "", errors.New("named resolution must not run for lifecycle commands")
	})
	for _, allow := range []bool{false, true} {
		cfg := &config.Config{}
		cfg.Docker.AllowHostAccess = allow
		useRuntimePermissionsForTest(t, cfg)
		for _, command := range []string{"down hidden", "stop hidden", "restart hidden", "rm -s hidden", "start hidden"} {
			if got := dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "prof/compose.yml", Command: command}); got != "" {
				t.Fatalf("allow=%v %s: blocked on old Compose: %s", allow, command, got)
			}
		}
	}
}

func TestDockerComposePolicySharesOneDeadlineAcrossResolutions(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", p1ComposeFile)
	var deadlines []time.Time
	stubDockerComposeResolverModes(t, func(ctx context.Context, _ string, opts tools.DockerComposeConfigOptions) (string, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("Compose resolution without a deadline")
		}
		deadlines = append(deadlines, deadline)
		switch {
		case len(opts.Services) > 0:
			return `{"services":{"hidden":{"image":"alpine","profiles":["later"]}}}`, nil
		case opts.AllProfiles:
			return "", errors.New("resolve Compose config: exit status 16: unknown flag: --no-env-resolution")
		}
		return `{"services":{"web":{"image":"alpine"}}}`, nil
	})
	start := time.Now()
	if got := dockerComposePolicy(context.Background(), &config.Config{}, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: "up -d hidden"}); got != "" {
		t.Fatalf("up -d hidden: %s", got)
	}
	if len(deadlines) != 3 {
		t.Fatalf("resolutions = %d, want default, all-profiles and named", len(deadlines))
	}
	for _, deadline := range deadlines {
		if !deadline.Equal(deadlines[0]) || deadline.After(start.Add(dockerComposePreflightTimeout+time.Second)) {
			t.Fatalf("deadlines = %v, want one shared deadline within %s", deadlines, dockerComposePreflightTimeout)
		}
	}
}
