package tools

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

// go2RTCLifecycleDocker is a small stateful fake of the Docker Engine calls the
// go2rtc sidecar lifecycle makes. The go2rtc API test server next to it only
// answers while the fake container runs.
type go2RTCLifecycleDocker struct {
	mu          sync.Mutex
	exists      bool
	running     bool
	fingerprint string
	owner       string

	created, started, stopped, deleted, infoCalls int
	lastCreate                                    string

	requests      int      // every request that reached the fake
	inspectNames  []string // container names asked for in inspect calls
	ncpu          int      // NCPU reported by /info
	infoStatus    int      // 0 = 200
	inspectStatus int      // 0 = answer from the container state
	inspectFail1  int      // number of leading inspect calls answered with 403
	createStatus  int      // 0 = 201
	createBody    string   // body of a failing create
	deleteStatus  int      // 0 = 204

	// stopGate, when set, holds every stop call until it is closed; stopEntered
	// receives one value as a stop call starts to wait.
	stopGate    chan struct{}
	stopEntered chan struct{}
}

func (f *go2RTCLifecycleDocker) isRunning() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running
}

func (f *go2RTCLifecycleDocker) counts() (created, started, deleted int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.created, f.started, f.deleted
}

func (f *go2RTCLifecycleDocker) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.requests++
		path := r.URL.Path
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/info"):
			f.infoCalls++
			if f.infoStatus != 0 && f.infoStatus != http.StatusOK {
				http.Error(w, `{"message":"info denied"}`, f.infoStatus)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"NCPU": f.ncpu})
		case r.Method == http.MethodGet && strings.Contains(path, "/containers/") && strings.HasSuffix(path, "/json"):
			f.inspectNames = append(f.inspectNames, strings.TrimSuffix(path[strings.Index(path, "/containers/")+len("/containers/"):], "/json"))
			if f.inspectFail1 > 0 {
				f.inspectFail1--
				http.Error(w, `{"message":"inspect denied"}`, http.StatusForbidden)
				return
			}
			if f.inspectStatus != 0 && f.inspectStatus != http.StatusOK {
				http.Error(w, `{"message":"inspect denied"}`, f.inspectStatus)
				return
			}
			if !f.exists {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"Id": "abc123",
				"Config": map[string]interface{}{"Labels": map[string]string{
					"aurago.managed":       "go2rtc",
					go2RTCOwnerLabel:       f.owner,
					go2RTCFingerprintLabel: f.fingerprint,
				}},
				"State": map[string]interface{}{"Running": f.running},
			})
		case r.Method == http.MethodGet && strings.Contains(path, "/images/") && strings.HasSuffix(path, "/json"):
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/containers/create"):
			raw, _ := io.ReadAll(r.Body)
			if f.createStatus != 0 && f.createStatus != http.StatusCreated {
				w.WriteHeader(f.createStatus)
				_, _ = w.Write([]byte(f.createBody))
				return
			}
			var payload struct {
				Labels map[string]string `json:"Labels"`
			}
			_ = json.Unmarshal(raw, &payload)
			f.lastCreate = string(raw)
			f.created++
			f.exists = true
			f.running = false
			f.fingerprint = payload.Labels[go2RTCFingerprintLabel]
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/start"):
			f.started++
			f.running = true
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/stop"):
			if gate := f.stopGate; gate != nil {
				f.stopEntered <- struct{}{}
				f.mu.Unlock()
				<-gate
				f.mu.Lock()
			}
			f.stopped++
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

type go2RTCLifecycleHarness struct {
	docker  *go2RTCLifecycleDocker
	cfg     *config.Config
	manager *Go2RTCManager
}

