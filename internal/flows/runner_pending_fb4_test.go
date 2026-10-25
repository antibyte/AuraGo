package flows

import (
	"errors"
	"strings"
	"testing"
)

// E5: the runner refuses a run that would wait while MaxPendingRuns runs wait already.
func TestFB4RunnerCapsWaitingRunsInTotal(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{MaxParallelRuns: 1, MaxPendingRuns: 2})
	f := fx.flow(t, "flow_aaaaaaaapf", ConcurrencyQueue)
	fx.start(t, f, ModeLive, "a1")
	fx.gate.waitStarted(t, "a1")
	fx.start(t, f, ModeLive, "a2")
	fx.start(t, f, ModeLive, "a3")
	_, err := fx.r.Start(StartRequest{Flow: f, Mode: ModeLive, TriggerNode: f.Nodes[0].ID, TriggerData: map[string]any{"gate": "a4"}})
	if !errors.Is(err, ErrRunnerQueueFull) || !errors.Is(err, ErrQueueFull) {
		t.Fatalf("fourth start = %v, want ErrRunnerQueueFull wrapping ErrQueueFull", err)
	}
}

// E5: waiting trigger data is bounded by MaxPendingBytes; a first waiting run always fits.
func TestFB4RunnerBoundsWaitingTriggerData(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{MaxParallelRuns: 1, MaxPendingBytes: 1000})
	busy := fx.flow(t, "flow_aaaaaaaapg", ConcurrencyQueue)
	par := fx.flow(t, "flow_aaaaaaaaph", ConcurrencyParallel)
	fx.start(t, busy, ModeLive, "b1")
	fx.gate.waitStarted(t, "b1")
	big := func(gate string) StartRequest {
		return StartRequest{Flow: par, Mode: ModeLive, TriggerNode: par.Nodes[0].ID,
			TriggerData: map[string]any{"gate": gate, "pad": strings.Repeat("x", 600)}}
	}
	if res, err := fx.r.Start(big("b2")); err != nil || res.Status != StartQueued {
		t.Fatalf("first waiting run = %+v, %v", res, err)
	}
	if _, err := fx.r.Start(big("b3")); !errors.Is(err, ErrRunnerQueueFull) {
		t.Fatalf("second big waiting run = %v, want ErrRunnerQueueFull", err)
	}
	if res, err := fx.r.Start(StartRequest{Flow: par, Mode: ModeLive, TriggerNode: par.Nodes[0].ID,
		TriggerData: map[string]any{"gate": "b4"}}); err != nil || res.Status != StartQueued {
		t.Fatalf("a small waiting run = %+v, %v", res, err)
	}
}
