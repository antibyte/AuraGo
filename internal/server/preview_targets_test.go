package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktopstore"
	"aurago/internal/tools"
)

func TestPreviewTargetRemoteDockerUsesLANHostnameAndPersistedPort(t *testing.T) {
	const hostPort = 19431
	svc := newPreviewTargetStore(t, desktopstore.BindModeLAN, hostPort)
	app, ok, err := svc.GetInstalled(context.Background(), "node-red")
	if err != nil || !ok {
		t.Fatalf("GetInstalled() = (%v, %v), want installed app", app, err)
	}
	if len(app.Ports) != 1 || app.Ports[0].HostPort != hostPort {
		t.Fatalf("persisted ports = %#v, want host port %d", app.Ports, hostPort)
	}

	s := testDesktopStoreServerWithService(t, svc)
	s.Cfg.Docker.Host = "tcp://docker.example.test:2375"
	target, err := s.previewTarget(t.Context(), previewResource{kind: "store", id: "node-red", port: app.Ports[0].ID})
	if err != nil {
		t.Fatalf("previewTarget() error = %v", err)
	}
	want := fmt.Sprintf("http://docker.example.test:%d/", hostPort)
	if target.String() != want {
		t.Fatalf("preview target = %q, want %q", target, want)
	}
}

func TestPreviewTargetSharedDockerNetworkUsesStoredContainerPort(t *testing.T) {
	const hostPort = 19432
	svc := newPreviewTargetStore(t, desktopstore.BindModeLocal, hostPort)
	app, ok, err := svc.GetInstalled(context.Background(), "node-red")
	if err != nil || !ok {
		t.Fatalf("GetInstalled() = (%v, %v), want installed app", app, err)
	}
	ownContainer := previewTargetTestHostname(t)
	seen := make(map[string]bool)
	dockerHost := previewTargetDockerAPI(t, func(container string) map[string]any {
		seen[container] = true
		switch container {
		case app.ContainerName:
			return previewTargetDockerInspect(map[string]string{"private-app-net": "172.29.0.22"})
		case ownContainer:
			return previewTargetDockerInspect(map[string]string{
				"private-app-net": "172.29.0.9",
				"other-net":       "172.30.0.9",
			})
		default:
			t.Errorf("unexpected container inspect %q", container)
			return map[string]any{}
		}
	})

	s := testDesktopStoreServerWithService(t, svc)
	s.Cfg.Runtime.IsDocker = true
	s.Cfg.Docker.Host = dockerHost
	target, err := s.previewTarget(t.Context(), previewResource{kind: "store", id: "node-red", port: app.Ports[0].ID})
	if err != nil {
		t.Fatalf("previewTarget() error = %v", err)
	}
	want := fmt.Sprintf("http://172.29.0.22:%d/", app.ContainerPort)
	if target.String() != want {
		t.Fatalf("preview target = %q, want %q", target, want)
	}
	if !seen[app.ContainerName] || !seen[ownContainer] || len(seen) != 2 {
		t.Fatalf("inspected containers = %#v, want AuraGo and %q", seen, app.ContainerName)
	}
}

func TestPreviewTargetSharedDockerNetworkFailsClosedWithoutCommonNetwork(t *testing.T) {
	const hostPort = 19433
	svc := newPreviewTargetStore(t, desktopstore.BindModeLocal, hostPort)
	app, ok, err := svc.GetInstalled(context.Background(), "node-red")
	if err != nil || !ok {
		t.Fatalf("GetInstalled() = (%v, %v), want installed app", app, err)
	}
	ownContainer := previewTargetTestHostname(t)
	dockerHost := previewTargetDockerAPI(t, func(container string) map[string]any {
		switch container {
		case app.ContainerName:
			return previewTargetDockerInspect(map[string]string{"app-net": "172.30.0.22"})
		case ownContainer:
			return previewTargetDockerInspect(map[string]string{"aura-net": "172.29.0.9"})
		default:
			t.Errorf("unexpected container inspect %q", container)
			return map[string]any{}
		}
	})

	s := testDesktopStoreServerWithService(t, svc)
	s.Cfg.Runtime.IsDocker = true
	s.Cfg.Docker.Host = dockerHost
	target, err := s.previewTarget(t.Context(), previewResource{kind: "store", id: "node-red", port: app.Ports[0].ID})
	if err == nil || target != nil {
		t.Fatalf("previewTarget() = (%v, %v), want missing shared network error", target, err)
	}
}

func newPreviewTargetStore(t *testing.T, bindMode string, hostPort int) *desktopstore.Service {
	t.Helper()
	root := t.TempDir()
	svc, err := desktopstore.NewService(desktopstore.Config{
		DBPath:        filepath.Join(root, "desktop_store.db"),
		WorkspaceDir:  filepath.Join(root, "workspace"),
		Docker:        &serverStoreDockerAdapter{},
		Secrets:       &serverStoreSecretStore{data: map[string]string{}},
		PortAllocator: serverFixedPorts(hostPort),
		PortProbe:     func(context.Context, string, int) bool { return true },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := svc.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	op, err := svc.StartInstall(context.Background(), desktopstore.InstallRequest{AppID: "node-red", BindMode: bindMode})
	if err != nil {
		t.Fatalf("StartInstall() error = %v", err)
	}
	if err := svc.RunOperation(context.Background(), op.ID); err != nil {
		t.Fatalf("RunOperation() error = %v", err)
	}
	return svc
}

func previewTargetDockerAPI(t *testing.T, inspect func(container string) map[string]any) string {
	t.Helper()
	previous, configured := tools.CurrentRuntimePermissionsForTest()
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(func() {
		if configured {
			tools.ConfigureRuntimePermissions(previous)
		} else {
			tools.ClearRuntimePermissionsForTest()
		}
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected Docker API method %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == "/version" {
			_, _ = w.Write([]byte(`{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`))
			return
		}
		const prefix = "/v1.45/containers/"
		if !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, "/json") {
			t.Errorf("unexpected Docker API path %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		container := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), "/json")
		body, err := json.Marshal(inspect(container))
		if err != nil {
			t.Errorf("marshal inspect result: %v", err)
			http.Error(w, "fixture error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return "tcp://" + strings.TrimPrefix(server.URL, "http://")
}

func previewTargetDockerInspect(networks map[string]string) map[string]any {
	inspectNetworks := make(map[string]any, len(networks))
	for name, address := range networks {
		inspectNetworks[name] = map[string]string{"IPAddress": address}
	}
	return map[string]any{
		"NetworkSettings": map[string]any{
			"Networks": inspectNetworks,
		},
	}
}

func previewTargetTestHostname(t *testing.T) string {
	t.Helper()
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("os.Hostname() error = %v", err)
	}
	return hostname
}
