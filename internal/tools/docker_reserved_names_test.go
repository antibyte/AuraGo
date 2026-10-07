package tools

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/acestep"
	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

func TestDockerRenameContainerRejectsReservedManagedNames(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	var requests atomic.Int64
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
	cfg := DockerConfig{Host: host}
	for _, name := range []string{dockerutil.BoringGarageContainerName, dockerutil.HomepageContainerName, dockerutil.HomepageWebContainerName,
		dockerutil.AppContainerName, "stack-aurago-1", dockerutil.LocalLLMContainerName, acestep.ContainerName, dockerutil.SecurityProxyContainerName} {
		if got := DockerRenameContainer(cfg, "victim", name); !strings.Contains(got, "reserved AuraGo") {
			t.Fatalf("rename to %s = %s, want a reserved-name refusal", name, got)
		}
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("Docker received %d requests for reserved names", got)
	}
	for _, name := range []string{"aurago-code-studio", "aurago-openscad", "worker"} {
		if got := DockerRenameContainer(cfg, "victim", name); strings.Contains(got, "reserved") {
			t.Fatalf("rename to %s refused: %s", name, got)
		}
	}
}

// The agent and Compose protections use dockerutil's name; the Garage
// manager uses config's. They must stay the same name.
func TestManagedGarageContainerNameMatchesDockerutil(t *testing.T) {
	if config.ManagedGarageContainerName != dockerutil.BoringGarageContainerName {
		t.Fatalf("config.ManagedGarageContainerName = %q, dockerutil.BoringGarageContainerName = %q; the agent and Compose protections use the latter", config.ManagedGarageContainerName, dockerutil.BoringGarageContainerName)
	}
}