func newGo2RTCLifecycleHarness(t *testing.T, autoStart bool, detectingCameras int) *go2RTCLifecycleHarness {
	t.Helper()
	previous, configured := currentRuntimePermissions()
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})
	t.Cleanup(func() {
		if configured {
			ConfigureRuntimePermissions(previous)
		} else {
			ClearRuntimePermissionsForTest()
		}
	})
	docker := &go2RTCLifecycleDocker{}
	dockerHost := fakeDockerHost(t, docker.handler(t))
	api := newDockerAPITestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !docker.isRunning() {
			http.Error(w, "go2rtc is not running", http.StatusServiceUnavailable)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/go2rtc/proxy/api":
			_, _ = w.Write([]byte(`{"version":"1.9.14"}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/go2rtc/proxy/api/streams":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/go2rtc/proxy/api/streams":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/go2rtc/proxy/api/preload":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{}`))
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)
	parsed, _ := url.Parse(api.URL)
	port, _ := strconv.Atoi(parsed.Port())

	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Docker.Host = dockerHost
	cfg.Go2RTC = detectingGo2RTCConfig(detectingCameras)
	cfg.Go2RTC.AutoStart = autoStart
	cfg.Go2RTC.URL = api.URL
	cfg.Go2RTC.APIHostPort = port
	cfg.Go2RTC.WebRTC = config.Go2RTCWebRTCConfig{Port: 8555}
	for i := range cfg.Go2RTC.Streams {
		cfg.Go2RTC.Streams[i].Source = "rtsp://camera.local/live"
	}
	manager := NewGo2RTCManager(cfg, nil, nil, nil)
	docker.owner = manager.go2RTCOwner()
	return &go2RTCLifecycleHarness{docker: docker, cfg: cfg, manager: manager}
}

// seedContainer makes the fake Docker hold a sidecar that was created for the
// harness config.
func (h *go2RTCLifecycleHarness) seedContainer(running bool) {
	goCfg := h.manager.Config()
	h.docker.mu.Lock()
	defer h.docker.mu.Unlock()
	h.docker.exists = true
	h.docker.running = running
	h.docker.fingerprint = go2RTCFingerprint(goCfg, renderGo2RTCConfig(goCfg), h.manager.go2RTCOwner())
}

func (h *go2RTCLifecycleHarness) setManualStop(stopped bool) {
	h.manager.mu.Lock()
	h.manager.manualStop = stopped
	h.manager.mu.Unlock()
}

// withDetectingCameras is the harness config with detection left on for the
// first n streams only.
func (h *go2RTCLifecycleHarness) withDetectingCameras(n int) *config.Config {
	next := *h.cfg
	next.Go2RTC.Streams = append([]config.Go2RTCStreamConfig(nil), h.cfg.Go2RTC.Streams...)
	for i := range next.Go2RTC.Streams {
		next.Go2RTC.Streams[i].Detection.Enabled = i < n
	}
	return &next
}

func TestGo2RTCReconfigureKeepsStoppedSidecarStopped(t *testing.T) {
	h := newGo2RTCLifecycleHarness(t, false, 1)
	h.seedContainer(false) // stopped by the administrator
	h.setManualStop(true)

	if err := h.manager.ReconfigureContainer(context.Background(), h.cfg, h.withDetectingCameras(0), false); err != nil {
		t.Fatalf("ReconfigureContainer: %v", err)
	}
	created, started, deleted := h.docker.counts()
	if deleted != 1 || created != 0 || started != 0 {
		t.Fatalf("stopped sidecar: deleted=%d created=%d started=%d, want the stale container removed and nothing started", deleted, created, started)
	}
	if !h.manager.ManualStop() {
		t.Fatal("a recreate that does not start must leave the administrator's stop in place")
	}
	if status := h.manager.currentStatus(); status.APIUsable || status.LastError != "" {
		t.Fatalf("status after a quiet recreate = %+v", status)
	}

	// The next start builds the container from the new (base) limits.
	if err := h.manager.StartContainer(context.Background()); err != nil {
		t.Fatalf("StartContainer: %v", err)
	}
	created, started, _ = h.docker.counts()
	if created != 1 || started != 1 || !strings.Contains(h.docker.lastCreate, `"NanoCpus":500000000`) || !strings.Contains(h.docker.lastCreate, `"Memory":268435456`) {
		t.Fatalf("next start: created=%d started=%d spec=%s", created, started, h.docker.lastCreate)
	}
}

