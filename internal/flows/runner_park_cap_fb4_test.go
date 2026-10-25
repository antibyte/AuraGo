package flows

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fb4MixedFlow stores a parallel flow with two branches: logic.wait (60 s) and a test.gate node
// named by the trigger data. While the gate is busy the run keeps its slot; once only the wait is
// left it parks.
func fb4MixedFlow(t *testing.T, fx *runnerFixture, id string) *Flow {
	t.Helper()
	b := newFlow("Mixed " + id)
	tr := b.node("start", "test.trigger", nil)
	w := b.node("pause", TypeWait, map[string]any{"mode": "duration", "seconds": 60.0})
	e := b.node("after", "test.echo", map[string]any{"value": "done"})
	g := b.node("busy", "test.gate", map[string]any{"name": "{{trigger.data.gate}}"})
	b.edge(tr, PortOut, w)
	b.edge(w, PortOut, e)
	b.edge(tr, PortOut, g)
	f := b.build()
	f.ID = id
	f.Settings.Concurrency = ConcurrencyParallel
	if _, err := fx.store.CreateFlow(context.Background(), f, "", storeNow); err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	return f
}

// fb4Poll waits until cond holds; cond reads the runner under its lock.
func fb4Poll(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for: %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func fb4FlowSlots(r *Runner, flowID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.flowSlots[flowID]
}

// fb4Balanced reports slots + parked == len(cancels) under the runner's lock.
func fb4Balanced(r *Runner) (ok bool, slots, parked, cancels int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.slots+r.parked == len(r.cancels), r.slots, r.parked, len(r.cancels)
}

// fb4ParkFirstRun starts a mixed-flow run with gate gateName, releases the gate and waits until
// the run is parked (its wait is the only busy node).
func fb4ParkFirstRun(t *testing.T, fx *runnerFixture, clock *fakeClock, f *Flow, gateName string, waiters int) StartResult {
	t.Helper()
	res := fx.start(t, f, ModeLive, gateName)
	fx.gate.waitStarted(t, gateName)
	clock.WaitForWaiters(t, waiters)
	fx.gate.release(gateName)
	fb4WaitParked(t, fx.r, 1)
	return res
}

func fb4WaitAll(t *testing.T, fx *runnerFixture, n int) map[string]RunStatus {
	t.Helper()
	got := map[string]RunStatus{}
	for i := 0; i < n; i++ {
		rec := fx.waitFinished(t)
		got[rec.ID] = rec.Status
	}
	return got
}

// a. A resumer whose flow is at MaxSlotsPerFlow is passed over while a free slot goes to a run of
// another flow; it gets a slot when its flow drops below the cap.
func TestFB4ResumerHeldBackByFlowCapLetsOtherFlowTakeTheSlot(t *testing.T) {
	fx, clock := newFB4ParkFixture(t, RunnerConfig{MaxParallelRuns: 2, MaxSlotsPerFlow: 1})
	f := fb4MixedFlow(t, fx, "flow_aaaaaaaaqa")
	other := fx.flow(t, "flow_aaaaaaaaqb", ConcurrencyQueue)
	r1 := fb4ParkFirstRun(t, fx, clock, f, "a1", 1)
	r2 := fx.start(t, f, ModeLive, "a2") // takes the flow's only share
	fx.gate.waitStarted(t, "a2")
	clock.WaitForWaiters(t, 2)
	clock.Advance(time.Minute)
	fb4WaitResumers(t, fx.r, 1)
	if slots, parked, _ := fb4Counts(fx.r); slots != 1 || parked != 1 {
		t.Fatalf("resumer behind the cap: slots %d parked %d, want 1 and 1 (a free slot stays free for it)", slots, parked)
	}
	o := fx.start(t, other, ModeLive, "o1")
	if o.Status != StartStarted {
		t.Fatalf("other flow = %+v, want started in the free slot", o)
	}
	fx.gate.waitStarted(t, "o1")
	if _, _, resumers := fb4Counts(fx.r); resumers != 1 {
		t.Fatalf("resumers = %d, the resumer must still wait for its flow's share", resumers)
	}
	if n := fb4FlowSlots(fx.r, f.ID); n != 1 {
		t.Fatalf("flow slots = %d, want 1", n)
	}
	fx.gate.release("a2") // the flow drops below its cap
	fb4Poll(t, "the resumer is granted", func() bool { _, parked, resumers := fb4Counts(fx.r); return parked == 0 && resumers == 0 })
	fx.gate.release("o1")
	got := fb4WaitAll(t, fx, 3)
	for _, id := range []string{r1.RunID, r2.RunID, o.RunID} {
		if got[id] != RunSuccess {
			t.Fatalf("results = %v", got)
		}
	}
	if ok, s, p, c := fb4Balanced(fx.r); !ok || s != 0 || p != 0 || c != 0 {
		t.Fatalf("after: slots %d parked %d cancels %d", s, p, c)
	}
}

// b. A resumer and a test run share one parallel flow: the test run does not count against the
// flow's share, so it takes the free slot while the resumer waits behind the cap.
func TestFB4ResumerAndTestRunShareAParallelFlow(t *testing.T) {
	fx, clock := newFB4ParkFixture(t, RunnerConfig{MaxParallelRuns: 2, MaxSlotsPerFlow: 1})
	f := fb4MixedFlow(t, fx, "flow_aaaaaaaaqc")
	r1 := fb4ParkFirstRun(t, fx, clock, f, "a1", 1)
	r2 := fx.start(t, f, ModeLive, "a2")
	fx.gate.waitStarted(t, "a2")
	clock.WaitForWaiters(t, 2)
	clock.Advance(time.Minute)
	fb4WaitResumers(t, fx.r, 1)
	tr := fx.start(t, f, ModeTest, "t1")
	if tr.Status != StartStarted {
		t.Fatalf("test run = %+v, want started: test runs ignore the flow's share", tr)
	}
	fx.gate.waitStarted(t, "t1")
	if n := fb4FlowSlots(fx.r, f.ID); n != 1 {
		t.Fatalf("flow slots = %d, want 1 (the test run is not counted)", n)
	}
	if _, parked, resumers := fb4Counts(fx.r); parked != 1 || resumers != 1 {
		t.Fatalf("parked %d resumers %d, want the resumer still waiting", parked, resumers)
	}
	fx.gate.release("t1")
	fx.gate.release("a2")
	clock.WaitForWaiters(t, 1) // the test run's own wait
	clock.Advance(time.Minute)
	got := fb4WaitAll(t, fx, 3)
	for _, id := range []string{r1.RunID, r2.RunID, tr.RunID} {
		if got[id] != RunSuccess {
			t.Fatalf("results = %v", got)
		}
	}
	if ok, s, p, c := fb4Balanced(fx.r); !ok || s != 0 || p != 0 || c != 0 {
		t.Fatalf("after: slots %d parked %d cancels %d", s, p, c)
	}
	if n := fb4FlowSlots(fx.r, f.ID); n != 0 {
		t.Fatalf("flow slots = %d after all runs", n)
	}
}

// c. Two logic.wait branches of one run: the run gives its slot back once (when both wait) and
// takes it back once, although both branches wake at the same time.
func TestFB4TwoParallelWaitBranchesReleaseAndAcquireOnce(t *testing.T) {
	fx, clock := newFB4ParkFixture(t, RunnerConfig{MaxParallelRuns: 1})
	b := newFlow("Two waits")
	tr := b.node("start", "test.trigger", nil)
	w1 := b.node("pause1", TypeWait, map[string]any{"mode": "duration", "seconds": 60.0})
	w2 := b.node("pause2", TypeWait, map[string]any{"mode": "duration", "seconds": 61.0})
	e1 := b.node("after1", "test.echo", map[string]any{"value": "one"})
	e2 := b.node("after2", "test.echo", map[string]any{"value": "two"})
	b.edge(tr, PortOut, w1)
	b.edge(tr, PortOut, w2)
	b.edge(w1, PortOut, e1)
	b.edge(w2, PortOut, e2)
	f := b.build()
	f.ID = "flow_aaaaaaaaqd"
	f.Settings.Concurrency = ConcurrencyParallel
	if _, err := fx.store.CreateFlow(context.Background(), f, "", storeNow); err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	other := fx.flow(t, "flow_aaaaaaaaqe", ConcurrencyQueue)
	w := fx.start(t, f, ModeLive, "")
	clock.WaitForWaiters(t, 2)
	fb4WaitParked(t, fx.r, 1)
	if slots, parked, _ := fb4Counts(fx.r); slots != 0 || parked != 1 {
		t.Fatalf("both branches wait: slots %d parked %d, want 0 and 1 (one release)", slots, parked)
	}
	g := fx.start(t, other, ModeLive, "g1")
	fx.gate.waitStarted(t, "g1")
	clock.Advance(2 * time.Minute) // both waits end together while the slot is taken
	fb4WaitResumers(t, fx.r, 1)
	if slots, parked, resumers := fb4Counts(fx.r); slots != 1 || parked != 1 || resumers != 1 {
		t.Fatalf("slots %d parked %d resumers %d, want 1, 1, 1: one run asks for one slot", slots, parked, resumers)
	}
	fx.gate.release("g1")
	got := fb4WaitAll(t, fx, 2)
	if got[w.RunID] != RunSuccess || got[g.RunID] != RunSuccess {
		t.Fatalf("results = %v", got)
	}
	r := fx.r
	r.mu.Lock()
	defer r.mu.Unlock()
	// Tickets: the two admitted runs and exactly one Acquire.
	if r.nextTicket != 3 {
		t.Fatalf("tickets handed out = %d, want 3 (2 launches + 1 Acquire)", r.nextTicket)
	}
	if r.slots != 0 || r.parked != 0 || len(r.resumers) != 0 || len(r.cancels) != 0 {
		t.Fatalf("after: slots %d parked %d resumers %d cancels %d", r.slots, r.parked, len(r.resumers), len(r.cancels))
	}
}

// d. A run that sits in r.resumers with every slot taken ends cleanly when it is cancelled or runs
// into its time limit: it holds no slot, so the accounting must stay balanced.
func TestFB4CancelAndTimeoutWhileResumerWaitsForSlot(t *testing.T) {
	fx, clock := newFB4ParkFixture(t, RunnerConfig{MaxParallelRuns: 1})
	wait := fb4WaitFlow(t, fx, "flow_aaaaaaaaqf")
	other := fx.flow(t, "flow_aaaaaaaaqg", ConcurrencyQueue)
	check := func(label string, wantCancels int) {
		t.Helper()
		ok, s, p, c := fb4Balanced(fx.r)
		_, _, resumers := fb4Counts(fx.r)
		if !ok || p != 0 || resumers != 0 || c != wantCancels || s != wantCancels {
			t.Fatalf("%s: slots %d parked %d cancels %d resumers %d", label, s, p, c, resumers)
		}
	}

	// Cancel.
	w1 := fx.start(t, wait, ModeLive, "")
	clock.WaitForWaiters(t, 1)
	fb4WaitParked(t, fx.r, 1)
	g1 := fx.start(t, other, ModeLive, "g1")
	fx.gate.waitStarted(t, "g1")
	clock.Advance(time.Minute)
	fb4WaitResumers(t, fx.r, 1)
	if known, _ := fx.r.CancelRun(w1.RunID); !known {
		t.Fatal("CancelRun does not know the resuming run")
	}
	if rec := fx.waitFinished(t); rec.ID != w1.RunID || rec.Status != RunCancelled {
		t.Fatalf("cancelled resumer = %+v", rec)
	}
	check("after cancel", 1) // only g1 is left

	// Run time limit (real time: the engine's limit is a context deadline).
	w2, err := fx.r.Start(StartRequest{Flow: wait, Mode: ModeLive, TriggerNode: wait.Nodes[0].ID,
		TriggerType: "manual", Timeout: time.Second})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if w2.Status != StartQueued {
		t.Fatalf("second wait run = %+v, want queued behind g1", w2)
	}
	fx.gate.release("g1")
	if rec := fx.waitFinished(t); rec.ID != g1.RunID {
		t.Fatalf("finished %+v, want g1", rec)
	}
	clock.WaitForWaiters(t, 1) // the new run's sleep (the first one fired with the advance)
	fb4WaitParked(t, fx.r, 1)
	g2 := fx.start(t, other, ModeLive, "g2")
	fx.gate.waitStarted(t, "g2")
	clock.Advance(time.Minute)
	fb4WaitResumers(t, fx.r, 1)
	rec := fx.waitFinished(t)
	if rec.ID != w2.RunID || rec.Status == RunSuccess {
		t.Fatalf("timed-out resumer = %+v, want its time limit to end it", rec)
	}
	check("after timeout", 1)
	fx.gate.release("g2")
	if rec := fx.waitFinished(t); rec.ID != g2.RunID || rec.Status != RunSuccess {
		t.Fatalf("g2 = %+v", rec)
	}
	check("at the end", 0)
}

// e. A gated run that waited for its slot is asked once, before it executes; resuming after a
// logic.wait is no new launch and must not ask the gate again.
func TestFB4GatedRunThatWaitedIsNotGatedAgainOnResume(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	fx, clock := newFB4ParkFixture(t, RunnerConfig{MaxParallelRuns: 1, Gate: func() error {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls > 1 {
			return context.Canceled
		}
		return nil
	}})
	wait := fb4WaitFlow(t, fx, "flow_aaaaaaaaqh")
	other := fx.flow(t, "flow_aaaaaaaaqi", ConcurrencyQueue)
	g := fx.start(t, other, ModeLive, "g1")
	fx.gate.waitStarted(t, "g1")
	res, err := fx.r.Start(StartRequest{Flow: wait, Mode: ModeLive, TriggerNode: wait.Nodes[0].ID,
		TriggerType: "manual", Gated: true})
	if err != nil || res.Status != StartQueued {
		t.Fatalf("Start = %+v, %v; want queued behind g1", res, err)
	}
	fx.gate.release("g1")
	clock.WaitForWaiters(t, 1)
	fb4WaitParked(t, fx.r, 1)
	clock.Advance(time.Minute) // resume: the slot is free
	got := fb4WaitAll(t, fx, 2)
	if got[g.RunID] != RunSuccess || got[res.RunID] != RunSuccess {
		t.Fatalf("results = %v", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatalf("the gate was asked %d times, want exactly once (at launch)", calls)
	}
}
