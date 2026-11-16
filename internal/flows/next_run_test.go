package flows

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// fakeNexter is a ScheduleNexter that answers one fixed fire and records the arguments.
type fakeNexter struct {
	at     time.Time
	node   string
	ok     bool
	flowID string
	now    time.Time
	calls  int
}

func (n *fakeNexter) NextScheduleRun(flowID string, now time.Time) (time.Time, string, bool) {
	n.calls++
	n.flowID, n.now = flowID, now
	return n.at, n.node, n.ok
}

// nextRunFixture publishes a flow with one Date/Time trigger at +24 h and switches it on.
func nextRunFixture(t *testing.T) (*svcFixture, *FlowRecord, time.Time) {
	t.Helper()
	fx := newSvcFixture(t)
	at := fx.clock.Now().Add(24 * time.Hour)
	rec := fx.published(t, "Zeit", svcDateTimeFlow("Zeit", 3, at.Format(time.RFC3339)))
	if err := fx.s.SetEnabled(context.Background(), rec.ID, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	return fx, rec, at
}

func TestNextRunDateTimeTrigger(t *testing.T) {
	fx, rec, at := nextRunFixture(t)
	got, ok := fx.s.NextRun(rec.ID)
	if !ok || !got.At.Equal(at) || got.NodeID != testNodeID(3) {
		t.Fatalf("NextRun = %+v %v, want %v at %s", got, ok, at, testNodeID(3))
	}
	byMission, ok := fx.s.NextRunForMission(rec.MissionID)
	if !ok || !byMission.At.Equal(got.At) || byMission.NodeID != got.NodeID {
		t.Fatalf("NextRunForMission = %+v %v, want %+v", byMission, ok, got)
	}
}

func TestNextRunScheduleThroughTheNexter(t *testing.T) {
	fx := newSvcFixture(t)
	b := newFlow("Takt")
	manual := b.node("start", TypeTriggerManual, nil)
	set := b.node("greet", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "x", "value": "1"}}})
	b.edge(manual, PortOut, set)
	rec := fx.published(t, "Takt", b.build())
	if err := fx.s.SetEnabled(context.Background(), rec.ID, true); err != nil {
		t.Fatal(err)
	}
	if got, ok := fx.s.NextRun(rec.ID); ok {
		t.Fatalf("NextRun without a time-based trigger = %+v", got)
	}
	fire := fx.clock.Now().Add(3 * time.Hour)
	nexter := &fakeNexter{at: fire, node: testNodeID(5), ok: true}
	fx.s.SetScheduleNexter(nexter)
	got, ok := fx.s.NextRun(rec.ID)
	if !ok || !got.At.Equal(fire) || got.NodeID != testNodeID(5) {
		t.Fatalf("NextRun = %+v %v, want %v at %s", got, ok, fire, testNodeID(5))
	}
	if nexter.flowID != rec.ID || !nexter.now.Equal(fx.clock.Now()) {
		t.Fatalf("the nexter was asked for %q at %v, want %q at %v", nexter.flowID, nexter.now, rec.ID, fx.clock.Now())
	}
	fx.s.SetScheduleNexter(nil)
	if got, ok := fx.s.NextRun(rec.ID); ok {
		t.Fatalf("NextRun after removing the nexter = %+v", got)
	}
}

func TestNextRunTheEarliestOfBothSourcesWins(t *testing.T) {
	fx, rec, at := nextRunFixture(t)
	earlier := fx.clock.Now().Add(2 * time.Hour)
	fx.s.SetScheduleNexter(&fakeNexter{at: earlier, node: testNodeID(5), ok: true})
	if got, ok := fx.s.NextRun(rec.ID); !ok || !got.At.Equal(earlier) || got.NodeID != testNodeID(5) {
		t.Fatalf("NextRun = %+v %v, want the schedule at %v", got, ok, earlier)
	}
	fx.s.SetScheduleNexter(&fakeNexter{at: at.Add(time.Hour), node: testNodeID(5), ok: true})
	if got, ok := fx.s.NextRun(rec.ID); !ok || !got.At.Equal(at) || got.NodeID != testNodeID(3) {
		t.Fatalf("NextRun = %+v %v, want the timer at %v", got, ok, at)
	}
}

func TestNextRunPausedFlowHasNone(t *testing.T) {
	fx, rec, _ := nextRunFixture(t)
	fx.s.SetScheduleNexter(&fakeNexter{at: fx.clock.Now().Add(time.Hour), node: testNodeID(5), ok: true})
	if err := fx.s.SetEnabled(context.Background(), rec.ID, false); err != nil {
		t.Fatal(err)
	}
	if got, ok := fx.s.NextRun(rec.ID); ok {
		t.Fatalf("NextRun of a paused flow = %+v", got)
	}
}

func TestNextRunUnpublishedFlowHasNone(t *testing.T) {
	fx := newSvcFixture(t)
	rec, err := fx.s.CreateFlow(context.Background(), CreateRequest{Name: "Entwurf"})
	if err != nil {
		t.Fatal(err)
	}
	nexter := &fakeNexter{at: fx.clock.Now().Add(time.Hour), node: testNodeID(5), ok: true}
	fx.s.SetScheduleNexter(nexter)
	if got, ok := fx.s.NextRun(rec.ID); ok {
		t.Fatalf("NextRun of an unpublished flow = %+v", got)
	}
	if _, ok := fx.s.NextRun("flow_unknown"); ok {
		t.Fatal("NextRun of an unknown flow answered")
	}
}

func TestNextRunSkipsAStoredFireInThePast(t *testing.T) {
	fx, rec, at := nextRunFixture(t)
	ctx := context.Background()
	past := fx.clock.Now().Add(-time.Hour)
	if err := fx.s.Store().ReplaceTimers(ctx, rec.ID, []TimerRecord{
		{NodeID: testNodeID(3), FireAt: past},
		{NodeID: testNodeID(4), FireAt: at},
	}); err != nil {
		t.Fatal(err)
	}
	if raw, ok, err := fx.s.Store().NextTimerAt(ctx, rec.ID); err != nil || !ok || !raw.Equal(past) {
		t.Fatalf("setup: the raw minimum is %v %v %v, want the past fire %v", raw, ok, err, past)
	}
	if got, ok := fx.s.NextRun(rec.ID); !ok || !got.At.Equal(at) || got.NodeID != testNodeID(4) {
		t.Fatalf("NextRun = %+v %v, want the later timer %v at %s", got, ok, at, testNodeID(4))
	}
	if err := fx.s.Store().ReplaceTimers(ctx, rec.ID, []TimerRecord{{NodeID: testNodeID(3), FireAt: past}}); err != nil {
		t.Fatal(err)
	}
	if got, ok := fx.s.NextRun(rec.ID); ok {
		t.Fatalf("NextRun with only a past fire = %+v", got)
	}
	fx.s.SetScheduleNexter(&fakeNexter{at: past, node: testNodeID(5), ok: true})
	if got, ok := fx.s.NextRun(rec.ID); ok {
		t.Fatalf("NextRun with a past schedule fire = %+v", got)
	}
}

func TestNextRunTakesNoFlowLock(t *testing.T) {
	fx, rec, at := nextRunFixture(t)
	unlock, err := fx.s.locks.lock(context.Background(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	err = svcWithin(t, 5*time.Second, "NextRun of a locked flow", func() error {
		if got, ok := fx.s.NextRun(rec.ID); !ok || !got.At.Equal(at) {
			return fmt.Errorf("NextRun = %+v %v, want %v", got, ok, at)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
