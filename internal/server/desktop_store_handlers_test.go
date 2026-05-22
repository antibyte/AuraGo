package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktopstore"
)

func TestDesktopStoreOperationContextCancelsOnShutdown(t *testing.T) {
	shutdownCh := make(chan struct{})
	ctx, cancel := desktopStoreOperationContext(shutdownCh, time.Minute)
	defer cancel()

	close(shutdownCh)
	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled {
			t.Fatalf("context error = %v, want canceled", ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("operation context did not cancel on shutdown")
	}
}

func TestDesktopStoreInstallRejectsVirtualDesktopReadOnly(t *testing.T) {
	s := testDesktopStorePolicyServer(t, true, true, false)
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/store/install", bytes.NewBufferString(`{"app_id":"node-red","bind_mode":"local"}`))
	rec := httptest.NewRecorder()

	handleDesktopStoreInstall(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestDesktopStoreInstallRejectsDockerReadOnly(t *testing.T) {
	s := testDesktopStorePolicyServer(t, false, true, true)
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/store/install", bytes.NewBufferString(`{"app_id":"node-red","bind_mode":"local"}`))
	rec := httptest.NewRecorder()

	handleDesktopStoreInstall(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestDesktopStoreOpenURLUsesRequestedPortID(t *testing.T) {
	svc, _, _ := testInstalledStoreApp(t, "emulatorjs", 19300, 19080, 14001)
	s := testDesktopStoreServerWithService(t, svc)
	req := httptest.NewRequest(http.MethodGet, "/api/desktop/store/apps/emulatorjs/open-url?port_id=frontend", nil)
	rec := httptest.NewRecorder()

	handleDesktopStoreOpenURL(s, "emulatorjs").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.URL != "http://127.0.0.1:19080/" {
		t.Fatalf("url = %q, want frontend port URL", body.URL)
	}

	badReq := httptest.NewRequest(http.MethodGet, "/api/desktop/store/apps/emulatorjs/open-url?port_id=missing", nil)
	badRec := httptest.NewRecorder()
	handleDesktopStoreOpenURL(s, "emulatorjs").ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusNotFound {
		t.Fatalf("invalid port status = %d, want 404; body=%s", badRec.Code, badRec.Body.String())
	}
}

func TestDesktopStoreCredentialsReturnsOnlyExposedGeneratedSecrets(t *testing.T) {
	svc, secrets, _ := testInstalledStoreApp(t, "code-server", 18443)
	s := testDesktopStoreServerWithService(t, svc)
	req := httptest.NewRequest(http.MethodGet, "/api/desktop/store/apps/code-server/credentials", nil)
	rec := httptest.NewRecorder()

	handleDesktopStoreAppRoute(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Credentials []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"credentials"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Credentials) != 1 || body.Credentials[0].Key != "password" {
		t.Fatalf("credentials = %#v, want only password", body.Credentials)
	}
	if body.Credentials[0].Value != secrets.data["desktop_store_code-server_password"] {
		t.Fatalf("credential value did not come from vault")
	}
}

func TestDesktopStoreBeszelAgentConfigStoresSecretsAndCreatesCompanion(t *testing.T) {
	svc, secrets, docker := testInstalledStoreApp(t, "beszel", 18090)
	s := testDesktopStoreServerWithService(t, svc)
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/store/apps/beszel/companions/agent/config", bytes.NewBufferString(`{"key":"ssh-key","token":"agent-token"}`))
	rec := httptest.NewRecorder()

	handleDesktopStoreAppRoute(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if secrets.data["desktop_store_beszel_agent_key"] != "ssh-key" || secrets.data["desktop_store_beszel_agent_token"] != "agent-token" {
		t.Fatalf("beszel secrets not stored: %#v", secrets.data)
	}
	if len(docker.created) != 2 || docker.created[1].Name != "aurago-store-beszel-agent" {
		t.Fatalf("beszel agent companion not created: %#v", docker.created)
	}
	if !containsServerTestString(docker.created[1].Env, "TOKEN=agent-token") {
		t.Fatalf("agent token missing from companion env: %#v", docker.created[1].Env)
	}
}

func TestDesktopStoreTailscaleProxySpecsIncludeEveryPublishedPort(t *testing.T) {
	apps := []desktopstore.InstalledApp{{
		AppID:            "emulatorjs",
		Status:           desktopstore.AppStatusRunning,
		TailscaleEnabled: true,
		Ports: []desktopstore.PortBinding{
			{ID: "manager", HostPort: 19300},
			{ID: "frontend", HostPort: 19080},
			{ID: "netplay", HostPort: 14001},
		},
	}}

	specs, active := desktopStoreTailscaleProxySpecs(apps)

	if len(specs) != 3 {
		t.Fatalf("specs = %#v, want 3", specs)
	}
	if _, ok := active["emulatorjs"]; !ok {
		t.Fatalf("primary active app missing: %#v", active)
	}
	want := map[string]int{"emulatorjs": 19300, "emulatorjs-frontend": 19080, "emulatorjs-netplay": 14001}
	for _, spec := range specs {
		if want[spec.ID] != spec.Port {
			t.Fatalf("unexpected proxy spec %#v, want map %#v", spec, want)
		}
		delete(want, spec.ID)
	}
	if len(want) != 0 {
		t.Fatalf("missing proxy specs: %#v", want)
	}
}

func testDesktopStorePolicyServer(t *testing.T, desktopReadOnly, dockerEnabled, dockerReadOnly bool) *Server {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.ReadOnly = desktopReadOnly
	cfg.VirtualDesktop.WorkspaceDir = filepath.Join(root, "desktop")
	cfg.SQLite.VirtualDesktopPath = filepath.Join(root, "virtual_desktop.db")
	cfg.Directories.DataDir = filepath.Join(root, "data")
	cfg.Docker.Enabled = dockerEnabled
	cfg.Docker.ReadOnly = dockerReadOnly
	return &Server{Cfg: cfg}
}

func testDesktopStoreServerWithService(t *testing.T, svc *desktopstore.Service) *Server {
	t.Helper()
	s := testDesktopStorePolicyServer(t, false, true, false)
	s.DesktopStore = svc
	return s
}

func testInstalledStoreApp(t *testing.T, appID string, ports ...int) (*desktopstore.Service, *serverStoreSecretStore, *serverStoreDockerAdapter) {
	t.Helper()
	secrets := &serverStoreSecretStore{data: map[string]string{}}
	docker := &serverStoreDockerAdapter{}
	svc, err := desktopstore.NewService(desktopstore.Config{
		DBPath:        filepath.Join(t.TempDir(), "desktop_store.db"),
		Docker:        docker,
		Secrets:       secrets,
		PortAllocator: serverFixedPorts(ports...),
		PortProbe:     func(context.Context, string, int) bool { return true },
	})
	if err != nil {
		t.Fatalf("new store service: %v", err)
	}
	if err := svc.Init(context.Background()); err != nil {
		t.Fatalf("init store service: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	op, err := svc.StartInstall(context.Background(), desktopstore.InstallRequest{AppID: appID, BindMode: desktopstore.BindModeLocal})
	if err != nil {
		t.Fatalf("start install %s: %v", appID, err)
	}
	if err := svc.RunOperation(context.Background(), op.ID); err != nil {
		t.Fatalf("run install %s: %v", appID, err)
	}
	return svc, secrets, docker
}

func serverFixedPorts(values ...int) desktopstore.PortAllocator {
	index := 0
	return func(context.Context, int) (int, error) {
		if len(values) == 0 {
			return 19000, nil
		}
		if index >= len(values) {
			return values[len(values)-1], nil
		}
		value := values[index]
		index++
		return value, nil
	}
}

type serverStoreDockerAdapter struct {
	created []desktopstore.ContainerSpec
}

func (f *serverStoreDockerAdapter) PullImage(context.Context, string) error { return nil }

func (f *serverStoreDockerAdapter) CreateContainer(_ context.Context, spec desktopstore.ContainerSpec) (string, error) {
	f.created = append(f.created, spec)
	return "container-" + spec.Name, nil
}

func (f *serverStoreDockerAdapter) CopyToContainer(context.Context, string, string, map[string]string) error {
	return nil
}

func (f *serverStoreDockerAdapter) StartContainer(context.Context, string) error { return nil }
func (f *serverStoreDockerAdapter) StopContainer(context.Context, string) error  { return nil }
func (f *serverStoreDockerAdapter) RestartContainer(context.Context, string) error {
	return nil
}
func (f *serverStoreDockerAdapter) RemoveContainer(context.Context, string, bool) error { return nil }
func (f *serverStoreDockerAdapter) RemoveVolume(context.Context, string, bool) error    { return nil }
func (f *serverStoreDockerAdapter) InspectContainer(_ context.Context, name string) (desktopstore.ContainerState, error) {
	return desktopstore.ContainerState{Name: name, Running: true, Status: "running"}, nil
}

type serverStoreSecretStore struct {
	data map[string]string
}

func (s *serverStoreSecretStore) ReadSecret(key string) (string, error) {
	value, ok := s.data[key]
	if !ok {
		return "", errors.New("secret not found")
	}
	return value, nil
}

func (s *serverStoreSecretStore) WriteSecret(key, value string) error {
	if s.data == nil {
		s.data = map[string]string{}
	}
	s.data[key] = value
	return nil
}

func (s *serverStoreSecretStore) DeleteSecret(key string) error {
	delete(s.data, key)
	return nil
}

func containsServerTestString(items []string, want string) bool {
	for _, item := range items {
		if strings.EqualFold(item, want) {
			return true
		}
	}
	return false
}
