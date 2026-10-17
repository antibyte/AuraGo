package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestDashboardSummaryRouteRequiresAdmin pins spec §8.3: the summary is
// mounted behind requireAdmin, and authMiddleware's bearer fallback (admin
// scope for every path outside the desktop and go2rtc special cases) keeps a
// read-scope token out before the gate is reached.
func TestDashboardSummaryRouteRequiresAdmin(t *testing.T) {
	s, adminToken, _, readToken := newBearerSchemeTestServer(t)
	s.Cfg.WebConfig.Enabled = true
	s.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	s.registerConfigAPIRoutes(mux, nil)
	session := &http.Cookie{Name: sessionCookieName, Value: createSessionValue(bearerSchemeTestSessionSecret, time.Now().Add(time.Hour))}

	handlers := []struct {
		name    string
		handler http.Handler
	}{
		{"gate", requireAdmin(s, handleDashboardSummary(s))},
		{"mux", mux},
		{"middleware+mux", authMiddleware(s, mux)},
	}
	cases := []struct {
		name   string
		bearer string
		cookie *http.Cookie
		want   int
	}{
		{"read-scope bearer", readToken, nil, http.StatusForbidden},
		{"admin bearer", adminToken, nil, http.StatusOK},
		{"browser session", "", session, http.StatusOK},
		{"anonymous", "", nil, http.StatusUnauthorized},
	}
	for _, h := range handlers {
		for _, tc := range cases {
			req := httptest.NewRequest(http.MethodGet, "/api/dashboard/summary", nil)
			if tc.bearer != "" {
				req.Header.Set("Authorization", "Bearer "+tc.bearer)
			}
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}
			rec := httptest.NewRecorder()
			h.handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("%s / %s: status = %d, want %d; body = %s", h.name, tc.name, rec.Code, tc.want, rec.Body.String())
			}
		}
	}
}
