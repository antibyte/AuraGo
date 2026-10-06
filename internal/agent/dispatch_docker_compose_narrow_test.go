package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

const narrowedComposeModel = `{"name":"lab","services":{` +
	`"web":{"image":"nginx:1.27"},` +
	`"api":{"image":"alpine","depends_on":{"helper":{"condition":"service_started","required":true}}},` +
	`"helper":{"image":"alpine","privileged":true},` +
	`"linked":{"image":"alpine","links":["traefik:proxy"]},` +
	`"sidecar":{"image":"alpine","network_mode":"service:traefik"},` +
	`"db":{"provider":{"type":"awesomecloud"}},` +
	`"traefik":{"image":"traefik:v3.1","volumes":[{"type":"bind","source":"/var/run/docker.sock","target":"/var/run/docker.sock","read_only":true}]}}}`

const narrowedHint = "the named services and the services they need were checked"

func TestDockerComposePolicyNarrowsHostAccessToNamedServices(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "lab/compose.yml", "services:\n  web:\n    image: nginx:1.27\n")
	for _, allProfiles := range []bool{true, false} {
		t.Run(fmt.Sprintf("all-profiles model %v", allProfiles), func(t *testing.T) {
			if allProfiles {
				stubDockerComposeResolverByMode(t, narrowedComposeModel, narrowedComposeModel)
			} else {
				stubDockerComposeResolverByMode(t, narrowedComposeModel, "")
			}
			cfg := &config.Config{}
			useRuntimePermissionsForTest(t, cfg)
			policy := func(command string) string {
				return dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "lab/compose.yml", Command: command})
			}
			for _, command := range []string{"up -d web", "up -d --build --force-recreate web", "up -dV --wait-timeout 30 web", "create web",
				"up -d -- web", "start web", "stop -t 5 web", "restart web", "rm -fs web", "down web"} {
				if got := policy(command); got != "" {
					t.Fatalf("%s: a clean named service was refused because of other services: %s", command, got)
				}
			}
			for _, command := range []string{"up -d", "up -d traefik", "up -d api", "up -d linked", "up -d sidecar",
				"up -d web traefik", "up -d --scale traefik=0 web", "up -d --attach traefik web", "up -d -x web", "stop", "stop db"} {
				if got := policy(command); !strings.Contains(got, `"code":"docker_compose_host_access_denied"`) {
					t.Fatalf("%s: got %s, want docker_compose_host_access_denied", command, got)
				}
			}
			if got := policy("up -d nosuch"); got == "" {
				t.Fatal("up -d nosuch was allowed: an unknown name must not narrow")
			}
			if got := policy("up -d api"); !strings.Contains(got, narrowedHint) {
				t.Fatalf("narrowed denial lacks its hint: %s", got)
			}
			if got := policy("up -d"); strings.Contains(got, narrowedHint) || !strings.Contains(got, "every service of the file's default profiles is checked") {
				t.Fatalf("whole-file denial changed its hint: %s", got)
			}
			cfg.Docker.AllowHostAccess = true
			if got := policy("up -d traefik"); got != "" {
				t.Fatalf("host access no longer allows traefik: %s", got)
			}
		})
	}
}

func TestDockerComposePolicyNarrowingKeepsWholeFileChecks(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "ws")
	dataDir := filepath.Join(root, "data")
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n")
	model := `{"services":{"web":{"image":"alpine"},"leak":{"image":"alpine","volumes":[{"type":"bind","source":` + jsonPath(t, dataDir) + `,"target":"/d"}]}}}`
	stubDockerComposeResolver(t, func(string) (string, error) { return model, nil })
	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	useRuntimePermissionsForTest(t, cfg)
	policy := func() string {
		return dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: "up -d web"})
	}
	if got := policy(); !strings.Contains(got, `"code":"docker_compose_protected_path_denied"`) {
		t.Fatalf("the always tier of another service was narrowed away: %s", got)
	}
	model = `{"services":{"web":{"image":"alpine"}},"volumes":{"host":{"name":"x_host","driver":"local","driver_opts":{"type":"none","o":"bind","device":"/srv/data"}}}}`
	if got := policy(); !strings.Contains(got, "volumes.host.driver_opts.device") {
		t.Fatalf("a top-level local bind volume was narrowed away: %s", got)
	}
}

// Stopping, removing or restarting a service also acts on the services that
// depend on it, and `up` may restart them; the narrowed check therefore keeps
// the dependents of the named services too (build and pull only need what the
// named services depend on).
func TestDockerComposePolicyNarrowingKeepsDependentsOfNamedServices(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  base:\n    image: alpine\n")
	model := `{"services":{"base":{"image":"alpine"},"other":{"image":"alpine"},` +
		`"hooked":{"image":"alpine","depends_on":{"base":{"condition":"service_started","required":true}},"pre_stop":[{"command":["sh","-c","true"],"privileged":true}]}}}`
	stubDockerComposeResolverByMode(t, model, model)
	cfg := &config.Config{}
	useRuntimePermissionsForTest(t, cfg)
	policy := func(command string) string {
		return dockerComposePolicy(context.Background(), cfg, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: command})
	}
	for _, command := range []string{"stop base", "down base", "rm -fs base", "restart base", "up -d base"} {
		if got := policy(command); !strings.Contains(got, `"code":"docker_compose_host_access_denied"`) || !strings.Contains(got, "hooked") {
			t.Fatalf("%s: got %s, want the dependent's privileged hook checked", command, got)
		}
	}
	for _, command := range []string{"stop other", "up -d other", "pull base"} {
		if got := policy(command); got != "" {
			t.Fatalf("%s: refused because of an unrelated service: %s", command, got)
		}
	}
}

// volumes_from cannot be part of the policy fixtures: any volumes_from in a
// Compose file is refused by the fail-closed local LLM check. The closure still
// follows it, so it is pinned here directly.
func TestDockerComposeServiceReferencesFollowEveryStartedService(t *testing.T) {
	raw := []byte(`{"depends_on":{"db":{"condition":"service_started","required":false}},"links":["cache:c","queue"],` +
		`"volumes_from":["data:ro","container:outside","service:logs"],"network_mode":"service:vpn","ipc":"service:shm","pid":"host",` +
		`"build":{"context":".","additional_contexts":{"base":"service:builder","docs":"./docs"}}}`)
	service := tools.DockerComposeService{Build: &tools.DockerComposeBuild{AdditionalContexts: map[string]string{"base": "service:builder", "docs": "./docs"}}}
	got, ok := dockerComposeServiceReferences(raw, service)
	if !ok {
		t.Fatal("references of a valid service were not read")
	}
	want := map[string]bool{"db": true, "builder": true, "cache": true, "queue": true, "data": true, "logs": true, "vpn": true, "shm": true}
	seen := map[string]bool{}
	for _, name := range got {
		if !want[name] {
			t.Fatalf("unexpected reference %q in %q", name, got)
		}
		seen[name] = true
	}
	if len(seen) != len(want) {
		t.Fatalf("references = %q, want every name of %v", got, want)
	}
	if _, ok := dockerComposeServiceReferences([]byte(`{"links":"not a list"}`), tools.DockerComposeService{}); ok {
		t.Fatal("an unreadable service must disable narrowing")
	}
}
