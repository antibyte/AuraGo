package server

import (
	"encoding/json"
	"net/http"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
)

func buildMaintenanceStatusSummary(s *Server, cfg *config.Config) map[string]interface{} {
	summary := map[string]interface{}{
		"enabled":     cfg != nil && cfg.Maintenance.Enabled,
		"last_run":    "",
		"last_status": "never",
		"next_run":    agent.ComputeNextMaintenanceRun(cfg, time.Now()).UTC().Format(time.RFC3339),
	}
	if cfg == nil {
		return summary
	}
	if s != nil && s.ShortTermMem != nil {
		if record, err := s.ShortTermMem.GetLatestMaintenanceRun(); err == nil && record != nil {
			summary["last_run"] = record.FinishedAt
			summary["last_status"] = record.Status
			summary["phase_results"] = record.PhaseResults
		}
	}
	return summary
}

func handleDashboardMaintenanceStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "ok",
			"maintenance": buildMaintenanceStatusSummary(s, cfg),
		})
	}
}