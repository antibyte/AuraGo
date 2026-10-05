package flows

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// svcRunHoldTools holds every tool call until the run's context ends: a run that reaches
// its tool only ends when it is cancelled.
type svcRunHoldTools struct{}

func (svcRunHoldTools) InvokeTool(ctx context.Context, _ ToolRequest) (ToolResponse, error) {
	<-ctx.Done()
	return ToolResponse{}, ctx.Err()
}

// svcRunUncancelled lists the runs of the flow the runner still has queued, waiting for
// a slot, or running without having been cancelled.
func svcRunUncancelled(s *Service, flowID string) []string {
	r := s.runner
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, p := range r.flowQueue[flowID] {
		out = append(out, "queued "+p.rec.ID)
	}
	for _, p := range r.waiting {
		if p.rec.FlowID == flowID {
			out = append(out, "waiting "+p.rec.ID)
		}
	}
	for id, run := range r.cancels {
		if run.flowID == flowID && !run.cancelled {
			out = append(out, "running "+id)
		}
	}
	return out
}

// svcRunActive counts the runs of the flow the runner still knows.
func svcRunActive(s *Service, flowID string) int {
	r := s.runner
	r.mu.Lock()
	defer r.mu.Unlock()
	n := len(r.flowQueue[flowID])
	for _, p := range r.waiting {
		if p.rec.FlowID == flowID {
			n++
		}
	}
	for _, run := range r.cancels {
		if run.flowID == flowID {
			n++
		}
	}
	return n
}

// svcRunAttempt is one start of a run during the race.
type svcRunAttempt struct {
	path  string
	after bool // the start began after DeleteFlow had returned
	res   StartResult
	err   error
}

// svcRunMaxStarts is how many starts a path makes before it waits for the delete and
// makes its last one, which keeps the number of runs per round bounded.
const svcRunMaxStarts = 10

