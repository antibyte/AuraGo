package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"aurago/internal/tools"
)

// handleMusicGenerationTest returns a handler that tests music generation API connectivity.
func handleMusicGenerationTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")

		if !cfg.MusicGeneration.Enabled {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Music generation is not enabled"})
			return
		}

		apiKey := cfg.MusicGeneration.APIKey
		providerType := cfg.MusicGeneration.ProviderType
		if apiKey == "" {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "No music generation provider configured"})
			return
		}

		// Allow override via query param for legacy callers, map to provider type
		providerParam := r.URL.Query().Get("provider")
		if providerParam != "" && providerParam != providerType {
			// If a specific provider was requested that doesn't match, return early
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Requested provider does not match configured provider"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		ok, msg := tools.TestMusicConnection(ctx, providerType, apiKey)
		if ok {
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": msg})
		} else {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": msg})
		}
	}
}
