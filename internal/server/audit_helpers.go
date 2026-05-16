package server

import (
	"encoding/json"
	"time"

	"aurago/internal/memory"
	"aurago/internal/remote"
)

func recordHeartbeatAuditStart(stm *memory.SQLiteMemory, correlationID string) {
	if stm == nil || correlationID == "" {
		return
	}
	_ = stm.UpsertAuditEventByCorrelation(memory.AuditEvent{
		Source:        memory.AuditSourceHeartbeat,
		EventType:     "heartbeat_run",
		Actor:         "agent",
		SessionID:     "heartbeat",
		TargetID:      "scheduler",
		TargetName:    "Heartbeat scheduler",
		Status:        memory.AuditStatusRunning,
		Summary:       "Heartbeat wake-up started",
		CorrelationID: correlationID,
	})
}

func recordHeartbeatAuditFinish(stm *memory.SQLiteMemory, correlationID, status, summary, detail string, duration time.Duration) {
	if stm == nil || correlationID == "" {
		return
	}
	if status == "" {
		status = memory.AuditStatusSuccess
	}
	_ = stm.UpsertAuditEventByCorrelation(memory.AuditEvent{
		Source:        memory.AuditSourceHeartbeat,
		EventType:     "heartbeat_run",
		Actor:         "agent",
		SessionID:     "heartbeat",
		TargetID:      "scheduler",
		TargetName:    "Heartbeat scheduler",
		Status:        status,
		Summary:       summary,
		Detail:        detail,
		DurationMS:    duration.Milliseconds(),
		CorrelationID: correlationID,
	})
}

func recordRemoteAuditEvent(stm *memory.SQLiteMemory, event remote.RemoteAuditEvent) {
	if stm == nil {
		return
	}
	metadata, _ := json.Marshal(event.Metadata)
	status := normalizeRemoteAuditStatus(event.Status)
	_, _ = stm.RecordAuditEvent(memory.AuditEvent{
		Source:       memory.AuditSourceRemote,
		EventType:    event.EventType,
		Actor:        "remote",
		TargetID:     event.DeviceID,
		TargetName:   event.DeviceName,
		Status:       status,
		Summary:      event.Summary,
		Detail:       event.Detail,
		DurationMS:   event.DurationMS,
		MetadataJSON: string(metadata),
	})
}

func normalizeRemoteAuditStatus(status string) string {
	switch status {
	case memory.AuditStatusSuccess, memory.AuditStatusError, memory.AuditStatusWarning, memory.AuditStatusBlocked, memory.AuditStatusSanitized, memory.AuditStatusRunning:
		return status
	case "denied", "timeout", "failed", "failure":
		return memory.AuditStatusError
	case "":
		return memory.AuditStatusWarning
	default:
		return memory.AuditStatusWarning
	}
}
