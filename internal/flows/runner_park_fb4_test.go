package flows

import (
	"context"
	"testing"
	"time"
)

// newFB4ParkFixture is newRunnerFixture with a fake clock, so logic.wait waits until the test
// advances it.
func newFB4ParkFixture(t *testing.T, cfg RunnerConfig) (*runnerFixture, *fakeClock) {
	t.Helper()
	clock := newFakeClock(storeNow)
	reg := newTestRegistry(t)
	g := newGate()
	reg.MustRegister(&NodeDef{Type: "test.gate", DefaultTimeout: 10 * time.Second,
		Params: []ParamSpec{{Name: "name", Kind: ParamText, Templatable: true}},
		Execute: func(ctx context.Context, in ExecInput) (ExecResult, error) {
			name := Stringify(in.Params["name"])
			g.started <- name
			select {
			case <-g.ch(name):
				return ExecResult{Output: map[string]any{"name": name}}, nil
			case <-ctx.Done():
				return ExecResult{}, ctx.Err()
			}
		}})
	store := openTestStore(t)
	started := make(chan RunRecord, 32)
	finished := make(chan RunRecord, 32)
	r := NewRunner(newTestEngine(reg, &Services{Clock: clock, Location: time.UTC}, 4), store, RunnerHooks{
		OnRunStarted:  func(rec RunRecord, _ *Flow) { started <- rec },
		OnRunFinished: func(rec RunRecord, _ RunResult) { finished <- rec },
	}, cfg, discardLogger())
	t.Cleanup(func() { _ = r.Shutdown(context.Background()) })
	return &runnerFixture{r: r, store: store, gate: g, started: started, finished: finished}, clock
}

// fb4WaitFlow stores a parallel flow that waits 60 s and then echoes.
func fb4WaitFlow(t *testing.T, fx *runnerFixture, id string) *Flow {
	t.Helper()
	b := newFlow("Wait " + id)
	tr := b.node("start", "test.trigger", nil)
	w := b.node("pause", TypeWait, map[string]any{"mode": "duration", "seconds": 60.0})
	e := b.node("after", "test.echo", map[string]any{"value": "done"})
	b.edge(tr, PortOut, w)
	b.edge(w, PortOut, e)
	f := b.build()
	f.ID = id
	f.Settings.Concurrency = ConcurrencyParallel
	if _, err := fx.store.CreateFlow(context.Background(), f, "", storeNow); err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	return f
}

func fb4Counts(r *Runner) (slots, parked, resumers int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.slots, r.parked, len(r.resumers)
}

// fb4WaitResumers polls (like fakeClock.WaitForWaiters) until n parked runs want their slot back.
func fb4WaitResumers(t *testing.T, r *Runner, n int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, _, got := fb4Counts(r); got == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected %d runs waiting to resume", n)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestFB4WaitingRunGivesItsSlotToAnotherFlow(t *testing.T) {
	fx, clock := newFB4ParkFixture(t, RunnerConfig{MaxParallelRuns: 1})
	wait := fb4WaitFlow(t, fx, "flow_aaaaaaaapa")
	other := fx.flow(t, "flow_aaaaaaaapb", ConcurrencyQueue)
	w := fx.start(t, wait, ModeLive, "")
	clock.WaitForWaiters(t, 1)
	if slots, parked, _ := fb4Counts(fx.r); slots != 0 || parked != 1 {
		t.Fatalf("while waiting: slots %d parked %d, want 0 and 1", slots, parked)
	}
	g := fx.start(t, other, ModeLive, "g1")
	if g.Status != StartStarted {
		t.Fatalf("other flow = %+v, want started in the freed slot", g)
	}
	fx.gate.waitStarted(t, "g1")
	clock.Advance(time.Minute)
	fb4WaitResumers(t, fx.r, 1) // the wait is over; the run queues for a slot behind g1
	select {
	case rec := <-fx.finished:
		t.Fatalf("run %s finished while the slot was taken", rec.ID)
	default:
	}
	fx.gate.release("g1")
	got := map[string]RunStatus{}
	for i := 0; i < 2; i++ {
		rec := fx.waitFinished(t)
		got[rec.ID] = rec.Status
	}
	if got[w.RunID] != RunSuccess || got[g.RunID] != RunSuccess {
		t.Fatalf("results = %v", got)
	}
	if slots, parked, resumers := fb4Counts(fx.r); slots != 0 || parked != 0 || resumers != 0 {
		t.Fatalf("after both: slots %d parked %d resumers %d", slots, parked, resumers)
	}
}

