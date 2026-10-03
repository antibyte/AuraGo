package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

func newCoAgentQueueTestRegistry() *CoAgentRegistry {
	return NewCoAgentRegistry(1, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func coAgentStateForTest(t *testing.T, registry *CoAgentRegistry, id string) CoAgentState {
	t.Helper()
	registry.mu.RLock()
	info := registry.agents[id]
	registry.mu.RUnlock()
	if info == nil {
		t.Fatalf("co-agent %s is not registered", id)
	}
	info.mu.Lock()
	defer info.mu.Unlock()
	return info.State
}

func TestCoAgentQueueTimeoutIsTerminalAndReleasesSlot(t *testing.T) {
	registry := newCoAgentQueueTestRegistry()
	runningID, _, err := registry.RegisterWithPriority("coagent", "running", func() {}, 2)
	if err != nil {
		t.Fatalf("register running co-agent: %v", err)
	}
	queuedID, state, err := registry.RegisterWithPriority("coagent", "queued", func() {}, 2)
	if err != nil {
		t.Fatalf("register queued co-agent: %v", err)
	}
	if state != CoAgentQueued {
		t.Fatalf("second co-agent state = %s, want queued", state)
	}

	err = awaitCoAgentSlot(registry, queuedID, context.Background(), 10*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("awaitCoAgentSlot error = %v, want deadline exceeded", err)
	}
	if got := coAgentStateForTest(t, registry, queuedID); got != CoAgentFailed {
		t.Fatalf("timed-out queued co-agent state = %s, want failed", got)
	}

	registry.Complete(runningID, "done", 0, 0)
	if got := coAgentStateForTest(t, registry, queuedID); got != CoAgentFailed {
		t.Fatalf("timed-out co-agent was promoted after a slot freed: state = %s", got)
	}
	if slots := registry.AvailableSlots(); slots != 1 {
		t.Fatalf("available slots = %d, want 1 (a timed-out queue entry must not hold a slot)", slots)
	}
}

func TestCoAgentQueueTimeoutRacingPromotionNeverLeaksSlot(t *testing.T) {
	for i := 0; i < 50; i++ {
		registry := newCoAgentQueueTestRegistry()
		runningID, _, err := registry.RegisterWithPriority("coagent", "running", func() {}, 2)
		if err != nil {
			t.Fatalf("iteration %d: register running co-agent: %v", i, err)
		}
		queuedID, _, err := registry.RegisterWithPriority("coagent", "queued", func() {}, 2)
		if err != nil {
			t.Fatalf("iteration %d: register queued co-agent: %v", i, err)
		}
		done := make(chan error, 1)
		go func() { done <- awaitCoAgentSlot(registry, queuedID, context.Background(), time.Millisecond) }()
		time.Sleep(time.Millisecond)
		registry.Complete(runningID, "done", 0, 0)
		if err := <-done; err == nil {
			// Promoted in time: the co-agent owns the slot and completes normally.
			registry.Complete(queuedID, "done", 0, 0)
		}
		if slots := registry.AvailableSlots(); slots != 1 {
			t.Fatalf("iteration %d: available slots = %d, want 1", i, slots)
		}
	}
}

func TestCoAgentFailIfActiveKeepsCancelledState(t *testing.T) {
	registry := newCoAgentQueueTestRegistry()
	if _, _, err := registry.RegisterWithPriority("coagent", "running", func() {}, 2); err != nil {
		t.Fatalf("register running co-agent: %v", err)
	}
	queuedID, _, err := registry.RegisterWithPriority("coagent", "queued", func() {}, 2)
	if err != nil {
		t.Fatalf("register queued co-agent: %v", err)
	}
	if err := registry.Stop(queuedID); err != nil {
		t.Fatalf("Stop queued co-agent: %v", err)
	}

	if registry.FailIfActive(queuedID, "late failure") {
		t.Fatal("FailIfActive changed a cancelled co-agent")
	}
	if got := coAgentStateForTest(t, registry, queuedID); got != CoAgentCancelled {
		t.Fatalf("stopped co-agent state = %s, want cancelled", got)
	}
	if slots := registry.AvailableSlots(); slots != 0 {
		t.Fatalf("available slots = %d, want 0 while the first co-agent runs", slots)
	}
}

func TestCoAgentFailIfActiveReleasesRunningSlotOnce(t *testing.T) {
	registry := newCoAgentQueueTestRegistry()
	runningID, _, err := registry.RegisterWithPriority("coagent", "running", func() {}, 2)
	if err != nil {
		t.Fatalf("register running co-agent: %v", err)
	}
	if !registry.FailIfActive(runningID, "start failed") {
		t.Fatal("expected FailIfActive to fail the running co-agent")
	}
	if registry.FailIfActive(runningID, "second failure") {
		t.Fatal("second FailIfActive must be a no-op")
	}
	if slots := registry.AvailableSlots(); slots != 1 {
		t.Fatalf("available slots = %d, want 1", slots)
	}
}

func TestCoAgentRunContextStartsAfterQueueWaitAndFollowsStop(t *testing.T) {
	lifecycleCtx, stop := context.WithCancel(context.Background())
	defer stop()
	time.Sleep(30 * time.Millisecond) // simulated queue wait

	runCtx, cancelRun := coAgentRunContext(lifecycleCtx, time.Second)
	defer cancelRun()
	deadline, ok := runCtx.Deadline()
	if !ok {
		t.Fatal("run context has no deadline")
	}
	if remaining := time.Until(deadline); remaining < 900*time.Millisecond {
		t.Fatalf("run budget = %s, want the full timeout after promotion", remaining)
	}

	stop()
	select {
	case <-runCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("stopping the co-agent did not cancel its run context")
	}
}
