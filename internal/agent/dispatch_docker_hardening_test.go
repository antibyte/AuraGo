package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

func TestDispatchDockerCreateRunHardeningBlocksBeforeDocker(t *testing.T) {
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	cfg.Directories.WorkspaceDir = t.TempDir()

	tests := []struct {
		name string
		call ToolCall
		code string
	}{
		{
			name: "create requires name",
			call: ToolCall{Action: "docker", Operation: "create", Image: "alpine:latest"},
			code: "docker_name_required",
		},
		{
			name: "run requires name",
			call: ToolCall{Action: "docker", Operation: "run", Image: "alpine:latest"},
			code: "docker_name_required",
		},
		{
			name: "reserved homepage container",
			call: ToolCall{Action: "docker", Operation: "inspect", ContainerID: "aurago-homepage-web"},
			code: "docker_managed_homepage_resource",
		},
		{
			name: "reserved aurago app container inspect",
			call: ToolCall{Action: "docker", Operation: "inspect", ContainerID: "aurago"},
			code: "docker_managed_aurago_resource",
		},
		{
			name: "reserved aurago app container exec",
			call: ToolCall{Action: "docker", Operation: "exec", ContainerID: "aurago", Command: "env"},
			code: "docker_managed_aurago_resource",
		},
		{
			name: "reserved homepage create name",
			call: ToolCall{Action: "docker", Operation: "create", Name: "aurago-homepage", Image: "alpine:latest"},
			code: "docker_managed_homepage_resource",
		},
		{
			name: "reserved homepage create name behind decoy container_id",
			call: ToolCall{Action: "docker", Operation: "create", ContainerID: "decoy", Name: "aurago-homepage", Image: "alpine:latest"},
			code: "docker_managed_homepage_resource",
		},
		{
			name: "reserved garage run name behind decoy container_id",
			call: ToolCall{Action: "docker", Operation: "run", ContainerID: "decoy", Name: "aurago-boring-garage", Image: "alpine:latest"},
			code: "docker_managed_garage_resource",
		},
		{
			name: "reserved garage create name",
			call: ToolCall{Action: "docker", Operation: "create", Name: "aurago-boring-garage", Image: "alpine:latest"},
			code: "docker_managed_garage_resource",
		},
		{
			name: "reserved app create name behind decoy container_id",
			call: ToolCall{Action: "docker", Operation: "create", ContainerID: "decoy", Name: "aurago", Image: "alpine:latest"},
			code: "docker_managed_aurago_resource",
		},
		{
			name: "reserved app replica run name behind decoy container_id",
			call: ToolCall{Action: "docker", Operation: "run", ContainerID: "decoy", Name: "stack-aurago-1", Image: "alpine:latest"},
			code: "docker_managed_aurago_resource",
		},
		{
			name: "reserved homepage name with case and spaces behind decoy container_id",
			call: ToolCall{Action: "docker", Operation: "create", ContainerID: "decoy", Name: " AURAGO-HOMEPAGE ", Image: "alpine:latest"},
			code: "docker_managed_homepage_resource",
		},
		{
			name: "reserved homepage web run name behind decoy container_id",
			call: ToolCall{Action: "docker", Operation: "run", ContainerID: "decoy", Name: "aurago-homepage-web", Image: "alpine:latest"},
			code: "docker_managed_homepage_resource",
		},
		{
			name: "reserved garage replica name behind decoy container_id",
			call: ToolCall{Action: "docker", Operation: "create", ContainerID: "decoy", Name: "proj-aurago-boring-garage-1", Image: "alpine:latest"},
			code: "docker_managed_garage_resource",
		},
		{
			name: "reserved garage name with case and spaces",
			call: ToolCall{Action: "docker", Operation: "run", Name: " AURAGO-BORING-GARAGE ", Image: "alpine:latest"},
			code: "docker_managed_garage_resource",
		},
		{
			name: "reserved security proxy create name",
			call: ToolCall{Action: "docker", Operation: "create", Name: "aurago-security-proxy", Image: "caddy:latest"},
			code: "docker_managed_security_proxy_resource",
		},
		{
			name: "reserved security proxy run name behind decoy container_id",
			call: ToolCall{Action: "docker", Operation: "run", ContainerID: "decoy", Name: "aurago-security-proxy", Image: "caddy:latest"},
			code: "docker_managed_security_proxy_resource",
		},
		{
			name: "reserved security proxy name with case and spaces",
			call: ToolCall{Action: "docker", Operation: "create", Name: " AURAGO-SECURITY-PROXY ", Image: "caddy:latest"},
			code: "docker_managed_security_proxy_resource",
		},
		{
			name: "reserved cyd builder create name",
			call: ToolCall{Action: "docker", Operation: "create", Name: "aurago-cyd-builder", Image: "alpine:latest"},
			code: "docker_managed_cyd_creator_resource",
		},
		{
			name: "reserved cyd builder run name behind decoy container_id",
			call: ToolCall{Action: "docker", Operation: "run", ContainerID: "decoy", Name: "aurago-cyd-builder", Image: "alpine:latest"},
			code: "docker_managed_cyd_creator_resource",
		},
		{
			name: "reserved cyd builder name with case and spaces",
			call: ToolCall{Action: "docker", Operation: "create", Name: " AURAGO-CYD-BUILDER ", Image: "alpine:latest"},
			code: "docker_managed_cyd_creator_resource",
		},
		{
			name: "reserved homepage image",
			call: ToolCall{Action: "docker", Operation: "run", Name: "test-homepage", Image: "registry.test/team/aurago-homepage:v1"},
			code: "docker_managed_homepage_resource",
		},
		{
			name: "reserved homepage image removal",
			call: ToolCall{Action: "docker", Operation: "remove_image", Image: "aurago-homepage:latest"},
			code: "docker_managed_homepage_resource",
		},
		{
			name: "auto remove is run only",
			call: ToolCall{Action: "docker", Operation: "create", Name: "job", Image: "alpine:latest", AutoRemove: true},
			code: "docker_auto_remove_conflict",
		},
		{
			name: "auto remove conflicts with restart",
			call: ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", AutoRemove: true, Restart: "always"},
			code: "docker_auto_remove_conflict",
		},
		{
			name: "command and args conflict",
			call: ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", Command: "echo ok", CommandArgs: []string{"echo", "ok"}},
			code: "docker_command_conflict",
		},
		{
			name: "legacy shell syntax is rejected",
			call: ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", Command: "ls /data | head"},
			code: "docker_command_args_required",
		},
		{
			name: "legacy shell quoting is rejected",
			call: ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", Command: `printf "%s" value`},
			code: "docker_command_args_required",
		},
		{
			name: "legacy redirection is rejected",
			call: ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", Command: "echo ok > /tmp/result"},
			code: "docker_command_args_required",
		},
		{
			name: "legacy chaining is rejected",
			call: ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", Command: "echo ok && true"},
			code: "docker_command_args_required",
		},
		{
			name: "legacy globbing is rejected",
			call: ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", Command: "ls *.txt"},
			code: "docker_command_args_required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, ok := dispatchServices(context.Background(), tt.call, &DispatchContext{Cfg: cfg, Logger: testLogger})
			if !ok {
				t.Fatal("expected docker operation to be handled")
			}
			if !strings.Contains(output, `"code":"`+tt.code+`"`) {
				t.Fatalf("output = %s, want code %s", output, tt.code)
			}
			if strings.Contains(output, "connect") {
				t.Fatalf("validation reached Docker API: %s", output)
			}
		})
	}
}

