package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/obsidian"
)

func registerObsidianHandlers(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/obsidian/status", handleObsidianStatus(s))
}

func handleObsidianStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		cfg := s.Cfg.Obsidian
		s.CfgMu.RUnlock()

		if !cfg.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "disabled",
			})
			return
		}

		client, err := obsidian.NewClient(cfg, s.Vault)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "error",
				"error":  "Failed to initialize Obsidian client",
			})
			return
		}
		defer client.Close()

		status, err := client.Ping(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "offline",
				"error":  "Failed to reach Obsidian REST API",
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":           "online",
			"authenticated":    status.Authenticated,
			"api_version":      status.Versions["self"],
			"obsidian_version": status.Versions["obsidian"],
		})
	}
}
