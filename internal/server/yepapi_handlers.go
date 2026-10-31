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
		w.Header().Set("Content-Type", "application/json")

		// Optional {"draft": ...} tests unsaved values (nothing is persisted).
		cfg, err := s.draftTestConfig(r)
		if err != nil {
			writeDraftTestConfigError(w, err)
			return
		}

		if !cfg.YepAPI.Enabled {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "YepAPI is not enabled"})
			return
		}

		// The dedicated vault key is not part of the config; it only counts while
		// the draft still points at the saved base URL (stored keys must not follow
		// a changed target).
		var secrets config.SecretReader
		if saved := s.ConfigSnapshot(); s.Vault != nil && saved != nil && saved.YepAPI.BaseURL == cfg.YepAPI.BaseURL {
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
