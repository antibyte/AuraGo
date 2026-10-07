package flows

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// newClockedRunnerFixture is newRunnerFixture with the engine, and so the event bus,
// on clock.
func newClockedRunnerFixture(t *testing.T, cfg RunnerConfig, clock Clock) *runnerFixture {
	t.Helper()
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
		OnRunStarted:  func(rec RunRecord) { started <- rec },
		OnRunFinished: func(rec RunRecord, _ RunResult) { finished <- rec },
	}, cfg, discardLogger())
	t.Cleanup(func() { _ = r.Shutdown(context.Background()) })
	return &runnerFixture{r: r, store: store, gate: g, started: started, finished: finished}
}

// drainEvents reads ch until it is closed and returns the events it got.
func drainEvents(t *testing.T, ch <-chan RunEvent) []RunEvent {
	t.Helper()
	var got []RunEvent
	timeout := time.After(3 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return got
			}
			got = append(got, ev)
		case <-timeout:
			t.Fatalf("the event channel was not closed; got %s", describeEvents(got))
		}
	}
}

// describeEvents prints events with their run summaries, which %+v shows as pointers.
func describeEvents(events []RunEvent) string {
	parts := make([]string, len(events))
	for i, ev := range events {
		parts[i] = fmt.Sprintf("#%d %s %s", ev.Seq, ev.Type, ev.NodeID)
		if ev.Run != nil {
			parts[i] += fmt.Sprintf(" %+v", *ev.Run)
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// A run that waits, behind its flow's run (queue policy) or for a global slot, can be
// subscribed to as soon as Start returned its id and streams its events once it starts.
func TestRunnerQueuedRunIsSubscribable(t *testing.T) {
	cases := []struct {
		name   string
		cfg    RunnerConfig
		policy ConcurrencyPolicy
	}{
		{"flow queue", RunnerConfig{}, ConcurrencyQueue},
		{"global limit", RunnerConfig{MaxParallelRuns: 1}, ConcurrencyParallel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newRunnerFixture(t, tc.cfg)
			f := fx.flow(t, "flow_aaaaaaaada", tc.policy)
			first := fx.start(t, f, ModeLive, "a")
			fx.gate.waitStarted(t, "a")
			queued := fx.start(t, f, ModeLive, "b")
			if queued.Status != StartQueued {
				t.Fatalf("second start = %+v, want queued", queued)
			}
			backlog, ch, cancel, ok := fx.r.Subscribe(queued.RunID, 0)
			defer cancel()
			if !ok || len(backlog) != 0 {
				t.Fatalf("Subscribe(queued run) = %s ok=%v, want ok and no events yet", describeEvents(backlog), ok)
			}
			fx.gate.release("a")
			if rec := fx.waitFinished(t); rec.ID != first.RunID {
				t.Fatalf("finished = %+v", rec)
			}
			fx.gate.waitStarted(t, "b")
			select {
			case ev := <-ch:
				if ev.Type != EventRunStarted || ev.RunID != queued.RunID || ev.Seq != 1 {
					t.Fatalf("first event of the queued run = %+v", ev)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("the subscriber got no event after the queued run started")
			}
			fx.gate.release("b")
			events := drainEvents(t, ch)
			if n := len(events); n == 0 || events[n-1].Type != EventRunFinished || events[n-1].Run.Status != RunSuccess {
				t.Fatalf("events after the start = %s", describeEvents(events))
			}
			if rec := fx.waitFinished(t); rec.ID != queued.RunID || rec.Status != RunSuccess {
				t.Fatalf("queued run finished = %+v", rec)
			}
		})
	}
}

// A run that ends before it started (cancelled while queued, or at shutdown) ends
// its event log with a run_finished event, and its subscribers' channels close.
func TestRunnerUnstartedRunsFinishTheirLog(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{})
	f := fx.flow(t, "flow_aaaaaaaadb", ConcurrencyQueue)
	running := fx.start(t, f, ModeLive, "a")
	fx.gate.waitStarted(t, "a")
	cancelled := fx.start(t, f, ModeLive, "b")
	shutDown := fx.start(t, f, ModeLive, "c")
	_, cancelledEvents, stopCancelled, ok1 := fx.r.Subscribe(cancelled.RunID, 0)
	defer stopCancelled()
	_, shutDownEvents, stopShutDown, ok2 := fx.r.Subscribe(shutDown.RunID, 0)
	defer stopShutDown()
	if !ok1 || !ok2 {
		t.Fatalf("Subscribe(queued runs) ok = %v, %v", ok1, ok2)
	}
	assertEnded := func(runID string, ch <-chan RunEvent, code string) {
		t.Helper()
		events := drainEvents(t, ch)
		if len(events) != 1 || events[0].Seq != 1 || events[0].RunID != runID || events[0].Type != EventRunFinished ||
			events[0].Run.Status != RunCancelled || events[0].Run.ErrorCode != code {
			t.Fatalf("events of %s = %s, want one run_finished %s", runID, describeEvents(events), code)
		}
	}

	if !fx.r.Cancel(cancelled.RunID) {
		t.Fatal("Cancel(queued) = false")
	}
	assertEnded(cancelled.RunID, cancelledEvents, "FLOW_CANCELLED")
	if rec := fx.waitFinished(t); rec.ID != cancelled.RunID {
		t.Fatalf("finished = %+v", rec)
	}

	if err := fx.r.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	assertEnded(shutDown.RunID, shutDownEvents, "FLOW_SHUTDOWN")
	got := map[string]RunStatus{}
	for i := 0; i < 2; i++ {
		rec := fx.waitFinished(t)
		got[rec.ID] = rec.Status
	}
	if got[running.RunID] != RunCancelled || got[shutDown.RunID] != RunCancelled {
		t.Fatalf("shutdown results = %v", got)
	}
}

// A panic in the runner's own code during a run (here in the event sink) is recorded
// as FLOW_RUNNER_PANIC, ends the run's event log and still frees the run's global and
// flow slots, so the flow's queued run starts.
func TestRunnerPanicReleasesTheRun(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{MaxParallelRuns: 1})
	fx.r.testOnEvent = func(ev RunEvent) {
		if ev.Type == EventStepFinished && ev.Step != nil && ev.Step.Output["name"] == "a" {
			panic("injected sink failure")
		}
	}
	f := fx.flow(t, "flow_aaaaaaaadc", ConcurrencyQueue)
	crashed := fx.start(t, f, ModeLive, "a")
	fx.gate.waitStarted(t, "a")
	next := fx.start(t, f, ModeLive, "b")
	if next.Status != StartQueued {
		t.Fatalf("second start = %+v, want queued", next)
	}
	fx.gate.release("a")
	rec := fx.waitFinished(t)
	if rec.ID != crashed.RunID || rec.Status != RunError || rec.ErrorCode != "FLOW_RUNNER_PANIC" || rec.FinishedAt == nil {
		t.Fatalf("crashed run finished = %+v", rec)
	}
	stored, _, err := fx.store.GetRun(context.Background(), crashed.RunID)
	if err != nil || stored.Status != RunError || stored.ErrorCode != "FLOW_RUNNER_PANIC" {
		t.Fatalf("stored crashed run = %+v, %v", stored, err)
	}
	backlog, ch, cancel, ok := fx.r.Subscribe(crashed.RunID, 0)
	defer cancel()
	n := len(backlog)
	if !ok || n < 2 || backlog[n-1].Type != EventRunFinished || backlog[n-1].Seq != backlog[n-2].Seq+1 ||
		backlog[n-1].Run.ErrorCode != "FLOW_RUNNER_PANIC" {
		t.Fatalf("crashed run's log = %s ok=%v", describeEvents(backlog), ok)
	}
	if _, open := <-ch; open {
		t.Fatal("the crashed run's log must be finished")
	}

	fx.gate.waitStarted(t, "b")
	fx.gate.release("b")
	if rec := fx.waitFinished(t); rec.ID != next.RunID || rec.Status != RunSuccess {
		t.Fatalf("next run finished = %+v", rec)
	}
	if fx.r.IsBusy(f.ID) {
		t.Fatal("the flow must be idle after its runs ended")
	}
}

// Start writes the run record without holding the runner's lock, so IsBusy and
// Cancel answer while the database is slow. An open write transaction holds
// SQLite's write lock, so Start's CreateRun waits in the busy handler (for up to
// the 5 s busy timeout) on a second connection of the pool.
func TestRunnerRecordsRunsOutsideTheLock(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{})
	f := fx.flow(t, "flow_aaaaaaaadd", ConcurrencyQueue)
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

	answered := make(chan bool, 1)
	go func() { answered <- fx.r.IsBusy(f.ID) || fx.r.Cancel("run_unknown") }()
	select {
	case got := <-answered:
		if got {
			t.Fatal("IsBusy or Cancel saw a run that is not recorded yet")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("IsBusy and Cancel waited for Start's database write")
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
	if out.err != nil || out.res.Status != StartStarted {
		t.Fatalf("Start = %+v, %v", out.res, out.err)
	}
	fx.gate.waitStarted(t, "a")
	fx.gate.release("a")
	if rec := fx.waitFinished(t); rec.ID != out.res.RunID || rec.Status != RunSuccess {
		t.Fatalf("finished = %+v", rec)
	}
}

// Runs waiting for a global slot start in arrival order: the next queued run of a
// flow that finished does not overtake runs that waited for a slot before it.
func TestRunnerGlobalQueueIsFIFO(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{MaxParallelRuns: 1})
	q := fx.flow(t, "flow_aaaaaaaade", ConcurrencyQueue)
	par := fx.flow(t, "flow_aaaaaaaadf", ConcurrencyParallel)
	fx.start(t, q, ModeLive, "a")
	fx.gate.waitStarted(t, "a")
	if res := fx.start(t, par, ModeLive, "x"); res.Status != StartQueued {
		t.Fatalf("parallel start = %+v, want queued for the global slot", res)
	}
	if res := fx.start(t, q, ModeLive, "b"); res.Status != StartQueued {
		t.Fatalf("second queue start = %+v, want queued behind its flow", res)
	}
	fx.gate.release("a")
	fx.waitFinished(t)
	fx.gate.waitStarted(t, "x")
	fx.gate.release("x")
	fx.waitFinished(t)
	fx.gate.waitStarted(t, "b")
	fx.gate.release("b")
	fx.waitFinished(t)
}

// A flow's parallel-policy and test runs that wait for a global slot are capped at
// MaxQueuedPerFlow, like a queue-policy flow's queue. A refused start records no run,
// and other flows are not affected.
func TestRunnerCapsRunsWaitingForASlot(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{MaxParallelRuns: 1, MaxQueuedPerFlow: 2})
	par := fx.flow(t, "flow_aaaaaaaadg", ConcurrencyParallel)
	fx.start(t, par, ModeLive, "a")
	fx.gate.waitStarted(t, "a")
	if res := fx.start(t, par, ModeLive, "b"); res.Status != StartQueued {
		t.Fatalf("first waiting start = %+v, want queued", res)
	}
	if res := fx.start(t, par, ModeTest, "c"); res.Status != StartQueued {
		t.Fatalf("second waiting start = %+v, want queued", res)
	}
	for _, mode := range []RunMode{ModeLive, ModeTest} {
		if _, err := fx.r.Start(StartRequest{Flow: par, Mode: mode, TriggerNode: par.Nodes[0].ID}); !errors.Is(err, ErrQueueFull) {
			t.Fatalf("third waiting %s start = %v, want ErrQueueFull", mode, err)
		}
	}
	if runs, err := fx.store.ListRuns(context.Background(), par.ID, RunFilter{}); err != nil || len(runs) != 3 {
		t.Fatalf("stored runs = %d, %v; refused starts must not record a run", len(runs), err)
	}
	other := fx.flow(t, "flow_aaaaaaaadh", ConcurrencyParallel)
	if res := fx.start(t, other, ModeLive, "d"); res.Status != StartQueued {
		t.Fatalf("another flow's start = %+v, want queued", res)
	}
	for _, name := range []string{"a", "b", "c", "d"} {
		fx.gate.release(name)
	}
	for i := 0; i < 4; i++ {
		fx.waitFinished(t)
	}
}

