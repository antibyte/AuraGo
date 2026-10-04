package flows

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type runCapture struct {
	mu     sync.Mutex
	events []RunEvent
}

func (c *runCapture) sink(ev RunEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, ev)
}

func (c *runCapture) types() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.events))
	for i, ev := range c.events {
		out[i] = ev.Type
	}
	return out
}

func newTestEngine(reg *Registry, svc *Services, parallel int) *Engine {
	if svc == nil {
		svc = &Services{Location: time.UTC}
	}
	return NewEngine(reg, svc, discardLogger(), parallel)
}

// runWith executes a run; it is safe to call from goroutines.
func runWith(ctx context.Context, eng *Engine, f *Flow, req RunRequest) (RunResult, *runCapture) {
	c := &runCapture{}
	req.Flow = f
	if req.RunID == "" {
		req.RunID = "run_test"
	}
	if req.Mode == "" {
		req.Mode = ModeTest
	}
	if req.TriggerType == "" {
		req.TriggerType = "manual"
	}
	return eng.Execute(ctx, req, c.sink), c
}

func stepOf(res RunResult, nodeID string) *StepRecord {
	for i := range res.Steps {
		if res.Steps[i].NodeID == nodeID {
			return &res.Steps[i]
		}
	}
	return nil
}

func TestEngineLinearRun(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("Linear")
	tr := b.node("start", "test.trigger", nil)
	a := b.node("first", "test.echo", map[string]any{"value": "{{trigger.data.msg}}"})
	c := b.node("second", "test.echo", map[string]any{"value": "{{first.value}}/{{run.mode}}/{{flow.name}}/{{start.node}}"})
	b.edge(tr, PortOut, a)
	b.edge(a, PortOut, c)
	res, capture := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(),
		RunRequest{TriggerNode: tr, TriggerData: map[string]any{"msg": "hi"}})
	if res.Status != RunSuccess {
		t.Fatalf("status = %s (%s: %s)", res.Status, res.ErrorCode, res.ErrorMessage)
	}
	if got := res.Outputs["second"]["value"]; got != "hi/test/Linear/start" {
		t.Fatalf("second.value = %#v", got)
	}
	if data, _ := res.Outputs["start"]["data"].(map[string]any); data["msg"] != "hi" {
		t.Fatalf("trigger output = %#v", res.Outputs["start"])
	}
	want := []string{EventRunStarted, EventStepFinished, EventStepStarted, EventStepFinished, EventStepStarted, EventStepFinished, EventRunFinished}
	if !reflect.DeepEqual(capture.types(), want) {
		t.Fatalf("events = %v, want %v", capture.types(), want)
	}
	for i, ev := range capture.events {
		if ev.Seq != i+1 || ev.RunID != "run_test" {
			t.Fatalf("event %d = seq %d run %q", i, ev.Seq, ev.RunID)
		}
	}
	s := stepOf(res, a)
	if s == nil || s.Status != StepSuccess || s.Params["value"] != "hi" || !reflect.DeepEqual(s.Ports, []string{PortOut}) {
		t.Fatalf("step first = %+v", s)
	}
}

func TestEngineIfBranchSkipAndMerge(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("Branch")
	tr := b.node("start", "test.trigger", nil)
	check := b.node("check", TypeIf, map[string]any{"condition": cond("{{trigger.data.n}}", "gt", 3.0)})
	yes := b.node("yes", "test.echo", map[string]any{"value": "big"})
	no := b.node("no", "test.echo", map[string]any{"value": "small"})
	afterNo := b.node("after_no", "test.echo", map[string]any{"value": "x"})
	join := b.node("join", TypeMerge, nil)
	b.edge(tr, PortOut, check)
	b.edge(check, PortTrue, yes)
	b.edge(check, PortFalse, no)
	b.edge(no, PortOut, afterNo)
	b.edge(yes, PortOut, join)
	b.edge(afterNo, PortOut, join)
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(),
		RunRequest{TriggerNode: tr, TriggerData: map[string]any{"n": 5.0}})
	if res.Status != RunSuccess {
		t.Fatalf("status = %s %s", res.Status, res.ErrorMessage)
	}
	if stepOf(res, no).Status != StepSkipped || stepOf(res, afterNo).Status != StepSkipped {
		t.Fatal("the false branch must be skipped")
	}
	if stepOf(res, yes).Status != StepSuccess || stepOf(res, join).Status != StepSuccess {
		t.Fatal("yes and join must run")
	}
	out := res.Outputs["join"]
	if out["yes"] == nil || out["after_no"] != nil {
		t.Fatalf("merge output = %#v", out)
	}
}

