package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"aurago/internal/tools"
)

// handleGotenbergTest checks whether the configured Gotenberg sidecar is reachable.
func handleGotenbergTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		cfg := s.Cfg.Tools.DocumentCreator
		s.CfgMu.RUnlock()

		if cfg.Gotenberg.URL == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Gotenberg URL is not configured",
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		result := tools.GotenbergHealth(ctx, &cfg.Gotenberg)
		w.Write([]byte(result))
	}
}
