package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/invasion/bridge"
)

func TestInvasionHTTPPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, method, path string
		enabled, readonly  bool
		status             int
	}{
		{"disabled read", "GET", "/api/invasion/nests", false, false, 503},
		{"disabled websocket", "GET", "/api/invasion/ws", false, false, 503},
		{"readonly hatch", "POST", "/api/invasion/nests/n/hatch", true, true, 403},
		{"readonly edit", "PUT", "/api/invasion/eggs/e", true, true, 403},
		{"readonly status", "GET", "/api/invasion/nests/n/status", true, true, 204},
		{"readonly probe", "POST", "/api/invasion/nests/n/validate", true, true, 204},
		{"enabled hatch", "POST", "/api/invasion/nests/n/hatch", true, false, 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.InvasionControl.Enabled, cfg.InvasionControl.ReadOnly = tc.enabled, tc.readonly
			s := &Server{Cfg: cfg}
			rec := httptest.NewRecorder()
			s.guardInvasion(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
		})
	}
}

func TestInvasionDisableRevokesConnectedEgg(t *testing.T) {
	cfg := &config.Config{}
	cfg.InvasionControl.Enabled = true
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := &Server{Cfg: cfg, Logger: log, EggHub: bridge.NewEggHub(log)}
	s.EggHub.Policy = s.invasionAllowed
	session, err := bridge.NewSession(strings.Repeat("a", 64), "egg", "nest", "master")
	if err != nil {
		t.Fatal(err)
	}
	conn := &bridge.EggConnection{EggID: "egg", NestID: "nest", Session: session}
	if err := s.EggHub.Register("nest", conn); err != nil {
		t.Fatal(err)
	}
	disabled := *cfg
	disabled.InvasionControl.Enabled = false
	s.replaceConfigSnapshot(&disabled)
	if s.EggHub.IsConnected("nest") {
		t.Fatal("disabled integration retained a connected egg")
	}
	if err := s.EggHub.Register("nest", conn); err == nil {
		t.Fatal("disabled integration admitted an egg")
	}
}
