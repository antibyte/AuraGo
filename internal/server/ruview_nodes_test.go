package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
	"aurago/internal/ruview"
)

func TestRuViewKnownNodesSurviveRestartAndKeepPermissions(t *testing.T) {
	s, reader, _ := testDesktopPermissionServer(t)
	s.Cfg.RuView = config.DefaultRuViewConfig()
	s.Cfg.RuView.Enabled, s.Cfg.RuView.AllowPrivate, s.Cfg.RuView.ReadOnly = true, true, false
	s.Cfg.Directories.DataDir = t.TempDir()
	live := `{"nodes":[{"node_id":7,"csi_status":"active","csi_last_seen_ms":42,"csi_sequence":3,"rssi_dbm":-51}]}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, live) }))
	defer upstream.Close()
	s.Cfg.RuView.Mode, s.Cfg.RuView.URL = "external", upstream.URL
	newManager := func() { s.RuView = ruview.New(s.ConfigSnapshot, nil, slog.New(slog.NewTextHandler(io.Discard, nil))) }
	newManager()
	defer func() { s.RuView.Close() }()
	mux := http.NewServeMux()
	registerRuViewRoutes(mux, s)
	desktopHandler := desktopIntegrationHandler(s, mux)
	admin, _, err := s.TokenManager.Create("known boards", []string{desktopScopeAdmin, "admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	call := func(prefix, method, token, origin, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://aurago.test"+prefix+"nodes", bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer "+token)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		if prefix == "/api/ruview/" {
			mux.ServeHTTP(w, r)
		} else {
			desktopHandler(w, r)
		}
		return w
	}
	for _, prefix := range []string{"/api/ruview/", "/api/desktop/integrations/ruview/"} {
		if w := call(prefix, "GET", admin, "", ""); w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"node_id":7`)) {
			t.Fatalf("known devices: %d %s", w.Code, w.Body)
		}
		for _, method := range []string{"GET", "POST"} {
			if w := call(prefix, method, reader, "http://aurago.test", `{"node_id":2}`); w.Code != 403 {
				t.Fatalf("reader %s: %d", method, w.Code)
			}
		}
		for _, origin := range []string{"", "https://evil.invalid"} {
			if w := call(prefix, "POST", admin, origin, `{"node_id":2}`); w.Code != 403 {
				t.Fatalf("origin %q: %d", origin, w.Code)
			}
		}
		for _, body := range []string{`{}`, `{"node_id":0}`, `{"node_id":256}`, `{"node_id":2,"password":"must-not-store"}`, `{"node_id":2} {}`} {
			if w := call(prefix, "POST", admin, "http://aurago.test", body); w.Code != 400 {
				t.Fatalf("invalid registration: %d", w.Code)
			}
		}
		if w := call(prefix, "POST", admin, "http://aurago.test", `{"node_id":2}`); w.Code != 200 {
			t.Fatalf("registration: %d %s", w.Code, w.Body)
		}
		s.Cfg.RuView.ReadOnly = true
		s.RuView.Configure()
		if w := call(prefix, "POST", admin, "http://aurago.test", `{"node_id":3}`); w.Code != 403 {
			t.Fatalf("read-only: %d", w.Code)
		}
		live = `{"nodes":[]}`
		s.RuView.Close()
		newManager()
		w := call(prefix, "GET", admin, "", "")
		var got struct {
			Nodes []struct {
				ID     int    `json:"node_id"`
				Age    *int64 `json:"csi_last_seen_ms"`
				Status string `json:"csi_status"`
			} `json:"nodes"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &got) != nil || len(got.Nodes) != 2 {
			t.Fatalf("restart lost devices: %d %s", w.Code, w.Body)
		}
		for i, id := range []int{2, 7} {
			if got.Nodes[i].ID != id || got.Nodes[i].Age != nil || got.Nodes[i].Status == "active" {
				t.Fatalf("invented live data: %+v", got)
			}
		}
		s.Cfg.RuView.ReadOnly = false
		s.RuView.Configure()
	}
}
