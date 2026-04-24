package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"aurago/internal/tools"
)

// handleVideoGenerationTest returns a handler that tests video generation API connectivity.
func handleVideoGenerationTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		cfg := s.Cfg
		if !cfg.VideoGeneration.Enabled {
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": "Video generation is not enabled",
			})
			return
		}

		apiKey := cfg.VideoGeneration.APIKey
		providerType := cfg.VideoGeneration.ProviderType
		if apiKey == "" || providerType == "" {
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": "Video generation provider is not configured",
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		ok, msg := tools.TestVideoConnection(ctx, strings.ToLower(providerType), apiKey, cfg.VideoGeneration.BaseURL)
		if ok {
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "ok",
				"message": msg,
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": msg,
		})
	}
}
