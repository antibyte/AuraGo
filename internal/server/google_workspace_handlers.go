package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/tools"
)

func handleGoogleWorkspaceTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		cfg := *s.Cfg
		s.CfgMu.RUnlock()

		if !cfg.GoogleWorkspace.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "disabled",
				"message": "Google Workspace integration is not enabled",
			})
			return
		}

		if s.Vault == nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Vault not available",
			})
			return
		}

		client, err := tools.NewGWorkspaceClient(cfg, s.Vault)
		if err != nil {
			s.Logger.Error("Failed to initialize Google Workspace client", "error", err)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to initialize Google Workspace client",
			})
			return
		}

		// Try a lightweight Gmail list call to verify the token works
		result := client.GmailList("", 1)
		if len(result) > 5 && result[:5] == "Error" {
			s.Logger.Error("Google Workspace test call failed", "result", result)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to contact Google Workspace",
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"message": "Connection successful — Google API responded",
		})
	}
}
