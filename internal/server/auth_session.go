package server

import (
	"encoding/json"
	"net/http"
	"time"
)

// handleAuthActivity renews a live browser session only on explicit UI activity.
// Ordinary API requests, status polling and streams never renew the cookie.
func handleAuthActivity(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		defer s.CfgMu.RUnlock()
		if !s.Cfg.Auth.Enabled {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"enabled":false}`))
			return
		}
		cookie, err := r.Cookie(sessionCookieName)
		var expiry time.Time
		if err == nil {
			expiry = sessionExpiry(s.Cfg.Auth.SessionSecret, cookie.Value)
		}
		if expiry.IsZero() {
			jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !checkCSRFOriginWithPolicy(r, s.Cfg.Auth.RequireOriginHeader) {
			jsonError(w, "csrf_check_failed", http.StatusForbidden)
			return
		}
		// Do not shorten the configured login duration or bank time on every click.
		const extension = 10 * time.Minute
		if time.Until(expiry) < extension {
			SetSessionCookie(w, r, s.Cfg.Auth.SessionSecret, extension)
			expiry = time.Now().Add(extension)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":            true,
			"authenticated":      true,
			"expires_in_seconds": max(0, int(time.Until(expiry).Seconds())),
		})
	}
}
