package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"

	"gopkg.in/yaml.v3"
)

// go2RTCLifecycleDocker fakes the Docker Engine calls of the managed go2rtc
// sidecar. It keeps the labels of the created container, so the manager's own
// ownership and fingerprint checks apply.
type go2RTCLifecycleDocker struct {
	mu                        sync.Mutex
	exists, running           bool
	labels                    map[string]string
	created, started, deleted int
	lastCreate                string
	deleteStatus              int
}

func (f *go2RTCLifecycleDocker) isRunning() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running
}

// counts returns how many containers were created, started and removed so far.
func (f *go2RTCLifecycleDocker) counts() (created, started, deleted int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.created, f.started, f.deleted
}

func (f *go2RTCLifecycleDocker) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		path := r.URL.Path
		switch {
		case path == "/version":
			_, _ = w.Write([]byte(`{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`))
		case r.Method == http.MethodGet && strings.Contains(path, "/containers/") && strings.HasSuffix(path, "/json"):
			if !f.exists {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"Id":     "abc123",
				"Config": map[string]interface{}{"Labels": f.labels},
				"State":  map[string]interface{}{"Running": f.running},
			})
		case r.Method == http.MethodGet && strings.Contains(path, "/images/") && strings.HasSuffix(path, "/json"):
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/containers/create"):
			raw, _ := io.ReadAll(r.Body)
			var payload struct {
				Labels map[string]string `json:"Labels"`
			}
			_ = json.Unmarshal(raw, &payload)
			f.lastCreate = string(raw)
			f.created++
			f.exists = true
			f.running = false
			f.labels = payload.Labels
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/start"):
			f.started++
			f.running = true
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/stop"):
			f.running = false
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && strings.Contains(path, "/containers/"):
			if f.deleteStatus != 0 && f.deleteStatus != http.StatusNoContent {
				http.Error(w, `{"message":"removal refused"}`, f.deleteStatus)
				return
			}
			f.deleted++
			f.exists = false
			f.running = false
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Logf("unexpected Docker request %s %s", r.Method, r.URL.String())
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}
}

