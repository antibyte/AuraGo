package optimizer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

type OptimizationStats struct {
	ActiveOverrides   int     `json:"active_overrides"`
	RunningShadows    int     `json:"running_shadows"`
	RejectedMutations int     `json:"rejected_mutations"`
	TotalTraceEvents  int     `json:"total_trace_events"`
	GlobalSuccessRate float64 `json:"global_success_rate"`
}

func (db *OptimizerDB) GetDashboardStats() (*OptimizationStats, error) {
	stats := &OptimizationStats{}

	if err := db.db.QueryRow(`SELECT COUNT(*) FROM prompt_overrides WHERE active = 1`).Scan(&stats.ActiveOverrides); err != nil {
		slog.Error("Failed to fetch active overrides", "error", err)
		return nil, fmt.Errorf("failed to fetch active overrides: %w", err)
	}
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM prompt_overrides WHERE active = 0 AND shadow = 1`).Scan(&stats.RunningShadows); err != nil {
		slog.Error("Failed to fetch running shadows", "error", err)
		return nil, fmt.Errorf("failed to fetch running shadows: %w", err)
	}

	if err := db.db.QueryRow(`SELECT value FROM optimizer_metrics WHERE key = 'rejected_mutations'`).Scan(&stats.RejectedMutations); err != nil && err != sql.ErrNoRows {
		slog.Error("Failed to fetch rejected mutations", "error", err)
		return nil, fmt.Errorf("failed to fetch rejected mutations: %w", err)
	}

	if err := db.db.QueryRow(`SELECT COUNT(*) FROM tool_traces`).Scan(&stats.TotalTraceEvents); err != nil {
		slog.Error("Failed to fetch total trace events", "error", err)
		return nil, fmt.Errorf("failed to fetch total trace events: %w", err)
	}

	var recentTotal int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM tool_traces WHERE timestamp > datetime('now', '-7 days')`).Scan(&recentTotal); err != nil {
		slog.Error("Failed to fetch recent total traces", "error", err)
		return nil, fmt.Errorf("failed to fetch recent total traces: %w", err)
	}
	if recentTotal > 0 {
		var succ int
		if err := db.db.QueryRow(`SELECT COUNT(*) FROM tool_traces WHERE success = 1 AND timestamp > datetime('now', '-7 days')`).Scan(&succ); err != nil {
			slog.Error("Failed to fetch successful recent traces", "error", err)
			return nil, fmt.Errorf("failed to fetch successful recent traces: %w", err)
		}
		stats.GlobalSuccessRate = float64(succ) / float64(recentTotal)
	} else {
		stats.GlobalSuccessRate = 0
	}

	return stats, nil
}

func OptimizationDashboardHandler(w http.ResponseWriter, r *http.Request) {
	if defaultDB == nil {
		http.Error(w, "Optimizer DB not initialized", http.StatusServiceUnavailable)
		return
	}

	stats, err := defaultDB.GetDashboardStats()
	if err != nil {
		slog.Error("Failed to fetch optimization stats", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