// Every start path races a delete of the flow, round after round: RunNow,
// TriggerFromMission, StartTestRun and the Date/Time timer callback. A start either
// records its run before the store delete, and then the delete has cancelled it when
// DeleteFlow returns, or it fails with ErrNotFound; a start that begins after DeleteFlow
// returned always fails. From the moment DeleteFlow returns the runner holds no run of
// the flow that was not cancelled, and every run ends without help: its tool returns
// only when the run is cancelled. Each live run is reported to Mission Control once,
// and nothing is logged as a warning.
func TestServiceStartsRacingADelete(t *testing.T) {
	logs := &svcLogs{}
	bridge := newSvcRunBridge()
	s := svcRunNewService(t, svcRunHoldTools{}, bridge, slog.New(logs), ServiceConfig{MaxQueuedPerFlow: 1000})
	runs := svcRunObserve(s)
	ctx := context.Background()
	at := time.Date(2099, 1, 1, 8, 0, 0, 0, time.UTC)
	for round := 1; round <= 4; round++ {
		b := newFlow("Rennen")
		start := b.node("start", TypeTriggerManual, nil)
		when := b.node("when", TypeTriggerDateTime, map[string]any{"at": "2099-01-01 08:00"})
		search := b.node("search", TypeWebSearch, map[string]any{"query": "wetter"})
		b.edge(start, PortOut, search)
		b.edge(when, PortOut, search)
		pub := svcRunPublish(t, s, b.build())
		if err := s.SetEnabled(ctx, pub.ID, true); err != nil {
			t.Fatal(err)
		}

		begin, deleted := make(chan struct{}), make(chan struct{})
		isDeleted := func() bool {
			select {
			case <-deleted:
				return true
			default:
				return false
			}
		}
		paths := []string{"RunNow", "RunNow", "TriggerFromMission", "TriggerFromMission", "StartTestRun", "StartTestRun", "timer", "timer"}
		results := make(chan []svcRunAttempt, len(paths))
		// Each path starts once before the delete begins, so every round has runs for the
		// delete to cancel; the later starts race with the delete.
		primed := make(chan struct{}, len(paths))
		for _, path := range paths {
			go func() {
				<-begin
				var out []svcRunAttempt
				for i := 0; ; i++ {
					if i == 1 {
						primed <- struct{}{}
					}
					if i == svcRunMaxStarts {
						<-deleted
					}
					a := svcRunAttempt{path: path, after: isDeleted()}
					switch path {
					case "RunNow":
						a.res, a.err = s.RunNow(ctx, pub.ID)
					case "TriggerFromMission":
						a.res, a.err = s.TriggerFromMission(pub.MissionID, "", "api", nil)
					case "StartTestRun":
						a.res, a.err = s.StartTestRun(ctx, pub.ID, TestRunRequest{})
					case "timer":
						s.onTimerFired(pub.ID, when, at)
					}
					out = append(out, a)
					if a.after {
						break
					}
				}
				results <- out
			}()
		}
		type deleteResult struct {
			err  error
			left []string
		}
		done := make(chan deleteResult, 1)
		go func() {
			<-begin
			for range paths {
				<-primed
			}
			err := s.DeleteFlow(ctx, pub.ID)
			left := svcRunUncancelled(s, pub.ID) // at once, while the starts go on
			close(deleted)
			done <- deleteResult{err, left}
		}()
		close(begin)

		select {
		case d := <-done:
			if d.err != nil || len(d.left) != 0 {
				t.Fatalf("round %d: DeleteFlow = %v; runs of the flow not cancelled when it returned: %v", round, d.err, d.left)
			}
		case <-time.After(30 * time.Second):
			t.Fatalf("round %d: DeleteFlow did not return", round)
		}
		var started []svcRunAttempt
		raced := 0 // starts during the delete that found the flow gone
		for range paths {
			var out []svcRunAttempt
			select {
			case out = <-results:
			case <-time.After(30 * time.Second):
				t.Fatalf("round %d: a start path did not finish", round)
			}
			for _, a := range out {
				switch {
				case a.path == "timer":
				case a.err == nil && !a.after && a.res.RunID != "":
					started = append(started, a)
				case !errors.Is(a.err, ErrNotFound):
					t.Fatalf("round %d: %s (after the delete: %v) = %+v, %v; want a run or ErrNotFound", round, a.path, a.after, a.res, a.err)
				case !a.after:
					raced++
				}
			}
		}
		if len(started) < 6 {
			t.Fatalf("round %d: %d runs started before the delete; every API path starts one first", round, len(started))
		}
		if left := svcRunUncancelled(s, pub.ID); len(left) != 0 {
			t.Fatalf("round %d: runs of the deleted flow not cancelled after the starts: %v", round, left)
		}
		svcRunWaitIdle(t, s, runs, pub.ID)
		for _, a := range started {
			id := a.res.RunID
			if rec := runs.wait(t, id); rec.Status != RunCancelled {
				t.Fatalf("round %d: %s run %s ended %s, want cancelled", round, a.path, id, rec.Status)
			}
			if n := len(bridge.reports(id)); (a.path == "StartTestRun") != (n == 0) || n > 1 {
				t.Fatalf("round %d: %s run %s was reported %d times", round, a.path, id, n)
			}
		}
		// The timer path returns nothing, so its runs are found in the snapshot. It is
		// protected mainly by the mission going first: once DeleteFlowMission ran,
		// FlowMissionEnabled is false and onTimerFired starts nothing. A callback that
		// passed that check before is covered like the API paths (CreateRun and the second
		// CancelFlow), and its runs must have ended cancelled too.
		timerRuns := 0
		for id, rec := range runs.snapshot() {
			if rec.FlowID != pub.ID {
				continue
			}
			if rec.Status != RunCancelled {
				t.Fatalf("round %d: run %s of the deleted flow ended %s", round, id, rec.Status)
			}
			if rec.TriggerType == "datetime" {
				timerRuns++
			}
		}
		if timerRuns == 0 {
			t.Fatalf("round %d: the timer path started no run before the delete", round)
		}
		t.Logf("round %d: %d runs started by the API paths and cancelled, %d starts failed during the delete", round, len(started), raced)
	}
	if warned := logs.messages(slog.LevelWarn, ""); len(warned) != 0 {
		t.Fatalf("logged: %v", warned)
	}
}

