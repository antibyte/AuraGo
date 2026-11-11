package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) invasionAllowed(mutating bool) bool {
	cfg := s.ConfigSnapshot()
	return cfg != nil && cfg.InvasionControl.Enabled && (!mutating || !cfg.InvasionControl.ReadOnly)
}

func (s *Server) guardInvasion(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.invasionAllowed(false) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invasion Control is disabled", "code": "invasion_disabled"})
			return
		}
		read := r.Method == http.MethodGet || r.Method == http.MethodHead ||
			(r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/validate"))
		worker := r.Method == http.MethodPost && (r.URL.Path == "/api/invasion/artifacts/offer" ||
			strings.HasPrefix(r.URL.Path, "/api/invasion/artifacts/upload/") || r.URL.Path == "/api/invasion/messages")
		if !read && !worker && !s.invasionAllowed(true) {
			jsonError(w, "Invasion Control is in read-only mode", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}
