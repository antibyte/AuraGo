package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/tools"
)

// handleAdGuardStatus returns the current AdGuard Home connection status.
func handleAdGuardStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if !s.Cfg.AdGuard.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "disabled",
				"message": "AdGuard Home integration is not enabled",
			})
			return
		}

		if s.Cfg.AdGuard.URL == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "no_url",
				"message": "AdGuard Home URL is not configured",
			})
			return
		}

		cfg := tools.AdGuardConfig{
			URL:      s.Cfg.AdGuard.URL,
			Username: s.Cfg.AdGuard.Username,
			Password: s.Cfg.AdGuard.Password,
		}

		result := tools.AdGuardStatus(cfg)
		w.Write([]byte(result))
	}
}

// handleAdGuardTest tests the AdGuard Home API connection.
func handleAdGuardTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if s.Cfg.AdGuard.URL == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "AdGuard Home URL is not configured",
			})
			return
		}

		cfg := tools.AdGuardConfig{
			URL:      s.Cfg.AdGuard.URL,
			Username: s.Cfg.AdGuard.Username,
			Password: s.Cfg.AdGuard.Password,
		}

		result := tools.AdGuardStatus(cfg)
		w.Write([]byte(result))
	}
}
