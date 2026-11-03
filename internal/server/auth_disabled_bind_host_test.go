package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestAuthDisabledUsesEffectiveBindHost(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost} {
		for _, tc := range []struct {
			name, configured, override, host string
			want                             int
		}{
			{name: "configured DNS bind", configured: "binding.fixture", host: "binding.fixture:8088", want: http.StatusNoContent},
			{name: "case insensitive bind", configured: "BINDING.fixture", host: "binding.fixture:8088", want: http.StatusNoContent},
			{name: "environment override", configured: "ignored.fixture", override: "binding.fixture", host: "binding.fixture:8088", want: http.StatusNoContent},
			{name: "overridden name is not trusted", configured: "ignored.fixture", override: "binding.fixture", host: "ignored.fixture:8088", want: http.StatusForbidden},
			{name: "bind name suffix is not trusted", configured: "binding.fixture", host: "binding.fixture.attacker.test:8088", want: http.StatusForbidden},
			{name: "wildcard IPv4 bind grants no DNS trust", configured: "0.0.0.0", host: "attacker.test:8088", want: http.StatusForbidden},
			{name: "wildcard IPv6 bind grants no DNS trust", configured: "::", host: "attacker.test:8088", want: http.StatusForbidden},
			{name: "empty wildcard bind grants no DNS trust", host: "attacker.test:8088", want: http.StatusForbidden},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				t.Setenv("AURAGO_SERVER_HOST", tc.override)
				cfg := &config.Config{}
				cfg.Server.Host = tc.configured
				cfg.Auth.AllowUnauthenticatedRemote = true
				if err := validateRemoteAuthExposure(cfg); err != nil {
					t.Fatalf("explicitly authorized bind was rejected at startup: %v", err)
				}
				called := false
				h := authMiddleware(&Server{Cfg: cfg}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					called = true
					w.WriteHeader(http.StatusNoContent)
				}))
				r := httptest.NewRequest(method, "/api/config", nil)
				r.Host = tc.host
				r.Header.Set("Origin", "http://"+tc.host)
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if w.Code != tc.want || called != (tc.want == http.StatusNoContent) {
					t.Fatalf("status=%d called=%v, want %d", w.Code, called, tc.want)
				}
			})
		}
	}
}