func TestGo2RTCReconfigureRecreatesAndRestartsRunningSidecar(t *testing.T) {
	h := newGo2RTCLifecycleHarness(t, false, 1)
	h.seedContainer(true)

	if err := h.manager.ReconfigureContainer(context.Background(), h.cfg, h.withDetectingCameras(0), false); err != nil {
		t.Fatalf("ReconfigureContainer: %v", err)
	}
	created, started, deleted := h.docker.counts()
	if deleted != 1 || created != 1 || started != 1 || !h.docker.isRunning() {
		t.Fatalf("running sidecar: deleted=%d created=%d started=%d running=%v", deleted, created, started, h.docker.isRunning())
	}
	if !strings.Contains(h.docker.lastCreate, `"NanoCpus":500000000`) {
		t.Fatalf("recreated sidecar did not get the base limits: %s", h.docker.lastCreate)
	}
}

func TestGo2RTCReconfigureStartDecision(t *testing.T) {
	for _, tc := range []struct {
		name       string
		autoStart  bool
		forceStart bool
		manualStop bool
		running    bool
		wantStart  bool
	}{
		{"forced start overrides an administrator stop", false, true, true, false, true},
		{"auto start brings a stopped sidecar back", true, false, false, false, true},
		{"auto start respects the administrator stop", true, false, true, false, false},
		{"no auto start, not running, no manual stop", false, false, false, false, false},
		{"running sidecar keeps running", false, false, false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newGo2RTCLifecycleHarness(t, tc.autoStart, 1)
			h.seedContainer(tc.running)
			h.setManualStop(tc.manualStop)
			if err := h.manager.ReconfigureContainer(context.Background(), h.cfg, h.withDetectingCameras(0), tc.forceStart); err != nil {
				t.Fatalf("ReconfigureContainer: %v", err)
			}
			created, started, deleted := h.docker.counts()
			if deleted != 1 {
				t.Fatalf("deleted = %d, want the stale container removed", deleted)
			}
			if tc.wantStart && (created != 1 || started != 1) {
				t.Fatalf("created=%d started=%d, want a started replacement", created, started)
			}
			if !tc.wantStart && (created != 0 || started != 0) {
				t.Fatalf("created=%d started=%d, want nothing started", created, started)
			}
			if tc.wantStart && h.manager.ManualStop() {
				t.Fatal("a start must clear the manual stop")
			}
			if !tc.wantStart && tc.manualStop && !h.manager.ManualStop() {
				t.Fatal("a recreate without start must keep the manual stop")
			}
		})
	}
}

func TestGo2RTCReconfigureFailureReportsWhetherAStartWasIntended(t *testing.T) {
	for _, tc := range []struct {
		name       string
		running    bool
		manualStop bool
		force      bool
		want       bool
	}{
		{"stopped by the administrator", false, true, false, false},
		{"running sidecar", true, false, false, true},
		{"forced start", false, true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newGo2RTCLifecycleHarness(t, false, 1)
			h.seedContainer(tc.running)
			h.setManualStop(tc.manualStop)
			h.docker.deleteStatus = http.StatusForbidden

			err := h.manager.ReconfigureContainer(context.Background(), h.cfg, h.withDetectingCameras(0), tc.force)
			if err == nil {
				t.Fatal("ReconfigureContainer succeeded although Docker refused the removal")
			}
			var reconfigureErr *Go2RTCReconfigureError
			if !errors.As(err, &reconfigureErr) {
				t.Fatalf("error %T %v is not a *Go2RTCReconfigureError", err, err)
			}
			if reconfigureErr.StartIntended != tc.want {
				t.Fatalf("StartIntended = %v, want %v", reconfigureErr.StartIntended, tc.want)
			}
			if !strings.Contains(err.Error(), "removal refused") && !strings.Contains(err.Error(), "HTTP 403") {
				t.Fatalf("error lost the Docker failure: %v", err)
			}
		})
	}
}

