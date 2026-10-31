package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func normalizeYepAPIBaseURL(raw string) string {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return "https://api.yepapi.com"
	}
	return raw
}

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

		// Provider keys (saved providers) and the dedicated vault key must not follow
		// a changed base URL: refuse before any key is resolved.
		saved := s.ConfigSnapshot()
		sameTarget := saved != nil && normalizeYepAPIBaseURL(saved.YepAPI.BaseURL) == normalizeYepAPIBaseURL(cfg.YepAPI.BaseURL)
		if saved != nil && !sameTarget {
			writeDraftTestConfigError(w, newDraftConfigError("The target of %q changed: save the new base URL first, then test (Ziel geändert – bitte zuerst speichern)", "yepapi"))
			return
		}
		var secrets config.SecretReader
		if s.Vault != nil && sameTarget {
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
