package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchDockerRefusesDecisionsContainers(t *testing.T) {
	const labeledID = "dddddddddddd1111"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/version":
			_, _ = w.Write([]byte(`{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`))
		case strings.HasSuffix(path, "/containers/custom-decisions/json"):
			_, _ = w.Write([]byte(`{"Id":"` + labeledID + `","Name":"/custom-decisions","Config":{"Labels":{"aurago.managed":"decisions"}}}`))
		case strings.HasSuffix(path, "/containers/json"):
			_, _ = w.Write([]byte(`[
				{"Id":"` + labeledID + `","Names":["/custom-decisions"],"Image":"decisions:local","State":"running","Status":"Up","Labels":{"aurago.managed":"decisions"}},
				{"Id":"eeeeeeeeeeee2222","Names":["/aurago-decisions-discover-cuda-abc"],"Image":"decisions:discover","State":"running","Status":"Up","Labels":{}},
				{"Id":"aaaaaaaaaaaa1111","Names":["/web"],"Image":"nginx","State":"running","Status":"Up","Labels":{}}
			]`))
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

	const code = `"code":"docker_managed_decisions_resource"`
	for _, call := range []ToolCall{
		{Action: "docker", Operation: "inspect", ContainerID: "custom-decisions"},
		{Action: "docker", Operation: "exec", ContainerID: "custom-decisions", Command: "id"},
		{Action: "docker", Operation: "inspect", ContainerID: "aurago-decisions-discover-cuda-abc"},
		{Action: "docker", Operation: "exec", ContainerID: "aurago-decisions-discover-cuda-abc", Command: "id"},
	} {
		output, ok := dispatchServices(context.Background(), call, &DispatchContext{Cfg: cfg, Logger: testLogger})
		if !ok {
			t.Fatalf("%s %s was not handled", call.Operation, call.ContainerID)
		}
		if !strings.Contains(output, code) {
			t.Fatalf("%s %s = %s, want %s", call.Operation, call.ContainerID, output, code)
		}
	}

	output, ok := dispatchServices(context.Background(), ToolCall{Action: "docker", Operation: "list_containers", All: true}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok || strings.Contains(output, `"status":"error"`) {
		t.Fatalf("list_containers = %s", output)
	}
	if strings.Contains(output, "custom-decisions") || strings.Contains(output, "aurago-decisions-discover-cuda-abc") || !strings.Contains(output, `"/web"`) {
		t.Fatalf("list_containers = %s, want Decisions containers hidden and web listed", output)
	}
}
