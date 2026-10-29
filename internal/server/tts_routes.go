package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/tools"
)

func handleTTSRoutes(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"routes": tools.EffectiveTTSRoutes(s.ConfigSnapshot()),
		})
	}
}
