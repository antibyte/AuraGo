package realtimespeech

import (
	"testing"
	"time"
)

func TestRegistryEnforcesSingleClientLeaseAndTakeover(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	registry := NewRegistry(func() time.Time { return now })
	first, _, err := registry.Acquire("browser", Session{ProfileID: "one"}, false)
	if err != nil {
		t.Fatalf("Acquire first: %v", err)
	}
	if _, conflictID, err := registry.Acquire("browser", Session{ProfileID: "two"}, false); err == nil || conflictID != first.ID {
		t.Fatalf("expected conflict with %q, got id=%q err=%v", first.ID, conflictID, err)
	}
	second, _, err := registry.Acquire("browser", Session{ProfileID: "two"}, true)
	if err != nil {
		t.Fatalf("Acquire takeover: %v", err)
	}
	if second.ID == first.ID {
		t.Fatal("takeover should create a new session")
	}
	if _, ok := registry.Get(first.ID, "browser"); ok {
		t.Fatal("old session survived takeover")
	}
}

func TestRegistryTracksResumeParkingActionsAndTurns(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	registry := NewRegistry(func() time.Time { return now })
	session, _, err := registry.Acquire("browser", Session{
		ProfileID:     "voice",
		Provider:      ProviderGemini,
		ChatSessionID: "default",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := registry.UpdateState(session.ID, "browser", "parked", "", "resume-handle")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ResumptionHandle != "resume-handle" || updated.State != "parked" {
		t.Fatalf("unexpected parked state: %+v", updated)
	}
	now = now.Add(7 * time.Second)
	if _, err := registry.UpdateState(session.ID, "browser", "listening", "", ""); err != nil {
		t.Fatal(err)
	}
	registry.RecordWakeLatency(240)
	registry.RecordUsage(ProviderGemini, map[string]interface{}{
		"totalTokenCount": float64(42),
		"ignoredText":     "must not be retained",
	})
	if err := registry.BeginAction("request-1", session.ID, "browser", "default"); err != nil {
		t.Fatal(err)
	}
	if chatSession, ok := registry.ActionSession("request-1", "browser"); !ok || chatSession != "default" {
		t.Fatalf("ActionSession = %q, %v", chatSession, ok)
	}
	registry.EndAction("request-1")
	if !registry.MarkTurn(session.ID, "browser", "turn-1") {
		t.Fatal("first turn should be accepted")
	}
	if registry.MarkTurn(session.ID, "browser", "turn-1") {
		t.Fatal("duplicate turn should be rejected")
	}
	registry.ForgetTurn(session.ID, "turn-1")
	if !registry.MarkTurn(session.ID, "browser", "turn-1") {
		t.Fatal("forgotten failed turn should be retryable")
	}
	status := registry.Status()
	if status["parked_transitions"].(uint64) != 1 {
		t.Fatalf("parked transitions = %v", status["parked_transitions"])
	}
	if status["parked_duration_ms"].(int64) != 7000 || status["wake_latency_ms"].(int64) != 240 {
		t.Fatalf("lifecycle telemetry = %+v", status)
	}
	usage := status["usage_metrics"].(map[string]float64)
	if usage["gemini.totaltokencount"] != 42 {
		t.Fatalf("usage metrics = %+v", usage)
	}
	if _, exists := usage["gemini.ignoredtext"]; exists {
		t.Fatalf("string usage value was retained: %+v", usage)
	}
}

func TestRegistryPrunesExpiredLeaseWithFakeClock(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	registry := NewRegistry(func() time.Time { return now })
	session, _, err := registry.Acquire("browser", Session{ProfileID: "voice"}, false)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(sessionLeaseTTL + time.Second)
	if _, ok := registry.Get(session.ID, "browser"); ok {
		t.Fatal("expired session was not pruned")
	}
}

func TestRegistryDoesNotRateLimitProviderResumption(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	registry := NewRegistry(func() time.Time { return now })
	session, _, err := registry.Acquire("browser", Session{ProfileID: "voice"}, false)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 25; attempt++ {
		session, _, err = registry.Acquire("browser", Session{
			ID:        session.ID,
			ProfileID: "voice",
			State:     "connecting",
		}, false)
		if err != nil {
			t.Fatalf("resume %d was rate limited: %v", attempt+1, err)
		}
	}
}
