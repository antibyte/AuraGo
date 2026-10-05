package flows

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// svcCancelBridge cancels the caller's context right after a mission call succeeded, as
// an HTTP client that disconnects mid-request does (1c passes r.Context()).
type svcCancelBridge struct {
	*svcBridge
	mu     sync.Mutex
	cancel context.CancelFunc
}

// arm returns a context that the next successful SetFlowMissionEnabled, SyncFlowMission
// or DeleteFlowMission cancels.
func (b *svcCancelBridge) arm() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	b.mu.Lock()
	b.cancel = cancel
	b.mu.Unlock()
	return ctx
}

func (b *svcCancelBridge) fire(err error) {
	if err != nil {
		return
	}
	b.mu.Lock()
	cancel := b.cancel
	b.cancel = nil
	b.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (b *svcCancelBridge) SetFlowMissionEnabled(missionID string, enabled bool) error {
	err := b.svcBridge.SetFlowMissionEnabled(missionID, enabled)
	b.fire(err)
	return err
}

func (b *svcCancelBridge) SyncFlowMission(missionID, name string, bindings []TriggerBinding) error {
	err := b.svcBridge.SyncFlowMission(missionID, name, bindings)
	b.fire(err)
	return err
}

func (b *svcCancelBridge) DeleteFlowMission(missionID string) error {
	err := b.svcBridge.DeleteFlowMission(missionID)
	b.fire(err)
	return err
}

// A caller whose context ends right after the irreversible step (the mission switch, the
// store publish and mission sync, the mission delete) still gets the whole operation:
// the timers and bindings end consistent with the store and the mission.
func TestServiceFinishesIrreversibleStepsAfterCancel(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	bridge := &svcCancelBridge{svcBridge: newSvcBridge()}
	s := svcNewService(t, &Services{Tools: &fakeTools{}, Clock: clock, Location: time.UTC}, bridge, nil)
	fx := &svcFixture{s: s, bridge: bridge.svcBridge, clock: clock}
	rec := fx.published(t, "Abbruch", svcDateTimeFlow("Abbruch", 10, "2026-10-04 09:00"))

	ctx := bridge.arm()
	if err := s.SetEnabled(ctx, rec.ID, true); err != nil {
		t.Fatalf("SetEnabled cancelled after the switch = %v", err)
	}
	if ctx.Err() == nil {
		t.Fatal("the bridge did not cancel the context")
	}
	svcCheckArmed(t, fx, rec.ID, true)

	rev, _, err := s.SaveDraft(context.Background(), rec.ID, svcDateTimeFlow("Abbruch", 11, "2026-10-05 09:00"), rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	ctx = bridge.arm()
	pub, _, err := s.Publish(ctx, rec.ID, rev)
	if err != nil || pub.LiveRevision != 2 || ctx.Err() == nil {
		t.Fatalf("Publish cancelled after the sync = %+v, %v (ctx %v)", pub, err, ctx.Err())
	}
	svcCheckArmed(t, fx, rec.ID, true) // revision 2's timer, not revision 1's

	ctx = bridge.arm()
	if err := s.SetEnabled(ctx, rec.ID, false); err != nil {
		t.Fatalf("disabling cancelled after the switch = %v", err)
	}
	svcCheckArmed(t, fx, rec.ID, false)

	if err := s.SetEnabled(context.Background(), rec.ID, true); err != nil {
		t.Fatal(err)
	}
	ctx = bridge.arm()
	if err := s.DeleteFlow(ctx, rec.ID); err != nil || ctx.Err() == nil {
		t.Fatalf("DeleteFlow cancelled after the mission delete = %v (ctx %v)", err, ctx.Err())
	}
	if _, err := s.GetFlow(context.Background(), rec.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetFlow after the delete = %v", err)
	}
	if got := svcTimers(t, s); len(got) != 0 {
		t.Fatalf("timers after the delete = %v", got)
	}

	// A context that ended before the call still stops it before anything happened.
	other := fx.published(t, "Frueh", svcDateTimeFlow("Frueh", 12, "2026-10-04 09:00"))
	ended, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.SetEnabled(ended, other.ID, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("SetEnabled with an ended context = %v", err)
	}
	if m, _ := bridge.mission(other.MissionID); m.enabled {
		t.Fatal("an ended context must not flip the switch")
	}
}

// svcLateRunFixture is a published flow (manual trigger, then a web search on blocking
// tools) with one run inside its tool call and one queued behind it.
type svcLateRunFixture struct {
	s       *Service
	tools   *svcBlockingTools
	rec     *FlowRecord
	req     StartRequest
	running StartResult
	queued  StartResult
}

func newSvcLateRunFixture(t *testing.T) *svcLateRunFixture {
	t.Helper()
	tools := newSvcBlockingTools()
	s := svcNewService(t, &Services{Tools: tools, Location: time.UTC}, newSvcBridge(), slog.New(&svcLogs{}))
	t.Cleanup(tools.letGo) // runs before the Shutdown
	ctx := context.Background()
	b := newFlow("Spaet")
	start := b.node("start", TypeTriggerManual, nil)
	search := b.node("search", TypeWebSearch, map[string]any{"query": "x"})
	b.edge(start, PortOut, search)
	rec, err := s.CreateFlow(ctx, CreateRequest{Name: "Spaet"})
	if err != nil {
		t.Fatal(err)
	}
	rev, _, err := s.SaveDraft(ctx, rec.ID, b.build(), rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	pub, _, err := s.Publish(ctx, rec.ID, rev)
	if err != nil {
		t.Fatal(err)
	}
	fx := &svcLateRunFixture{s: s, tools: tools, rec: pub,
		req: StartRequest{Flow: pub.Live, Revision: pub.LiveRevision, Mode: ModeLive, TriggerNode: start, TriggerType: TypeTriggerManual}}
	if fx.running, err = s.runner.Start(fx.req); err != nil {
		t.Fatal(err)
	}
	select {
	case <-tools.called:
	case <-time.After(5 * time.Second):
		t.Fatal("the run did not reach its tool")
	}
	if fx.queued, err = s.runner.Start(fx.req); err != nil || fx.queued.Status != StartQueued {
		t.Fatalf("second Start = %+v, %v", fx.queued, err)
	}
	return fx
}

// A run recorded after the first cancel but before the store delete is caught by the
// second cancel. The run is started from the OnRunFinished call the first CancelFlow makes
// for the queued run, which is exactly that window, and must be reported finished
// (cancelled) before DeleteFlow returns.
func TestServiceDeleteCancelsARunRecordedDuringTheDelete(t *testing.T) {
	fx := newSvcLateRunFixture(t)
	var mu sync.Mutex
	finished := map[string]RunStatus{}
	inject := true
	var late StartResult
	var lateErr error
	hook := fx.s.runner.hooks.OnRunFinished
	fx.s.runner.hooks.OnRunFinished = func(rec RunRecord, res RunResult) {
		if hook != nil {
			hook(rec, res)
		}
		mu.Lock()
		finished[rec.ID] = rec.Status
		doInject := inject
		inject = false
		mu.Unlock()
		if doInject { // the queued run, ended by the first CancelFlow
			res, err := fx.s.runner.Start(fx.req)
			mu.Lock()
			late, lateErr = res, err
			mu.Unlock()
		}
	}
	if err := fx.s.DeleteFlow(context.Background(), fx.rec.ID); err != nil {
		t.Fatalf("DeleteFlow: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if finished[fx.queued.RunID] != RunCancelled {
		t.Fatalf("the queued run = %q, want cancelled", finished[fx.queued.RunID])
	}
	if lateErr != nil || late.RunID == "" {
		t.Fatalf("the late Start = %+v, %v; it must record its run before the store delete", late, lateErr)
	}
	if st, ok := finished[late.RunID]; !ok || st != RunCancelled {
		t.Fatalf("the late run when DeleteFlow returned = %q (finished %v), want cancelled", st, ok)
	}
}

// DeleteFlowForMission ignores the end of ctx once it holds the lock: a context that ends
// while it cancels the runs (here from the queued run's OnRunFinished) does not stop the
// store delete.
func TestServiceDeleteFlowForMissionFinishesAfterCancel(t *testing.T) {
	fx := newSvcLateRunFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	hook := fx.s.runner.hooks.OnRunFinished
	fx.s.runner.hooks.OnRunFinished = func(rec RunRecord, res RunResult) {
		if hook != nil {
			hook(rec, res)
		}
		cancel()
	}
	if err := fx.s.DeleteFlowForMission(ctx, fx.rec.MissionID); err != nil || ctx.Err() == nil {
		t.Fatalf("DeleteFlowForMission cancelled during the run cancel = %v (ctx %v)", err, ctx.Err())
	}
	if _, err := fx.s.GetFlow(context.Background(), fx.rec.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetFlow after the delete = %v", err)
	}
}
