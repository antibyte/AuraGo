package flows

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServiceMissionStateUsesOnlyStoredFlowBinding(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	s, bridge := newServiceFixture(t, clock)
	ctx := context.Background()
	rec, err := s.CreateFlow(ctx, CreateRequest{Name: "State"})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.MissionState(ctx, rec.ID); err != nil || got != MissionStateDisabled {
		t.Fatalf("initial state = %q, %v", got, err)
	}
	if err := bridge.SetFlowMissionEnabled(rec.MissionID, rec.ID, true); err != nil {
		t.Fatal(err)
	}
	for name, draft := range map[string]string{
		"invalid":   "not-json",
		"oversized": strings.Repeat("x", MaxDocumentBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := s.store.db.ExecContext(ctx, `UPDATE flows SET draft_json=? WHERE id=?`, draft, rec.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := s.GetFlow(ctx, rec.ID); err == nil {
				t.Fatal("corrupt flow document unexpectedly parsed")
			}
			if got, err := s.MissionState(ctx, rec.ID); err != nil || got != MissionStateEnabled {
				t.Fatalf("mission state = %q, %v", got, err)
			}
		})
	}
	block, err := s.CreateBlock(ctx, "Block")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.db.ExecContext(ctx, `UPDATE flows SET draft_json=? WHERE id=?`, "not-json", block.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetBlock(ctx, block.ID); err == nil {
		t.Fatal("corrupt block document unexpectedly parsed")
	}
	if _, err := s.MissionState(ctx, block.ID); !errors.Is(err, ErrWrongFlowKind) {
		t.Fatalf("block mission state error = %v, want ErrWrongFlowKind", err)
	}
}

// waitFlow: manual trigger -> wait 60 s (blocks until the fake clock advances).
func waitFlow(name string) *Flow {
	b := newFlow(name)
	start := b.node("start", TypeTriggerManual, nil)
	pause := b.node("pause", TypeWait, map[string]any{"mode": "duration", "seconds": 60.0})
	b.edge(start, PortOut, pause)
	return b.build()
}

func TestServiceCancelMissionRuns(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	s, _ := newServiceFixture(t, clock)
	ctx := context.Background()
	rec, _ := s.CreateFlow(ctx, CreateRequest{Name: "Pause"})
	rev, _, err := s.SaveDraft(ctx, rec.ID, waitFlow("Pause"), rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Publish(ctx, rec.ID, rev); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEnabled(ctx, rec.ID, true); err != nil { // FF1: Run now needs the flow switched on
		t.Fatal(err)
	}
	started, err := s.RunNow(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		detail, _ := s.Run(ctx, started.RunID, false)
		if detail != nil && detail.Run.Status == RunRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the run did not start")
		}
		time.Sleep(5 * time.Millisecond)
	}
	n, err := s.CancelMissionRuns(ctx, rec.MissionID)
	if err != nil || n != 1 {
		t.Fatalf("CancelMissionRuns = %d, %v", n, err)
	}
	if run := waitRun(t, s, started.RunID); run.Status != RunCancelled {
		t.Fatalf("run = %+v", run)
	}
	if n, err := s.CancelMissionRuns(ctx, rec.MissionID); err != nil || n != 0 {
		t.Fatalf("second cancel = %d, %v", n, err)
	}
	if _, err := s.CancelMissionRuns(ctx, "mission_missing"); err == nil {
		t.Fatal("unknown missions must fail")
	}
}

func TestServiceMissionEnabledChangedAndNextTimer(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	s, bridge := newServiceFixture(t, clock)
	ctx := context.Background()
	rec, _ := s.CreateFlow(ctx, CreateRequest{Name: "Plan"})
	doc, _, _, _ := scheduleFlow("Plan")
	rev, _, err := s.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	pub, _, err := s.Publish(ctx, rec.ID, rev)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.NextTimer(ctx, pub.MissionID); ok {
		t.Fatal("a disabled flow has no armed timer")
	}
	// Mission Control switches the mission on directly.
	if err := bridge.SetFlowMissionEnabled(pub.MissionID, pub.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.MissionEnabledChanged(ctx, pub.MissionID); err != nil {
		t.Fatal(err)
	}
	next, ok := s.NextTimer(ctx, pub.MissionID)
	if !ok || !next.Equal(time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("NextTimer = %v %v", next, ok)
	}
	if err := bridge.SetFlowMissionEnabled(pub.MissionID, pub.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := s.MissionEnabledChanged(ctx, pub.MissionID); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.NextTimer(ctx, pub.MissionID); ok {
		t.Fatal("switching the mission off must clear the timers")
	}
	if err := s.MissionEnabledChanged(ctx, "mission_missing"); err != nil {
		t.Fatalf("unknown missions are ignored: %v", err)
	}
}

func TestReconcileTimerDisableRequiresMissionSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name      string
		explicit  bool
		wantTimer bool
	}{
		{name: "ambiguous false during reconciliation", wantTimer: true},
		{name: "explicit disable notification", explicit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
			s, bridge := newServiceFixture(t, clock)
			ctx := context.Background()
			doc, _, _, _ := scheduleFlow("Reconcile timers")
			rec, err := s.CreateFlow(ctx, CreateRequest{Import: doc})
			if err != nil {
				t.Fatal(err)
			}
			rev, _, err := s.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision)
			if err != nil {
				t.Fatal(err)
			}
			pub, _, err := s.Publish(ctx, rec.ID, rev)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.SetEnabled(ctx, pub.ID, true); err != nil {
				t.Fatal(err)
			}
			if err := bridge.SetFlowMissionEnabled(pub.MissionID, pub.ID, false); err != nil {
				t.Fatal(err)
			}
			if tc.explicit {
				err = s.MissionEnabledChanged(ctx, pub.MissionID)
			} else {
				err = s.ReconcileMissions(ctx)
			}
			if err != nil {
				t.Fatalf("disable handling: %v", err)
			}
			if _, ok := s.NextTimer(ctx, pub.MissionID); ok != tc.wantTimer {
				t.Fatalf("timer present = %v, want %v", ok, tc.wantTimer)
			}
		})
	}
}
