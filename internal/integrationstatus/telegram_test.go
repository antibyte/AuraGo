package integrationstatus

import (
	"testing"
	"time"
)

func TestTelegramStatusMarksHealthyPollerStale(t *testing.T) {
	original := TelegramStalePollAfter
	TelegramStalePollAfter = 3 * time.Minute
	t.Cleanup(func() { TelegramStalePollAfter = original })

	SetTelegramConfigured(true, true)
	MarkTelegramPollSuccess(time.Now())
	if status := TelegramStatus(); status.State != "healthy" {
		t.Fatalf("fresh poll state = %q, want healthy", status.State)
	}

	MarkTelegramPollSuccess(time.Now().Add(-4 * time.Minute))
	status := TelegramStatus()
	if status.State != "stale" {
		t.Fatalf("aged poll state = %q, want stale", status.State)
	}
	if status.LastSuccessfulPollAt == "" {
		t.Fatal("stale snapshot lost the last successful poll timestamp")
	}
}

func TestTelegramStatusKeepsFailureStates(t *testing.T) {
	SetTelegramConfigured(true, true)
	MarkTelegramPollFailure("telegram_poll_timeout", time.Now())
	status := TelegramStatus()
	if status.State != "degraded" {
		t.Fatalf("failed poll state = %q, want degraded", status.State)
	}
	if status.Polling {
		t.Fatal("a failed poll must not report an active poller")
	}
	if status.ConsecutivePollErrors != 1 || status.LastErrorCode != "telegram_poll_timeout" {
		t.Fatalf("failed poll status = %#v", status)
	}
}

func TestTelegramStatusIgnoresUnparsableTimestamp(t *testing.T) {
	SetTelegramConfigured(true, true)
	MarkTelegramPollSuccess(time.Now())
	telegramRuntime.Lock()
	telegramRuntime.status.LastSuccessfulPollAt = "not-a-timestamp"
	telegramRuntime.Unlock()

	status := TelegramStatus()
	if status.State != "healthy" {
		t.Fatalf("state = %q, want healthy for an unparsable timestamp", status.State)
	}
}
