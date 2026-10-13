package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
	"aurago/internal/ruview"
)

func TestRuViewBoardSetupIsAdminOnlyAndPassive(t *testing.T) {
	s, reader, _ := testDesktopPermissionServer(t)
	s.Cfg.RuView = config.DefaultRuViewConfig()
	s.Cfg.RuView.Mode, s.Cfg.RuView.URL = "external", "http://192.168.6.20:3000"
	s.Cfg.RuView.ReceiverIP, s.Cfg.RuView.UDPPort = "192.168.6.30", 5010
	s.RuView = ruview.New(s.ConfigSnapshot, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer s.RuView.Close()
	mux := http.NewServeMux()
	registerRuViewRoutes(mux, s)
	desktopHandler := desktopIntegrationHandler(s, mux)
	admin, _, err := s.TokenManager.Create("board setup fixture", []string{desktopScopeAdmin, "admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{"/api/ruview/", "/api/desktop/integrations/ruview/"} {
		for _, token := range []string{reader, admin} {
			r := httptest.NewRequest(http.MethodGet, "http://aurago.test"+prefix+"board-setup", nil)
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			if prefix == "/api/ruview/" {
				mux.ServeHTTP(w, r)
			} else {
				desktopHandler(w, r)
			}
			if token == reader {
				if w.Code != http.StatusForbidden {
					t.Fatalf("%s non-admin response: %d", prefix, w.Code)
				}
				continue
			}
			var got ruview.BoardSetup
			if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &got) != nil || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("%s setup response: %d %s", prefix, w.Code, w.Body.String())
			}
			if got.ReceiverIP != "192.168.6.30" || got.UDPPort != 5010 || got.UDPEnabled || len(got.Candidates) != 1 || got.Candidates[0] != "192.168.6.20" {
				t.Fatalf("wrong saved defaults: %+v", got)
			}
		}
	}
}
