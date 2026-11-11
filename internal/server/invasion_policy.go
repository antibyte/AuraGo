package server

import (
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
			jsonError(w, "Invasion Control is disabled", http.StatusServiceUnavailable)
			return
		}
		read := r.Method == http.MethodGet || r.Method == http.MethodHead ||
			(r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/validate"))
		if !read && !s.invasionAllowed(true) {
			jsonError(w, "Invasion Control is in read-only mode", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}
