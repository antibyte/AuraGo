package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestRustFSComposeBackingAncestorDeniedWithHostAccess(t *testing.T) {
	for _, hostAccess := range []bool{false, true} {
		for _, model := range []string{
			`{"services":{"reader":{"image":"alpine","volumes":[{"type":"bind","source":"/var/lib/docker/volumes","target":"/keys"}]}}}`,
			`{"services":{"reader":{"image":"alpine","volumes":[{"type":"volume","source":"alias","target":"/keys"}]}},"volumes":{"alias":{"driver":"local","driver_opts":{"type":"none","o":"bind","device":"/host_mnt/var/lib/docker"}}}}`,
		} {
			workspace := t.TempDir()
			writeComposeFixture(t, workspace, "compose.yml", "services:\n  reader:\n    image: alpine\n")
			stubDockerComposeResolver(t, func(string) (string, error) { return model, nil })
			c := &config.Config{}
			c.Docker.AllowHostAccess = hostAccess
			got := dockerComposePolicy(context.Background(), c, tools.DockerConfig{WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: "up -d"})
			if !strings.Contains(got, "docker_managed_rustfs_resource") {
				t.Fatalf("host access=%v: %s", hostAccess, got)
			}
		}
	}
}

func TestRustFSComposeExistingAliasCannotBypassHostAccess(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusForbidden} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/version" {
				fmt.Fprint(w, `{"ApiVersion":"1.47","MinAPIVersion":"1.25"}`)
				return
			}
			w.WriteHeader(status)
			fmt.Fprint(w, `{"Name":"alias","Mountpoint":"/var/lib/docker/volumes/alias/_data","Options":{"device":"/var/lib/docker/volumes/aurago_rustfs_credentials/_data"}}`)
		}))
		t.Cleanup(server.Close)
		workspace := t.TempDir()
		writeComposeFixture(t, workspace, "compose.yml", "services:\n  reader:\n    image: alpine\n")
		stubDockerComposeResolver(t, func(string) (string, error) {
			return `{"services":{"reader":{"image":"alpine","volumes":[{"type":"volume","source":"keys","target":"/keys"}]}},"volumes":{"keys":{"name":"alias"}}}`, nil
		})
		c := &config.Config{}
		c.Docker.Enabled = true
		c.Docker.AllowHostAccess = true
		useRuntimePermissionsForTest(t, c)
		host := "tcp://" + strings.TrimPrefix(server.URL, "http://")
		got := dockerComposePolicy(context.Background(), c, tools.DockerConfig{Host: host, WorkspaceDir: workspace}, dockerArgs{Operation: "compose", File: "compose.yml", Command: "up -d"})
		want := "docker_managed_rustfs_resource"
		if status != 200 {
			want = "docker_ownership_unverified"
		}
		if !strings.Contains(got, want) {
			t.Fatalf("alias inspection HTTP %d: %s", status, got)
		}
	}
}

// Existing policy fixtures use an absent volume to represent a new ordinary stack.
func rustFSAbsentDockerVolumeHost(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			fmt.Fprint(w, `{"ApiVersion":"1.47","MinAPIVersion":"1.25"}`)
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/volumes/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		t.Errorf("unexpected Docker fixture request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(500)
	}))
	t.Cleanup(server.Close)
	return "tcp://" + strings.TrimPrefix(server.URL, "http://")
}
