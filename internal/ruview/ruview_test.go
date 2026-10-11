package ruview

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
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
	next.RuView.ReadOnly = false
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
