package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/tools"
)

// handleVercelStatus returns the current Vercel connection status.
func handleVercelStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if !s.Cfg.Vercel.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "disabled",
				"message": "Vercel integration is not enabled",
			})
			return
		}

		token, err := s.Vault.ReadSecret("vercel_token")
		if err != nil || token == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "no_token",
				"message": "Vercel token not found in vault",
			})
			return
		}

		vcfg := tools.VercelConfig{
			Token:                  token,
			DefaultProjectID:       s.Cfg.Vercel.DefaultProjectID,
			TeamID:                 s.Cfg.Vercel.TeamID,
			TeamSlug:               s.Cfg.Vercel.TeamSlug,
			ReadOnly:               s.Cfg.Vercel.ReadOnly,
			AllowDeploy:            s.Cfg.Vercel.AllowDeploy,
			AllowProjectManagement: s.Cfg.Vercel.AllowProjectManagement,
			AllowEnvManagement:     s.Cfg.Vercel.AllowEnvManagement,
			AllowDomainManagement:  s.Cfg.Vercel.AllowDomainManagement,
		}

		result := tools.VercelCheckConnection(vcfg)
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to parse Vercel account info",
			})
			return
		}

		if status, _ := parsed["status"].(string); status == "ok" {
			projectResult := tools.VercelListProjects(vcfg)
			var projects map[string]interface{}
			if err := json.Unmarshal([]byte(projectResult), &projects); err == nil {
				if c, ok := projects["count"].(float64); ok {
					parsed["project_count"] = int(c)
				}
			}
		}

		json.NewEncoder(w).Encode(parsed)
	}
}

// handleVercelTestConnection tests the Vercel API connection using the stored token.
func handleVercelTestConnection(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		token, err := s.Vault.ReadSecret("vercel_token")
		if err != nil || token == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Vercel token not found in vault. Store it with key 'vercel_token' first.",
			})
			return
		}

		vcfg := tools.VercelConfig{
			Token:                  token,
			DefaultProjectID:       s.Cfg.Vercel.DefaultProjectID,
			TeamID:                 s.Cfg.Vercel.TeamID,
			TeamSlug:               s.Cfg.Vercel.TeamSlug,
			ReadOnly:               s.Cfg.Vercel.ReadOnly,
			AllowDeploy:            s.Cfg.Vercel.AllowDeploy,
			AllowProjectManagement: s.Cfg.Vercel.AllowProjectManagement,
			AllowEnvManagement:     s.Cfg.Vercel.AllowEnvManagement,
			AllowDomainManagement:  s.Cfg.Vercel.AllowDomainManagement,
		}

		diagResult := tools.VercelCheckConnection(vcfg)
		var diag map[string]interface{}
		if err := json.Unmarshal([]byte(diagResult), &diag); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to parse Vercel diagnostic response",
			})
			return
		}

		if status, _ := diag["status"].(string); status != "ok" {
			json.NewEncoder(w).Encode(diag)
			return
		}

		projectResult := tools.VercelListProjects(vcfg)
		var projects map[string]interface{}
		if err := json.Unmarshal([]byte(projectResult), &projects); err == nil {
			if c, ok := projects["count"].(float64); ok {
				diag["project_count"] = int(c)
			}
		}

		json.NewEncoder(w).Encode(diag)
	}
}