// Every Garage denial uses the same machine-readable code as the homepage and
// app denials; the message text stays unchanged.
func TestDispatchDockerGarageDenialsCarryReservedResourceCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/version":
			_, _ = w.Write([]byte(`{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`))
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://" + strings.TrimPrefix(server.URL, "http://")
	cfg.Directories.WorkspaceDir = t.TempDir()
	useRuntimePermissionsForTest(t, cfg)

	tests := []struct {
		name string
		call ToolCall
		text string
	}{
		{
			name: "inspect of the Garage container",
			call: ToolCall{Action: "docker", Operation: "inspect", ContainerID: "aurago-boring-garage"},
			text: "managed Boring Computers Garage container is blocked",
		},
		{
			name: "bind mount of the Garage data path",
			call: ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", Volumes: []string{"/opt/aurago/data/sidecars/garage:/data"}},
			text: "Garage data paths cannot be mounted",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, ok := dispatchServices(context.Background(), tt.call, &DispatchContext{Cfg: cfg, Logger: testLogger})
			if !ok {
				t.Fatal("expected docker operation to be handled")
			}
			if !strings.Contains(output, `"code":"docker_managed_garage_resource"`) || !strings.Contains(output, tt.text) {
				t.Fatalf("output = %s, want code docker_managed_garage_resource and text %q", output, tt.text)
			}
		})
	}

	got := dockerComposeOwnerDenial(dockerutil.BoringGarageOwner)
	if !strings.Contains(got, `"code":"docker_managed_garage_resource"`) || !strings.Contains(got, "Docker Compose access to AuraGo's managed Boring Computers Garage is blocked.") {
		t.Fatalf("compose Garage denial = %s, want code docker_managed_garage_resource and the unchanged text", got)
	}
}

