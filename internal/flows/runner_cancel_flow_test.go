package flows

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// CancelFlow cancels the flow's running run, the run queued behind it and the flow's
// test run that waits for a global slot, and leaves another flow's waiting run alone,
// which then takes the freed slot. The two runs that never started end before
// CancelFlow returns.
func TestRunnerCancelFlow(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{MaxParallelRuns: 1})
	a := fx.flow(t, "flow_aaaaaaaaea", ConcurrencyQueue)
	b := fx.flow(t, "flow_aaaaaaaaeb", ConcurrencyParallel)
	running := fx.start(t, a, ModeLive, "a1")
	fx.gate.waitStarted(t, "a1")
	queued := fx.start(t, a, ModeLive, "a2")  // behind the flow's active run
	other := fx.start(t, b, ModeLive, "b1")   // waits for the global slot
	testRun := fx.start(t, a, ModeTest, "t1") // waits for the global slot behind b1
	for name, res := range map[string]StartResult{"queued": queued, "other": other, "test": testRun} {
		if res.Status != StartQueued {
			t.Fatalf("%s start = %+v, want queued", name, res)
		}
	}

	if n := fx.r.CancelFlow(a.ID); n != 3 {
		t.Fatalf("CancelFlow = %d, want 3", n)
	}
	got := map[string]RunRecord{}
drain:
	for {
		select {
		case rec := <-fx.finished:
			got[rec.ID] = rec
		default:
			break drain
		}
	}
	for _, id := range []string{queued.RunID, testRun.RunID} {
		rec, ok := got[id]
		if !ok || rec.Status != RunCancelled || rec.ErrorCode != "FLOW_CANCELLED" {
			t.Fatalf("unstarted run %s when CancelFlow returned = %+v (finished: %v)", id, rec, ok)
		}
	}
	rec, ok := got[running.RunID] // it ends in the background, maybe already
	if !ok {
		rec = fx.waitFinished(t)
	}
	if rec.ID != running.RunID || rec.Status != RunCancelled {
		t.Fatalf("finished = %+v, want the running run cancelled", rec)
	}

	// The other flow's run is the one that gets the slot, and it runs normally.
	fx.gate.waitStarted(t, "b1")
	fx.gate.release("b1")
	if rec := fx.waitFinished(t); rec.ID != other.RunID || rec.Status != RunSuccess {
		t.Fatalf("the other flow's run = %+v", rec)
	}
	ctx := context.Background()
	for _, id := range []string{running.RunID, queued.RunID, testRun.RunID} {
		stored, _, err := fx.store.GetRun(ctx, id)
		if err != nil || stored.Status != RunCancelled {
			t.Fatalf("stored run %s = %+v, %v", id, stored, err)
		}
	}
	fx.r.mu.Lock()
	state := struct{ slots, live, queues, waiting, active int }{fx.r.slots, len(fx.r.live), len(fx.r.flowQueue), len(fx.r.waiting), len(fx.r.cancels)}
	fx.r.mu.Unlock()
	if state != (struct{ slots, live, queues, waiting, active int }{}) {
		t.Fatalf("runner state after all runs ended = %+v, want empty", state)
	}
	if n := fx.r.CancelFlow(a.ID); n != 0 {
		t.Fatalf("CancelFlow of an idle flow = %d", n)
	}
}

