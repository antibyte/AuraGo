package flows

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

// svcGateBridge is svcRunBridge with the optional RunGate: it refuses automatic runs
// with err while err is set.
type svcGateBridge struct {
	*svcRunBridge
	err error
}

func (b *svcGateBridge) AutomaticRunsAllowed() error { return b.err }

// A Date/Time timer that fires while the bridge's RunGate refuses automatic runs (missions
// read-only, flows switched off) starts no run and is handled: the occurrence is consumed,
// as for a flow that is switched off, and the skip is logged at Info with the reason. A
// bridge without the gate, or one that allows, starts the run as before.
func TestTimerFiredAsksTheRunGate(t *testing.T) {
	logs := &svcLogs{}
	bridge := &svcGateBridge{svcRunBridge: newSvcRunBridge(), err: errors.New("missions are read-only")}
	s := svcRunNewService(t, &fakeTools{}, bridge, slog.New(logs), ServiceConfig{})
	runs := svcRunObserve(s)
	pub := svcRunPublish(t, s, simpleFlow("Gated"))
	trigger := pub.Live.Nodes[0].ID
	at := time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)

	if err := s.onTimerFired(pub.ID, trigger, at); err != nil {
		t.Fatalf("a refused timer must be handled (nil), got %v", err)
	}
	if recs, err := s.Store().ListRuns(context.Background(), pub.ID, RunFilter{Limit: 10}); err != nil || len(recs) != 0 || len(bridge.startedRuns()) != 0 {
		t.Fatalf("a run was created behind the gate: %+v, %v, %d started", recs, err, len(bridge.startedRuns()))
	}
	if got := logs.messages(slog.LevelInfo, "no run was started"); len(got) != 1 {
		t.Fatalf("Info records = %v, want one", got)
	}

	bridge.err = nil
	if err := s.onTimerFired(pub.ID, trigger, at); err != nil {
		t.Fatalf("an allowed timer: %v", err)
	}
	recs, err := s.Store().ListRuns(context.Background(), pub.ID, RunFilter{Limit: 10})
	if err != nil || len(recs) != 1 {
		t.Fatalf("an allowed timer started %d runs, %v", len(recs), err)
	}
	runs.wait(t, recs[0].ID)
}