func TestEngineParallelBranchesRespectLimit(t *testing.T) {
	reg := newTestRegistry(t)
	var active, peak int32
	reg.MustRegister(&NodeDef{Type: "test.slow", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		cur := atomic.AddInt32(&active, 1)
		for {
			old := atomic.LoadInt32(&peak)
			if cur <= old || atomic.CompareAndSwapInt32(&peak, old, cur) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		return ExecResult{}, nil
	}})
	b := newFlow("Parallel")
	tr := b.node("start", "test.trigger", nil)
	for i := 0; i < 4; i++ {
		b.edge(tr, PortOut, b.node(fmt.Sprintf("s%d", i), "test.slow", nil))
	}
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 2), b.build(), RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess {
		t.Fatalf("status = %s %s", res.Status, res.ErrorMessage)
	}
	if got := atomic.LoadInt32(&peak); got != 2 {
		t.Fatalf("peak parallel nodes = %d, want 2", got)
	}
}

func TestEngineOnlyNode(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("Only")
	tr := b.node("start", "test.trigger", nil)
	a := b.node("a", "test.echo", map[string]any{"value": "1"})
	bb := b.node("b", "test.echo", map[string]any{"value": "{{a.value}}2"})
	c := b.node("c", "test.echo", nil)
	b.edge(tr, PortOut, a)
	b.edge(a, PortOut, bb)
	b.edge(bb, PortOut, c)
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr, OnlyNode: bb})
	if res.Status != RunSuccess || res.Outputs["b"]["value"] != "12" {
		t.Fatalf("result = %s %#v", res.Status, res.Outputs)
	}
	if stepOf(res, c) != nil {
		t.Fatal("nodes after the tested node must not run or be recorded")
	}
}

func TestEngineDisabledAndOrphanNodes(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("Disabled")
	tr := b.node("start", "test.trigger", nil)
	a := b.node("a", "test.echo", nil)
	bb := b.node("b", "test.echo", nil)
	orphan := b.node("orphan", "test.echo", nil)
	b.edge(tr, PortOut, a)
	b.edge(a, PortOut, bb)
	f := b.build()
	f.NodeByID(a).Settings.Disabled = true
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), f, RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess {
		t.Fatalf("status = %s", res.Status)
	}
	for _, id := range []string{a, bb, orphan} {
		if s := stepOf(res, id); s == nil || s.Status != StepSkipped {
			t.Fatalf("node %s step = %+v, want skipped", id, s)
		}
	}
}

func TestEngineInvalidTrigger(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("Invalid")
	tr := b.node("start", "test.trigger", nil)
	a := b.node("a", "test.echo", nil)
	b.edge(tr, PortOut, a)
	f := b.build()
	eng := newTestEngine(reg, nil, 4)
	for _, trigger := range []string{"n_zzzzzzzz", a} {
		if res, _ := runWith(context.Background(), eng, f, RunRequest{TriggerNode: trigger}); res.Status != RunError || res.ErrorCode != "FLOW_TRIGGER_INVALID" {
			t.Fatalf("trigger %s: %s %s", trigger, res.Status, res.ErrorCode)
		}
	}
	f.NodeByID(tr).Settings.Disabled = true
	if res, _ := runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr}); res.ErrorCode != "FLOW_TRIGGER_DISABLED" {
		t.Fatalf("disabled trigger: %s", res.ErrorCode)
	}
}