func TestGo2RTCTickRetryNeverStartsWhatTheAdministratorStopped(t *testing.T) {
	h := newGo2RTCLifecycleHarness(t, false, 1)
	h.seedContainer(false)
	h.setManualStop(true)
	h.manager.SetRuntimeTransitionPending(true, false, false)

	h.manager.reconcileTick(context.Background())
	created, started, _ := h.docker.counts()
	if created != 0 || started != 0 {
		t.Fatalf("retry tick started a stopped sidecar: created=%d started=%d", created, started)
	}
	if recreate, start, stop := h.manager.RuntimeTransitionPending(); recreate || start || stop {
		t.Fatalf("the administrator's stop must drop the pending retry, got recreate=%v start=%v stop=%v", recreate, start, stop)
	}
	if !h.manager.ManualStop() {
		t.Fatal("the retry tick cleared the manual stop")
	}

	// Without an administrator stop the pending recreate still starts the sidecar.
	h.setManualStop(false)
	h.manager.SetRuntimeTransitionPending(true, false, false)
	h.manager.reconcileTick(context.Background())
	created, started, _ = h.docker.counts()
	if created != 1 || started != 1 {
		t.Fatalf("pending recreate without a stop: created=%d started=%d, want a started sidecar", created, started)
	}
}

func TestGo2RTCApplyConfiguredLeavesStoppedSidecarAlone(t *testing.T) {
	t.Run("auto start does not clear an administrator stop", func(t *testing.T) {
		h := newGo2RTCLifecycleHarness(t, true, 1)
		h.seedContainer(false)
		h.setManualStop(true)
		h.manager.setAPIStatus(false, "", "")

		if err := h.manager.ApplyConfigured(context.Background(), false); err != nil {
			t.Fatalf("ApplyConfigured: %v", err)
		}
		created, started, _ := h.docker.counts()
		if created != 0 || started != 0 || !h.manager.ManualStop() {
			t.Fatalf("created=%d started=%d manualStop=%v, want a sidecar that stays stopped", created, started, h.manager.ManualStop())
		}
		if status := h.manager.currentStatus(); status.LastError != "" {
			t.Fatalf("a stopped sidecar must not surface a reconcile error: %+v", status)
		}
	})
	t.Run("no auto start and a stopped container is not an error", func(t *testing.T) {
		h := newGo2RTCLifecycleHarness(t, false, 1)
		h.seedContainer(false)
		if err := h.manager.ApplyConfigured(context.Background(), false); err != nil {
			t.Fatalf("ApplyConfigured: %v", err)
		}
		if created, started, _ := h.docker.counts(); created != 0 || started != 0 {
			t.Fatalf("created=%d started=%d, want nothing started", created, started)
		}
		if status := h.manager.currentStatus(); status.LastError != "" {
			t.Fatalf("a stopped sidecar must not surface a reconcile error: %+v", status)
		}
	})
	t.Run("a Docker that cannot say whether the sidecar runs still reports the failure", func(t *testing.T) {
		h := newGo2RTCLifecycleHarness(t, false, 1)
		h.seedContainer(false)
		h.docker.inspectStatus = http.StatusForbidden
		if err := h.manager.ApplyConfigured(context.Background(), false); err == nil {
			t.Fatal("neither the API nor Docker answered, but ApplyConfigured reported success")
		}
	})
}

func TestGo2RTCApplyConfiguredStartsWhenIntended(t *testing.T) {
	for _, tc := range []struct {
		name       string
		autoStart  bool
		force      bool
		manualStop bool
	}{
		{"auto start without a stop", true, false, false},
		{"forced start overrides the stop", false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newGo2RTCLifecycleHarness(t, tc.autoStart, 1)
			h.seedContainer(false)
			h.setManualStop(tc.manualStop)
			if err := h.manager.ApplyConfigured(context.Background(), tc.force); err != nil {
				t.Fatalf("ApplyConfigured: %v", err)
			}
			if _, started, _ := h.docker.counts(); started != 1 || !h.docker.isRunning() {
				t.Fatalf("started=%d running=%v, want the sidecar started", started, h.docker.isRunning())
			}
			if h.manager.ManualStop() {
				t.Fatal("starting the sidecar must clear the manual stop")
			}
		})
	}
}

