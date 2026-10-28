package ruview

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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

func fixture(t *testing.T, handler http.HandlerFunc) (*Manager, *config.Config) {
	t.Helper()
	upstream := httptest.NewServer(handler)
	t.Cleanup(upstream.Close)
	c := &config.Config{RuView: config.DefaultRuViewConfig()}
	c.RuView.Enabled, c.RuView.AllowPrivate, c.RuView.AgentAccess = true, true, true
	c.RuView.Mode, c.RuView.URL = "external", upstream.URL
	vault, err := security.NewVault(strings.Repeat("01", 32), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.WriteSecret(TokenKey, "test-ruview-private-token"); err != nil {
		t.Fatal(err)
	}
	m := New(func() *config.Config { return c }, vault, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(m.Close)
	return m, c
}

func TestStatusIsPassiveAndAgesRealFrames(t *testing.T) {
	var calls atomic.Int32
	m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer test-ruview-private-token" {
			t.Error("missing Vault authorization")
		}
		io.WriteString(w, `{"status":"ready","source":"esp32","source_state":"live_unverified","last_frame_age_ms":20}`)
	})
	if s := m.Status(); s.Ready || s.Fresh || calls.Load() != 0 {
		t.Fatal("passive status contacted upstream")
	}
	s, err := m.Test(context.Background())
	if err != nil || !s.Ready || !s.Fresh || s.Signal != "CSI" {
		t.Fatalf("status: %+v %v", s, err)
	}
	m.mu.Lock()
	m.observed = time.Now().Add(-11 * time.Second)
	m.mu.Unlock()
	if s = m.Status(); s.Fresh || s.DataState != "stale" {
		t.Fatalf("frame did not age: %+v", s)
	}
	c.RuView.Enabled = false
	m.Configure()
	if s = m.Status(); s.Ready || s.DataState != "disabled" {
		t.Fatalf("disable failed: %+v", s)
	}
}

func TestSourceStatesAndWiFiPrivacy(t *testing.T) {
	for _, tc := range []struct {
		source, state, configured, signal string
		fresh                             bool
	}{
		{"wifi:private-ssid", "live_unverified", "wifi", "RSSI", true},
		{"simulated", "synthetic", "esp32", "demo", false},
		{"simulated", "synthetic", "simulated", "demo", false},
		{"esp32:offline", "disconnected", "esp32", "CSI", false},
		{"esp32", "live_unverified", "wifi", "CSI", false},
	} {
		t.Run(tc.source+tc.configured, func(t *testing.T) {
			m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]any{"status": "ready", "source": tc.source, "source_state": tc.state, "last_frame_age_ms": 10})
			})
			c.RuView.Source = tc.configured
			s, err := m.Test(context.Background())
			if err != nil || s.Fresh != tc.fresh || s.Signal != tc.signal || strings.Contains(s.Source, "private-ssid") {
				t.Fatalf("status %+v %v", s, err)
			}
		})
	}
}

func TestStatusAgingPreservesMissingDataAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body, state string
		failed            bool
	}{
		{"connection failure", "", "disconnected", true},
		{"disconnected sensor", `{"status":"ready","source":"esp32","source_state":"disconnected"}`, "disconnected", false},
		{"waiting for first frame", `{"status":"ready","source":"esp32","source_state":"live_unverified","waiting_for_frames":true}`, "live_unverified", false},
		{"unknown frame age", `{"status":"ready","source":"esp32","source_state":"live_unverified","last_frame_age_ms":-1}`, "live_unverified", false},
		{"explicit demo", `{"status":"ready","source":"simulated","source_state":"synthetic"}`, "synthetic", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				if tc.failed {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				io.WriteString(w, tc.body)
			})
			if tc.name == "explicit demo" {
				c.RuView.Source = "simulated"
			}
			before, err := m.Test(context.Background())
			if (err != nil) != tc.failed {
				t.Fatalf("test error: %v", err)
			}
			m.observed = time.Now().Add(-11 * time.Second)
			after := m.Status()
			if after.DataState != tc.state || after.Fresh || after.Error != before.Error || after.Ready != before.Ready || after.WaitingForFrames != (tc.name == "waiting for first frame") {
				t.Fatalf("missing measurements became stale readings: %+v", after)
			}
		})
	}
}

