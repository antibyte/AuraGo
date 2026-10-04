package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// handleYepAPITest returns a handler that tests the YepAPI connection.
func handleYepAPITest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")

		if !cfg.YepAPI.Enabled {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "YepAPI is not enabled"})
			return
		}

		var secrets config.SecretReader
		if s.Vault != nil {
			secrets = s.Vault
		}
		apiKey, err := tools.ResolveYepAPIKey(cfg, secrets)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "No YepAPI API key found: " + err.Error()})
			return
		}

		client := tools.NewYepAPIClientWithBaseURL(apiKey, cfg.YepAPI.BaseURL)
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		err = client.Probe(ctx)
		if err != nil {
			s.Logger.Error("YepAPI test failed", "error", err)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "YepAPI test failed: " + err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "probe": "model_catalog", "billable": false, "authentication_verified": false})
	}
}
