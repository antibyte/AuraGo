package agent

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/planner"
)

func recordToolFailureOperationalIssue(runCfg RunConfig, tc ToolCall, resultContent string, logger *slog.Logger) {
	if !shouldRecordOperationalIssueForRun(runCfg) {
		return
	}
	detail := extractErrorMessage(resultContent)
	if detail == "" {
		detail = resultContent
	}
	source := operationalIssueSource(runCfg)
	context := operationalIssueContext(runCfg)
	action := strings.TrimSpace(tc.Action)
	if action == "" {
		action = "unknown_tool"
	}
	title := fmt.Sprintf("Tool %s failed during %s", action, source)
	if context != "" {
		title = fmt.Sprintf("Tool %s failed during %s %s", action, source, context)
	}

	recordOperationalIssue(runCfg, planner.OperationalIssue{
		Source:      source,
		Context:     context,
		Title:       title,
		Detail:      detail,
		Severity:    "warning",
		Reference:   action,
		Fingerprint: strings.Join([]string{source, context, "tool", action}, "|"),
		OccurredAt:  time.Now(),
	}, logger)
}

func recordOperationalIssue(runCfg RunConfig, issue planner.OperationalIssue, logger *slog.Logger) {
	if runCfg.PlannerDB == nil {
		return
	}
	if _, err := planner.RecordOperationalIssue(runCfg.PlannerDB, issue); err != nil && logger != nil {
		logger.Warn("[OperationalIssue] Failed to record internal issue", "source", issue.Source, "title", issue.Title, "error", err)
	}
}

func shouldRecordOperationalIssueForRun(runCfg RunConfig) bool {
	if runCfg.PlannerDB == nil {
		return false
	}
	if runCfg.IsMission || runCfg.IsMaintenance || runCfg.IsCoAgent {
		return true
	}
	source := strings.ToLower(strings.TrimSpace(runCfg.MessageSource))
	switch source {
	case "web_chat":
		return false
	case "":
		return strings.TrimSpace(runCfg.SessionID) != "" && runCfg.SessionID != "default"
	case "mission", "maintenance", "planner_notification", "cron", "daemon", "webhook", "mqtt", "email", "a2a":
		return true
	default:
		return source != ""
	}
}

func operationalIssueSource(runCfg RunConfig) string {
	switch {
	case runCfg.IsMission:
		return "mission"
	case runCfg.IsMaintenance:
		return "maintenance"
	case runCfg.IsCoAgent:
		return "co_agent"
	case strings.TrimSpace(runCfg.MessageSource) != "":
		return strings.TrimSpace(runCfg.MessageSource)
	default:
		return "background"
	}
}

func operationalIssueContext(runCfg RunConfig) string {
	if strings.TrimSpace(runCfg.MissionID) != "" {
		return strings.TrimSpace(runCfg.MissionID)
	}
	if strings.TrimSpace(runCfg.SessionID) != "" && runCfg.SessionID != "default" {
		return strings.TrimSpace(runCfg.SessionID)
	}
	return ""
}

func operationalIssueReminderText(runCfg RunConfig, initialUserMsg string, isFirstTurn bool, logger *slog.Logger) string {
	if !shouldConsiderOperationalIssueReminder(runCfg, initialUserMsg) {
		return ""
	}
	issues, err := planner.ListOperationalIssueTodos(runCfg.PlannerDB, 5)
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to load operational issue reminder", "error", err)
		}
		return ""
	}
	reminder := planner.BuildOperationalIssueReminderText(issues)
	if strings.TrimSpace(reminder) == "" {
		return ""
	}

	debugMode := runCfg.Config != nil && runCfg.Config.Agent.DebugMode
	if GetDebugMode() {
		debugMode = true
	}
	if isFirstTurn {
		if _, err := planner.ClaimOperationalIssueReminderForDay(runCfg.PlannerDB, time.Now()); err != nil && logger != nil {
			logger.Warn("Failed to claim operational issue reminder", "error", err)
		}
		return reminder
	}
	if shouldInjectOperationalIssueReminderForTurn(initialUserMsg, reminder, debugMode) {
		return reminder
	}
	claimed, err := planner.ClaimOperationalIssueReminderForDay(runCfg.PlannerDB, time.Now())
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to claim operational issue reminder", "error", err)
		}
		return ""
	}
	if claimed {
		return reminder
	}
	return ""
}

func shouldConsiderOperationalIssueReminder(runCfg RunConfig, initialUserMsg string) bool {
	if runCfg.PlannerDB == nil || strings.TrimSpace(initialUserMsg) == "" {
		return false
	}
	if runCfg.IsCoAgent || runCfg.IsMission || runCfg.IsMaintenance {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(runCfg.MessageSource)) {
	case "", "web_chat", "telegram", "discord", "sms", "rocketchat", "agodesk_chat", "virtual_desktop_chat":
		return true
	case "mission", "maintenance", "a2a", "planner_notification", "cron", "daemon", "heartbeat", "follow_up", "uptime_kuma", "webhook", "mqtt":
		return false
	default:
		return false
	}
}
