package flows

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type gate struct {
	mu      sync.Mutex
	chans   map[string]chan struct{}
	started chan string
}

func newGate() *gate {
	return &gate{chans: map[string]chan struct{}{}, started: make(chan string, 64)}
}

func (g *gate) ch(name string) chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	c, ok := g.chans[name]
	if !ok {
		c = make(chan struct{})
		g.chans[name] = c
	}
	return c
}

func (g *gate) release(name string) { close(g.ch(name)) }

func (g *gate) waitStarted(t *testing.T, name string) {
	t.Helper()
	select {
	case got := <-g.started:
		if got != name {
			t.Fatalf("gate %q started, want %q", got, name)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("gate %q did not start", name)
	}
}

func (g *gate) assertNotStarted(t *testing.T) {
	t.Helper()
	select {
	case got := <-g.started:
		t.Fatalf("gate %q started unexpectedly", got)
	case <-time.After(100 * time.Millisecond):
	}
}

type runnerFixture struct {
	r        *Runner
	store    *Store
	gate     *gate
	started  chan RunRecord
	finished chan RunRecord
}

func newRunnerFixture(t *testing.T, cfg RunnerConfig) *runnerFixture {
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
	r := NewRunner(newTestEngine(reg, nil, 4), store, RunnerHooks{
		OnRunStarted:  func(rec RunRecord) { started <- rec },
		OnRunFinished: func(rec RunRecord, _ RunResult) { finished <- rec },
	}, cfg, discardLogger())
	t.Cleanup(func() { _ = r.Shutdown(context.Background()) })
	return &runnerFixture{r: r, store: store, gate: g, started: started, finished: finished}
}

func (fx *runnerFixture) flow(t *testing.T, id string, policy ConcurrencyPolicy) *Flow {
	t.Helper()
	b := newFlow("Gate " + id)
	tr := b.node("start", "test.trigger", nil)
	g := b.node("gate", "test.gate", map[string]any{"name": "{{trigger.data.gate}}"})
	b.edge(tr, PortOut, g)
	f := b.build()
	f.ID = id
	f.Settings.Concurrency = policy
	if _, err := fx.store.CreateFlow(context.Background(), f, "", storeNow); err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	return f
}

func (fx *runnerFixture) start(t *testing.T, f *Flow, mode RunMode, gateName string) StartResult {
	t.Helper()
	res, err := fx.r.Start(StartRequest{Flow: f, Mode: mode, TriggerNode: f.Nodes[0].ID, TriggerType: "manual",
		TriggerData: map[string]any{"gate": gateName}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return res
}

func (fx *runnerFixture) waitFinished(t *testing.T) RunRecord {
	t.Helper()
	select {
	case rec := <-fx.finished:
		return rec
	case <-time.After(3 * time.Second):
		t.Fatal("no run finished")
	}
	return RunRecord{}
}

func TestRunnerRunsAndPersists(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{})
	f := fx.flow(t, "flow_aaaaaaaaca", ConcurrencyQueue)
	res := fx.start(t, f, ModeLive, "a")
	if res.Status != StartStarted || res.RunID == "" {
		t.Fatalf("Start = %+v", res)
	}
	fx.gate.waitStarted(t, "a")
	select {
	case rec := <-fx.started:
		if rec.ID != res.RunID || rec.Status != RunRunning {
			t.Fatalf("OnRunStarted = %+v", rec)
		}
	default:
		t.Fatal("OnRunStarted must fire before the first node runs")
	}
	fx.gate.release("a")
	rec := fx.waitFinished(t)
	if rec.ID != res.RunID || rec.Status != RunSuccess || rec.FinishedAt == nil {
		t.Fatalf("finished = %+v", rec)
	}
	stored, steps, err := fx.store.GetRun(context.Background(), res.RunID)
	if err != nil || stored.Status != RunSuccess || len(steps) != 2 {
		t.Fatalf("stored = %+v, %d steps, %v", stored, len(steps), err)
	}
}

func TestRunnerQueuePolicy(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{})
	f := fx.flow(t, "flow_aaaaaaaacb", ConcurrencyQueue)
	first := fx.start(t, f, ModeLive, "a")
	fx.gate.waitStarted(t, "a")
	second := fx.start(t, f, ModeLive, "b")
	if first.Status != StartStarted || second.Status != StartQueued {
		t.Fatalf("statuses = %s, %s", first.Status, second.Status)
	}
	fx.gate.assertNotStarted(t)
	fx.gate.release("a")
	if rec := fx.waitFinished(t); rec.ID != first.RunID {
		t.Fatalf("first finished = %+v", rec)
	}
	fx.gate.waitStarted(t, "b")
	fx.gate.release("b")
	if rec := fx.waitFinished(t); rec.ID != second.RunID || rec.Status != RunSuccess {
		t.Fatalf("second finished = %+v", rec)
	}
}

func TestRunnerSkipPolicyAndTestRuns(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{})
	f := fx.flow(t, "flow_aaaaaaaacc", ConcurrencySkip)
	fx.start(t, f, ModeLive, "a")
	fx.gate.waitStarted(t, "a")
	if res := fx.start(t, f, ModeLive, "b"); res.Status != StartSkipped || res.RunID != "" {
		t.Fatalf("skip = %+v", res)
	}
	if res := fx.start(t, f, ModeTest, "t"); res.Status != StartStarted {
		t.Fatalf("test runs ignore the policy: %+v", res)
	}
	fx.gate.waitStarted(t, "t")
	fx.gate.release("a")
	fx.gate.release("t")
	fx.waitFinished(t)
	fx.waitFinished(t)
}