func TestDashboardHealthEndpointsAreReadOnly(t *testing.T) {
	var calls atomic.Int32
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer test-ruview-private-token" {
			t.Error("health request missing managed authorization")
		}
		io.WriteString(w, `{"status":"ok"}`)
	})
	paths := []string{"/health", "/health/health", "/health/live", "/health/ready", "/health/metrics", "/health/version"}
	for _, path := range paths {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
			w := httptest.NewRecorder()
			m.Proxy(w, httptest.NewRequest(method, "/", nil), path)
			want := http.StatusNotFound
			if method == http.MethodGet {
				want = http.StatusOK
			}
			if w.Code != want {
				t.Errorf("%s %s: got %d, want %d", method, path, w.Code, want)
			}
		}
	}
	if int(calls.Load()) != len(paths) {
		t.Fatalf("expected only the %d read requests upstream, got %d", len(paths), calls.Load())
	}
}

func TestReadOnlyProxyAndRequestBoundaries(t *testing.T) {
	var calls atomic.Int32
	m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Cookie") != "" || r.Header.Get("X-Private") != "" {
			t.Error("browser credentials leaked")
		}
		w.Header().Set("Set-Cookie", "upstream=secret")
		io.WriteString(w, `{"ok":true}`)
	})
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{"POST", "/api/v1/recording/start", `{}`, 403},
		{"GET", "/api/v1/models/load", "", 404},
		{"POST", "/api/v1/models/load", `{}`, 404},
		{"GET", "/api/v1/pose/stats", "", 404},
		{"GET", "/api/v1/../nodes", "", 404},
		{"GET", "/api/v1/nodes", "", 200},
	} {
		r := httptest.NewRequest(tc.method, "http://aurago.test/", strings.NewReader(tc.body))
		r.Header.Set("Cookie", "auth=private")
		r.Header.Set("X-Private", "private")
		w := httptest.NewRecorder()
		m.Proxy(w, r, tc.path)
		if w.Code != tc.want {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		if w.Header().Get("Set-Cookie") != "" {
			t.Fatal("upstream cookie forwarded")
		}
	}
	if calls.Load() != 1 {
		t.Fatal("denied request reached upstream")
	}
	c.RuView.ReadOnly = false
	for _, body := range []string{`{"file_path":"/private"}`, `{"dataset_ids":[{"url":"http://private"}]}`, strings.Repeat("x", 65537)} {
		w := httptest.NewRecorder()
		m.Proxy(w, httptest.NewRequest("POST", "/", strings.NewReader(body)), "/api/v1/train/start")
		if w.Code != 400 {
			t.Fatalf("invalid control: %d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	m.Proxy(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"dataset_ids":["recording.jsonl"]}`)), "/api/v1/train/start")
	if w.Code != 200 {
		t.Fatalf("valid training blocked: %d", w.Code)
	}
	c.RuView.AgentAccess = false
	if _, err := m.Read(context.Background(), "nodes"); err == nil {
		t.Fatal("agent bypassed gate")
	}
	c.RuView.AgentAccess = true
	if _, err := m.Read(context.Background(), "recording_start"); err == nil {
		t.Fatal("agent mutator accepted")
	}
}

func TestNoRedirectAndNoPrivateNetworkWithoutGrant(t *testing.T) {
	var leaked atomic.Int32
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Add(1) }))
	defer sink.Close()
	m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, sink.URL, 307) })
	if _, err := m.Test(context.Background()); err == nil || leaked.Load() != 0 {
		t.Fatal("credentialed redirect followed")
	}
	c.RuView.AllowPrivate = false
	if _, err := m.Test(context.Background()); err == nil {
		t.Fatal("private target accepted without grant")
	}
	u, _ := url.Parse(sink.URL)
	client := newClient(u, false)
	defer client.CloseIdleConnections()
	if _, err := client.Get(sink.URL); err == nil {
		t.Fatal("dial bypassed policy")
	}
}

func TestReconfigurationCancelsLiveRequests(t *testing.T) {
	entered := make(chan struct{})
	m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done() })
	done := make(chan error, 1)
	go func() { _, err := m.Test(context.Background()); done <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("local test server could not be reached")
	}
	// Replace the immutable snapshot only after the request has captured it.
	next := *c
	next.RuView = c.RuView
	next.RuView.Source = "wifi"
	m.config = func() *config.Config { return &next }
	m.Configure()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("request survived revoke")
		}
	case <-time.After(time.Second):
		t.Fatal("request was not cancelled")
	}
}

