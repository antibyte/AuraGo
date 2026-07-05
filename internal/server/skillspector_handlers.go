package server

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"aurago/internal/tools"
)

func skillSpectorConfig(s *Server) tools.SkillSpectorConfig {
	if s == nil || s.Cfg == nil {
		return tools.SkillSpectorConfig{}
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	cfg := s.Cfg.Tools.SkillManager.SkillSpector
	return tools.SkillSpectorConfig{
		Enabled:        cfg.Enabled,
		CommandPath:    cfg.CommandPath,
		Timeout:        time.Duration(cfg.TimeoutSeconds) * time.Second,
		MaxOutputBytes: int64(cfg.MaxOutputKB) * 1024,
	}
}

func handleSkillSpectorStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := skillSpectorConfig(s)
		commandPath := strings.TrimSpace(cfg.CommandPath)
		if commandPath == "" {
			commandPath = "skillspector"
		}
		timeoutSeconds := int(cfg.Timeout.Seconds())
		if timeoutSeconds <= 0 {
			timeoutSeconds = 60
		}
		maxOutputKB := int(cfg.MaxOutputBytes / 1024)
		if maxOutputKB <= 0 {
			maxOutputKB = 512
		}
		available := false
		message := "SkillSpector is disabled"
		if cfg.Enabled {
			if _, err := exec.LookPath(commandPath); err == nil {
				available = true
				message = "SkillSpector CLI is available"
			} else {
				message = err.Error()
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "ok",
			"enabled":         cfg.Enabled,
			"command_path":    commandPath,
			"available":       available,
			"message":         message,
			"timeout_seconds": timeoutSeconds,
			"max_output_kb":   maxOutputKB,
		})
	}
}
