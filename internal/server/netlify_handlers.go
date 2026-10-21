package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/security"
	"aurago/internal/tools"
)

// netlifyServerConfig builds the tool configuration from saved settings and the
// stored token. The token is registered for scrubbing before any network call.
func netlifyServerConfig(s *Server, token string) tools.NetlifyConfig {
	security.RegisterSensitive(token)
	return tools.NetlifyConfig{
		Token:               token,
		DefaultSiteID:       s.Cfg.Netlify.DefaultSiteID,
		TeamSlug:            s.Cfg.Netlify.TeamSlug,
		ReadOnly:            s.Cfg.Netlify.ReadOnly,
		AllowDeploy:         s.Cfg.Netlify.AllowDeploy,
		AllowSiteManagement: s.Cfg.Netlify.AllowSiteManagement,
		AllowEnvManagement:  s.Cfg.Netlify.AllowEnvManagement,
	}
}

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

		cfg := netlifyServerConfig(s, token)

		result := tools.NetlifyGetAccount(cfg)
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to parse account info",
			})
			return
		}

		// The connected card shows the site count next to the account name.
		if parsed["status"] == "ok" {
			var sites map[string]interface{}
			if json.Unmarshal([]byte(tools.NetlifyListSites(cfg)), &sites) == nil {
				if count, ok := sites["count"].(float64); ok {
					parsed["site_count"] = int(count)
				}
				if truncated, ok := sites["truncated"].(bool); ok && truncated {
					parsed["site_count_truncated"] = true
				}
			}
		}

		_ = json.NewEncoder(w).Encode(parsed)
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

		cfg := netlifyServerConfig(s, token)

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

		var sites map[string]interface{}
		if json.Unmarshal([]byte(tools.NetlifyListSites(cfg)), &sites) == nil {
			if c, ok := sites["count"].(float64); ok {
				diag["site_count"] = int(c)
			}
		}

		json.NewEncoder(w).Encode(diag)
	}
}
