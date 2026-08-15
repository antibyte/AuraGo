package agent

import (
	"database/sql"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/planner"
)

func emitAffectFromTrigger(stm *memory.SQLiteMemory, cfg *config.Config, logger *slog.Logger, trigger memory.EmotionTriggerType, detail, source string) {
	event, ok := memory.AffectEventForTrigger(trigger, detail, source)
	if !ok {
		return
	}
	applyAffectEvent(stm, cfg, logger, event)
}

func applyAffectEvent(stm *memory.SQLiteMemory, cfg *config.Config, logger *slog.Logger, event memory.AffectEvent) {
	if stm == nil || cfg == nil || !cfg.Personality.Engine {
		return
	}
	if _, err := stm.ApplyAffectEvent(event, time.Now()); err != nil && logger != nil {
		logger.Debug("[Affect] Failed to apply event", "cause", event.CauseCode, "error", err)
	}
}

func syncEnvironmentAffect(stm *memory.SQLiteMemory, cfg *config.Config, plannerDB *sql.DB, isAutonomous bool, inactivityHours float64, logger *slog.Logger) {
	if stm == nil || cfg == nil || !cfg.Personality.Engine {
		return
	}
	syncOperationalIssueAffect(stm, cfg, plannerDB, logger)
	if !isAutonomous {
		emitQuietHoursAffect(stm, cfg, logger, inactivityHours)
	}
}

func syncOperationalIssueAffect(stm *memory.SQLiteMemory, cfg *config.Config, plannerDB *sql.DB, logger *slog.Logger) {
	if plannerDB == nil {
		return
	}
	openHigh := hasOpenHighSeverityOperationalIssue(plannerDB)
	current, err := stm.GetAffectState()
	if err != nil {
		return
	}
	switch {
	case openHigh && current.CauseCode != memory.AffectCauseOpsIssueOpened:
		applyAffectEvent(stm, cfg, logger, mustAffectEvent(memory.AffectCauseOpsIssueOpened, "ops", "open high-severity operational issue"))
	case !openHigh && current.CauseCode == memory.AffectCauseOpsIssueOpened:
		applyAffectEvent(stm, cfg, logger, mustAffectEvent(memory.AffectCauseOpsIssueResolved, "ops", "high-severity operational issues cleared"))
	}
}

func emitQuietHoursAffect(stm *memory.SQLiteMemory, cfg *config.Config, logger *slog.Logger, inactivityHours float64) {
	if inactivityHours < 6 {
		return
	}
	hour := time.Now().Hour()
	if hour >= 6 && hour < 18 {
		return
	}
	applyAffectEvent(stm, cfg, logger, mustAffectEvent(memory.AffectCauseQuietHours, "time", "quiet hours after user absence"))
}

func emitAutonomousRunAffect(stm *memory.SQLiteMemory, cfg *config.Config, logger *slog.Logger, consecutiveErrors, toolCallCount int) {
	switch {
	case consecutiveErrors >= 2:
		applyAffectEvent(stm, cfg, logger, mustAffectEvent(memory.AffectCauseAutonomousRunFailed, "autonomous", "background run ended with repeated errors"))
	case consecutiveErrors == 0 && toolCallCount >= 1:
		applyAffectEvent(stm, cfg, logger, mustAffectEvent(memory.AffectCauseAutonomousRunSucceeded, "autonomous", "background run completed without errors"))
	}
}

func hasOpenHighSeverityOperationalIssue(db *sql.DB) bool {
	page, err := planner.ListOperationalIssues(db, planner.OperationalIssueListFilter{
		Status: "active",
		Limit:  20,
	})
	if err != nil {
		return false
	}
	for _, item := range page.Items {
		switch strings.ToLower(strings.TrimSpace(item.Severity)) {
		case "critical", "error", "high":
			return true
		}
	}
	return false
}

func mustAffectEvent(cause, source, detail string) memory.AffectEvent {
	event, ok := memory.AffectEventForTrigger(memory.EmotionTriggerType(cause), detail, source)
	if !ok {
		return memory.AffectEvent{CauseCode: cause, Source: source, Detail: detail, Weight: memory.AffectDefaultWeight}
	}
	event.Source = source
	event.Detail = detail
	return event
}