func TestGo2RTCApplyConfiguredReconcilesRunningSidecar(t *testing.T) {
	h := newGo2RTCLifecycleHarness(t, false, 1)
	h.seedContainer(true)
	if err := h.manager.ApplyConfigured(context.Background(), false); err != nil {
		t.Fatalf("ApplyConfigured: %v", err)
	}
	if created, started, deleted := h.docker.counts(); created != 0 || started != 0 || deleted != 0 {
		t.Fatalf("running sidecar was touched: created=%d started=%d deleted=%d", created, started, deleted)
	}
	if status := h.manager.currentStatus(); !status.APIUsable {
		t.Fatalf("running sidecar was not probed: %+v", status)
	}
}

func TestGo2RTCContainerPayloadClampsCPUsToTheHost(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Go2RTC = detectingGo2RTCConfig(3) // 1.25 CPUs
	manager := NewGo2RTCManager(cfg, nil, nil, nil)
	configPath := t.TempDir() + "/go2rtc.yaml"
	nanoCPUs := func(hostCPUs int) string {
		t.Helper()
		payload, err := manager.go2RTCContainerPayload(cfg.Go2RTC, configPath, "internal-password", "fingerprint", hostCPUs)
		if err != nil {
			t.Fatalf("go2RTCContainerPayload: %v", err)
		}
		encoded, _ := json.Marshal(payload)
		start := strings.Index(string(encoded), `"NanoCpus":`)
		if start < 0 {
			t.Fatalf("no NanoCpus in %s", encoded)
		}
		rest := string(encoded)[start+len(`"NanoCpus":`):]
		end := strings.IndexAny(rest, ",}")
		return rest[:end]
	}
	for _, tc := range []struct {
		hostCPUs int
		want     string
	}{
		{1, "1000000000"},
		{2, "1250000000"},
		{16, "1250000000"},
		{0, "1250000000"},
		{-1, "1250000000"},
	} {
		if got := nanoCPUs(tc.hostCPUs); got != tc.want {
			t.Fatalf("host CPUs %d: NanoCpus = %s, want %s", tc.hostCPUs, got, tc.want)
		}
	}
}

func TestGo2RTCStartContainerClampsCPUsToDockerNCPU(t *testing.T) {
	for _, tc := range []struct {
		name      string
		cameras   int
		ncpu      int
		infoCode  int
		wantNano  string
		wantInfo  bool
		wantMemMB string
	}{
		{"one CPU host with three cameras", 3, 1, 0, `"NanoCpus":1000000000`, true, `"Memory":536870912`},
		{"four CPU host with six cameras", 6, 4, 0, `"NanoCpus":2000000000`, true, `"Memory":536870912`},
		{"info denied by a socket proxy", 3, 0, http.StatusForbidden, `"NanoCpus":1250000000`, true, `"Memory":536870912`},
		{"info without a CPU count", 3, 0, 0, `"NanoCpus":1250000000`, true, `"Memory":536870912`},
		{"limit within every host's range skips the lookup", 2, 1, 0, `"NanoCpus":1000000000`, false, `"Memory":536870912`},
		{"base limits skip the lookup", 0, 1, 0, `"NanoCpus":500000000`, false, `"Memory":268435456`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newGo2RTCLifecycleHarness(t, true, tc.cameras)
			h.docker.ncpu = tc.ncpu
			h.docker.infoStatus = tc.infoCode
			if err := h.manager.StartContainer(context.Background()); err != nil {
				t.Fatalf("StartContainer: %v", err)
			}
			if !strings.Contains(h.docker.lastCreate, tc.wantNano) || !strings.Contains(h.docker.lastCreate, tc.wantMemMB) {
				t.Fatalf("container spec %s, want %s and %s", h.docker.lastCreate, tc.wantNano, tc.wantMemMB)
			}
			if asked := h.docker.infoCalls > 0; asked != tc.wantInfo {
				t.Fatalf("/info asked = %v (%d calls), want %v", asked, h.docker.infoCalls, tc.wantInfo)
			}
		})
	}
}

