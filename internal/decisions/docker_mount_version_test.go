package decisions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
)

func TestDockerMountPreflightRequiresVolumeSubpathCapability(t *testing.T) {
	for _, tc := range []struct {
		name, version, mountType, want string
	}{
		{"old volume", "1.44", "volume", CodeDockerAPITooOld},
		{"supported volume", "1.45", "volume", ""},
		{"old bind", "1.44", "bind", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := filepath.Join(t.TempDir(), "data")
			var mutations atomic.Int32
			engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					mutations.Add(1)
					t.Errorf("preflight mutated Docker: %s", r.URL)
				}
				switch {
				case r.URL.Path == "/version":
					json.NewEncoder(w).Encode(map[string]string{"ApiVersion": tc.version, "MinAPIVersion": "1.25"})
				case strings.HasSuffix(r.URL.Path, "/containers/self/json"):
					json.NewEncoder(w).Encode(map[string]any{
						"Id": "self", "State": map[string]bool{"Running": true},
						"Mounts": []map[string]string{{"Type": tc.mountType, "Name": "aurago_data", "Source": filepath.ToSlash(t.TempDir()), "Destination": filepath.ToSlash(data)}},
					})
				default:
					t.Errorf("unexpected preflight request: %s", r.URL)
					http.NotFound(w, r)
				}
			}))
			defer engine.Close()
			access := RuntimeAccess{IsDocker: true, DockerEnabled: true, DockerHost: strings.Replace(engine.URL, "http://", "tcp://", 1)}
			resolveSelf := func(_ context.Context, _ string) (string, error) { return "self", nil }
			err := preflightDockerMounts(t.Context(), access, data, resolveSelf)
			if ErrorCode(err) != tc.want || mutations.Load() != 0 {
				t.Fatalf("preflight error=%v, mutations=%d", err, mutations.Load())
			}
			if tc.want != "" {
				if !terminalRuntimeStartError(err) || !IsNonRetryable(err) {
					t.Fatal("unsupported mount can trigger device retries")
				}
				manager := NewManager(config.DecisionsConfig{Deployment: "docker"}, data, WithRuntimeAccess(func() RuntimeAccess { return access }), WithDockerSelfContainer(resolveSelf))
				if check := manager.CheckInstallation(t.Context()); check.Code != tc.want || check.RuntimeAvailable {
					t.Fatalf("installation check: %+v", check)
				}
				factory := managedRuntimeFactory{options: manager.options}
				if err := factory.install(t.Context(), "docker", access, data, false, nil); ErrorCode(err) != tc.want {
					t.Fatalf("installation did not reject unsupported mount first: %v", err)
				}
				docker := newDecisionsDocker(access.DockerHost)
				defer docker.client.CloseIdleConnections()
				if _, err := docker.runDiscover(t.Context(), "vulkan", "sha256:fixture", modelPaths{Model: filepath.Join(data, "decisions", "models", "model.gguf")}, access, resolveSelf); ErrorCode(err) != tc.want {
					t.Fatalf("discovery mount failure: %v", err)
				}
			}
			if _, err := os.Stat(data); !os.IsNotExist(err) {
				t.Fatalf("preflight created data: %v", err)
			}
		})
	}
}
