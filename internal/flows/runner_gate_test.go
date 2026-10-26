package flows

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// b3Gate is a RunnerConfig.Gate a test closes and opens; it counts its calls.
type b3Gate struct {
	mu    sync.Mutex
	err   error
	calls int
}

func (g *b3Gate) allow() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	return g.err
}

func (g *b3Gate) set(err error) {
	g.mu.Lock()
	g.err = err
	g.mu.Unlock()
}

func (g *b3Gate) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls
}

func b3Start(t *testing.T, fx *runnerFixture, f *Flow, gateName string, gated bool) StartResult {
	t.Helper()
	res, err := fx.r.Start(StartRequest{Flow: f, Mode: ModeLive, TriggerNode: f.Nodes[0].ID, TriggerType: "webhook",
		TriggerData: map[string]any{"gate": gateName}, Gated: gated})
	if err != nil {
		t.Fatalf("Start %s: %v", gateName, err)
	}
	return res
}

// R1 (audit 2026-10-25): read-only missions and flows switched off stopped only new automatic
// runs; runs already waiting in a queue still started, with all their effects. The runner now
// asks RunnerConfig.Gate when such a run leaves its queue: refused, it ends cancelled with
// FLOW_RUNS_PAUSED and never executes. A run someone asked for (Gated false) still runs, and a
// run that never waited is not asked.
func TestB3QueuedAutomaticRunsAskTheGateAtLaunch(t *testing.T) {
	for _, c := range []struct {
		name   string
		cfg    RunnerConfig
		policy ConcurrencyPolicy
	}{
		{"flow queue", RunnerConfig{}, ConcurrencyQueue},
		{"global waiting list", RunnerConfig{MaxParallelRuns: 1}, ConcurrencyParallel},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := &b3Gate{}
			c.cfg.Gate = g.allow
			fx := newRunnerFixture(t, c.cfg)
			f := fx.flow(t, "flow_aaaaaaaarg", c.policy)
			first := b3Start(t, fx, f, "a", true)
			fx.gate.waitStarted(t, "a")
			second := b3Start(t, fx, f, "b", true)
			third := b3Start(t, fx, f, "c", true)
			asked := b3Start(t, fx, f, "d", false)
			for _, res := range []StartResult{second, third, asked} {
				if res.Status != StartQueued {
					t.Fatalf("a run behind the active one = %+v, want queued", res)
				}
			}
			if g.count() != 0 {
				t.Fatalf("the gate was asked %d times before a queued run left its queue", g.count())
			}
			g.set(errors.New("flow runs are paused while missions are read-only"))
			fx.gate.release("a")
			fx.gate.waitStarted(t, "d")
			fx.gate.release("d")
			got := map[string]RunRecord{}
			for i := 0; i < 4; i++ {
				rec := fx.waitFinished(t)
				got[rec.ID] = rec
			}
			fx.gate.assertNotStarted(t)
			if got[first.RunID].Status != RunSuccess || got[asked.RunID].Status != RunSuccess {
				t.Fatalf("the running run and the asked-for run = %+v, %+v; want both to succeed", got[first.RunID], got[asked.RunID])
			}
			for _, id := range []string{second.RunID, third.RunID} {
				rec := got[id]
				if rec.Status != RunCancelled || rec.ErrorCode != RunsPausedCode || rec.FinishedAt == nil {
					t.Errorf("queued automatic run %s = %+v, want cancelled with %s", id, rec, RunsPausedCode)
				}
				stored, steps, err := fx.store.GetRun(context.Background(), id)
				if err != nil || stored.Status != RunCancelled || stored.ErrorCode != RunsPausedCode || len(steps) != 0 {
					t.Errorf("stored run %s = %+v, %d steps, %v; want cancelled without steps", id, stored, len(steps), err)
				}
			}
			if g.count() != 2 {
				t.Errorf("the gate was asked %d times, want 2 (once per queued automatic run)", g.count())
			}
			startedIDs := map[string]bool{}
			for len(fx.started) > 0 {
				startedIDs[(<-fx.started).ID] = true
			}
			if len(startedIDs) != 2 || !startedIDs[first.RunID] || !startedIDs[asked.RunID] {
				t.Errorf("OnRunStarted fired for %v, want only the running run and the asked-for run", startedIDs)
			}
		})
	}
}

// b3GateBridge is svcRunBridge with a RunGate that a test switches while runs go on.
type b3GateBridge struct {
	*svcRunBridge
	gateMu  sync.Mutex
	gateErr error
}

func (b *b3GateBridge) AutomaticRunsAllowed() error {
	b.gateMu.Lock()
	defer b.gateMu.Unlock()
	return b.gateErr
}

func (b *b3GateBridge) setGate(err error) {
	b.gateMu.Lock()
	b.gateErr = err
	b.gateMu.Unlock()
}

// The Service wires the bridge's RunGate into the runner and marks Mission Control's starts as
// automatic, Run now not: with the gate closed while they wait, the triggered runs are reported
// as never started with FLOW_RUNS_PAUSED, and the Run now behind them still runs.
func TestB3ServiceGatesQueuedTriggerRuns(t *testing.T) {
	tools := newSvcRunGateTools()
	bridge := &b3GateBridge{svcRunBridge: newSvcRunBridge()}
	s := svcRunNewService(t, tools, bridge, nil, ServiceConfig{})
	t.Cleanup(tools.open)
	ctx := context.Background()
	pub := svcRunPublish(t, s, svcRunSearchFlow("Gated queue", "done"))
	first, err := s.RunNow(ctx, pub.ID)
	if err != nil {
		t.Fatal(err)
	}
	tools.waitCalled(t, first.RunID)
	var triggered []StartResult
	for i := 0; i < 2; i++ {
		res, err := s.TriggerFromMission(pub.MissionID, "", "webhook", nil)
		if err != nil || res.Status != StartQueued {
			t.Fatalf("TriggerFromMission = %+v, %v; want a queued run", res, err)
		}
		triggered = append(triggered, res)
	}
	asked, err := s.RunNow(ctx, pub.ID)
	if err != nil || asked.Status != StartQueued {
		t.Fatalf("RunNow behind them = %+v, %v", asked, err)
	}
	bridge.setGate(errors.New("flow runs are paused while missions are read-only"))
	tools.open()
	for _, res := range triggered {
		info := bridge.waitInfo(t, res.RunID)
		if info.Started || info.Result.Status != RunCancelled || info.Result.ErrorCode != RunsPausedCode || info.MissionID != pub.MissionID {
			t.Errorf("triggered run %s reported %+v, want never started, cancelled with %s in %s", res.RunID, info, RunsPausedCode, pub.MissionID)
		}
	}
	if info := bridge.waitInfo(t, asked.RunID); !info.Started || info.Result.Status != RunSuccess {
		t.Errorf("the Run now behind them reported %s (started %v), want it to run", info.Result.Status, info.Started)
	}
}
