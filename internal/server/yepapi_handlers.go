package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"aurago/internal/tools"
)

// handleYepAPITest returns a handler that tests the YepAPI connection.
func handleYepAPITest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")

		if !cfg.YepAPI.Enabled {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "YepAPI is not enabled"})
			return
		}

		apiKey, err := tools.ResolveYepAPIKey(cfg, s.Vault)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "No YepAPI API key found: " + err.Error()})
			return
		}

		client := tools.NewYepAPIClient(apiKey)
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		// Use a minimal SERP query as the cheapest possible test call
		payload := map[string]interface{}{
			"query": "test",
			"limit": 1,
		}
		_, err = client.Post(ctx, "/v1/serp/google", payload)
		if err != nil {
			s.Logger.Error("YepAPI test failed", "error", err)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "YepAPI test failed: " + err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Connection successful — API key is valid."})
	}
}
