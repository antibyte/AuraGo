package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/config"
	"aurago/internal/discord"
)

func currentDiscordConfig(s *Server) *config.Config {
	if s == nil || s.Cfg == nil {
		return &config.Config{}
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	cfg := *s.Cfg
	return &cfg
}

func handleDiscordHealth(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		st := discord.Status(currentDiscordConfig(s))
		w.Header().Set("Content-Type", "application/json")
		if st.Enabled && !st.Connected {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		_ = json.NewEncoder(w).Encode(st)
	}
}