// newGo2RTCLifecycleServer builds a server whose go2rtc manager talks to a fake
// Docker engine and a go2rtc API that only answers while the fake container runs.
func newGo2RTCLifecycleServer(t *testing.T, autoStart, detecting bool) (*Server, *go2RTCLifecycleDocker) {
	t.Helper()
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	docker := &go2RTCLifecycleDocker{}
	dockerAPI := httptest.NewServer(docker.handler(t))
	t.Cleanup(dockerAPI.Close)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !docker.isRunning() {
			http.Error(w, "go2rtc is not running", http.StatusServiceUnavailable)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/go2rtc/proxy/api":
			_, _ = w.Write([]byte(`{"version":"1.9.14"}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/go2rtc/proxy/api/streams":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)
	parsed, _ := url.Parse(api.URL)
	port, _ := strconv.Atoi(parsed.Port())

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := &config.Config{ConfigPath: configPath}
	cfg.Directories.DataDir = filepath.Join(tmpDir, "data")
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://" + strings.TrimPrefix(dockerAPI.URL, "http://")
	cfg.RuView = config.DefaultRuViewConfig()
	cfg.Go2RTC = config.Go2RTCConfig{
		Enabled: true, AutoStart: autoStart, Image: config.Go2RTCDefaultImage,
		URL: api.URL, APIHostPort: port, ContainerName: "aurago_go2rtc",
		WebRTC:    config.Go2RTCWebRTCConfig{Port: 8555},
		Detection: config.NormalizeGo2RTCDetectionSettings(config.Go2RTCDetectionConfig{Enabled: true}),
		Streams: []config.Go2RTCStreamConfig{{
			ID: "front-door", Name: "Front door", Enabled: true,
			Detection: config.DefaultGo2RTCStreamDetection(detecting),
		}},
	}
	rawConfig, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, rawConfig, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	vault, err := security.NewVault(strings.Repeat("66", 32), filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if err := vault.WriteSecret(config.Go2RTCAPIPasswordVaultKey, "internal-password"); err != nil {
		t.Fatalf("seed API password: %v", err)
	}
	if err := vault.WriteSecret(config.Go2RTCStreamSourceVaultKey("front-door"), "rtsp://camera.local/live"); err != nil {
		t.Fatalf("seed stream source: %v", err)
	}
	manager := tools.NewGo2RTCManager(cfg, vault, nil, slog.Default())
	return &Server{Cfg: cfg, Vault: vault, Go2RTC: manager, Logger: slog.Default()}, docker
}

func setGo2RTCDetection(enabled bool) func(*config.Go2RTCConfig) error {
	return func(value *config.Go2RTCConfig) error {
		value.Streams[0].Detection.Enabled = enabled
		return nil
	}
}

// stopGo2RTCSidecar starts the sidecar and then stops it the way the
// administrator's stop button does.
func stopGo2RTCSidecar(t *testing.T, server *Server) {
	t.Helper()
	ctx := context.Background()
	if err := server.Go2RTC.StartContainer(ctx); err != nil {
		t.Fatalf("start sidecar: %v", err)
	}
	if err := server.Go2RTC.StopContainer(); err != nil {
		t.Fatalf("stop sidecar: %v", err)
	}
	if !server.Go2RTC.ManualStop() {
		t.Fatal("the administrator's stop was not recorded")
	}
}

// The camera app saves the config on every zone edit; with auto start on, such a
// save must not undo the administrator's stop.
func TestGo2RTCConfigSaveKeepsTheAdministratorStop(t *testing.T) {
	server, docker := newGo2RTCLifecycleServer(t, true, true)
	stopGo2RTCSidecar(t, server)
	createdBefore, startedBefore, _ := docker.counts()

	result, err := updateManagedGo2RTCConfig(context.Background(), server, func(value *config.Go2RTCConfig) error {
		value.Streams[0].Detection.Zones = []config.Go2RTCDetectionZone{{Name: "Driveway", Points: [][2]float64{{0, 0}, {1, 0}, {1, 1}}}}
		return nil
	}, nil, false)
	if err != nil {
		t.Fatalf("updateManagedGo2RTCConfig: %v", err)
	}
	if !result.Published || result.ReconcileErr != nil {
		t.Fatalf("result = %+v, want a published change without a reconcile error", result)
	}
	created, started, _ := docker.counts()
	if created != createdBefore || started != startedBefore || docker.isRunning() {
		t.Fatalf("created %d->%d started %d->%d running=%v: a zone edit started the stopped sidecar", createdBefore, created, startedBefore, started, docker.isRunning())
	}
	if !server.Go2RTC.ManualStop() {
		t.Fatal("a zone edit cleared the administrator's stop")
	}
	if got := server.Go2RTC.Config().Streams[0].Detection.Zones; len(got) != 1 {
		t.Fatalf("manager did not take over the published zones: %#v", got)
	}
}

func TestGo2RTCExplicitSetupStartsAfterAdministratorStop(t *testing.T) {
	server, docker := newGo2RTCLifecycleServer(t, false, true)
	stopGo2RTCSidecar(t, server)
	_, startedBefore, _ := docker.counts()

	result, err := updateManagedGo2RTCConfig(context.Background(), server, setGo2RTCDetection(true), nil, true)
	if err != nil || result.ReconcileErr != nil {
		t.Fatalf("update = %+v, %v", result, err)
	}
	if _, started, _ := docker.counts(); started != startedBefore+1 || !docker.isRunning() {
		t.Fatalf("started %d->%d running=%v, want an explicit start to run the sidecar", startedBefore, started, docker.isRunning())
	}
	if server.Go2RTC.ManualStop() {
		t.Fatal("an explicit start must clear the administrator's stop")
	}
}

func TestGo2RTCDetectionOffKeepsAStoppedSidecarStopped(t *testing.T) {
	server, docker := newGo2RTCLifecycleServer(t, false, true)
	stopGo2RTCSidecar(t, server)
	createdBefore, startedBefore, deletedBefore := docker.counts()

	result, err := updateManagedGo2RTCConfig(context.Background(), server, setGo2RTCDetection(false), nil, false)
	if err != nil {
		t.Fatalf("updateManagedGo2RTCConfig: %v", err)
	}
	if !result.Published || result.ReconcileErr != nil {
		t.Fatalf("result = %+v, want a published change without a reconcile error", result)
	}
	created, started, deleted := docker.counts()
	if deleted != deletedBefore+1 || created != createdBefore || started != startedBefore {
		t.Fatalf("created %d->%d started %d->%d deleted %d->%d, want the stale sidecar removed and nothing started",
			createdBefore, created, startedBefore, started, deletedBefore, deleted)
	}
	if !server.Go2RTC.ManualStop() {
		t.Fatal("the recreate cleared the administrator's stop")
	}
	if recreate, start, stop := server.Go2RTC.RuntimeTransitionPending(); recreate || start || stop {
		t.Fatalf("nothing should be pending, got recreate=%v start=%v stop=%v", recreate, start, stop)
	}
}

func TestGo2RTCDetectionOffRecreatesARunningSidecarWithBaseLimits(t *testing.T) {
	server, docker := newGo2RTCLifecycleServer(t, false, true)
	if err := server.Go2RTC.StartContainer(context.Background()); err != nil {
		t.Fatalf("start sidecar: %v", err)
	}
	if !strings.Contains(docker.lastCreate, `"NanoCpus":750000000`) {
		t.Fatalf("detecting sidecar was not created with raised limits: %s", docker.lastCreate)
	}
	createdBefore, startedBefore, deletedBefore := docker.counts()

	result, err := updateManagedGo2RTCConfig(context.Background(), server, setGo2RTCDetection(false), nil, false)
	if err != nil || result.ReconcileErr != nil {
		t.Fatalf("update = %+v, %v", result, err)
	}
	created, started, deleted := docker.counts()
	if deleted != deletedBefore+1 || created != createdBefore+1 || started != startedBefore+1 || !docker.isRunning() {
		t.Fatalf("created %d->%d started %d->%d deleted %d->%d running=%v, want a recreated running sidecar",
			createdBefore, created, startedBefore, started, deletedBefore, deleted, docker.isRunning())
	}
	if !strings.Contains(docker.lastCreate, `"NanoCpus":500000000`) || !strings.Contains(docker.lastCreate, `"Memory":268435456`) {
		t.Fatalf("recreated sidecar did not get the base limits: %s", docker.lastCreate)
	}
}

// A failed recreate queues a retry only when the sidecar was meant to run.
func TestGo2RTCFailedRecreateRetriesOnlyWhenAStartWasIntended(t *testing.T) {
	t.Run("stopped by the administrator", func(t *testing.T) {
		server, docker := newGo2RTCLifecycleServer(t, false, true)
		stopGo2RTCSidecar(t, server)
		docker.mu.Lock()
		docker.deleteStatus = http.StatusForbidden
		docker.mu.Unlock()

		result, err := updateManagedGo2RTCConfig(context.Background(), server, setGo2RTCDetection(false), nil, false)
		if err != nil || !result.Published || result.ReconcileErr == nil {
			t.Fatalf("update = %+v, %v, want a published change with a reconcile error", result, err)
		}
		if recreate, start, stop := server.Go2RTC.RuntimeTransitionPending(); recreate || start || stop {
			t.Fatalf("a retry was queued for a sidecar nobody wants running: recreate=%v start=%v stop=%v", recreate, start, stop)
		}
	})
	t.Run("running", func(t *testing.T) {
		server, docker := newGo2RTCLifecycleServer(t, false, true)
		if err := server.Go2RTC.StartContainer(context.Background()); err != nil {
			t.Fatalf("start sidecar: %v", err)
		}
		docker.mu.Lock()
		docker.deleteStatus = http.StatusForbidden
		docker.mu.Unlock()

		result, err := updateManagedGo2RTCConfig(context.Background(), server, setGo2RTCDetection(false), nil, false)
		if err != nil || !result.Published || result.ReconcileErr == nil {
			t.Fatalf("update = %+v, %v, want a published change with a reconcile error", result, err)
		}
		if recreate, _, _ := server.Go2RTC.RuntimeTransitionPending(); !recreate {
			t.Fatal("a running sidecar whose recreate failed must be retried by the background tick")
		}
	})
}