// svcRunCrossBridge starts a run of another flow's mission from FlowRunFinished, the way
// Mission Control fires a dependent mission synchronously.
type svcRunCrossBridge struct {
	*svcRunBridge
	crossMu sync.Mutex
	s       *Service
	next    map[string]string // flow id -> mission id to trigger when one of its runs ends
	starts  int
}

func (b *svcRunCrossBridge) FlowRunFinished(info RunFinishedInfo) {
	b.svcRunBridge.FlowRunFinished(info)
	b.crossMu.Lock()
	s, mission := b.s, b.next[info.Record.FlowID]
	b.crossMu.Unlock()
	if s == nil || mission == "" {
		return
	}
	if _, err := s.TriggerFromMission(mission, "", "mission_completed", nil); err == nil {
		b.crossMu.Lock()
		b.starts++
		b.crossMu.Unlock()
	}
}

// Two flows whose ending runs start each other through Mission Control are deleted at
// the same time. TriggerFromMission runs inside each delete (from FlowRunFinished, with
// that flow's lock held) and takes no flow lock, so neither delete waits for the other,
// and no run of either flow is left uncancelled.
func TestServiceDeletesWithCrossFlowTriggers(t *testing.T) {
	bridge := &svcRunCrossBridge{svcRunBridge: newSvcRunBridge(), next: map[string]string{}}
	s := svcRunNewService(t, svcRunHoldTools{}, bridge, nil, ServiceConfig{})
	ctx := context.Background()
	a := svcRunPublish(t, s, svcRunSearchFlow("A", "done"))
	b := svcRunPublish(t, s, svcRunSearchFlow("B", "done"))
	for _, f := range []*FlowRecord{a, b} {
		for i := 0; i < 5; i++ {
			if _, err := s.RunNow(ctx, f.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	bridge.crossMu.Lock()
	bridge.s = s
	bridge.next[a.ID], bridge.next[b.ID] = b.MissionID, a.MissionID
	bridge.crossMu.Unlock()
	begin := make(chan struct{})
	errs := make(chan error, 2)
	for _, f := range []*FlowRecord{a, b} {
		go func() {
			<-begin
			errs <- s.DeleteFlow(ctx, f.ID)
		}()
	}
	close(begin)
	for i := 0; i < 2; i++ {
		select {
		case err := <-errs:
			if err != nil {
				t.Fatalf("DeleteFlow: %v", err)
			}
		case <-time.After(30 * time.Second):
			t.Fatal("the deletes wait for each other")
		}
	}
	bridge.crossMu.Lock()
	bridge.s = nil
	starts := bridge.starts
	bridge.crossMu.Unlock()
	for _, f := range []*FlowRecord{a, b} {
		if left := svcRunUncancelled(s, f.ID); len(left) != 0 {
			t.Fatalf("runs of flow %s left uncancelled: %v", f.Name, left)
		}
	}
	t.Logf("runs started from inside the deletes: %d", starts)
}

// svcRunWaitIdle waits until the runner knows no run of the flow; every run ends with
// OnRunFinished, which wakes the observer after the run was released.
func svcRunWaitIdle(t *testing.T, s *Service, runs *svcRunObserver, flowID string) {
	t.Helper()
	deadline := time.After(30 * time.Second)
	for svcRunActive(s, flowID) > 0 {
		select {
		case <-runs.notify:
		case <-deadline:
			t.Fatalf("%d runs of the deleted flow did not end", svcRunActive(s, flowID))
		}
	}
}

func (o *svcRunObserver) snapshot() map[string]RunRecord {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make(map[string]RunRecord, len(o.finished))
	for id, rec := range o.finished {
		out[id] = rec
	}
	return out
}
