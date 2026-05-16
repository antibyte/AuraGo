package remote

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestExecuteRemoteCommand_Timeout(t *testing.T) {
	// This test just ensures the function signature exists and compiles.
	// We use a short timeout and an unreachable address to trigger a quick error.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := ExecuteRemoteCommand(ctx, "127.0.0.1", 2222, "user", []byte("pass"), "ls")
	if err == nil {
		t.Error("Expected error for unreachable host, got nil")
	}
}

func TestRemoteHubAuditCallbackReceivesRemoteEvents(t *testing.T) {
	hub := NewRemoteHub(nil, nil, slog.Default())
	var events []RemoteAuditEvent
	hub.OnAudit = func(event RemoteAuditEvent) {
		events = append(events, event)
	}

	hub.emitAudit(RemoteAuditEvent{
		DeviceID:   "dev-1",
		DeviceName: "Kitchen Node",
		EventType:  "remote_heartbeat",
		Status:     "success",
		Summary:    "Remote heartbeat received",
	})
	hub.emitAudit(RemoteAuditEvent{
		DeviceID:   "dev-1",
		DeviceName: "Kitchen Node",
		EventType:  "remote_command",
		Status:     "error",
		Summary:    "Remote command failed",
		DurationMS: 150,
	})

	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0].EventType != "remote_heartbeat" || events[0].Status != "success" {
		t.Fatalf("heartbeat event = %+v", events[0])
	}
	if events[1].EventType != "remote_command" || events[1].DurationMS != 150 {
		t.Fatalf("command event = %+v", events[1])
	}
}