// A generic name must not be treated as reserved just because a container_id
// is present next to it, and the managed desktop container names stay usable.
func TestDispatchDockerReservedNameChecksAllowGenericNamesNextToContainerID(t *testing.T) {
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.ReadOnly = true // stops every call at the read-only gate, before any Docker API access
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	cfg.Directories.WorkspaceDir = t.TempDir()

	for _, operation := range []string{"create", "run"} {
		for _, name := range []string{"worker", "custom-caddy", "aurago-lab", "homepage-notes", "aurago-code-studio", "aurago-openscad", "aurago-cyd-builder-it", "cyd-builder"} {
			for _, containerID := range []string{"", "decoy"} {
				call := ToolCall{Action: "docker", Operation: operation, ContainerID: containerID, Name: name, Image: "alpine:latest"}
				req := decodeDockerArgs(call)
				if dockerRequestTargetsManagedHomepage(req) || dockerRequestTargetsAuraGoApp(req) || dockerRequestCreatesReservedGarageName(req) ||
					dockerRequestCreatesReservedCYDCreatorName(req) {
					t.Errorf("%s name=%q container_id=%q: reserved-name check matched a generic name", operation, name, containerID)
				}
				output, ok := dispatchServices(context.Background(), call, &DispatchContext{Cfg: cfg, Logger: testLogger})
				if !ok {
					t.Fatalf("%s name=%q: expected docker operation to be handled", operation, name)
				}
				if strings.Contains(output, `"code":"docker_managed_`) {
					t.Errorf("%s name=%q container_id=%q: output = %s, want no reserved-name denial", operation, name, containerID, output)
				}
				if !strings.Contains(output, "read-only mode") {
					t.Errorf("%s name=%q container_id=%q: output = %s, want to reach the read-only gate", operation, name, containerID, output)
				}
			}
		}
	}
}

func TestValidateAgentDockerCreateRunAllowsNamedGenericContainers(t *testing.T) {
	tests := []struct {
		name string
		req  dockerArgs
		want []string
	}{
		{
			name: "ordinary named container",
			req:  dockerArgs{Operation: "create", Name: "worker", Image: "alpine:latest", Command: "echo ok"},
			want: []string{"echo", "ok"},
		},
		{
			name: "named caddy container",
			req:  dockerArgs{Operation: "run", Name: "custom-caddy", Image: "caddy:2.11.2-alpine"},
		},
		{
			name: "exact command arguments",
			req:  dockerArgs{Operation: "run", Name: "shell-job", Image: "alpine:latest", CommandArgs: []string{"/bin/sh", "-lc", "printf '%s' \"$VALUE\""}, AutoRemove: true},
			want: []string{"/bin/sh", "-lc", "printf '%s' \"$VALUE\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, restart, options, validationError := validateAgentDockerCreateRun(tt.req)
			if validationError != "" {
				t.Fatalf("unexpected validation error: %s", validationError)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("command = %#v, want %#v", got, tt.want)
			}
			if restart != "no" {
				t.Fatalf("restart = %q, want no", restart)
			}
			if options.AutoRemove != tt.req.AutoRemove {
				t.Fatalf("AutoRemove = %v, want %v", options.AutoRemove, tt.req.AutoRemove)
			}
		})
	}
}

