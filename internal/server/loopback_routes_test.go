package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestLoopbackTargetRequiresProcessToken(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, internalToken: "fixture-process-token"}
	s.Cfg.CloudflareTunnel.ExposeHomepage = true
	target := func(name string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Target", name) })
	}
	handler := s.loopbackTargetHandler(target("homepage"), target("ui"))
	for _, tc := range []struct {
		name, token, followUp, peer, want string
	}{
		{"ordinary tunnel request", "", "", "127.0.0.1:1234", "homepage"},
		{"forged token", "arbitrary", "", "127.0.0.1:1234", "homepage"},
		{"forged followup", "", "true", "127.0.0.1:1234", "homepage"},
		{"forged pair", "arbitrary", "true", "127.0.0.1:1234", "homepage"},
		{"nonloopback peer", s.internalToken, "true", "192.0.2.1:1234", "homepage"},
		{"internal readiness", s.internalToken, "", "127.0.0.1:1234", "ui"},
		{"internal followup", s.internalToken, "true", "[::1]:1234", "ui"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, path := range []string{"/", "/auth/login", "/api/auth/password", "/api/openrouter/models"} {
				r := httptest.NewRequest(http.MethodGet, path, nil)
				r.RemoteAddr = tc.peer
				r.Header.Set("X-Internal-Token", tc.token)
				r.Header.Set("X-Internal-FollowUp", tc.followUp)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				if got := w.Header().Get("Target"); got != tc.want {
					t.Fatalf("%s reached %s, want %s", path, got, tc.want)
				}
			}
		})
	}
	s.Cfg.CloudflareTunnel.ExposeWebUI = true
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Header().Get("Target") != "ui" {
		t.Fatal("updated Web UI exposure was not applied")
	}
}
