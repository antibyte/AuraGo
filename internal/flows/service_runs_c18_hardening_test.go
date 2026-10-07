package flows

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// c18StubbornFlow registers a node that ignores its context on fx's registry and returns a
// queue flow trigger → stubborn. The node signals started and returns once release closes,
// so a cancelled run stays active (in the runner's cancels) until the test lets it go.
func c18StubbornFlow(t *testing.T, fx *runnerFixture, id string, started chan struct{}, release chan struct{}) *Flow {
	t.Helper()
	if _, ok := fx.r.engine.reg.Lookup("test.c18stubborn"); !ok {
		fx.r.engine.reg.MustRegister(&NodeDef{Type: "test.c18stubborn", DefaultTimeout: 10 * time.Second,
			Execute: func(ctx context.Context, in ExecInput) (ExecResult, error) {
				started <- struct{}{}
				<-release
				return ExecResult{Output: map[string]any{}}, nil
			}})
	}
	b := newFlow("Stubborn " + id)
	tr := b.node("start", "test.trigger", nil)
	n := b.node("stubborn", "test.c18stubborn", nil)
	b.edge(tr, PortOut, n)
	f := b.build()
	f.ID = id
	f.Settings.Concurrency = ConcurrencyQueue
	if _, err := fx.store.CreateFlow(context.Background(), f, "", storeNow); err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	return f
}

func TestC18RunnerCancelRunReportsTheFirstCancel(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{})
	started, release := make(chan struct{}, 4), make(chan struct{})
	f := c18StubbornFlow(t, fx, "flow_aaaaaaaac8", started, release)
	running, err := fx.r.Start(StartRequest{Flow: f, Mode: ModeLive, TriggerNode: f.Nodes[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("the stubborn node did not start")
	}
	queued, err := fx.r.Start(StartRequest{Flow: f, Mode: ModeLive, TriggerNode: f.Nodes[0].ID})
	if err != nil || queued.Status != StartQueued {
		t.Fatalf("second start = %+v %v", queued, err)
	}

	if known, first := fx.r.CancelRun(running.RunID); !known || !first {
		t.Fatalf("first CancelRun(running) = %v %v", known, first)
	}
	// The node ignores its context, so the run is still active: known, but not first.
	if known, first := fx.r.CancelRun(running.RunID); !known || first {
		t.Fatalf("second CancelRun(running) = %v %v", known, first)
	}
	if !fx.r.Cancel(running.RunID) {
		t.Fatal("Cancel of a run that winds down must stay true")
	}
	if known, first := fx.r.CancelRun(queued.RunID); !known || !first {
		t.Fatalf("CancelRun(queued) = %v %v", known, first)
	}
	if rec := fx.waitFinished(t); rec.ID != queued.RunID || rec.Status != RunCancelled {
		t.Fatalf("cancelled queued = %+v", rec)
	}
	if known, first := fx.r.CancelRun(queued.RunID); known || first {
		t.Fatalf("CancelRun of a removed queued run = %v %v", known, first)
	}
	close(release)
	if rec := fx.waitFinished(t); rec.ID != running.RunID {
		t.Fatalf("finished = %+v", rec)
	}
	if known, first := fx.r.CancelRun(running.RunID); known || first {
		t.Fatalf("CancelRun of a finished run = %v %v", known, first)
	}
	if known, first := fx.r.CancelRun("run_unknown"); known || first {
		t.Fatalf("CancelRun(unknown) = %v %v", known, first)
	}
}

func TestC18RunHeaderSkipsTriggerDataAndSteps(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: simpleFlow("Kopf")})
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.StartTestRun(ctx, rec.ID, TestRunRequest{TriggerData: map[string]any{"name": "Andi"}})
	if err != nil {
		t.Fatal(err)
	}
	waitRun(t, s, res.RunID)
	head, err := s.RunHeader(ctx, res.RunID)
	if err != nil || head.ID != res.RunID || head.FlowID != rec.ID || head.Mode != ModeTest || head.Status != RunSuccess ||
		len(head.TriggerData) != 0 || head.FinishedAt == nil {
		t.Fatalf("RunHeader = %+v, %v", head, err)
	}
	if detail, err := s.Run(ctx, res.RunID, false); err != nil || detail.Run.TriggerData["name"] != "Andi" {
		t.Fatalf("Run still reads the trigger data: %+v %v", detail, err)
	}
	if _, err := s.RunHeader(ctx, "run_unknown"); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("RunHeader(unknown) = %v", err)
	}
}