func TestEngineUnknownTypeTemplateErrorAndDefaults(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.defaults", Params: []ParamSpec{{Name: "mode", Kind: ParamText, Default: "x"}},
		Execute: func(_ context.Context, in ExecInput) (ExecResult, error) {
			return ExecResult{Output: map[string]any{"mode": in.Params["mode"]}}, nil
		}})
	eng := newTestEngine(reg, nil, 4)

	b := newFlow("Unknown")
	tr := b.node("start", "test.trigger", nil)
	x := b.node("x", "nope.x", nil)
	b.edge(tr, PortOut, x)
	res, _ := runWith(context.Background(), eng, b.build(), RunRequest{TriggerNode: tr})
	if res.Status != RunError || res.ErrorCode != "NODE_TYPE_UNKNOWN" || res.ErrorNodeID != x {
		t.Fatalf("unknown type = %s %s %s", res.Status, res.ErrorCode, res.ErrorNodeID)
	}

	b = newFlow("Template")
	tr = b.node("start", "test.trigger", nil)
	y := b.node("y", "test.echo", map[string]any{"value": "{{bad"})
	b.edge(tr, PortOut, y)
	if res, _ := runWith(context.Background(), eng, b.build(), RunRequest{TriggerNode: tr}); res.ErrorCode != "FLOW_TEMPLATE_ERROR" {
		t.Fatalf("template error = %s", res.ErrorCode)
	}

	b = newFlow("Defaults")
	tr = b.node("start", "test.trigger", nil)
	d := b.node("d", "test.defaults", nil)
	b.edge(tr, PortOut, d)
	if res, _ := runWith(context.Background(), eng, b.build(), RunRequest{TriggerNode: tr}); res.Outputs["d"]["mode"] != "x" {
		t.Fatalf("defaults = %#v", res.Outputs["d"])
	}
}

// mustStep returns the step of nodeID and fails the test when there is none.
func mustStep(t *testing.T, res RunResult, nodeID string) *StepRecord {
	t.Helper()
	s := stepOf(res, nodeID)
	if s == nil {
		t.Fatalf("no step for node %s; steps: %v", nodeID, stepIDs(res))
	}
	return s
}

// stepIDs returns the node ids of the run's steps in record order.
func stepIDs(res RunResult) []string {
	ids := make([]string, len(res.Steps))
	for i, s := range res.Steps {
		ids[i] = s.NodeID
	}
	return ids
}

func TestEngineRejectsLoops(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("Loop")
	tr := b.node("start", "test.trigger", nil)
	a := b.node("a", "test.echo", nil)
	bb := b.node("b", "test.echo", nil)
	b.edge(tr, PortOut, a)
	b.edge(a, PortOut, bb)
	b.edge(bb, PortOut, a)
	res, capture := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr})
	if res.Status != RunError || res.ErrorCode != "FLOW_CYCLE" || res.ErrorMessage != "the flow contains a loop" || len(res.Steps) != 0 {
		t.Fatalf("loop = %s %s %q, %d steps", res.Status, res.ErrorCode, res.ErrorMessage, len(res.Steps))
	}
	if !reflect.DeepEqual(capture.types(), []string{EventRunStarted, EventRunFinished}) {
		t.Fatalf("events = %v", capture.types())
	}
}

// TestEngineSchedulesInTopologicalOrder pins the scheduling order: skips
// propagate in one pass over the topological order, and ready nodes start in
// that order (document order breaks ties), whatever order the document lists
// the nodes in.
func TestEngineSchedulesInTopologicalOrder(t *testing.T) {
	reg := newTestRegistry(t)
	eng := newTestEngine(reg, nil, 1)
	b := newFlow("Topo")
	tr := b.node("start", "test.trigger", nil)
	m := b.node("m", "test.echo", nil)
	s1 := b.node("s1", "test.echo", nil)
	s2 := b.node("s2", "test.echo", nil)
	n := b.node("n", "test.echo", nil)
	b.edge(tr, PortOut, m)
	b.edge(tr, PortOut, s1)
	b.edge(s1, PortOut, s2)
	b.edge(s2, PortOut, m)
	b.edge(tr, PortOut, n)
	f := b.build()
	f.NodeByID(s1).Settings.Disabled = true
	res, _ := runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess {
		t.Fatalf("status = %s %s", res.Status, res.ErrorMessage)
	}
	if got, want := stepIDs(res), []string{tr, s1, s2, m, n}; !reflect.DeepEqual(got, want) {
		t.Fatalf("step order = %v, want %v", got, want)
	}
	if mustStep(t, res, s2).Status != StepSkipped || mustStep(t, res, m).Status != StepSuccess {
		t.Fatalf("s2 = %s, m = %s", mustStep(t, res, s2).Status, mustStep(t, res, m).Status)
	}

	// A chain listed in reverse document order is skipped in chain order.
	b = newFlow("Reverse")
	tr = b.node("start", "test.trigger", nil)
	chain := make([]string, MaxNodes-1)
	for i := range chain {
		chain[i] = b.node(fmt.Sprintf("c%d", i), "test.echo", nil)
	}
	b.edge(tr, PortOut, chain[len(chain)-1])
	for i := len(chain) - 1; i > 0; i-- {
		b.edge(chain[i], PortOut, chain[i-1])
	}
	f = b.build()
	f.NodeByID(chain[len(chain)-1]).Settings.Disabled = true
	res, _ = runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess || len(res.Steps) != MaxNodes {
		t.Fatalf("reverse chain = %s, %d steps", res.Status, len(res.Steps))
	}
	for k, s := range res.Steps[1:] {
		if want := chain[len(chain)-1-k]; s.NodeID != want || s.Status != StepSkipped {
			t.Fatalf("step %d = %s %s, want %s skipped", k+1, s.NodeID, s.Status, want)
		}
	}
}

