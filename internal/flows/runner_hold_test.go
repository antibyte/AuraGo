package flows

import (
	"errors"
	"testing"
	"time"
)

func TestHoldDispatchRefusesWhileARunRunsAndQueuesNewRunsUntilReleased(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{})
	a := fx.flow(t, "flow_holdaaaaaa", ConcurrencyQueue)
	b := fx.flow(t, "flow_holdbbbbbb", ConcurrencyQueue)
	fx.start(t, a, ModeLive, "a")
	fx.gate.waitStarted(t, "a")
	if _, err := fx.r.HoldDispatch(); !errors.Is(err, ErrRunsActive) {
		t.Fatalf("HoldDispatch while a run runs = %v, want ErrRunsActive", err)
	}
	fx.gate.release("a")
	fx.waitFinished(t)
	var release func()
	for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		var err error
		if release, err = fx.r.HoldDispatch(); err == nil {
			break
		}
		if !errors.Is(err, ErrRunsActive) || time.Now().After(deadline) {
			t.Fatalf("HoldDispatch after the run ended = %v", err)
		}
	}
	if _, err := fx.r.HoldDispatch(); !errors.Is(err, ErrDispatchHeld) {
		t.Fatalf("a second hold = %v, want ErrDispatchHeld", err)
	}
	res := fx.start(t, b, ModeLive, "b")
	if res.Status != StartQueued {
		t.Fatalf("a run started during the hold has status %s, want queued", res.Status)
	}
	fx.gate.assertNotStarted(t)
	release()
	release() // idempotent
	fx.gate.waitStarted(t, "b")
	fx.gate.release("b")
	if rec := fx.waitFinished(t); rec.ID != res.RunID || rec.Status != RunSuccess {
		t.Fatalf("held run finished = %+v", rec)
	}
}
