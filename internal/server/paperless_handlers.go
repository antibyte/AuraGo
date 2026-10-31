package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/tools"
)

// handlePaperlessTest tests the Paperless-ngx API connection.
// POST /api/paperless/test
func handlePaperlessTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// Optional {"draft": ...} tests unsaved values (nothing is persisted).
		full, err := s.draftTestConfig(r)
		if err != nil {
			writeDraftTestConfigError(w, err)
			return
		}
		cfg := full.PaperlessNGX

		if !cfg.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Paperless-ngx integration is not enabled",
			})
			return
		}

		if cfg.URL == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Paperless-ngx URL is not configured",
			})
			return
		}

		// draftTestConfig resolved the token (draft first, vault second).
		apiToken := cfg.APIToken

		if apiToken == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Paperless-ngx API token is not configured. Please save a token in the vault.",
			})
			return
		}

		result := tools.PaperlessTestConnection(tools.PaperlessConfig{
			Context:  r.Context(),
			URL:      cfg.URL,
			APIToken: apiToken,
		})
		w.Write([]byte(result))
	}
}