// TestEngineClosesUnstartedNodes checks that every node gets a final step when a
// run ends early: nodes that never started are cancelled after a failure or a
// cancel and skipped after a stop node.
func TestEngineClosesUnstartedNodes(t *testing.T) {
	cases := []struct {
		name      string
		firstType string
		cancel    bool
		wantRun   RunStatus
		wantFirst StepStatus
		wantRest  StepStatus
	}{
		{"failure", "test.fail", false, RunError, StepError, StepCancelled},
		{"stop", TypeStop, false, RunSuccess, StepSuccess, StepSkipped},
		{"cancel", "test.hang", true, RunCancelled, StepCancelled, StepCancelled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := newTestRegistry(t)
			started := make(chan struct{})
			registerHang(reg, 10*time.Second, started)
			b, tr, first := singleNodeFlow("Close", "first", tc.firstType, nil)
			x := b.node("x", "test.echo", nil)
			y := b.node("y", "test.echo", nil)
			b.edge(tr, PortOut, x)
			b.edge(x, PortOut, y)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				go func() {
					<-started
					cancel()
				}()
			}
			// One worker: x is ready but must wait behind the first node.
			res, capture := runWith(ctx, newTestEngine(reg, nil, 1), b.build(), RunRequest{TriggerNode: tr})
			if res.Status != tc.wantRun {
				t.Fatalf("run = %s %s", res.Status, res.ErrorCode)
			}
			if got, want := stepIDs(res), []string{tr, first, x, y}; !reflect.DeepEqual(got, want) {
				t.Fatalf("steps = %v, want %v (one per node)", got, want)
			}
			if s := mustStep(t, res, first); s.Status != tc.wantFirst {
				t.Fatalf("first = %s, want %s", s.Status, tc.wantFirst)
			}
			for _, id := range []string{x, y} {
				if s := mustStep(t, res, id); s.Status != tc.wantRest {
					t.Fatalf("node %s = %s, want %s", id, s.Status, tc.wantRest)
				}
			}
			evs := capture.events
			tail := evs[len(evs)-3:]
			if tail[0].Type != EventStepFinished || tail[0].NodeID != x || tail[1].Type != EventStepFinished || tail[1].NodeID != y || tail[2].Type != EventRunFinished {
				t.Fatalf("events = %v", capture.types())
			}
		})
	}
}

// registerLateStop registers test.latestop: it returns a stop (status success)
// only after the run's context ended. started, when not nil, is closed on entry.
func registerLateStop(reg *Registry, started chan<- struct{}) {
	reg.MustRegister(&NodeDef{Type: "test.latestop", Execute: func(ctx context.Context, _ ExecInput) (ExecResult, error) {
		if started != nil {
			close(started)
		}
		<-ctx.Done()
		return ExecResult{Stop: &StopSignal{Status: RunSuccess}}, nil
	}})
}

// TestEngineStopDoesNotOverrideFailureOrCancel checks that the first terminal
// outcome wins: a stop that arrives after a failure or a cancel is ignored.
func TestEngineStopDoesNotOverrideFailureOrCancel(t *testing.T) {
	t.Run("failure", func(t *testing.T) {
		reg := newTestRegistry(t)
		registerLateStop(reg, nil)
		b, tr, fail := singleNodeFlow("StopAfterFailure", "fail", "test.fail", nil)
		halt := b.node("halt", "test.latestop", nil)
		b.edge(tr, PortOut, halt)
		res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr})
		if res.Status != RunError || res.ErrorCode != "TEST_FAILED" || res.ErrorNodeID != fail {
			t.Fatalf("result = %s %s %s", res.Status, res.ErrorCode, res.ErrorNodeID)
		}
		if s := mustStep(t, res, halt); s.Status != StepSuccess {
			t.Fatalf("halt = %s", s.Status)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		reg := newTestRegistry(t)
		started := make(chan struct{})
		registerLateStop(reg, started)
		b, tr, halt := singleNodeFlow("StopAfterCancel", "halt", "test.latestop", nil)
		after := b.node("after", "test.echo", nil)
		b.edge(halt, PortOut, after)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			<-started
			cancel()
		}()
		res, _ := runWith(ctx, newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr})
		if res.Status != RunCancelled || res.ErrorCode != "FLOW_CANCELLED" {
			t.Fatalf("result = %s %s", res.Status, res.ErrorCode)
		}
		if s := mustStep(t, res, after); s.Status != StepCancelled {
			t.Fatalf("after = %s, want cancelled", s.Status)
		}
	})
}