// The clamp is a create-time detail: the fingerprint stays config-based, so a
// 1-CPU host does not recreate the clamped container on every reconcile tick.
func TestGo2RTCClampedSidecarIsNotRecreatedByTheNextTick(t *testing.T) {
	h := newGo2RTCLifecycleHarness(t, true, 3)
	h.docker.ncpu = 1
	h.manager.reconcileTick(context.Background())
	for i := 0; i < 3; i++ {
		h.manager.reconcileTick(context.Background())
	}
	created, started, deleted := h.docker.counts()
	if created != 1 || started != 1 || deleted != 0 {
		t.Fatalf("created=%d started=%d deleted=%d, want one container created and left alone", created, started, deleted)
	}
}

func TestGo2RTCCreateErrorCarriesTheDaemonMessage(t *testing.T) {
	const password = "internal-password"
	for _, tc := range []struct {
		name     string
		password string // replaces the harness API password when set
		body     string
		want     string
		forbid   string
	}{
		{
			name: "json message",
			body: `{"message":"range of CPUs is from 0.01 to 1.00, as there are only 1 CPUs available"}`,
			want: "range of CPUs is from 0.01 to 1.00",
		},
		{
			name:   "multi-line text with control characters and the credential",
			body:   "bad request\r\nAURAGO_GO2RTC_API_PASSWORD=" + password + "\x1b[31m\n" + strings.Repeat("x", 1000),
			want:   "bad request",
			forbid: password,
		},
		{
			// Shorter than the scrubber's minimum, so only the explicit replacement
			// catches it; it must happen before the 300-byte cut or a prefix stays.
			name:     "unregistered credential across the cut",
			password: "pw-7chr",
			body:     strings.Repeat("a", 297) + "pw-7chr" + "-tail",
			want:     "aaaaaaaaaa",
			forbid:   "pw-",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newGo2RTCLifecycleHarness(t, true, 0)
			if tc.password != "" {
				h.cfg.Go2RTC.APIPassword = tc.password
				h.manager.Configure(h.cfg)
			}
			h.docker.createStatus = http.StatusBadRequest
			h.docker.createBody = tc.body
			err := h.manager.StartContainer(context.Background())
			if err == nil {
				t.Fatal("StartContainer succeeded although create failed")
			}
			text := err.Error()
			if !strings.Contains(text, "create go2rtc container returned HTTP 400") || !strings.Contains(text, tc.want) {
				t.Fatalf("error = %q, want the status and %q", text, tc.want)
			}
			if tc.forbid != "" && strings.Contains(text, tc.forbid) {
				t.Fatalf("error leaks %q: %q", tc.forbid, text)
			}
			if strings.ContainsAny(text, "\r\n\x1b") {
				t.Fatalf("error is not a single printable line: %q", text)
			}
			if len(text) > len("create go2rtc container returned HTTP 400: ")+300+3 {
				t.Fatalf("error is %d bytes long, want the daemon text bounded to about 300 bytes", len(text))
			}
		})
	}
}

// The explicit secrets are replaced before the text is cut to 300 bytes, so a
// secret that straddles the cut never leaves a prefix behind (raw and JSON body).
func TestGo2RTCDockerErrorDetailReplacesSecretsBeforeTheCut(t *testing.T) {
	const secret = "s2t4-unregistered-secret-0123456789abcdef"
	for offset := 280; offset <= 300; offset++ {
		for _, body := range []string{
			strings.Repeat("a", offset) + secret + strings.Repeat("b", 50),
			`{"message":"` + strings.Repeat("a", offset) + secret + `"}`,
		} {
			out := go2RTCDockerErrorDetail([]byte(body), secret)
			for n := 4; n <= len(secret); n++ {
				if strings.Contains(out, secret[:n]) {
					t.Fatalf("offset %d leaks %q: %q", offset, secret[:n], out)
				}
			}
			if len(out) > len(": ")+300+3 {
				t.Fatalf("offset %d: %d bytes", offset, len(out))
			}
		}
	}
	out := go2RTCDockerErrorDetail([]byte(strings.Repeat("\u00e4", 200)))
	if !strings.HasSuffix(out, "...") || !strings.HasPrefix(out, ": \u00e4") {
		t.Fatalf("multi-byte cut: %q", out)
	}
	if got := go2RTCDockerErrorDetail(nil); got != "" {
		t.Fatalf("empty body: %q", got)
	}
}

