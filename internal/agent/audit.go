package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/memory"
)

func recordToolAuditEvent(stm *memory.SQLiteMemory, logger *slog.Logger, tc ToolCall, result toolExecutionResult, sessionID, messageSource string, execTimeMs int64) {
	if stm == nil {
		return
	}
	trackingTC := toolCallForExecutionTracking(tc)
	toolName := strings.TrimSpace(trackingTC.Action)
	if toolName == "" {
		toolName = "tool"
	}
	operation := strings.TrimSpace(trackingTC.Operation)
	if operation == "" {
		operation = strings.TrimSpace(trackingTC.SubOperation)
	}
	status := auditStatusForToolOutcome(result.Outcome, result.Failed)
	metadata, _ := json.Marshal(map[string]interface{}{
		"operation":   operation,
		"outcome":     result.Outcome.String(),
		"failed":      result.Failed,
		"has_command": strings.TrimSpace(trackingTC.Command) != "",
		"has_path":    strings.TrimSpace(firstNonEmpty(trackingTC.Path, trackingTC.FilePath, trackingTC.LocalPath, trackingTC.RemotePath)) != "",
	})
	summary := fmt.Sprintf("%s completed", toolName)
	if status != memory.AuditStatusSuccess {
		summary = fmt.Sprintf("%s finished with status %s", toolName, status)
	}
	if operation != "" {
		summary = fmt.Sprintf("%s %s", toolName, operation)
		if status == memory.AuditStatusSuccess {
			summary += " completed"
		} else {
			summary += " finished with status " + status
		}
	}
	if _, err := stm.RecordAuditEvent(memory.AuditEvent{
		Source:       memory.AuditSourceAgentTool,
		EventType:    "tool_call",
		Actor:        "agent",
		SessionID:    sessionID,
		TargetID:     toolName,
		TargetName:   toolName,
		Status:       status,
		Summary:      summary,
		Detail:       result.Content,
		DurationMS:   execTimeMs,
		MetadataJSON: string(metadata),
	}); err != nil && logger != nil {
		logger.Warn("Failed to record tool audit event", "tool", toolName, "error", err)
	}
}

func auditStatusForToolOutcome(outcome ExecutionOutcome, failed bool) string {
	switch outcome {
	case ExecutionOutcomeGuardianBlocked:
		return memory.AuditStatusBlocked
	case ExecutionOutcomeSanitized:
		return memory.AuditStatusSanitized
	case ExecutionOutcomeFailed:
		return memory.AuditStatusError
	case ExecutionOutcomeSuccess:
		if failed {
			return memory.AuditStatusError
		}
		return memory.AuditStatusSuccess
	default:
		if failed {
			return memory.AuditStatusError
		}
		return memory.AuditStatusWarning
	}
}
