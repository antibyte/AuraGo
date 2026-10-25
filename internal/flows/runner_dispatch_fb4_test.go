package flows

import "testing"

// S5: a parallel flow holds at most its share of the global slots; its extra runs wait without
// blocking another flow's run, and test runs are not capped.
func TestFB4ParallelFlowHoldsAtMostItsShareOfSlots(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{MaxParallelRuns: 4})
	par := fx.flow(t, "flow_aaaaaaaasa", ConcurrencyParallel)
	other := fx.flow(t, "flow_aaaaaaaasb", ConcurrencyQueue)
	fx.start(t, par, ModeLive, "p1")
	fx.gate.waitStarted(t, "p1")
	fx.start(t, par, ModeLive, "p2")
	fx.gate.waitStarted(t, "p2")
	if third := fx.start(t, par, ModeLive, "p3"); third.Status != StartQueued {
		t.Fatalf("third run of the parallel flow = %+v, want queued (2 of 4 slots per flow)", third)
	}
	fx.gate.assertNotStarted(t)
	if q := fx.start(t, other, ModeLive, "q1"); q.Status != StartStarted {
		t.Fatalf("another flow's run = %+v, want started past the waiting parallel run", q)
	}
	fx.gate.waitStarted(t, "q1")
	if tr := fx.start(t, par, ModeTest, "t1"); tr.Status != StartStarted {
		t.Fatalf("a test run = %+v, want started (test runs are not capped)", tr)
	}
	fx.gate.waitStarted(t, "t1")
	fx.gate.release("p1")
	fx.waitFinished(t)
	fx.gate.waitStarted(t, "p3")
	for _, name := range []string{"p2", "p3", "q1", "t1"} {
		fx.gate.release(name)
	}
	for i := 0; i < 4; i++ {
		fx.waitFinished(t)
	}
}
