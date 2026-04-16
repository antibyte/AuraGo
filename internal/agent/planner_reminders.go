package agent

import (
	"log/slog"
	"strings"
	"time"

	"aurago/internal/planner"
)

func dailyTodoReminderText(runCfg RunConfig, initialUserMsg string, now time.Time, logger *slog.Logger) string {
	if !shouldInjectDailyTodoReminder(runCfg, initialUserMsg) {
		return ""
	}
	todos, err := planner.ClaimDailyTodoReminderTodos(runCfg.PlannerDB, now)
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to claim daily todo reminder", "error", err)
		}
		return ""
	}
	return planner.BuildDailyTodoReminderText(todos)
}

func shouldInjectDailyTodoReminder(runCfg RunConfig, initialUserMsg string) bool {
	if runCfg.PlannerDB == nil || strings.TrimSpace(initialUserMsg) == "" {
		return false
	}
	if runCfg.IsCoAgent || runCfg.IsMission || runCfg.IsMaintenance {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(runCfg.MessageSource)) {
	case "mission", "a2a":
		return false
	default:
		return true
	}
}
