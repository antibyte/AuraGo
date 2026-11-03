package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestAuthDisabledRejectsRebindingHosts(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost} {
		for _, tc := range []struct {
			name, host, origin, bearer, forwarded string
			want                                  int
		}{
			{name: "matching attacker origin", host: "evil.test", origin: "http://evil.test", want: http.StatusForbidden},
			{name: "no origin", host: "evil.test", want: http.StatusForbidden},
			{name: "unvalidated bearer", host: "evil.test", bearer: "Bearer arbitrary", want: http.StatusForbidden},
			{name: "forged forwarded host", host: "evil.test", forwarded: "localhost", want: http.StatusForbidden},
			{name: "localhost", host: "localhost:8088", want: http.StatusNoContent},
			{name: "IPv4", host: "127.0.0.1:8088", want: http.StatusNoContent},
			{name: "IPv6", host: "[::1]:8088", want: http.StatusNoContent},
			{name: "IPv6 zone", host: "[fe80::1%en0]:8088", want: http.StatusNoContent},
			{name: "configured host", host: "aurago.test", origin: "http://aurago.test", want: http.StatusNoContent},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				cfg := &config.Config{}
				cfg.Server.HTTPS.Domain = "aurago.test"
				called := false
				h := authMiddleware(&Server{Cfg: cfg}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					called = true
					w.WriteHeader(http.StatusNoContent)
				}))
				r := httptest.NewRequest(method, "/api/config", nil)
				r.Host = tc.host
				r.Header.Set("Origin", tc.origin)
				r.Header.Set("Authorization", tc.bearer)
				r.Header.Set("X-Forwarded-Host", tc.forwarded)
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if w.Code != tc.want || called != (tc.want == http.StatusNoContent) {
					t.Fatalf("status=%d called=%v, want %d", w.Code, called, tc.want)
				}
			})
		}
	}
}
