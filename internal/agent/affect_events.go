package agent

import (
	"crypto/rand"
	"database/sql"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/planner"
)

func emitConfirmedToolPersonality(stm *memory.SQLiteMemory, cfg *config.Config, run RunConfig, tc, tracking ToolCall, status ToolResultStatus, logger *slog.Logger) {
	if stm == nil || cfg == nil || !cfg.Personality.Engine || run.SuppressTurnSideEffects || isRelayAutonomousRun(run, run.SessionID) {
		return
	}
	if cfg.AuthorizationSnapshots != nil {
		_, live := cfg.AuthorizationSnapshots()
		if live == nil || !live.Personality.Engine {
			return
		}
	}
	if status != ToolResultSuccess && status != ToolResultFailed {
		return
	}
	cause := memory.AffectCauseToolSuccessStreak
	if status == ToolResultFailed {
		cause = memory.AffectCauseToolErrorStreak
	}
	event := mustAffectEvent(cause, "tool", tracking.Action+":"+tracking.Operation)
	id := tc.NativeCallID
	if id == "" {
		id = rand.Text()
	}
	_, err := stm.ApplyPersonalityObservation(memory.PersonalityObservation{
		ID: "tool:" + run.DiscoveryRunID + ":" + id, Source: "tool", Target: "task", Confidence: 1, At: time.Now(), Event: &event,
	})
	if err != nil && logger != nil {
		logger.Debug("[Personality] Tool observation unavailable", "error", err)
	}
}

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
	openHigh, err := hasOpenHighSeverityOperationalIssue(plannerDB)
	if err != nil {
		return
	}
	cause := memory.AffectCauseOpsIssueResolved
	if openHigh {
		cause = memory.AffectCauseOpsIssueOpened
	}
	event := mustAffectEvent(cause, "ops", "high-severity operational health")
	if _, err := stm.ApplyPersonalityObservation(memory.PersonalityObservation{Source: "ops", Target: "task", Confidence: 1, At: time.Now(), Event: &event, OperationalIssueOpen: &openHigh}); err != nil && logger != nil {
		logger.Debug("[Personality] Operational transition unavailable", "error", err)
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

func hasOpenHighSeverityOperationalIssue(db *sql.DB) (bool, error) {
	page, err := planner.ListOperationalIssues(db, planner.OperationalIssueListFilter{
		Status: "active",
		Limit:  20,
	})
	if err != nil {
		return false, err
	}
	for _, item := range page.Items {
		switch strings.ToLower(strings.TrimSpace(item.Severity)) {
		case "critical", "error", "high":
			return true, nil
		}
	}
	return false, nil
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