// Protecting the security proxy only reserves its name for create and run:
// the agent keeps its access to the existing proxy container.
func TestDispatchDockerKeepsSecurityProxyContainerAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/version":
			_, _ = w.Write([]byte(`{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`))
		case strings.HasSuffix(r.URL.Path, "/containers/aurago-security-proxy/json"):
			_, _ = w.Write([]byte(`{"Id":"dddddddddddd4444","Name":"/aurago-security-proxy","Config":{"Labels":{"aurago.managed":"security-proxy","aurago.component":"caddy","aurago.role":"proxy"}},"State":{"Status":"running","Running":true}}`))
		case strings.HasSuffix(r.URL.Path, "/containers/aurago-security-proxy/logs"):
			_, _ = w.Write([]byte("caddy serving localhost\n"))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/aurago-security-proxy/stop"):
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://" + strings.TrimPrefix(server.URL, "http://")
	cfg.Directories.WorkspaceDir = t.TempDir()
	useRuntimePermissionsForTest(t, cfg)

	for _, operation := range []string{"logs", "stop"} {
		call := ToolCall{Action: "docker", Operation: operation, ContainerID: "aurago-security-proxy"}
		output, ok := dispatchServices(context.Background(), call, &DispatchContext{Cfg: cfg, Logger: testLogger})
		if !ok {
			t.Fatal("expected docker operation to be handled")
		}
		if strings.Contains(output, `"status":"error"`) || strings.Contains(output, "docker_managed") {
			t.Fatalf("%s on the security proxy = %s, want the existing access", operation, output)
		}
	}
	for _, name := range []string{"caddy", "security-proxy", "aurago-security-proxy-test"} {
		if dockerRequestCreatesReservedSecurityProxyName(decodeDockerArgs(ToolCall{Action: "docker", Operation: "run", Name: name, Image: "caddy:latest"})) {
			t.Fatalf("generic name %q was treated as the reserved security proxy name", name)
		}
	}
}