// An administrator stop holds the lifecycle lock inside its Docker call and sets
// the manual stop only at the end. A tick that read its flags before the stop
// began must see the stop once it gets the lock, and neither recreate nor start
// the container.
func TestGo2RTCTickDoesNotUndoAnInFlightAdministratorStop(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		autoStart              bool
		pendingRecreate, start bool
	}{
		{"pending recreate retry", false, true, false},
		{"pending start retry", false, false, true},
		{"auto start tick", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newGo2RTCLifecycleHarness(t, tc.autoStart, 1)
			h.seedContainer(true)
			h.docker.stopGate = make(chan struct{})
			h.docker.stopEntered = make(chan struct{}, 4)
			h.manager.SetRuntimeTransitionPending(tc.pendingRecreate, tc.start, false)

			stopDone := make(chan error, 1)
			go func() { stopDone <- h.manager.StopContainer() }()
			select {
			case <-h.docker.stopEntered:
			case <-time.After(10 * time.Second):
				t.Fatal("the stop never reached Docker")
			}
			tickDone := make(chan struct{})
			go func() {
				h.manager.reconcileTick(context.Background())
				close(tickDone)
			}()
			time.Sleep(200 * time.Millisecond) // the tick has read its flags and waits for the lock
			close(h.docker.stopGate)
			if err := <-stopDone; err != nil {
				t.Fatalf("StopContainer: %v", err)
			}
			select {
			case <-tickDone:
			case <-time.After(10 * time.Second):
				t.Fatal("the tick never finished")
			}

			created, started, deleted := h.docker.counts()
			if created != 0 || started != 0 || deleted != 0 || h.docker.isRunning() {
				t.Fatalf("created=%d started=%d deleted=%d running=%v: the tick undid the stop", created, started, deleted, h.docker.isRunning())
			}
			if !h.manager.ManualStop() {
				t.Fatal("the tick cleared the administrator's stop")
			}
			if recreate, start, stop := h.manager.RuntimeTransitionPending(); recreate || start || stop {
				t.Fatalf("pending flags survived the stop: recreate=%v start=%v stop=%v", recreate, start, stop)
			}
		})
	}
}

// When Docker cannot say whether the sidecar runs, an administrator's stop still
// wins; without one the pre-existing start after a recreate stays.
func TestGo2RTCStartIntentWhenDockerCannotSay(t *testing.T) {
	h := newGo2RTCLifecycleHarness(t, false, 1)
	h.seedContainer(false)
	h.docker.inspectStatus = http.StatusForbidden
	for _, tc := range []struct {
		name                         string
		autoStart, force, manualStop bool
		want                         bool
	}{
		{"no stop keeps the start after a recreate", false, false, false, true},
		{"administrator stop wins over an unknown state", false, false, true, false},
		{"forced start overrides the stop", false, true, true, true},
		{"auto start without a stop", true, false, false, true},
		{"auto start with a stop", true, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h.setManualStop(tc.manualStop)
			if got := h.manager.startIntendedLocked(tc.autoStart, tc.force); got != tc.want {
				t.Fatalf("startIntendedLocked(auto=%v, force=%v) with manualStop=%v = %v, want %v", tc.autoStart, tc.force, tc.manualStop, got, tc.want)
			}
		})
	}
}

// A container under the sidecar's name that belongs to someone else is not a
// verifiably stopped sidecar: the failure must stay visible.
func TestGo2RTCForeignContainerIsNotVerifiablyStopped(t *testing.T) {
	h := newGo2RTCLifecycleHarness(t, false, 1)
	h.seedContainer(false)
	h.docker.mu.Lock()
	h.docker.owner = "sha256:another-instance"
	h.docker.mu.Unlock()

	if running, known := h.manager.sidecarRunning(); running || known {
		t.Fatalf("sidecarRunning = running %v known %v, want an unknown state for a foreign container", running, known)
	}
	if err := h.manager.ApplyConfigured(context.Background(), false); err == nil {
		t.Fatal("ApplyConfigured reported success next to a container owned by someone else")
	}
	h.manager.reconcileTick(context.Background())
	if h.manager.currentStatus().LastError == "" {
		t.Fatal("the tick hid a container name collision")
	}
}

