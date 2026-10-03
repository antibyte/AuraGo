package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchDockerBlocksContainerOpsWhenOwnershipUnverified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "daemon overloaded", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://" + strings.TrimPrefix(server.URL, "http://")
	cfg.Directories.WorkspaceDir = t.TempDir()
	useRuntimePermissionsForTest(t, cfg)

	for _, op := range []string{"inspect", "logs", "stop"} {
		output, ok := dispatchServices(context.Background(), ToolCall{Action: "docker", Operation: op, ContainerID: "abc123"}, &DispatchContext{Cfg: cfg, Logger: testLogger})
		if !ok {
			t.Fatalf("%s: docker operation not handled", op)
		}
		if !strings.Contains(output, `"code":"docker_ownership_unverified"`) {
			t.Fatalf("%s output = %s, want docker_ownership_unverified", op, output)
		}
	}
	output, _ := dispatchServices(context.Background(), ToolCall{Action: "docker", Operation: "list_containers"}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if strings.Contains(output, "docker_ownership_unverified") {
		t.Fatalf("list_containers was blocked by the ownership check: %s", output)
	}
}
