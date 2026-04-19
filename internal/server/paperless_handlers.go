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

		s.CfgMu.RLock()
		cfg := s.Cfg.PaperlessNGX
		s.CfgMu.RUnlock()

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

		// Retrieve API token from vault
		apiToken := ""
		if s.Vault != nil {
			if token, err := s.Vault.ReadSecret("paperless_ngx_api_token"); err == nil && token != "" {
				apiToken = token
			}
		}

		if apiToken == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Paperless-ngx API token is not configured. Please save a token in the vault.",
			})
			return
		}

		result := tools.PaperlessTestConnection(tools.PaperlessConfig{
			URL:      cfg.URL,
			APIToken: apiToken,
		})
		w.Write([]byte(result))
	}
}