// The CYD Creator builder compiles every project from its shared cache volume:
// the agent docker tool refuses every operation on the builder (by name, or by
// ID through its label), every mount and volume create/remove of its two
// volumes, and hides them from the lists. The test and CI volumes stay usable.
func TestDispatchDockerBlocksTheCYDCreatorBuilderAndVolumes(t *testing.T) {
	const builderLabels = `{"aurago.managed":"cyd-creator","aurago.component":"cyd-creator","aurago.role":"builder","aurago.fingerprint":"0123456789abcdef"}`
	var mu sync.Mutex
	var mutations []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			mu.Lock()
			mutations = append(mutations, r.Method+" "+path+"?"+r.URL.RawQuery)
			mu.Unlock()
		}
		switch {
		case path == "/version":
			_, _ = w.Write([]byte(`{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`))
		case strings.HasSuffix(path, "/containers/aurago-cyd-builder/json"):
			_, _ = w.Write([]byte(`{"Id":"cccccccccccc3333","Name":"/aurago-cyd-builder","Config":{"Labels":` + builderLabels + `}}`))
		case strings.HasSuffix(path, "/containers/cccccccccccc/json"):
			_, _ = w.Write([]byte(`{"Id":"cccccccccccc3333","Name":"/renamed-builder","Config":{"Labels":` + builderLabels + `}}`))
		case strings.HasSuffix(path, "/containers/json"):
			_, _ = w.Write([]byte(`[
				{"Id":"cccccccccccc3333","Names":["/aurago-cyd-builder"],"Image":"ghcr.io/antibyte/aurago-cyd-builder:1","State":"running","Status":"Up","Labels":` + builderLabels + `},
				{"Id":"aaaaaaaaaaaa1111","Names":["/web"],"Image":"nginx:1","State":"running","Status":"Up","Labels":{}}
			]`))
		case strings.HasSuffix(path, "/volumes") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"Volumes":[
				{"Name":"aurago-cyd-work","Driver":"local","Labels":{"aurago.managed":"cyd-creator","aurago.role":"work"}},
				{"Name":"aurago-cyd-cache","Driver":"local","Labels":{"aurago.managed":"cyd-creator","aurago.role":"cache"}},
				{"Name":"aurago-cyd-cache-it","Driver":"local","Labels":null}
			]}`))
		case strings.HasSuffix(path, "/containers/create"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"bbbbbbbbbbbb2222"}`))
		case strings.HasSuffix(path, "/volumes/create"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Name":"aurago-cyd-cache-it"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/start"):
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://" + strings.TrimPrefix(server.URL, "http://")
	cfg.Directories.WorkspaceDir = t.TempDir()
	useRuntimePermissionsForTest(t, cfg)
	dispatch := func(call ToolCall) string {
		t.Helper()
		output, ok := dispatchServices(context.Background(), call, &DispatchContext{Cfg: cfg, Logger: testLogger})
		if !ok {
			t.Fatal("expected docker operation to be handled")
		}
		return output
	}
	const code = `"code":"docker_managed_cyd_creator_resource"`

	var blocked []ToolCall
	for _, target := range []string{"aurago-cyd-builder", "cccccccccccc"} {
		for _, operation := range []string{"inspect", "logs", "stats", "top", "port", "start", "stop", "restart", "pause", "unpause", "remove", "connect", "disconnect"} {
			blocked = append(blocked, ToolCall{Action: "docker", Operation: operation, ContainerID: target, Network: "bridge"})
		}
		blocked = append(blocked,
			ToolCall{Action: "docker", Operation: "exec", ContainerID: target, Command: "sh -c 'touch /cache/x'"},
			ToolCall{Action: "docker", Operation: "cp", ContainerID: target, Source: "evil.o", Destination: "/cache/build/x.o", Direction: "to_container"},
		)
	}
	for _, volumes := range [][]string{{"aurago-cyd-cache:/cache"}, {"aurago-cyd-work:/work:ro"}, {"data:/data", " AURAGO-CYD-CACHE :/x"}} {
		blocked = append(blocked, ToolCall{Action: "docker", Operation: "run", Name: "job", Image: "alpine:latest", Volumes: volumes})
	}
	for _, operation := range []string{"create_volume", "remove_volume"} {
		for _, name := range []string{"aurago-cyd-work", "aurago-cyd-cache"} {
			blocked = append(blocked, ToolCall{Action: "docker", Operation: operation, Name: name, Force: true})
		}
	}
	for _, call := range blocked {
		if output := dispatch(call); !strings.Contains(output, code) || !strings.Contains(output, "CYD Creator") {
			t.Fatalf("%s %s %v = %s, want %s", call.Operation, call.ContainerID+call.Name, call.Volumes, output, code)
		}
	}
	mu.Lock()
	if len(mutations) != 0 {
		t.Fatalf("refused operations reached Docker: %v", mutations)
	}
	mu.Unlock()

	// Test and CI volumes stay usable.
	for _, call := range []ToolCall{
		{Action: "docker", Operation: "run", Name: "it-job", Image: "aurago-cyd-builder:dev", Volumes: []string{"aurago-cyd-work-it:/work", "aurago-cyd-cache-it:/cache"}},
		{Action: "docker", Operation: "create", Name: "ci-job", Image: "alpine:latest", Volumes: []string{"cyd-ci-cache:/cache"}},
		{Action: "docker", Operation: "create_volume", Name: "aurago-cyd-cache-it"},
	} {
		if output := dispatch(call); strings.Contains(output, "docker_managed") || strings.Contains(output, `"status":"error"`) {
			t.Fatalf("%s %s %v = %s, want it to reach Docker", call.Operation, call.Name, call.Volumes, output)
		}
	}

	// The lists hide the builder and both volumes.
	containers := dispatch(ToolCall{Action: "docker", Operation: "list_containers", All: true})
	if strings.Contains(containers, "aurago-cyd-builder") || !strings.Contains(containers, `"/web"`) {
		t.Fatalf("list_containers = %s, want the builder hidden and web listed", containers)
	}
	volumes := dispatch(ToolCall{Action: "docker", Operation: "list_volumes"})
	if strings.Contains(volumes, `"aurago-cyd-work"`) || strings.Contains(volumes, `"aurago-cyd-cache"`) || !strings.Contains(volumes, `"aurago-cyd-cache-it"`) {
		t.Fatalf("list_volumes = %s, want both builder volumes hidden and the test volume listed", volumes)
	}

	got := dockerComposeOwnerDenial(dockerutil.CYDCreatorOwner)
	if !strings.Contains(got, code) || !strings.Contains(got, "Use the CYD Creator app") {
		t.Fatalf("compose CYD Creator denial = %s, want %s", got, code)
	}
}
