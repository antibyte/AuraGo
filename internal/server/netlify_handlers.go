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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			Token:         token,
			DefaultSiteID: s.Cfg.Netlify.DefaultSiteID,
			TeamSlug:      s.Cfg.Netlify.TeamSlug,
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			Token:         token,
			DefaultSiteID: s.Cfg.Netlify.DefaultSiteID,
			TeamSlug:      s.Cfg.Netlify.TeamSlug,
		}

		// Test: get account info
		accountResult := tools.NetlifyGetAccount(cfg)
		var account map[string]interface{}
		if err := json.Unmarshal([]byte(accountResult), &account); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to parse account response",
			})
			return
		}

		if status, _ := account["status"].(string); status != "ok" {
			msg, _ := account["message"].(string)
			if msg == "" {
				msg = "Connection failed"
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": msg,
			})
			return
		}

		// Test: list sites
		sitesResult := tools.NetlifyListSites(cfg)
		var sites map[string]interface{}
		if err := json.Unmarshal([]byte(sitesResult), &sites); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to parse sites response",
			})
			return
		}

		siteCount := 0
		if c, ok := sites["count"].(float64); ok {
			siteCount = int(c)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "ok",
			"message":    "Connected successfully",
			"email":      account["email"],
			"full_name":  account["full_name"],
			"site_count": siteCount,
		})
	}
}
