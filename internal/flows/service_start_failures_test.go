package flows

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// svcStartFailBridge is svcGateBridge (a RunGate a test can close) with the optional
// StartFailureReporter, so a test sees what the Service reports.
type svcStartFailBridge struct {
	*svcGateBridge
	failMu sync.Mutex
	failed []RunFinishedInfo
}

func newSvcStartFailBridge() *svcStartFailBridge {
	return &svcStartFailBridge{svcGateBridge: &svcGateBridge{svcRunBridge: newSvcRunBridge()}}
}

func (b *svcStartFailBridge) FlowStartFailed(info RunFinishedInfo) {
	b.failMu.Lock()
	b.failed = append(b.failed, info)
	b.failMu.Unlock()
}

func (b *svcStartFailBridge) reported() []RunFinishedInfo {
	b.failMu.Lock()
	defer b.failMu.Unlock()
	return append([]RunFinishedInfo(nil), b.failed...)
}

// waitStartFailures waits for the record goroutines of start failures; the Service counts them
// before TriggerFromMission returns, so this is deterministic.
func waitStartFailures(s *Service) { s.startFailures.Wait() }

// startFailureRuns returns the stored runs of the flow that carry FLOW_START_FAILED.
func startFailureRuns(t *testing.T, s *Service, flowID string) []RunRecord {
	t.Helper()
	runs, err := s.Store().ListRuns(context.Background(), flowID, RunFilter{Limit: 50})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	var out []RunRecord
	for _, r := range runs {
		if r.ErrorCode == StartFailedCode {
			out = append(out, r)
		}
	}
	return out
}

