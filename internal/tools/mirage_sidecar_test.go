package tools

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/testutil"
)

func mirageTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func mirageSidecarTestConfig(t *testing.T, dockerHost string) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = dockerHost
	m := &cfg.Vision.Mirage
	m.Enabled = true
	m.Mode = config.MirageModeFallback
	m.ContainerName = config.MirageDefaultContainerName
	m.ContainerPort = 8791
	m.Image = "python:3.12-slim"
	m.DataPath = t.TempDir()
	m.GPU = "none"
	m.MaxObjects = 20
	m.MaxTextLines = 30
	return cfg
}

func readyHealthServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ready":true,"backend":"cpu","gpu_requested":"none"}`)
	}))
	prev := mirageBaseURLFor
	mirageBaseURLFor = func(*config.Config) string { return server.URL + "/v1" }
	t.Cleanup(func() { mirageBaseURLFor = prev; server.Close() })
	return server
}

func TestMirageNormalizeEOLRemovesCarriageReturns(t *testing.T) {
	got := string(mirageNormalizeEOL([]byte("FROM x\r\nARG A\r\nRUN y\n")))
	if got != "FROM x\nARG A\nRUN y\n" {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(string(mirageDockerfile()), "\r") || strings.Contains(string(mirageServerPy()), "\r") {
		t.Fatal("embedded build inputs must be LF-only")
	}
}

func TestMirageImageTagChangesWithTheBaseImage(t *testing.T) {
	a, b := mirageImageTag("python:3.12-slim"), mirageImageTag("python:3.13-slim")
	if a == b || !strings.HasPrefix(a, "aurago/mirage:") {
		t.Fatalf("tags %q / %q", a, b)
	}
	if a != mirageImageTag("python:3.12-slim") {
		t.Fatal("tag must be stable")
	}
}

func TestMirageApplyGPUNvidiaUsesDeviceRequests(t *testing.T) {
	host := map[string]interface{}{}
	mirageApplyGPU(host, GPUInfo{Backend: "nvidia"})
	reqs, ok := host["DeviceRequests"].([]map[string]interface{})
	if !ok || len(reqs) != 1 || reqs[0]["Driver"] != "nvidia" {
		t.Fatalf("DeviceRequests = %#v", host["DeviceRequests"])
	}
	if _, has := host["Devices"]; has {
		t.Fatal("nvidia must not add raw devices")
	}
}

func TestMirageApplyGPUDriNodesBecomeDevices(t *testing.T) {
	host := map[string]interface{}{}
	mirageApplyGPU(host, GPUInfo{Backend: "vulkan", Devices: []string{"/dev/dri/renderD128"}})
	devices, ok := host["Devices"].([]map[string]string)
	if !ok || len(devices) != 1 || devices[0]["PathOnHost"] != "/dev/dri/renderD128" || devices[0]["PathInContainer"] != "/dev/dri/renderD128" {
		t.Fatalf("Devices = %#v", host["Devices"])
	}
	none := map[string]interface{}{}
	mirageApplyGPU(none, GPUInfo{Backend: "none"})
	if len(none) != 0 {
		t.Fatalf("none must not touch the host config: %#v", none)
	}
}

func TestMirageDetectGPUNoneSkipsDetection(t *testing.T) {
	if got := mirageDetectGPU(" None "); got.Backend != "none" {
		t.Fatalf("backend = %q", got.Backend)
	}
}

func TestMirageStatusDisabled(t *testing.T) {
	if got := MirageStatus(MirageStatusTarget{}); got.Status != "disabled" {
		t.Fatalf("status = %q", got.Status)
	}
}

func TestMirageStatusReportsTheEngineBackend(t *testing.T) {
	server := readyHealthServer(t)
	got := MirageStatus(MirageStatusTarget{Enabled: true, HealthURL: server.URL + "/health"})
	if got.Status != "running" || !got.Ready || got.Backend != "cpu" {
		t.Fatalf("status = %+v", got)
	}
}

func TestMirageStatusStoppedWhenNothingAnswers(t *testing.T) {
	got := MirageStatus(MirageStatusTarget{Enabled: true, HealthURL: "http://127.0.0.1:1/health"})
	if got.Status != "stopped" {
		t.Fatalf("status = %q", got.Status)
	}
}

type mirageDockerRecorder struct {
	built, created, started, deleted atomic.Int32
	createBody                       map[string]interface{}
}

func newMirageFakeDocker(t *testing.T, rec *mirageDockerRecorder, existing func() (code int, body string), onDelete func()) string {
	t.Helper()
	return fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case r.Method == http.MethodGet && strings.Contains(path, "/images/") && strings.HasSuffix(path, "/json"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/build"):
			rec.built.Add(1)
			_, _ = io.WriteString(w, "{\"stream\":\"ok\"}\n")
		case r.Method == http.MethodGet && strings.Contains(path, "/containers/aurago_mirage/json"):
			code, body := existing()
			w.WriteHeader(code)
			_, _ = io.WriteString(w, body)
		case r.Method == http.MethodDelete && strings.Contains(path, "/containers/aurago_mirage"):
			rec.deleted.Add(1)
			if onDelete != nil {
				onDelete()
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/containers/create"):
			rec.created.Add(1)
			_ = json.NewDecoder(r.Body).Decode(&rec.createBody)
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"Id":"abc"}`)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/containers/aurago_mirage/start"):
			rec.started.Add(1)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestEnsureMirageRunningBuildsAndCreatesTheContainer(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	readyHealthServer(t)
	mirageBuildBackoff.reset()
	rec := &mirageDockerRecorder{}
	host := newMirageFakeDocker(t, rec, func() (int, string) { return http.StatusNotFound, "" }, nil)
	cfg := mirageSidecarTestConfig(t, host)

	if err := EnsureMirageRunning(cfg, mirageTestLogger(), true); err != nil {
		t.Fatalf("EnsureMirageRunning: %v", err)
	}
	if rec.built.Load() != 1 || rec.created.Load() != 1 || rec.started.Load() != 1 {
		t.Fatalf("built=%d created=%d started=%d", rec.built.Load(), rec.created.Load(), rec.started.Load())
	}
	labels, _ := rec.createBody["Labels"].(map[string]interface{})
	if labels[mirageFingerprintLabel] == nil {
		t.Fatalf("labels = %#v, want the fingerprint label", labels)
	}
	hostCfg, _ := rec.createBody["HostConfig"].(map[string]interface{})
	if caps, _ := hostCfg["CapDrop"].([]interface{}); len(caps) != 1 || caps[0] != "ALL" {
		t.Fatalf("CapDrop = %#v", hostCfg["CapDrop"])
	}
	ports, _ := hostCfg["PortBindings"].(map[string]interface{})
	if _, ok := ports["8080/tcp"]; !ok {
		t.Fatalf("PortBindings = %#v", ports)
	}
	binds, _ := hostCfg["Binds"].([]interface{})
	if len(binds) != 1 || !strings.HasSuffix(binds[0].(string), ":/data") {
		t.Fatalf("Binds = %#v", binds)
	}
}

func TestEnsureMirageRunningReplacesAContainerWithAStaleFingerprint(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	readyHealthServer(t)
	mirageBuildBackoff.reset()
	rec := &mirageDockerRecorder{}
	var deleted atomic.Bool
	host := newMirageFakeDocker(t, rec, func() (int, string) {
		if deleted.Load() {
			return http.StatusNotFound, ""
		}
		return http.StatusOK, `{"State":{"Running":true},"Config":{"Labels":{"` + mirageFingerprintLabel + `":"stale"}}}`
	}, func() { deleted.Store(true) }) // after DELETE the container is gone
	cfg := mirageSidecarTestConfig(t, host)
	if err := EnsureMirageRunning(cfg, mirageTestLogger(), true); err != nil {
		t.Fatalf("EnsureMirageRunning: %v", err)
	}
	if rec.deleted.Load() != 1 || rec.created.Load() != 1 {
		t.Fatalf("deleted=%d created=%d", rec.deleted.Load(), rec.created.Load())
	}
}

func TestEnsureMirageRunningKeepsAMatchingRunningContainer(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	readyHealthServer(t)
	mirageBuildBackoff.reset()
	rec := &mirageDockerRecorder{}
	var fp string
	host := newMirageFakeDocker(t, rec, func() (int, string) {
		return http.StatusOK, `{"State":{"Running":true},"Config":{"Labels":{"` + mirageFingerprintLabel + `":"` + fp + `"}}}`
	}, nil)
	cfg := mirageSidecarTestConfig(t, host)
	fp = mirageCurrentFingerprint(cfg)
	if err := EnsureMirageRunning(cfg, mirageTestLogger(), true); err != nil {
		t.Fatalf("EnsureMirageRunning: %v", err)
	}
	if rec.deleted.Load() != 0 || rec.created.Load() != 0 {
		t.Fatalf("deleted=%d created=%d, want the container left alone", rec.deleted.Load(), rec.created.Load())
	}
}

func TestEnsureMirageRunningDoesNothingWhenDisabled(t *testing.T) {
	cfg := &config.Config{}
	if err := EnsureMirageRunning(cfg, mirageTestLogger(), true); err != nil {
		t.Fatalf("err = %v", err)
	}
}
