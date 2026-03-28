package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/jellyfin"
)

func registerJellyfinHandlers(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/jellyfin/status", handleJellyfinStatus(s))
}

func handleJellyfinStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		cfg := s.Cfg.Jellyfin
		s.CfgMu.RUnlock()

		if !cfg.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "disabled",
			})
			return
		}

		client, err := jellyfin.NewClient(cfg, s.Vault)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
			})
			return
		}
		defer client.Close()

		if err := client.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "offline",
				"error":  err.Error(),
			})
			return
		}

		info, err := client.GetSystemInfo(r.Context())
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "online",
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "online",
			"server_name": info.ServerName,
			"version":     info.Version,
		})
	}
}
