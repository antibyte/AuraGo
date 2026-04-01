package server

import (
	"net/http"

	"aurago/internal/agent"
	"aurago/internal/llm"
)

func handleHelperLLMStats(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		enabled := llm.IsHelperLLMAvailable(s.Cfg)
		s.CfgMu.RUnlock()

		snapshot := agent.SnapshotHelperLLMRuntimeStats()
		if snapshot.Operations == nil {
			snapshot.Operations = map[string]agent.HelperLLMOperationStats{}
		}
		var updatedAt interface{}
		if !snapshot.UpdatedAt.IsZero() {
			updatedAt = snapshot.UpdatedAt
		}

		writeJSON(w, map[string]interface{}{
			"enabled":    enabled,
			"updated_at": updatedAt,
			"operations": snapshot.Operations,
		})
	}
}