func TestStartFailureStreak(t *testing.T) {
	// One record per streak: fail, fail, success, fail records two failed runs.
	t.Run("a success ends the streak", func(t *testing.T) {
		bridge := newSvcStartFailBridge()
		s := svcRunNewService(t, &fakeTools{}, bridge, nil, ServiceConfig{})
		runs := svcRunObserve(s)
		pub := svcRunPublish(t, s, simpleFlow("Streak"))
		fire := func(node string) (StartResult, error) {
			return s.TriggerFromMission(pub.MissionID, node, "webhook", nil)
		}
		for i := 0; i < 2; i++ {
			if _, err := fire("n_nope"); !errors.Is(err, ErrNoTrigger) {
				t.Fatalf("fire %d: err = %v, want ErrNoTrigger", i, err)
			}
		}
		res, err := fire("")
		if err != nil {
			t.Fatalf("a valid start: %v", err)
		}
		runs.wait(t, res.RunID)
		if _, err := fire("n_nope"); !errors.Is(err, ErrNoTrigger) {
			t.Fatalf("the fire after the success: err = %v, want ErrNoTrigger", err)
		}
		waitStartFailures(s)
		got := startFailureRuns(t, s, pub.ID)
		if len(got) != 2 || len(bridge.reported()) != 2 {
			t.Fatalf("recorded %d failed runs, reported %d; want 2 and 2", len(got), len(bridge.reported()))
		}
		rec := got[0]
		if rec.Status != RunError || rec.Mode != ModeLive || rec.Revision != pub.LiveRevision || rec.TriggerNode != "n_nope" ||
			rec.TriggerType != "webhook" || rec.FinishedAt == nil || !rec.StartedAt.Equal(*rec.FinishedAt) || !strings.Contains(rec.ErrorMessage, "trigger") {
			t.Fatalf("failed run = %+v", rec)
		}
		info := bridge.reported()[0]
		if info.Started || info.MissionID != pub.MissionID || info.FlowName != "Streak" || info.Record.ErrorCode != StartFailedCode ||
			info.Result.Status != RunError || info.Outputs == nil {
			t.Fatalf("reported info = %+v", info)
		}
	})

	// A different reason is a new streak: not_published, then no_trigger, records two.
	t.Run("a new reason records again", func(t *testing.T) {
		bridge := newSvcStartFailBridge()
		s := svcRunNewService(t, &fakeTools{}, bridge, nil, ServiceConfig{})
		ctx := context.Background()
		rec, err := s.CreateFlow(ctx, CreateRequest{Name: "Reasons"})
		if err != nil {
			t.Fatal(err)
		}
		rev, _, err := s.SaveDraft(ctx, rec.ID, simpleFlow("Reasons"), rec.DraftRevision)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.TriggerFromMission(rec.MissionID, "", "manual", nil); !errors.Is(err, ErrNotPublished) {
			t.Fatalf("unpublished: err = %v", err)
		}
		pub, _, err := s.Publish(ctx, rec.ID, rev)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.TriggerFromMission(rec.MissionID, "n_nope", "manual", nil); !errors.Is(err, ErrNoTrigger) {
			t.Fatalf("published, unknown trigger: err = %v", err)
		}
		waitStartFailures(s)
		got := startFailureRuns(t, s, rec.ID)
		if len(got) != 2 {
			t.Fatalf("recorded %d failed runs, want 2", len(got))
		}
		byRevision := map[int]RunRecord{got[0].Revision: got[0], got[1].Revision: got[1]}
		if r, ok := byRevision[0]; !ok || r.ErrorMessage != "the flow is not published" {
			t.Fatalf("the unpublished record (revision 0) = %+v", byRevision)
		}
		if r, ok := byRevision[pub.LiveRevision]; !ok || !strings.Contains(r.ErrorMessage, "trigger") {
			t.Fatalf("the no-trigger record (revision %d) = %+v", pub.LiveRevision, byRevision)
		}
	})

	// A refusal on purpose records nothing: a closed RunGate (runs paused) and a closed runner.
	t.Run("refusals record nothing", func(t *testing.T) {
		bridge := newSvcStartFailBridge()
		s := svcRunNewService(t, &fakeTools{}, bridge, nil, ServiceConfig{})
		ctx := context.Background()
		rec, err := s.CreateFlow(ctx, CreateRequest{Name: "Paused"})
		if err != nil {
			t.Fatal(err)
		}
		bridge.svcGateBridge.err = errors.New("flow runs are paused while missions are read-only")
		if _, err := s.TriggerFromMission(rec.MissionID, "", "manual", nil); !errors.Is(err, ErrNotPublished) {
			t.Fatalf("paused, unpublished: err = %v", err)
		}
		waitStartFailures(s)
		if got := startFailureRuns(t, s, rec.ID); len(got) != 0 {
			t.Fatalf("a refused trigger recorded %d runs", len(got))
		}
		bridge.svcGateBridge.err = nil // the same failure now counts, so the gate was the reason before
		if _, err := s.TriggerFromMission(rec.MissionID, "", "manual", nil); !errors.Is(err, ErrNotPublished) {
			t.Fatalf("unpaused, unpublished: err = %v", err)
		}
		waitStartFailures(s)
		if got := startFailureRuns(t, s, rec.ID); len(got) != 1 {
			t.Fatalf("after the gate opened: %d runs, want 1", len(got))
		}

		closed := newSvcStartFailBridge()
		s2 := svcRunNewService(t, &fakeTools{}, closed, nil, ServiceConfig{})
		pub := svcRunPublish(t, s2, simpleFlow("Closing"))
		if err := s2.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := s2.TriggerFromMission(pub.MissionID, "", "manual", nil); !errors.Is(err, ErrRunnerClosed) {
			t.Fatalf("closed runner: err = %v", err)
		}
		waitStartFailures(s2)
		if got := startFailureRuns(t, s2, pub.ID); len(got) != 0 || len(closed.reported()) != 0 {
			t.Fatalf("a closed runner recorded %d runs and reported %d", len(got), len(closed.reported()))
		}
	})

	// A Date/Time timer has no caller to answer, so its start failures count like a trigger's.
	// onTimerFired is the unexported fire function the timer service calls; the test calls it
	// directly, which is deterministic (the callback runs on the caller's goroutine).
	t.Run("a timer start that fails twice records one run", func(t *testing.T) {
		bridge := newSvcStartFailBridge()
		s := svcRunNewService(t, &fakeTools{}, bridge, nil, ServiceConfig{})
		pub := svcRunPublish(t, s, simpleFlow("Timed"))
		at := time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)
		for i := 0; i < 2; i++ {
			// The node is no enabled trigger of the published revision: ErrNoTrigger, a consumed occurrence.
			if err := s.onTimerFired(pub.ID, "n_nope", at.Add(time.Duration(i)*time.Minute)); err != nil {
				t.Fatalf("fire %d: %v", i, err)
			}
		}
		waitStartFailures(s)
		got := startFailureRuns(t, s, pub.ID)
		if len(got) != 1 || got[0].TriggerType != "datetime" || got[0].TriggerNode != "n_nope" || len(bridge.reported()) != 1 {
			t.Fatalf("two failed timer starts recorded %+v, reported %d; want one run of type datetime", got, len(bridge.reported()))
		}

		// A timer of a flow without a published revision is the other early return.
		rec, err := s.CreateFlow(context.Background(), CreateRequest{Name: "Timed draft"})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			if err := s.onTimerFired(rec.ID, "n_nope", at.Add(time.Duration(i)*time.Minute)); err != nil {
				t.Fatalf("draft fire %d: %v", i, err)
			}
		}
		waitStartFailures(s)
		if got := startFailureRuns(t, s, rec.ID); len(got) != 1 || got[0].ErrorMessage != "the flow is not published" {
			t.Fatalf("two timers of an unpublished flow recorded %+v, want one not-published run", got)
		}
	})

	// The classification itself: what counts and what does not.
	t.Run("classification", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			err  error
			want string
		}{
			{"not published", ErrNotPublished, "not_published"},
			{"no trigger", fmt.Errorf("%w: x", ErrNoTrigger), "no_trigger"},
			{"other start error", errors.New("a node could not be prepared"), "start_error: a node could not be prepared"},
			{"flow deleted", fmt.Errorf("record run: %w", ErrNotFound), ""},
			{"runner closed", ErrRunnerClosed, ""},
			{"queue full", ErrRunnerQueueFull, ""},
			{"switched off", ErrFlowDisabled, ""},
			{"mission missing", ErrFlowMissionMissing, ""},
			{"mission control unavailable", ErrMissionControlUnavailable, ""},
			{"caller gave up", context.Canceled, ""},
		} {
			if got := startFailureReason(tc.err); got != tc.want {
				t.Errorf("%s: reason = %q, want %q", tc.name, got, tc.want)
			}
			if got := IsStartFailure(tc.err); got != (tc.want != "") {
				t.Errorf("%s: IsStartFailure = %v", tc.name, got)
			}
		}
	})
}
