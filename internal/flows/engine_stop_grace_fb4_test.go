package flows

import (
	"context"
	"testing"
	"time"
)

// fb4StopFlow builds trigger → halt (logic.stop) and trigger → post (test.effect) as parallel
// branches. test.effect has an outward effect, reports its start and returns what result says
// (nil: success) or ends with its context.
func fb4StopFlow(t *testing.T) (*Engine, *fakeClock, *Flow, string, chan struct{}, chan error) {
	t.Helper()
	reg := newTestRegistry(t)
	started := make(chan struct{}, 1)
	result := make(chan error, 1)
	reg.MustRegister(&NodeDef{Type: "test.effect", Effects: []Effect{EffectSendsMessage}, DefaultTimeout: 10 * time.Minute,
		Execute: func(ctx context.Context, _ ExecInput) (ExecResult, error) {
			started <- struct{}{}
			select {
			case err := <-result:
				if err != nil {
					return ExecResult{}, err
				}
				return ExecResult{Output: map[string]any{"sent": true}}, nil
			case <-ctx.Done():
				return ExecResult{}, ctx.Err()
			}
		}})
	clock := newFakeClock(storeNow)
	eng := newTestEngine(reg, &Services{Clock: clock, Location: time.UTC}, 4)
	b := newFlow("StopGrace")
	tr := b.node("start", "test.trigger", nil)
	halt := b.node("halt", TypeStop, nil)
	post := b.node("post", "test.effect", nil)
	b.edge(tr, PortOut, halt)
	b.edge(tr, PortOut, post)
	return eng, clock, b.build(), post, started, result
}

// fb4RunStop starts the run and returns once the post step runs and the stop's grace is armed.
func fb4RunStop(t *testing.T, eng *Engine, clock *fakeClock, f *Flow, started chan struct{}) <-chan RunResult {
	t.Helper()
	out := make(chan RunResult, 1)
	go func() {
		res, _ := runWith(context.Background(), eng, f, RunRequest{TriggerNode: f.Nodes[0].ID})
		out <- res
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("the post step did not start")
	}
	clock.WaitForWaiters(t, 1) // the stop's grace
	return out
}

func fb4Result(t *testing.T, out <-chan RunResult) RunResult {
	t.Helper()
	select {
	case res := <-out:
		return res
	case <-time.After(3 * time.Second):
		t.Fatal("the run did not end")
	}
	return RunResult{}
}

func TestFB4StopLetsRunningStepsFinish(t *testing.T) {
	eng, clock, f, post, started, result := fb4StopFlow(t)
	out := fb4RunStop(t, eng, clock, f, started)
	result <- nil
	res := fb4Result(t, out)
	if s := stepOf(res, post); res.Status != RunSuccess || s == nil || s.Status != StepSuccess || s.EffectUncertain {
		t.Fatalf("run %s %s, post %+v", res.Status, res.ErrorCode, s)
	}
}

func TestFB4StopCutsStepsOffAfterTheGraceAndMarksTheirEffectUncertain(t *testing.T) {
	eng, clock, f, post, started, _ := fb4StopFlow(t)
	out := fb4RunStop(t, eng, clock, f, started)
	clock.Advance(defaultAbandonAfter)
	res := fb4Result(t, out)
	if s := stepOf(res, post); res.Status != RunSuccess || s == nil || s.Status != StepCancelled || !s.EffectUncertain {
		t.Fatalf("run %s %s, post %+v", res.Status, res.ErrorCode, s)
	}
}

func TestFB4LateFailureAfterASuccessStopFailsTheRun(t *testing.T) {
	eng, clock, f, post, started, result := fb4StopFlow(t)
	out := fb4RunStop(t, eng, clock, f, started)
	result <- NewNodeError("TEST_FAILED", "the server refused it")
	res := fb4Result(t, out)
	if res.Status != RunError || res.ErrorCode != "TEST_FAILED" || res.ErrorNodeID != post {
		t.Fatalf("run = %s %s %s", res.Status, res.ErrorCode, res.ErrorNodeID)
	}
}

func TestFB4ErrorStopKeepsItsOutcomeButRecordsALateFailure(t *testing.T) {
	eng, clock, f, post, started, result := fb4StopFlow(t)
	f.Nodes[1].Params = map[string]any{"status": "error", "message": "angehalten"}
	out := fb4RunStop(t, eng, clock, f, started)
	result <- NewNodeError("TEST_FAILED", "the server refused it")
	res := fb4Result(t, out)
	s := stepOf(res, post)
	if res.Status != RunError || res.ErrorCode != "FLOW_STOPPED" || res.ErrorMessage != "angehalten" ||
		s == nil || s.Status != StepError || s.ErrorCode != "TEST_FAILED" {
		t.Fatalf("run = %s %s %q, post %+v", res.Status, res.ErrorCode, res.ErrorMessage, s)
	}
}

func TestFB4EffectUncertainIsStored(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaafe")
	addRunAt(t, s, f, "run_aaaaaaaafea", ModeLive, RunRunning, storeNow)
	if err := s.SaveStep(ctx, "run_aaaaaaaafea", 1, StepRecord{NodeID: f.Nodes[1].ID, Status: StepCancelled, EffectUncertain: true}); err != nil {
		t.Fatal(err)
	}
	if _, steps, err := s.GetRun(ctx, "run_aaaaaaaafea"); err != nil || len(steps) != 1 || !steps[0].EffectUncertain {
		t.Fatalf("GetRun = %+v, %v", steps, err)
	}
}