// The auto_start:false tick shows no error for a sidecar that is verifiably not
// running, so the status does not flap between the save path and the next tick.
func TestGo2RTCTickDoesNotReportAVerifiablyStoppedSidecar(t *testing.T) {
	for _, tc := range []struct {
		name      string
		setup     func(h *go2RTCLifecycleHarness)
		wantError bool
	}{
		{"stopped container", func(h *go2RTCLifecycleHarness) { h.seedContainer(false) }, false},
		{"no container yet", func(h *go2RTCLifecycleHarness) {}, false},
		{"Docker cannot say", func(h *go2RTCLifecycleHarness) {
			h.seedContainer(false)
			h.docker.inspectStatus = http.StatusForbidden
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newGo2RTCLifecycleHarness(t, false, 1)
			tc.setup(h)
			h.manager.reconcileTick(context.Background())
			status := h.manager.currentStatus()
			if status.APIUsable {
				t.Fatalf("status = %+v", status)
			}
			if (status.LastError != "") != tc.wantError {
				t.Fatalf("LastError = %q, want an error: %v", status.LastError, tc.wantError)
			}
			if created, started, _ := h.docker.counts(); created != 0 || started != 0 {
				t.Fatalf("the auto_start:false tick touched the sidecar: created=%d started=%d", created, started)
			}
		})
	}
}

// A recreate caused by a container-name or Docker-host change removes the old
// sidecar through the old target and starts nothing when it was stopped.
func TestGo2RTCReconfigureTargetChangeKeepsStoppedSidecarStopped(t *testing.T) {
	t.Run("docker host change", func(t *testing.T) {
		h := newGo2RTCLifecycleHarness(t, false, 1)
		h.seedContainer(false)
		h.setManualStop(true)
		other := &go2RTCLifecycleDocker{}
		next := h.withDetectingCameras(1)
		next.Docker.Host = fakeDockerHost(t, other.handler(t))

		if err := h.manager.ReconfigureContainer(context.Background(), h.cfg, next, false); err != nil {
			t.Fatalf("ReconfigureContainer: %v", err)
		}
		created, started, deleted := h.docker.counts()
		if deleted != 1 || created != 0 || started != 0 {
			t.Fatalf("old target: created=%d started=%d deleted=%d, want only the removal", created, started, deleted)
		}
		other.mu.Lock()
		requests := other.requests
		other.mu.Unlock()
		if requests != 0 {
			t.Fatalf("the new Docker target received %d requests, want none while the sidecar stays stopped", requests)
		}
		h.manager.mu.RLock()
		host := h.manager.docker.Host
		h.manager.mu.RUnlock()
		if host != next.Docker.Host {
			t.Fatalf("manager Docker target = %q, want the new one %q", host, next.Docker.Host)
		}
		if !h.manager.ManualStop() {
			t.Fatal("the administrator's stop was cleared")
		}
	})
	t.Run("container name change", func(t *testing.T) {
		h := newGo2RTCLifecycleHarness(t, false, 1)
		h.seedContainer(false)
		h.setManualStop(true)
		next := h.withDetectingCameras(1)
		next.Go2RTC.ContainerName = "aurago_go2rtc_new"

		if err := h.manager.ReconfigureContainer(context.Background(), h.cfg, next, false); err != nil {
			t.Fatalf("ReconfigureContainer: %v", err)
		}
		created, started, deleted := h.docker.counts()
		if deleted != 1 || created != 0 || started != 0 {
			t.Fatalf("created=%d started=%d deleted=%d, want only the removal", created, started, deleted)
		}
		h.docker.mu.Lock()
		names := append([]string(nil), h.docker.inspectNames...)
		h.docker.mu.Unlock()
		for _, name := range names {
			if name != "aurago_go2rtc" {
				t.Fatalf("inspected %q, want the old container name only (all: %v)", name, names)
			}
		}
		if got := h.manager.Config().ContainerName; got != "aurago_go2rtc_new" {
			t.Fatalf("manager container name = %q, want the new one", got)
		}
		if !h.manager.ManualStop() {
			t.Fatal("the administrator's stop was cleared")
		}
	})
}
