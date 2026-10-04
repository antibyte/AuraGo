package flows

import (
	"context"
	"testing"
	"time"
)

// A Replace that lands between processDue's snapshot and its settle step (a flow edited
// and republished while its timer fires) must not be undone by settling the old occurrence.

func TestTimerServiceReplaceDuringCallbackKeepsNewOneOffTimer(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	later := storeNow.Add(48 * time.Hour)
	calls := &timerCalls{}
	var svc *TimerService
	svc = h.service(t, func(flowID, nodeID string, at time.Time) {
		calls.fire(flowID, nodeID, at)
		if err := svc.Replace(ctx, flowID, []TimerRecord{{NodeID: nodeID, FireAt: later}}); err != nil {
			t.Errorf("Replace: %v", err)
		}
	}, nil, nil)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow.Add(-time.Minute)})
	if err := svc.processDue(ctx, false); err != nil {
		t.Fatal(err)
	}
	calls.assertCalls(t, "fire:n_aaaaaaab")
	list := h.timers(t)
	if len(list) != 1 || list[0].NodeID != "n_aaaaaaab" || !list[0].FireAt.Equal(later) || list[0].Repeat != "" {
		t.Fatalf("the timer armed during the callback must survive, got %+v", list)
	}
	if len(svc.unsettled) != 0 {
		t.Fatalf("the fired occurrence is settled, have %v", svc.unsettled)
	}
}

func TestTimerServiceReplaceDuringCallbackKeepsNewYearlyTime(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	later := storeNow.Add(48 * time.Hour)
	calls := &timerCalls{}
	var svc *TimerService
	svc = h.service(t, func(flowID, nodeID string, at time.Time) {
		calls.fire(flowID, nodeID, at)
		if err := svc.Replace(ctx, flowID, []TimerRecord{{NodeID: nodeID, FireAt: later, Repeat: RepeatYearly}}); err != nil {
			t.Errorf("Replace: %v", err)
		}
	}, nil, nil)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow.Add(-time.Minute), Repeat: RepeatYearly})
	if err := svc.processDue(ctx, false); err != nil {
		t.Fatal(err)
	}
	calls.assertCalls(t, "fire:n_aaaaaaab")
	// Moving the old occurrence a year ahead would overwrite the edited time.
	list := h.timers(t)
	if len(list) != 1 || !list[0].FireAt.Equal(later) || list[0].Repeat != RepeatYearly {
		t.Fatalf("the time armed during the callback must survive, got %+v", list)
	}
}

func TestTimerServiceUnreadableTimerRowIsDroppedNotRepeated(t *testing.T) {
	h := newHardTimers(t, storeNow)
	calls := &timerCalls{}
	svc := h.service(t, calls.fire, calls.missed, nil)
	h.arm(t,
		TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow},
		TimerRecord{NodeID: "n_aaaaaaac", FireAt: storeNow, Repeat: RepeatYearly})
	// Not reachable through the store's own writes; a damaged database is.
	h.exec(t, `UPDATE flow_timers SET fire_at = 'not a time'`)
	if err := svc.processDue(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	// Settling by the fired time can never match such a row; it must still go, or
	// the loop would fire it again and again.
	calls.assertCalls(t, "fire:n_aaaaaaab", "fire:n_aaaaaaac")
	if list := h.timers(t); len(list) != 0 {
		t.Fatalf("unreadable timers must be dropped, got %+v", list)
	}
}