// TestEngineFinishedRunIsNotInterrupted checks that a cancel which interrupts
// nothing does not change the outcome of a run whose nodes all finished.
func TestEngineFinishedRunIsNotInterrupted(t *testing.T) {
	reg := newTestRegistry(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reg.MustRegister(&NodeDef{Type: "test.cancelparent", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		cancel()
		return ExecResult{Output: map[string]any{"ok": true}}, nil
	}})
	b, tr, n := singleNodeFlow("Late", "late", "test.cancelparent", nil)
	res, _ := runWith(ctx, newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr})
	if s := mustStep(t, res, n); res.Status != RunSuccess || res.ErrorCode != "" || s.Status != StepSuccess {
		t.Fatalf("result = %s %s, node %s", res.Status, res.ErrorCode, s.Status)
	}
}

func TestEngineOnlyNodeNotFound(t *testing.T) {
	reg := newTestRegistry(t)
	b, tr, _ := singleNodeFlow("Missing", "a", "test.echo", nil)
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr, OnlyNode: "n_zzzzzzzz"})
	if res.Status != RunError || res.ErrorCode != "FLOW_NODE_NOT_FOUND" || len(res.Steps) != 0 {
		t.Fatalf("result = %s %s, %d steps", res.Status, res.ErrorCode, len(res.Steps))
	}
}

func TestEngineEventSeqOnBranchingRun(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("Branch")
	tr := b.node("start", "test.trigger", nil)
	check := b.node("check", TypeIf, map[string]any{"condition": cond("{{trigger.data.n}}", "gt", 3.0)})
	yes := b.node("yes", "test.echo", map[string]any{"value": "big"})
	no := b.node("no", "test.echo", map[string]any{"value": "small"})
	other := b.node("other", "test.echo", map[string]any{"value": "parallel"})
	join := b.node("join", TypeMerge, nil)
	b.edge(tr, PortOut, check)
	b.edge(tr, PortOut, other)
	b.edge(check, PortTrue, yes)
	b.edge(check, PortFalse, no)
	b.edge(yes, PortOut, join)
	b.edge(no, PortOut, join)
	b.edge(other, PortOut, join)
	res, capture := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(),
		RunRequest{RunID: "run_branch", TriggerNode: tr, TriggerData: map[string]any{"n": 5.0}})
	if res.Status != RunSuccess || len(res.Steps) != 6 {
		t.Fatalf("result = %s, %d steps", res.Status, len(res.Steps))
	}
	evs := capture.events
	if evs[0].Type != EventRunStarted || evs[len(evs)-1].Type != EventRunFinished {
		t.Fatalf("events = %v", capture.types())
	}
	started, finished := map[string]int{}, map[string]int{}
	for i, ev := range evs {
		if ev.Seq != i+1 || ev.RunID != "run_branch" {
			t.Fatalf("event %d (%s) = seq %d run %q", i, ev.Type, ev.Seq, ev.RunID)
		}
		switch ev.Type {
		case EventStepStarted:
			started[ev.NodeID]++
		case EventStepFinished:
			if ev.Step == nil || ev.Step.NodeID != ev.NodeID || (ev.Step.Status != StepSkipped && ev.NodeID != tr && started[ev.NodeID] != 1) {
				t.Fatalf("event %d: step_finished for %s without a matching start", i, ev.NodeID)
			}
			finished[ev.NodeID]++
		}
	}
	for _, id := range []string{tr, check, yes, no, other, join} {
		if finished[id] != 1 {
			t.Fatalf("node %s finished %d times", id, finished[id])
		}
	}
	if len(started) != 4 || started[no] != 0 {
		t.Fatalf("started = %v", started)
	}
}