func TestC18TriggerSamplesNeedAnEnabledTrigger(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	doc := simpleFlow("Probe")
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: doc})
	if err != nil {
		t.Fatal(err)
	}
	start, greet := doc.Nodes[0].ID, doc.Nodes[1].ID
	long := strings.Repeat("n", 4000)
	for _, node := range []string{greet, "n_zzzzzzzz", "", long} {
		if err := s.SaveTriggerSample(ctx, rec.ID, node, map[string]any{"x": 1}); !errors.Is(err, ErrNoTrigger) {
			t.Fatalf("SaveTriggerSample(%.20q) = %v", node, err)
		} else if len(err.Error()) > 200 {
			t.Fatalf("the error echoes the node id unbounded: %d bytes", len(err.Error()))
		}
		if _, err := s.TriggerSampleData(ctx, rec.ID, node); !errors.Is(err, ErrNoTrigger) {
			t.Fatalf("TriggerSampleData(%.20q) = %v", node, err)
		}
	}
	var rows int
	if err := s.Store().db.QueryRowContext(ctx, `SELECT COUNT(*) FROM flow_test_data`).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("refused samples stored %d rows (%v)", rows, err)
	}
	if err := s.SaveTriggerSample(ctx, rec.ID, start, map[string]any{"name": "Andi"}); err != nil {
		t.Fatalf("SaveTriggerSample(trigger) = %v", err)
	}
	if data, err := s.TriggerSampleData(ctx, rec.ID, start); err != nil || data["name"] != "Andi" {
		t.Fatalf("TriggerSampleData(trigger) = %v %v", data, err)
	}
	if _, err := s.TriggerSampleData(ctx, "flow_missing00", start); !errors.Is(err, ErrNotFound) {
		t.Fatalf("TriggerSampleData of an unknown flow = %v", err)
	}

	// A disabled trigger has no sample data any more; its remembered data stays stored.
	disabled, err := doc.Clone()
	if err != nil {
		t.Fatal(err)
	}
	disabled.Nodes[0].Settings.Disabled = true
	if _, _, err := s.SaveDraft(ctx, rec.ID, disabled, rec.DraftRevision); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTriggerSample(ctx, rec.ID, start, map[string]any{"name": "Bea"}); !errors.Is(err, ErrNoTrigger) {
		t.Fatalf("SaveTriggerSample(disabled trigger) = %v", err)
	}
	if _, err := s.TriggerSampleData(ctx, rec.ID, start); !errors.Is(err, ErrNoTrigger) {
		t.Fatalf("TriggerSampleData(disabled trigger) = %v", err)
	}
}

func TestC18RunModeAndStatusValid(t *testing.T) {
	for _, m := range []RunMode{ModeTest, ModeLive, ModeAgent, ModeCall} {
		if !m.Valid() {
			t.Errorf("%q is not valid", m)
		}
	}
	for _, m := range []RunMode{"", "TEST", "bogus"} {
		if m.Valid() {
			t.Errorf("%q is valid", m)
		}
	}
	for _, st := range []RunStatus{RunQueued, RunRunning, RunWaiting, RunSuccess, RunError, RunCancelled} {
		if !st.Valid() {
			t.Errorf("%q is not valid", st)
		}
	}
	for _, st := range []RunStatus{"", "done", "Success"} {
		if st.Valid() {
			t.Errorf("%q is valid", st)
		}
	}
}
