package optimizer

import (
	"encoding/json"
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

	db.db.QueryRow(`SELECT COUNT(*) FROM prompt_overrides WHERE active = 1`).Scan(&stats.ActiveOverrides)
	db.db.QueryRow(`SELECT COUNT(*) FROM prompt_overrides WHERE active = 0 AND shadow = 1`).Scan(&stats.RunningShadows)

	db.db.QueryRow(`SELECT value FROM optimizer_metrics WHERE key = 'rejected_mutations'`).Scan(&stats.RejectedMutations)

	db.db.QueryRow(`SELECT COUNT(*) FROM tool_traces`).Scan(&stats.TotalTraceEvents)

	var recentTotal int
	db.db.QueryRow(`SELECT COUNT(*) FROM tool_traces WHERE timestamp > datetime('now', '-7 days')`).Scan(&recentTotal)
	if recentTotal > 0 {
		var succ int
		db.db.QueryRow(`SELECT COUNT(*) FROM tool_traces WHERE success = 1 AND timestamp > datetime('now', '-7 days')`).Scan(&succ)
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

