package agent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// dockerToolSchemaProperties returns the operation enum and the property
// names of the native docker tool schema.
func dockerToolSchemaProperties(t *testing.T) ([]string, map[string]bool) {
	t.Helper()
	for _, tool := range BuildNativeToolSchemas("", nil, ToolFeatureFlags{DockerEnabled: true}, nil) {
		if tool.Function == nil || tool.Function.Name != "docker" {
			continue
		}
		params, ok := tool.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("docker parameters = %T", tool.Function.Parameters)
		}
		props, _ := params["properties"].(map[string]interface{})
		operation, _ := props["operation"].(map[string]interface{})
		enum, _ := operation["enum"].([]string)
		names := map[string]bool{}
		for name := range props {
			names[name] = true
		}
		return enum, names
	}
	t.Fatal("no docker tool in the native schemas")
	return nil, nil
}

// F-C19: the native docker schema offers exactly the operations the
// dispatcher accepts (its error message lists the canonical names), so a
// strict schema no longer hides exec, cp, the network and volume operations
// or compose, and every argument they read is a schema property.
func TestDockerToolSchemaMatchesTheDispatcherOperations(t *testing.T) {
	enum, props := dockerToolSchemaProperties(t)
	source := `Unknown docker operation. Use: list_containers, inspect, start, stop, restart, pause, unpause, remove, logs, create, run, list_images, pull, remove_image, list_networks, create_network, remove_network, connect, disconnect, list_volumes, create_volume, remove_volume, exec, stats, top, port, cp, compose, info`
	dispatched := strings.Split(strings.TrimPrefix(source, "Unknown docker operation. Use: "), ", ")
	if got, want := slices.Sorted(slices.Values(enum)), slices.Sorted(slices.Values(dispatched)); !slices.Equal(got, want) {
		t.Fatalf("schema enum = %q, want the dispatcher's operations %q", got, want)
	}
	for _, name := range []string{"operation", "container_id", "image", "name", "command", "command_args", "auto_remove", "env", "ports", "volumes",
		"restart", "force", "tail", "all", "user", "source", "destination", "direction", "driver", "network", "file"} {
		if !props[name] {
			t.Fatalf("docker schema lacks the %q argument that decodeDockerArgs reads", name)
		}
	}
}

// Every schema operation reaches its own dispatch branch, never the
// unknown-operation error.
func TestDockerToolSchemaOperationsAreDispatched(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/version") {
			_, _ = w.Write([]byte(`{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://" + strings.TrimPrefix(server.URL, "http://")
	cfg.Directories.WorkspaceDir = t.TempDir()
	useRuntimePermissionsForTest(t, cfg)
	stubDockerComposeResolverModes(t, func(context.Context, string, tools.DockerComposeConfigOptions) (string, error) {
		return "", errors.New("resolve Compose config: stubbed")
	})
	enum, _ := dockerToolSchemaProperties(t)
	for _, operation := range enum {
		call := ToolCall{Action: "docker", Operation: operation, ContainerID: "worker", Name: "worker", Image: "alpine:latest", Command: "true",
			Network: "net", File: "compose.yml", Source: "a.txt", Destination: "/tmp/a.txt", Direction: "to_container"}
		if operation == "compose" {
			call.Command = "ps"
		}
		output, ok := dispatchServices(context.Background(), call, &DispatchContext{Cfg: cfg, Logger: testLogger})
		if !ok || strings.Contains(output, "Unknown docker operation") {
			t.Fatalf("%s: handled = %v, output = %s", operation, ok, output)
		}
	}
}
