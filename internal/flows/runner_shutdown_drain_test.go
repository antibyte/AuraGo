package flows

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunnerRepeatedShutdownWaitsForFinalCallback(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.fast", DefaultTimeout: time.Second, Execute: func(context.Context, ExecInput) (ExecResult, error) { return ExecResult{}, nil }})
	store := openTestStore(t)
	entered, resume := make(chan struct{}), make(chan struct{})
	released := false
	r := NewRunner(newTestEngine(reg, nil, 1), store, RunnerHooks{OnRunFinished: func(RunRecord, RunResult) { close(entered); <-resume }}, RunnerConfig{}, discardLogger())
	t.Cleanup(func() {
		if !released {
			close(resume)
		}
		_ = r.Shutdown(context.Background())
	})
	b := newFlow("Drain")
	trigger := b.node("start", "test.trigger", nil)
	work := b.node("work", "test.fast", nil)
	b.edge(trigger, PortOut, work)
	f := b.build()
	f.ID = "flow_iiiiiiiiii"
	if _, err := store.CreateFlow(context.Background(), f, "", storeNow); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Start(StartRequest{Flow: f, Mode: ModeLive, TriggerNode: f.Nodes[0].ID, TriggerType: "manual"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("final callback did not start")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.Shutdown(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("first shutdown=%v", err)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	if err := r.Shutdown(ctx2); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("repeat authorized closing store before final callback: %v", err)
	}
	close(resume)
	released = true
	if err := r.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}