// The leak clock of a run's event log restarts when the run leaves its queue: a run
// that waited 20 h keeps its log while it runs, also past the leak horizon counted
// from when it was queued.
func TestRunnerRearmsTheLogAtLaunch(t *testing.T) {
	clock := newFakeClock(storeNow)
	fx := newClockedRunnerFixture(t, RunnerConfig{}, clock)
	f := fx.flow(t, "flow_aaaaaaaadi", ConcurrencyQueue)
	fx.start(t, f, ModeLive, "a")
	fx.gate.waitStarted(t, "a")
	queued := fx.start(t, f, ModeLive, "b")
	clock.Advance(20 * time.Hour)
	fx.gate.release("a")
	fx.waitFinished(t)
	fx.gate.waitStarted(t, "b")
	clock.Advance(6 * time.Hour) // 26 h after b was queued, 6 h after it started
	fx.r.Bus().Sweep()
	_, _, cancel, ok := fx.r.Subscribe(queued.RunID, 0)
	cancel()
	if !ok {
		t.Fatal("the log of a run that waited 20 h was swept 6 h after the run started")
	}
	fx.gate.release("b")
	if rec := fx.waitFinished(t); rec.ID != queued.RunID || rec.Status != RunSuccess {
		t.Fatalf("finished = %+v", rec)
	}
}
