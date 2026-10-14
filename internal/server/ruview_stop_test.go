package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/ruview"
)

func TestRuViewStopsKeepAdminOriginAndExactReadOnlyExceptions(t *testing.T) {
	s, readToken, _ := testDesktopPermissionServer(t)
	s.Cfg.RuView = config.DefaultRuViewConfig()
	s.Cfg.RuView.Enabled, s.Cfg.RuView.AllowPrivate = true, true
	s.Cfg.VirtualDesktop.ReadOnly = true
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; io.WriteString(w, `{"success":true}`) }))
	defer upstream.Close()
	s.Cfg.RuView.Mode, s.Cfg.RuView.URL = "external", upstream.URL
	s.RuView = ruview.New(s.ConfigSnapshot, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer s.RuView.Close()
	mux := http.NewServeMux()
	registerRuViewRoutes(mux, s)
	admin, _, err := s.TokenManager.Create("ruview stops", []string{desktopScopeAdmin, "admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{"/api/ruview/", "/api/desktop/integrations/ruview/"} {
		var h http.Handler = mux
		if strings.Contains(base, "desktop") {
			h = desktopIntegrationHandler(s, mux)
		}
		for _, tc := range []struct {
			route, token, origin string
			want                 int
		}{
			{"proxy/api/v1/recording/stop", admin, "http://aurago.test", 200},
			{"proxy/api/v1/train/stop", admin, "http://aurago.test", 200},
			{"proxy/api/v1/recording/stop", readToken, "http://aurago.test", 403},
			{"proxy/api/v1/train/stop", admin, "https://evil.invalid", 403},
			{"runtime/stop", admin, "http://aurago.test", 403}, // External runtime stays operator-owned.
			{"proxy/api/v1/recording/start", admin, "http://aurago.test", 403},
			{"proxy/api/v1/train/start", admin, "http://aurago.test", 403},
			{"proxy/api/v1/calibration/stop", admin, "http://aurago.test", 403},
		} {
			r := httptest.NewRequest("POST", "http://aurago.test"+base+tc.route, strings.NewReader(`{}`))
			r.Header.Set("Authorization", "Bearer "+tc.token)
			r.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Errorf("%s %s: %d %s", base, tc.route, w.Code, w.Body.String())
			}
		}
		s.Cfg.RuView.Mode = "native"
		for _, token := range []string{admin, readToken} {
			r := httptest.NewRequest("POST", "http://aurago.test"+base+"runtime/stop", nil)
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("Origin", "http://aurago.test")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			want := 200
			if token == readToken {
				want = 403
			}
			if w.Code != want {
				t.Errorf("runtime stop: %d %s", w.Code, w.Body.String())
			}
		}
		s.Cfg.RuView.Mode = "external"
	}
	if calls != 4 {
		t.Fatalf("unexpected upstream calls: %d", calls)
	}
	for _, route := range []string{"ruview/runtime/stop", "ruview/proxy/api/v1/recording/stop", "ruview/proxy/api/v1/train/stop"} {
		if desktopIntegrationOperation("POST", route) != desktopStop {
			t.Fatal(route)
		}
	}
	for _, route := range []string{"ruview/proxyruntime/stop", "ruview/proxy/api/v1/train/stop/extra", "ruview/proxy/api/v1/train/start", "ruview/runtime/stop/"} {
		if desktopIntegrationOperation("POST", route) == desktopStop {
			t.Fatal("widened stop policy: " + route)
		}
	}
}