// CancelFlow counts only the runs it cancels itself. A running run whose node ignores
// its context stays active after a cancel; neither a second CancelFlow nor one after
// Cancel or Shutdown counts it again, while a newly queued run of the flow is counted.
func TestRunnerCancelFlowCountsOnlyNewCancels(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{})
	entered := make(chan string, 4)
	release := make(chan struct{})
	var once sync.Once
	letGo := func() { once.Do(func() { close(release) }) }
	t.Cleanup(letGo) // runs before the fixture's Shutdown
	fx.r.engine.reg.MustRegister(&NodeDef{Type: "test.stubborn", DefaultTimeout: 10 * time.Second,
		Execute: func(_ context.Context, in ExecInput) (ExecResult, error) {
			entered <- in.Run.ID
			<-release // ignores the context on purpose
			return ExecResult{}, nil
		}})
	stubborn := func(id string) *Flow {
		b := newFlow("Stur")
		tr := b.node("start", "test.trigger", nil)
		b.edge(tr, PortOut, b.node("stubborn", "test.stubborn", nil))
		f := b.build()
		f.ID = id
		if _, err := fx.store.CreateFlow(context.Background(), f, "", storeNow); err != nil {
			t.Fatalf("CreateFlow: %v", err)
		}
		return f
	}
	start := func(f *Flow) StartResult {
		t.Helper()
		res, err := fx.r.Start(StartRequest{Flow: f, Mode: ModeLive, TriggerNode: f.Nodes[0].ID})
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		return res
	}
	waitEntered := func(runID string) {
		t.Helper()
		select {
		case got := <-entered:
			if got != runID {
				t.Fatalf("node entered by %s, want %s", got, runID)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("run %s did not reach its node", runID)
		}
	}

	a := stubborn("flow_aaaaaaaaed")
	runA := start(a)
	waitEntered(runA.RunID)
	if n := fx.r.CancelFlow(a.ID); n != 1 {
		t.Fatalf("first CancelFlow = %d, want 1", n)
	}
	if n := fx.r.CancelFlow(a.ID); n != 0 {
		t.Fatalf("second CancelFlow = %d, want 0 for the run cancelled already", n)
	}
	queued := start(a)
	if queued.Status != StartQueued {
		t.Fatalf("second start = %+v, want queued", queued)
	}
	if n := fx.r.CancelFlow(a.ID); n != 1 {
		t.Fatalf("CancelFlow with a new queued run = %d, want 1", n)
	}

	b := stubborn("flow_aaaaaaaaee")
	runB := start(b)
	waitEntered(runB.RunID)
	if !fx.r.Cancel(runB.RunID) {
		t.Fatal("Cancel(running) = false")
	}
	if n := fx.r.CancelFlow(b.ID); n != 0 {
		t.Fatalf("CancelFlow after Cancel = %d, want 0", n)
	}

	c := stubborn("flow_aaaaaaaaef")
	runC := start(c)
	waitEntered(runC.RunID)
	stopped, stop := context.WithCancel(context.Background())
	stop() // Shutdown cancels the runs, then returns at once instead of waiting for them
	if err := fx.r.Shutdown(stopped); !errors.Is(err, context.Canceled) {
		t.Fatalf("Shutdown with stuck runs and an ended context = %v", err)
	}
	if n := fx.r.CancelFlow(c.ID); n != 0 {
		t.Fatalf("CancelFlow after Shutdown = %d, want 0 for the run Shutdown cancelled", n)
	}

	letGo()
	ended := map[string]bool{}
	for i := 0; i < 4; i++ {
		ended[fx.waitFinished(t).ID] = true
	}
	if !ended[runA.RunID] || !ended[queued.RunID] || !ended[runB.RunID] || !ended[runC.RunID] {
		t.Fatalf("finished runs = %v", ended)
	}
}

// CancelFlow waits for a Start that is still writing its run record, so that run is
// cancelled rather than left to start after CancelFlow returned.
func TestRunnerCancelFlowWaitsForAStartInProgress(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{})
	f := fx.flow(t, "flow_aaaaaaaaec", ConcurrencyQueue)
	ctx := context.Background()
	tx, err := fx.store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() }) // runs before the fixture's Shutdown
	if _, err := tx.ExecContext(ctx, `UPDATE flows SET name = name WHERE id = ?`, f.ID); err != nil {
		t.Fatalf("taking the write lock: %v", err)
	}
	inUse := fx.store.db.Stats().InUse
	type outcome struct {
		res StartResult
		err error
	}
	started := make(chan outcome, 1)
	go func() {
		res, err := fx.r.Start(StartRequest{Flow: f, Mode: ModeLive, TriggerNode: f.Nodes[0].ID, TriggerType: "manual",
			TriggerData: map[string]any{"gate": "a"}})
		started <- outcome{res, err}
	}()
	for deadline := time.Now().Add(3 * time.Second); fx.store.db.Stats().InUse <= inUse; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("Start did not reach the database")
		}
	}

	cancelled := make(chan int, 1)
	go func() { cancelled <- fx.r.CancelFlow(f.ID) }()
	select {
	case n := <-cancelled:
		t.Fatalf("CancelFlow returned %d while a Start was still recording its run", n)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	var out outcome
	select {
	case out = <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return after the database was free")
	}
	if out.err != nil || out.res.RunID == "" {
		t.Fatalf("Start = %+v, %v", out.res, out.err)
	}
	select {
	case n := <-cancelled:
		if n != 1 {
			t.Fatalf("CancelFlow = %d, want 1 (the run whose Start it waited for)", n)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("CancelFlow did not return after Start")
	}
	if rec := fx.waitFinished(t); rec.ID != out.res.RunID || rec.Status != RunCancelled {
		t.Fatalf("finished = %+v, want the run cancelled", rec)
	}
}