func TestRecordingIDsStayInsideStorageAndNeverOverwrite(t *testing.T) {
	var ids []string
	m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		id, _ := body["id"].(string)
		ids = append(ids, id)
		io.WriteString(w, `{"success":true}`)
	})
	c.RuView.ReadOnly = false
	for _, body := range []string{`{"id":"../outside"}`, `{"id":"C:\\outside"}`, `{"id":null}`, `[]`} {
		w := httptest.NewRecorder()
		m.Proxy(w, httptest.NewRequest("POST", "/", strings.NewReader(body)), "/api/v1/recording/start")
		if w.Code != 400 {
			t.Errorf("unsafe recording identifier reached upstream: HTTP %d", w.Code)
		}
	}
	if len(ids) != 0 {
		t.Fatal("invalid recordings were forwarded")
	}
	for range 2 {
		w := httptest.NewRecorder()
		m.Proxy(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"id":"room","label":"pose"}`)), "/api/v1/recording/start")
		if w.Code != 200 {
			t.Fatalf("valid recording refused: HTTP %d", w.Code)
		}
	}
	if len(ids) != 2 || ids[0] == ids[1] || !resourceID.MatchString(ids[0]) || !resourceID.MatchString(ids[1]) {
		t.Fatalf("recording identifiers are not distinct safe filenames: %v", ids)
	}
	if s := m.Status(); s.RecordingSession != "started" || s.RecordingAck != "" {
		t.Fatalf("confirmed recording start was not remembered: %+v", s)
	}
}

func TestStatusUDPFieldsStayPassive(t *testing.T) {
	var calls atomic.Int32
	m, c := fixture(t, func(http.ResponseWriter, *http.Request) { calls.Add(1) })
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	c.Directories.DataDir = blocked
	c.RuView.UDPEnabled = true
	c.RuView.UDPBind = "192.168.1.20"
	c.RuView.UDPAllow = []string{"192.168.1.50"}
	c.RuView.ReceiverIP = "192.168.1.20"
	s := m.Status()
	if calls.Load() != 0 || s.UDPScope != "external" || s.UDPEnabled || s.UDPReady || !s.ReceiverConfigured || s.RecordingSession != "unknown" || s.RecordingAck != "" {
		t.Fatalf("external status contacted something or exposed UDP as ready: %+v calls=%d", s, calls.Load())
	}
	c.RuView.Mode = "docker"
	s = m.Status()
	if calls.Load() != 0 || s.UDPScope != "managed" || !s.UDPEnabled || s.UDPReady {
		t.Fatalf("managed UDP settings were not reported, or a stopped runtime looked ready: %+v", s)
	}
	m.mu.Lock()
	m.managed = true
	m.mu.Unlock()
	s = m.Status()
	if calls.Load() != 0 || s.UDPScope != "managed" || !s.UDPEnabled || !s.UDPReady {
		t.Fatalf("running managed UDP was not reported from saved settings: %+v", s)
	}
	c.RuView.UDPAllow = nil
	if s = m.Status(); s.UDPReady || s.UDPScope != "managed" {
		t.Fatalf("empty allowlist still looked ready: %+v", s)
	}
	c.RuView.ReceiverIP = "receiver.example"
	if s = m.Status(); s.ReceiverConfigured {
		t.Fatal("hostname was accepted as a receiver")
	}
	c.RuView.Enabled = false
	if s = m.Status(); s.UDPScope != "off" || s.DataState != "disabled" || s.UDPReady || calls.Load() != 0 {
		t.Fatalf("disabled status probed or kept a UDP failure: %+v calls=%d", s, calls.Load())
	}
}

func TestRuViewUDPReadyRequiresRunningRuntime(t *testing.T) {
	managed := config.DefaultRuViewConfig()
	managed.Enabled, managed.UDPEnabled, managed.UDPBind, managed.UDPAllow = true, true, "192.168.1.20", []string{"192.168.1.50"}
	for _, tc := range []struct {
		name    string
		edit    func(*config.RuViewConfig)
		running bool
		ready   bool
		scope   string
	}{
		{"managed and stopped", func(*config.RuViewConfig) {}, false, false, "managed"},
		{"managed and running", func(*config.RuViewConfig) {}, true, true, "managed"},
		{"running without UDP", func(c *config.RuViewConfig) { c.UDPEnabled = false }, true, false, "managed"},
		{"running without allowlist", func(c *config.RuViewConfig) { c.UDPAllow = nil }, true, false, "managed"},
		{"running with a public bind", func(c *config.RuViewConfig) { c.UDPBind = "8.8.8.8" }, true, false, "managed"},
		{"external operator", func(c *config.RuViewConfig) { c.Mode = "external" }, true, false, "external"},
		{"disabled", func(c *config.RuViewConfig) { c.Enabled = false }, true, false, "off"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := managed
			tc.edit(&c)
			s := applyPassiveStatus(Status{}, c, tc.running, "", "")
			if s.UDPReady != tc.ready || s.UDPScope != tc.scope {
				t.Fatalf("udp_ready=%v scope=%q", s.UDPReady, s.UDPScope)
			}
			if tc.name == "managed and stopped" && !s.UDPEnabled {
				t.Fatal("saved UDP setting was hidden")
			}
		})
	}
	// An exited native process must not count as running, even before the health loop reaps it.
	m, c := fixture(t, func(http.ResponseWriter, *http.Request) {})
	c.RuView.Mode, c.RuView.UDPEnabled, c.RuView.UDPBind, c.RuView.UDPAllow = "docker", true, "192.168.1.20", []string{"192.168.1.50"}
	p := &nativeProcess{done: make(chan struct{})}
	m.mu.Lock()
	m.managed, m.process = true, p
	m.mu.Unlock()
	if s := m.Status(); !s.UDPReady {
		t.Fatalf("running process was not ready: %+v", s)
	}
	close(p.done)
	if s := m.Status(); s.UDPReady || !s.UDPEnabled {
		t.Fatalf("exited process still looked ready: %+v", s)
	}
}

func TestRecordingAcknowledgementFollowsForwardedResults(t *testing.T) {
	var code atomic.Int32
	code.Store(http.StatusBadGateway)
	entered := make(chan struct{})
	var once sync.Once
	m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		// HTTP/1 notices a client close only after the body has been read.
		_, _ = io.Copy(io.Discard, r.Body)
		if code.Load() == 0 {
			once.Do(func() { close(entered) })
			<-r.Context().Done()
			return
		}
		w.WriteHeader(int(code.Load()))
		io.WriteString(w, `{"ok":false}`)
	})
	c.RuView.ReadOnly = true
	w := httptest.NewRecorder()
	m.Proxy(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)), "/api/v1/recording/start")
	if w.Code != http.StatusForbidden {
		t.Fatalf("read-only start: HTTP %d", w.Code)
	}
	if s := m.Status(); s.RecordingSession != "unknown" || s.RecordingAck != "" {
		t.Fatalf("local reject changed the recording marker: %+v", s)
	}
	c.RuView.ReadOnly = false
	w = httptest.NewRecorder()
	m.Proxy(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)), "/api/v1/recording/start")
	if w.Code != http.StatusBadGateway {
		t.Fatalf("upstream failure: HTTP %d", w.Code)
	}
	if s := m.Status(); s.RecordingSession != "unknown" || s.RecordingAck != "unconfirmed" {
		t.Fatalf("failed start confirmed a session: %+v", s)
	}
	code.Store(http.StatusOK)
	w = httptest.NewRecorder()
	m.Proxy(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)), "/api/v1/recording/start")
	if s := m.Status(); w.Code != http.StatusOK || s.RecordingSession != "started" || s.RecordingAck != "" {
		t.Fatalf("start was not confirmed: HTTP %d %+v", w.Code, s)
	}
	code.Store(http.StatusBadGateway)
	w = httptest.NewRecorder()
	m.Proxy(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)), "/api/v1/recording/stop")
	if s := m.Status(); s.RecordingSession != "started" || s.RecordingAck != "unconfirmed" {
		t.Fatalf("failed stop replaced the confirmed session: %+v", s)
	}
	code.Store(0)
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)).WithContext(ctx)
	finished := make(chan struct{})
	go func() {
		m.Proxy(httptest.NewRecorder(), req, "/api/v1/recording/stop")
		close(finished)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled stop never reached upstream")
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled stop did not return")
	}
	if s := m.Status(); s.RecordingSession != "started" || s.RecordingAck != "unconfirmed" {
		t.Fatalf("cancelled stop changed the confirmed session: %+v", s)
	}
	code.Store(http.StatusOK)
	w = httptest.NewRecorder()
	m.Proxy(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)), "/api/v1/recording/stop")
	if s := m.Status(); w.Code != http.StatusOK || s.RecordingSession != "stopped" || s.RecordingAck != "" {
		t.Fatalf("stop was not confirmed: HTTP %d %+v", w.Code, s)
	}
}
