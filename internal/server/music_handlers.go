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

		provider := r.URL.Query().Get("provider")
		if provider == "" {
			provider = cfg.MusicGeneration.Provider
		}

		var apiKey string
		switch provider {
		case "minimax":
			apiKey = cfg.MusicGeneration.MiniMax.APIKey
		case "google_lyria":
			apiKey = cfg.MusicGeneration.GoogleLyria.APIKey
		default:
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Unknown provider: " + provider})
			return
		}

		if apiKey == "" {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "No API key configured for " + provider})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		ok, msg := tools.TestMusicConnection(ctx, provider, apiKey)
		if ok {
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": msg})
		} else {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": msg})
		}
	}
}