func TestFB4ParkedRunsAreCapped(t *testing.T) {
	fx, clock := newFB4ParkFixture(t, RunnerConfig{MaxParallelRuns: 1, MaxParkedRuns: 1})
	wait := fb4WaitFlow(t, fx, "flow_aaaaaaaapc")
	other := fx.flow(t, "flow_aaaaaaaapd", ConcurrencyQueue)
	fx.start(t, wait, ModeLive, "")
	clock.WaitForWaiters(t, 1)
	if second := fx.start(t, wait, ModeLive, ""); second.Status != StartStarted {
		t.Fatalf("second wait run = %+v", second)
	}
	clock.WaitForWaiters(t, 2)
	if g := fx.start(t, other, ModeLive, "g2"); g.Status != StartQueued {
		t.Fatalf("other flow = %+v, want queued: over the parking cap the second wait keeps its slot", g)
	}
	if slots, parked, _ := fb4Counts(fx.r); slots != 1 || parked != 1 {
		t.Fatalf("slots %d parked %d, want 1 and 1", slots, parked)
	}
}

func TestFB4CancelAndShutdownEndParkedRuns(t *testing.T) {
	fx, clock := newFB4ParkFixture(t, RunnerConfig{MaxParallelRuns: 1})
	wait := fb4WaitFlow(t, fx, "flow_aaaaaaaape")
	first := fx.start(t, wait, ModeLive, "")
	clock.WaitForWaiters(t, 1)
	if known, _ := fx.r.CancelRun(first.RunID); !known {
		t.Fatal("the parked run is unknown to CancelRun")
	}
	if rec := fx.waitFinished(t); rec.ID != first.RunID || rec.Status != RunCancelled {
		t.Fatalf("cancelled run = %+v", rec)
	}
	if slots, parked, resumers := fb4Counts(fx.r); slots != 0 || parked != 0 || resumers != 0 {
		t.Fatalf("after cancel: slots %d parked %d resumers %d", slots, parked, resumers)
	}
	second := fx.start(t, wait, ModeLive, "")
	clock.WaitForWaiters(t, 2) // the cancelled sleep's channel stays pending in the fake clock
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := fx.r.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown = %v", err)
	}
	if rec := fx.waitFinished(t); rec.ID != second.RunID || rec.Status != RunCancelled {
		t.Fatalf("run at shutdown = %+v", rec)
	}
}

// fb4WaitParked polls until n runs have given their slot back.
func fb4WaitParked(t *testing.T, r *Runner, n int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, got, _ := fb4Counts(r); got == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected %d parked runs", n)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestFB4ResumingRunQueuesBehindEarlierWaitingRuns(t *testing.T) {
	fx, clock := newFB4ParkFixture(t, RunnerConfig{MaxParallelRuns: 1})
	wait := fb4WaitFlow(t, fx, "flow_aaaaaaaapf")
	other := fx.flow(t, "flow_aaaaaaaapg", ConcurrencyQueue)
	third := fx.flow(t, "flow_aaaaaaaaph", ConcurrencyQueue)
	w := fx.start(t, wait, ModeLive, "")
	clock.WaitForWaiters(t, 1)
	g := fx.start(t, other, ModeLive, "g1")
	fx.gate.waitStarted(t, "g1")
	c := fx.start(t, third, ModeLive, "c1")
	if c.Status != StartQueued {
		t.Fatalf("third flow = %+v, want queued", c)
	}
	clock.Advance(time.Minute)
	fb4WaitResumers(t, fx.r, 1) // the resumer's ticket is newer than the queued run's
	fx.gate.release("g1")
	fx.gate.waitStarted(t, "c1") // the run that waited first gets the slot first
	fx.gate.release("c1")
	got := map[string]RunStatus{}
	var order []string
	for i := 0; i < 3; i++ {
		rec := fx.waitFinished(t)
		got[rec.ID] = rec.Status
		order = append(order, rec.ID)
	}
	if order[2] != w.RunID {
		t.Fatalf("finish order %v: the resuming run must come last", order)
	}
	if got[w.RunID] != RunSuccess || got[g.RunID] != RunSuccess || got[c.RunID] != RunSuccess {
		t.Fatalf("results = %v", got)
	}
}

