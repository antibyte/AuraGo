package agent

import (
	"encoding/json"
	"log/slog"

	"aurago/internal/memory"
)

func emitSessionPlanUpdate(broker FeedbackBroker, shortTermMem *memory.SQLiteMemory, sessionID string, logger *slog.Logger) {
	if broker == nil || shortTermMem == nil {
		return
	}
	plan, err := shortTermMem.GetSessionPlan(sessionID)
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to fetch session plan for SSE update", "session_id", sessionID, "error", err)
		}
		return
	}
	payload := map[string]interface{}{"plan": plan}
	raw, err := json.Marshal(payload)
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to marshal session plan SSE payload", "session_id", sessionID, "error", err)
		}
		return
	}
	broker.Send("plan_update", string(raw))
}
