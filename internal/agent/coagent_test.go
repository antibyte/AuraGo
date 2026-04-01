package agent

import (
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestTruncateUTF8ToLimitKeepsCoAgentResultValid(t *testing.T) {
	limit := 120
	notice := "\n\n[Result truncated — exceeded 120 bytes]"
	result := truncateUTF8ToLimit(strings.Repeat("界", 100), limit, notice)

	if len(result) > limit {
		t.Fatalf("result length = %d, want <= %d", len(result), limit)
	}
	if !utf8.ValidString(result) {
		t.Fatalf("expected valid UTF-8 result, got %q", result)
	}
	if !strings.Contains(result, "[Result truncated") {
		t.Fatalf("expected truncation notice, got %q", result)
	}
}

func TestRecoverCoAgentPanicMarksRegistryEntryFailed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	registry := NewCoAgentRegistry(1, logger)
	id, state, err := registry.RegisterWithPriority("coagent", "panic test", func() {}, 2)
	if err != nil {
		t.Fatalf("register co-agent: %v", err)
	}
	if state != CoAgentRunning {
		t.Fatalf("initial state = %s, want running", state)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer recoverCoAgentPanic(registry, id, logger)
		panic("boom")
	}()
	<-done

	agentInfo := registry.agents[id]
	if agentInfo == nil {
		t.Fatal("expected co-agent registry entry")
	}
	agentInfo.mu.Lock()
	defer agentInfo.mu.Unlock()
	if agentInfo.State != CoAgentFailed {
		t.Fatalf("state = %s, want failed", agentInfo.State)
	}
	if !strings.Contains(agentInfo.Error, "co-agent panic: boom") {
		t.Fatalf("error = %q, want panic marker", agentInfo.Error)
	}
	if agentInfo.CompletedAt.IsZero() {
		t.Fatal("expected CompletedAt to be set")
	}
}

func TestCoAgentRegistryStopCleanupLoopStopsBackgroundCleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	registry := NewCoAgentRegistry(1, logger)
	registry.ConfigureLifecycle(10*time.Millisecond, time.Millisecond)

	registry.mu.Lock()
	registry.agents["old-1"] = &CoAgentInfo{
		ID:          "old-1",
		State:       CoAgentCompleted,
		StartedAt:   time.Now().Add(-time.Second),
		CompletedAt: time.Now().Add(-time.Second),
	}
	registry.mu.Unlock()

	registry.StartCleanupLoop()
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		registry.mu.RLock()
		_, exists := registry.agents["old-1"]
		registry.mu.RUnlock()
		if !exists {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	registry.mu.RLock()
	_, exists := registry.agents["old-1"]
	registry.mu.RUnlock()
	if exists {
		t.Fatal("expected cleanup loop to remove stale co-agent")
	}

	registry.StopCleanupLoop()

	registry.mu.Lock()
	registry.agents["old-2"] = &CoAgentInfo{
		ID:          "old-2",
		State:       CoAgentCompleted,
		StartedAt:   time.Now().Add(-time.Second),
		CompletedAt: time.Now().Add(-time.Second),
	}
	registry.mu.Unlock()

	time.Sleep(50 * time.Millisecond)
	registry.mu.RLock()
	_, exists = registry.agents["old-2"]
	running := registry.cleanupRunning
	registry.mu.RUnlock()
	if !exists {
		t.Fatal("expected stale co-agent to remain after cleanup loop was stopped")
	}
	if running {
		t.Fatal("expected cleanup loop to report stopped state")
	}
}

func TestDeepCloneConfigDetachesNestedSlicesAndMaps(t *testing.T) {
	original := config.Config{}
	original.Budget.Models = []config.ModelCost{{Name: "main"}}
	original.CircuitBreaker.RetryIntervals = []string{"1s", "2s"}
	original.EmailAccounts = []config.EmailAccount{{ID: "mail-1", Name: "Primary"}}
	original.Webhooks.Outgoing = []config.OutgoingWebhook{{
		ID:         "hook-1",
		Name:       "Primary hook",
		Headers:    map[string]string{"Authorization": "Bearer token"},
		Parameters: []config.WebhookParameter{{Name: "message", Type: "string"}},
	}}

	cloned := deepClone(original)
	cloned.Budget.Models[0].Name = "coagent"
	cloned.CircuitBreaker.RetryIntervals[0] = "10s"
	cloned.EmailAccounts[0].Name = "Changed"
	cloned.Webhooks.Outgoing[0].Headers["Authorization"] = "changed"
	cloned.Webhooks.Outgoing[0].Parameters[0].Name = "changed"

	if original.Budget.Models[0].Name != "main" {
		t.Fatalf("original budget model mutated: %q", original.Budget.Models[0].Name)
	}
	if original.CircuitBreaker.RetryIntervals[0] != "1s" {
		t.Fatalf("original retry interval mutated: %q", original.CircuitBreaker.RetryIntervals[0])
	}
	if original.EmailAccounts[0].Name != "Primary" {
		t.Fatalf("original email account mutated: %q", original.EmailAccounts[0].Name)
	}
	if got := original.Webhooks.Outgoing[0].Headers["Authorization"]; got != "Bearer token" {
		t.Fatalf("original webhook header mutated: %q", got)
	}
	if got := original.Webhooks.Outgoing[0].Parameters[0].Name; got != "message" {
		t.Fatalf("original webhook parameter mutated: %q", got)
	}
}

func TestExtractCoAgentPartialResultPrefersSummary(t *testing.T) {
	history := memory.NewEphemeralHistoryManager()
	if err := history.Add("assistant", "Draft answer", 1, false, false); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := history.SetSummary("Concise summary of current progress"); err != nil {
		t.Fatalf("SetSummary: %v", err)
	}

	got := extractCoAgentPartialResult(history)
	if got != "Concise summary of current progress" {
		t.Fatalf("extractCoAgentPartialResult() = %q", got)
	}
}

func TestCoAgentRegistryGetStatusIncludesPartialResultAndRetryHint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	registry := NewCoAgentRegistry(1, logger)
	id, _, err := registry.RegisterWithPriority("coagent", "Investigate deployment issue", func() {}, 2)
	if err != nil {
		t.Fatalf("RegisterWithPriority: %v", err)
	}

	registry.RecordPartialResult(id, "Build step passed, deploy step still pending.")
	registry.RecordRetry(id, "temporary network timeout")

	status, err := registry.GetStatus(id)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status["state"] != string(CoAgentRunning) {
		t.Fatalf("state = %v, want %q", status["state"], CoAgentRunning)
	}
	if status["partial_result"] != "Build step passed, deploy step still pending." {
		t.Fatalf("partial_result = %v", status["partial_result"])
	}
	if status["retry_count"] != 1 {
		t.Fatalf("retry_count = %v, want 1", status["retry_count"])
	}
	hint, _ := status["retry_hint"].(string)
	if !strings.Contains(hint, "already retried") {
		t.Fatalf("retry_hint = %q, want running retry guidance", hint)
	}
}