func TestRunnerGlobalLimitAndQueueFull(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{MaxParallelRuns: 1, MaxQueuedPerFlow: 1})
	par := fx.flow(t, "flow_aaaaaaaacd", ConcurrencyParallel)
	first := fx.start(t, par, ModeLive, "a")
	fx.gate.waitStarted(t, "a")
	if second := fx.start(t, par, ModeLive, "b"); second.Status != StartQueued {
		t.Fatalf("global limit = %+v", second)
	}
	fx.gate.assertNotStarted(t)
	fx.gate.release("a")
	if rec := fx.waitFinished(t); rec.ID != first.RunID {
		t.Fatalf("finished = %+v", rec)
	}
	fx.gate.waitStarted(t, "b")
	fx.gate.release("b")
	fx.waitFinished(t)

	q := fx.flow(t, "flow_aaaaaaaace", ConcurrencyQueue)
	fx.start(t, q, ModeLive, "c")
	fx.gate.waitStarted(t, "c")
	fx.start(t, q, ModeLive, "d")
	if _, err := fx.r.Start(StartRequest{Flow: q, Mode: ModeLive, TriggerNode: q.Nodes[0].ID}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("third start = %v, want ErrQueueFull", err)
	}
	fx.gate.release("c")
	fx.gate.release("d")
	fx.waitFinished(t)
	fx.waitFinished(t)
}

func TestRunnerCancel(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{})
	f := fx.flow(t, "flow_aaaaaaaacf", ConcurrencyQueue)
	running := fx.start(t, f, ModeLive, "a")
	fx.gate.waitStarted(t, "a")
	queued := fx.start(t, f, ModeLive, "b")
	if !fx.r.Cancel(queued.RunID) {
		t.Fatal("Cancel(queued) = false")
	}
	if rec := fx.waitFinished(t); rec.ID != queued.RunID || rec.Status != RunCancelled {
		t.Fatalf("cancelled queued = %+v", rec)
	}
	if !fx.r.Cancel(running.RunID) {
		t.Fatal("Cancel(running) = false")
	}
	if rec := fx.waitFinished(t); rec.ID != running.RunID || rec.Status != RunCancelled {
		t.Fatalf("cancelled running = %+v", rec)
	}
	if fx.r.Cancel("run_unknown") {
		t.Fatal("Cancel(unknown) = true")
	}
	if fx.r.IsBusy(f.ID) {
		t.Fatal("the flow must be idle after cancelling everything")
	}
}

func TestRunnerSubscribe(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{})
	f := fx.flow(t, "flow_aaaaaaaacg", ConcurrencyQueue)
	res := fx.start(t, f, ModeLive, "a")
	fx.gate.waitStarted(t, "a")
	backlog, ch, cancel, ok := fx.r.Subscribe(res.RunID, 0)
	defer cancel()
	if !ok || len(backlog) < 3 || backlog[0].Type != EventRunStarted {
		t.Fatalf("backlog = %+v ok=%v", backlog, ok)
	}
	fx.gate.release("a")
	var last RunEvent
	for ev := range ch {
		last = ev
	}
	if last.Type != EventRunFinished {
		t.Fatalf("last event = %+v", last)
	}
	fx.waitFinished(t)
}

func TestRunnerShutdown(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{})
	f := fx.flow(t, "flow_aaaaaaaach", ConcurrencyQueue)
	running := fx.start(t, f, ModeLive, "a")
	fx.gate.waitStarted(t, "a")
	queued := fx.start(t, f, ModeLive, "b")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := fx.r.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	got := map[string]RunRecord{}
	for i := 0; i < 2; i++ {
		rec := fx.waitFinished(t)
		got[rec.ID] = rec
	}
	if got[queued.RunID].ErrorCode != "FLOW_SHUTDOWN" || got[running.RunID].Status != RunCancelled {
		t.Fatalf("shutdown results = %+v", got)
	}
	if _, err := fx.r.Start(StartRequest{Flow: f, Mode: ModeLive, TriggerNode: f.Nodes[0].ID}); !errors.Is(err, ErrRunnerClosed) {
		t.Fatalf("Start after Shutdown = %v", err)
	}
}
