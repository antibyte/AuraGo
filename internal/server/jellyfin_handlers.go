package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"aurago/internal/jellyfin"
	"aurago/internal/security"
)

func registerJellyfinHandlers(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/jellyfin/status", handleJellyfinStatus(s))
}

func handleJellyfinStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
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
				"error":  jellyfinStatusError("Failed to initialize Jellyfin client", err),
			})
			return
		}
		defer client.Close()

		if err := client.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "offline",
				"error":  jellyfinStatusError("Failed to reach Jellyfin", err),
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

func jellyfinStatusError(prefix string, err error) string {
	if err == nil {
		return prefix
	}
	detail := sanitizeJellyfinStatusError(security.Scrub(err.Error()))
	if strings.TrimSpace(detail) == "" {
		return prefix
	}
	return prefix + ": " + detail
}

func sanitizeJellyfinStatusError(detail string) string {
	detail = strings.TrimSpace(detail)
	const apiErrorPrefix = "API error "
	if !strings.HasPrefix(detail, apiErrorPrefix) {
		return detail
	}

	statusAndBody := strings.TrimPrefix(detail, apiErrorPrefix)
	if idx := strings.IndexByte(statusAndBody, ':'); idx >= 0 {
		status := strings.TrimSpace(statusAndBody[:idx])
		if status != "" {
			return "Jellyfin API returned HTTP " + status
		}
	}
	return "Jellyfin API returned an error"
}
