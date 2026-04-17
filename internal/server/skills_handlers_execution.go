package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"aurago/internal/tools"
)

// handleSkillStats returns skill statistics for the dashboard.
// GET /api/skills/stats
func handleSkillStats(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SkillManager == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "ok",
				"enabled": false,
			})
			return
		}

		total, agentCount, userCount, pending, err := s.SkillManager.GetStats()
		if err != nil {
			jsonError(w, "Failed to get stats", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"enabled": true,
			"total":   total,
			"agent":   agentCount,
			"user":    userCount,
			"pending": pending,
		})
	}
}

// handleTestSkill executes a skill with a JSON payload and returns the raw output.
// POST /api/skills/{id}/test
func handleTestSkill(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractSkillPathID(r.URL.Path, "/api/skills/")
		if id == "" {
			jsonError(w, "Skill ID is required", http.StatusBadRequest)
			return
		}
		skill, err := s.SkillManager.GetSkill(id)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusNotFound, "Skill not found", "Skill lookup failed", err, "skill_id", id)
			return
		}

		var req struct {
			Args map[string]interface{} `json:"args"`
		}
		if r.Body != nil {
			if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil && err != io.EOF {
				jsonError(w, "Invalid request body", http.StatusBadRequest)
				return
			}
		}
		if req.Args == nil {
			req.Args = map[string]interface{}{}
		}

		secrets := loadPlainSkillSecrets(s, skill)
		var output string
		if len(secrets) > 0 {
			output, err = tools.ExecuteSkillWithSecrets(r.Context(), s.Cfg.Directories.SkillsDir, s.Cfg.Directories.WorkspaceDir, skill.Name, req.Args, secrets, nil, "", "", nil)
		} else {
			output, err = tools.ExecuteSkill(r.Context(), s.Cfg.Directories.SkillsDir, s.Cfg.Directories.WorkspaceDir, skill.Name, req.Args)
		}
		status := "ok"
		message := ""
		if err != nil {
			status = "error"
			// Intentionally preserve the execution error here: this endpoint is the
			// explicit skill-development debug channel and users need the concrete
			// traceback/output to fix broken generated skills.
			message = err.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  status,
			"output":  output,
			"message": message,
		})
	}
}

func loadPlainSkillSecrets(s *Server, skill *tools.SkillRegistryEntry) map[string]string {
	if s.Vault == nil || skill == nil {
		return nil
	}
	secrets := make(map[string]string)
	for _, key := range skill.VaultKeys {
		if strings.HasPrefix(key, "cred:") {
			continue
		}
		value, err := s.Vault.ReadSecret(key)
		if err != nil || value == "" {
			continue
		}
		secrets[key] = value
	}
	return secrets
}
