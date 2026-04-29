package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/tools"
)

// handleNetlifyStatus returns the current Netlify connection status.
func handleNetlifyStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if !s.Cfg.Netlify.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "disabled",
				"message": "Netlify integration is not enabled",
			})
			return
		}

		token, err := s.Vault.ReadSecret("netlify_token")
		if err != nil || token == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "no_token",
				"message": "Netlify token not found in vault",
			})
			return
		}

		cfg := tools.NetlifyConfig{
			Token:               token,
			DefaultSiteID:       s.Cfg.Netlify.DefaultSiteID,
			TeamSlug:            s.Cfg.Netlify.TeamSlug,
			ReadOnly:            s.Cfg.Netlify.ReadOnly,
			AllowDeploy:         s.Cfg.Netlify.AllowDeploy,
			AllowSiteManagement: s.Cfg.Netlify.AllowSiteManagement,
			AllowEnvManagement:  s.Cfg.Netlify.AllowEnvManagement,
		}

		result := tools.NetlifyGetAccount(cfg)
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to parse account info",
			})
			return
		}

		w.Write([]byte(result))
	}
}

// handleNetlifyTestConnection tests the Netlify API connection using the stored token.
func handleNetlifyTestConnection(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		token, err := s.Vault.ReadSecret("netlify_token")
		if err != nil || token == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Netlify token not found in vault. Store it with key 'netlify_token' first.",
			})
			return
		}

		cfg := tools.NetlifyConfig{
			Token:               token,
			DefaultSiteID:       s.Cfg.Netlify.DefaultSiteID,
			TeamSlug:            s.Cfg.Netlify.TeamSlug,
			ReadOnly:            s.Cfg.Netlify.ReadOnly,
			AllowDeploy:         s.Cfg.Netlify.AllowDeploy,
			AllowSiteManagement: s.Cfg.Netlify.AllowSiteManagement,
			AllowEnvManagement:  s.Cfg.Netlify.AllowEnvManagement,
		}

		diagResult := tools.NetlifyTestConnection(cfg)
		var diag map[string]interface{}
		if err := json.Unmarshal([]byte(diagResult), &diag); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to parse Netlify diagnostic response",
			})
			return
		}

		if status, _ := diag["status"].(string); status != "ok" {
			json.NewEncoder(w).Encode(diag)
			return
		}

		sitesResult := tools.NetlifyListSites(cfg)
		var sites map[string]interface{}
		if err := json.Unmarshal([]byte(sitesResult), &sites); err == nil {
			if c, ok := sites["count"].(float64); ok {
				diag["site_count"] = int(c)
			}
		}

		json.NewEncoder(w).Encode(diag)
	}
}
