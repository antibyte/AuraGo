package server

import (
	"encoding/json"
	"net/http"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
)

func buildMaintenanceStatusSummary(s *Server, cfg *config.Config) map[string]interface{} {
	enabled := cfg != nil && cfg.Maintenance.Enabled
	nextRun := time.Time{}
	if s != nil && s.MaintenanceScheduler != nil {
		status := s.MaintenanceScheduler.Status()
		enabled = status.Enabled
		nextRun = status.NextRun
	} else if enabled {
		nextRun = agent.ComputeNextMaintenanceRun(cfg, time.Now())
	}
	nextRunText := ""
	if !nextRun.IsZero() {
		nextRunText = nextRun.UTC().Format(time.RFC3339)
	}
	summary := map[string]interface{}{
		"enabled":     enabled,
		"last_run":    "",
		"last_status": "never",
		"next_run":    nextRunText,
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
		cfg := s.ConfigSnapshot()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "ok",
			"maintenance": buildMaintenanceStatusSummary(s, cfg),
		})
	}
}