func TestFB4ResumingRunIsNotGatedOrRecordedAgain(t *testing.T) {
	gateCalls := 0
	fx, clock := newFB4ParkFixture(t, RunnerConfig{MaxParallelRuns: 1, Gate: func() error {
		gateCalls++
		return context.Canceled
	}})
	wait := fb4WaitFlow(t, fx, "flow_aaaaaaaapi")
	res, err := fx.r.Start(StartRequest{Flow: wait, Mode: ModeLive, TriggerNode: wait.Nodes[0].ID,
		TriggerType: "manual", Gated: true})
	if err != nil || res.Status != StartStarted {
		t.Fatalf("Start = %+v, %v", res, err)
	}
	clock.WaitForWaiters(t, 1)
	fb4WaitParked(t, fx.r, 1)
	other := fx.flow(t, "flow_aaaaaaaapj", ConcurrencyQueue)
	fx.start(t, other, ModeLive, "g1")
	fx.gate.waitStarted(t, "g1")
	clock.Advance(time.Minute)
	fb4WaitResumers(t, fx.r, 1)
	fx.gate.release("g1")
	for i := 0; i < 2; i++ {
		if rec := fx.waitFinished(t); rec.Status != RunSuccess {
			t.Fatalf("run %s = %s (%s), want success", rec.ID, rec.Status, rec.ErrorCode)
		}
	}
	if gateCalls != 0 {
		t.Fatalf("the gate was asked %d times", gateCalls)
	}
	starts := 0
	for len(fx.started) > 0 {
		<-fx.started
		starts++
	}
	if starts != 2 {
		t.Fatalf("OnRunStarted ran %d times for 2 runs", starts)
	}
}

func TestFB4RunKeepsItsSlotWhileAnotherBranchIsBusy(t *testing.T) {
	fx, clock := newFB4ParkFixture(t, RunnerConfig{MaxParallelRuns: 2})
	b := newFlow("Branches")
	tr := b.node("start", "test.trigger", nil)
	w := b.node("pause", TypeWait, map[string]any{"mode": "duration", "seconds": 60.0})
	g := b.node("busy", "test.gate", map[string]any{"name": "p1"})
	b.edge(tr, PortOut, w)
	b.edge(tr, PortOut, g)
	f := b.build()
	f.ID = "flow_aaaaaaaapk"
	f.Settings.Concurrency = ConcurrencyParallel
	if _, err := fx.store.CreateFlow(context.Background(), f, "", storeNow); err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	res := fx.start(t, f, ModeLive, "")
	clock.WaitForWaiters(t, 1)
	fx.gate.waitStarted(t, "p1")
	if slots, parked, _ := fb4Counts(fx.r); slots != 1 || parked != 0 {
		t.Fatalf("with a busy sibling: slots %d parked %d, want 1 and 0", slots, parked)
	}
	fx.gate.release("p1")
	fb4WaitParked(t, fx.r, 1) // now only the wait is left
	if slots, _, _ := fb4Counts(fx.r); slots != 0 {
		t.Fatalf("slots %d after the sibling finished, want 0", slots)
	}
	clock.Advance(time.Minute)
	if rec := fx.waitFinished(t); rec.ID != res.RunID || rec.Status != RunSuccess {
		t.Fatalf("run = %+v", rec)
	}
	if slots, parked, resumers := fb4Counts(fx.r); slots != 0 || parked != 0 || resumers != 0 {
		t.Fatalf("after: slots %d parked %d resumers %d", slots, parked, resumers)
	}
}
