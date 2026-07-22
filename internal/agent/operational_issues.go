package agent

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/i18n"
	"aurago/internal/planner"
)

const operationalIssueNoticeEvent = "operational_issue_notice"

type operationalIssueNoticeState struct {
	Items            []planner.OperationalIssueNotice
	Refs             []planner.OperationalIssueNoticeRef
	Text             string
	PromptContext    string
	TypedDelivered   bool
	FallbackRequired bool
}

type operationalIssueNoticePayload struct {
	SessionID string                           `json:"session_id"`
	Text      string                           `json:"text"`
	Count     int                              `json:"count"`
	Issues    []planner.OperationalIssueNotice `json:"issues"`
}

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

func resolveToolFailureOperationalIssue(runCfg RunConfig, tc ToolCall, logger *slog.Logger) {
	if !shouldRecordOperationalIssueForRun(runCfg) || runCfg.PlannerDB == nil {
		return
	}
	action := strings.TrimSpace(tc.Action)
	if action == "" {
		action = "unknown_tool"
	}
	fingerprint := strings.Join([]string{operationalIssueSource(runCfg), operationalIssueContext(runCfg), "tool", action}, "|")
	if _, err := planner.ResolveOperationalIssue(runCfg.PlannerDB, fingerprint, "The same tool operation completed successfully.", time.Now()); err != nil && logger != nil {
		logger.Warn("[OperationalIssue] Failed to resolve internal issue", "tool", action, "error", err)
	}
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

func prepareOperationalIssueNotice(runCfg RunConfig, initialUserMsg string, logger *slog.Logger) operationalIssueNoticeState {
	if !shouldConsiderOperationalIssueReminder(runCfg, initialUserMsg) {
		return operationalIssueNoticeState{}
	}
	issues, err := planner.ListPendingOperationalIssueNotices(runCfg.PlannerDB, time.Now(), 2)
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to load operational issue notices", "error", err)
		}
		return operationalIssueNoticeState{}
	}
	if len(issues) == 0 {
		return operationalIssueNoticeState{}
	}
	lang := "en"
	if runCfg.Config != nil {
		lang = runCfg.Config.Server.UILanguage
	}
	text := formatOperationalIssueNotice(lang, issues)
	refs := make([]planner.OperationalIssueNoticeRef, 0, len(issues))
	for _, issue := range issues {
		refs = append(refs, planner.OperationalIssueNoticeRef{Fingerprint: issue.Fingerprint, Revision: issue.Revision})
	}
	return operationalIssueNoticeState{
		Items:         issues,
		Refs:          refs,
		Text:          text,
		PromptContext: strings.TrimSpace("The supervisor has already displayed the following REQUIRED USER NOTICE, or will prepend it deterministically to the final answer. Do not repeat it unless it is directly relevant to the answer.\n\n" + text),
	}
}

func formatOperationalIssueNotice(lang string, issues []planner.OperationalIssueNotice) string {
	title := translatedOperationalIssueText(lang, "backend.operational_issue_notice_title", "Current operational issues")
	summary := translatedOperationalIssueText(lang, "backend.operational_issue_notice_summary", "AuraGo detected {0} issue(s) during background work.", len(issues))
	var b strings.Builder
	b.WriteString("### ")
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString(summary)
	for _, issue := range issues {
		b.WriteString("\n\n- ")
		b.WriteString(strings.TrimSpace(issue.Title))
		if detail := strings.TrimSpace(issue.Detail); detail != "" {
			b.WriteString(": ")
			b.WriteString(detail)
		}
		if issue.Occurrences > 1 {
			occurrences := translatedOperationalIssueText(lang, "backend.operational_issue_notice_occurrences", "occurred {0} times", issue.Occurrences)
			b.WriteString(" (")
			b.WriteString(occurrences)
			b.WriteString(")")
		}
	}
	return b.String()
}

func translatedOperationalIssueText(lang, key, fallback string, params ...any) string {
	translated := i18n.T(lang, key, params...)
	if translated == key {
		translated = fallback
		for index, value := range params {
			translated = strings.ReplaceAll(translated, fmt.Sprintf("{%d}", index), fmt.Sprint(value))
		}
	}
	return translated
}

func deliverOperationalIssueNotice(state *operationalIssueNoticeState, runCfg RunConfig, broker FeedbackBroker, logger *slog.Logger) {
	if state == nil || len(state.Items) == 0 || strings.TrimSpace(state.Text) == "" {
		return
	}
	payload := operationalIssueNoticePayload{
		SessionID: strings.TrimSpace(runCfg.SessionID), Text: state.Text, Count: len(state.Items), Issues: state.Items,
	}
	delivered := false
	delivery := "pending"
	if typed, ok := broker.(TypedFeedbackDeliveryBroker); ok {
		delivered, delivery = typed.SendTypedWithTransport(operationalIssueNoticeEvent, payload)
	} else if typed, ok := broker.(TypedFeedbackBroker); ok {
		delivered = typed.SendTyped(operationalIssueNoticeEvent, payload)
		if delivered {
			delivery = "typed_session"
		}
	}
	if !delivered {
		state.FallbackRequired = true
		if logger != nil {
			logger.Info("Operational issue notice pending persisted fallback", "delivery", "pending")
		}
		return
	}
	state.TypedDelivered = true
	if delivery != "typed_session" && delivery != "direct_stream" {
		delivery = "typed_session"
	}
	if logger != nil {
		logger.Info("Operational issue notice delivered", "delivery", delivery)
	}
	if err := planner.MarkOperationalIssuesNotified(runCfg.PlannerDB, state.Refs, time.Now()); err != nil && logger != nil {
		logger.Warn("Failed to persist delivered operational issue notice", "error", err)
	}
}

func prependOperationalIssueNotice(state operationalIssueNoticeState, content string) string {
	if !state.FallbackRequired || strings.TrimSpace(state.Text) == "" {
		return content
	}
	content = strings.TrimSpace(content)
	if content == "" || content == "[Empty Response]" {
		return state.Text
	}
	return state.Text + "\n\n" + content

}

func markPersistedOperationalIssueNotice(state operationalIssueNoticeState, runCfg RunConfig, logger *slog.Logger) {
	if !state.FallbackRequired || len(state.Refs) == 0 {
		return
	}
	if err := planner.MarkOperationalIssuesNotified(runCfg.PlannerDB, state.Refs, time.Now()); err != nil && logger != nil {
		logger.Warn("Failed to persist fallback operational issue notice state", "error", err)
	} else if logger != nil {
		logger.Info("Operational issue notice persisted with final response", "delivery", "persisted_fallback")
	}
}

// operationalIssueReminderText remains as a compatibility helper for prompt
// tests and callers outside the loop. It no longer claims or marks a notice.
func operationalIssueReminderText(runCfg RunConfig, initialUserMsg string, _ bool, logger *slog.Logger) string {
	return prepareOperationalIssueNotice(runCfg, initialUserMsg, logger).PromptContext
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
