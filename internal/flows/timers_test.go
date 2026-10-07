package flows

import (
	"context"
	"testing"
	"time"
)

type firedTimer struct {
	flowID, nodeID string
	at             time.Time
}

type timerFixture struct {
	store  *Store
	clock  *fakeClock
	svc    *TimerService
	fired  chan firedTimer
	missed chan firedTimer
	flow   *Flow
}

func newTimerFixture(t *testing.T) *timerFixture {
	t.Helper()
	fx := &timerFixture{
		store:  openTestStore(t),
		clock:  newFakeClock(storeNow),
		fired:  make(chan firedTimer, 8),
		missed: make(chan firedTimer, 8),
	}
	fx.flow = createStoredFlow(t, fx.store, "flow_aaaaaaaada")
	fx.svc = NewTimerService(fx.store, fx.clock,
		func(flowID, nodeID string, at time.Time) { fx.fired <- firedTimer{flowID, nodeID, at} },
		func(flowID, nodeID string, at time.Time) { fx.missed <- firedTimer{flowID, nodeID, at} },
		discardLogger())
	t.Cleanup(fx.svc.Stop)
	return fx
}

func (fx *timerFixture) expect(t *testing.T, ch chan firedTimer, nodeID string) firedTimer {
	t.Helper()
	select {
	case got := <-ch:
		if got.nodeID != nodeID || got.flowID != fx.flow.ID {
			t.Fatalf("got timer %+v, want node %s", got, nodeID)
		}
		return got
	case <-time.After(3 * time.Second):
		t.Fatalf("timer for %s did not fire", nodeID)
	}
	return firedTimer{}
}

func TestTimerServiceFiresDueTimer(t *testing.T) {
	fx := newTimerFixture(t)
	ctx := context.Background()
	if err := fx.store.ReplaceTimers(ctx, fx.flow.ID, []TimerRecord{{FlowID: fx.flow.ID, NodeID: "n_aaaaaaaa", FireAt: storeNow.Add(time.Hour)}}); err != nil {
		t.Fatal(err)
	}
	if err := fx.svc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	fx.clock.WaitForWaiters(t, 1)
	fx.clock.Advance(time.Hour)
	got := fx.expect(t, fx.fired, "n_aaaaaaaa")
	if !got.at.Equal(storeNow.Add(time.Hour)) {
		t.Fatalf("scheduled time = %v", got.at)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		list, _ := fx.store.ListTimers(ctx)
		if len(list) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("a one-off timer must be deleted after firing, still %+v", list)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestTimerServiceYearlyRepeat(t *testing.T) {
	fx := newTimerFixture(t)
	ctx := context.Background()
	at := storeNow.Add(time.Minute)
	if err := fx.store.ReplaceTimers(ctx, fx.flow.ID, []TimerRecord{{FlowID: fx.flow.ID, NodeID: "n_aaaaaaab", FireAt: at, Repeat: RepeatYearly}}); err != nil {
		t.Fatal(err)
	}
	if err := fx.svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	fx.clock.WaitForWaiters(t, 1)
	fx.clock.Advance(time.Minute)
	fx.expect(t, fx.fired, "n_aaaaaaab")
	deadline := time.Now().Add(2 * time.Second)
	for {
		list, _ := fx.store.ListTimers(ctx)
		if len(list) == 1 && list[0].FireAt.Equal(at.AddDate(1, 0, 0)) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("yearly timer not moved to next year: %+v", list)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestTimerServiceStartupMissedAndLate(t *testing.T) {
	fx := newTimerFixture(t)
	ctx := context.Background()
	if err := fx.store.ReplaceTimers(ctx, fx.flow.ID, []TimerRecord{
		{FlowID: fx.flow.ID, NodeID: "n_aaaaaaac", FireAt: storeNow.Add(-time.Hour)},
		{FlowID: fx.flow.ID, NodeID: "n_aaaaaaad", FireAt: storeNow.Add(-5 * time.Minute)},
	}); err != nil {
		t.Fatal(err)
	}
	if err := fx.svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	fx.expect(t, fx.missed, "n_aaaaaaac")
	fx.expect(t, fx.fired, "n_aaaaaaad")
	if list, _ := fx.store.ListTimers(ctx); len(list) != 0 {
		t.Fatalf("processed timers must be removed, got %+v", list)
	}
}

func TestTimerServiceReplaceRearms(t *testing.T) {
	fx := newTimerFixture(t)
	ctx := context.Background()
	if err := fx.svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := fx.svc.Replace(ctx, fx.flow.ID, []TimerRecord{{FlowID: fx.flow.ID, NodeID: "n_aaaaaaae", FireAt: storeNow.Add(10 * time.Minute)}}); err != nil {
		t.Fatal(err)
	}
	fx.clock.WaitForWaiters(t, 1)
	fx.clock.Advance(10 * time.Minute)
	fx.expect(t, fx.fired, "n_aaaaaaae")
}
