package flows

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// Audit 2026-10-08, finding 1.7: a Date/Time occurrence is consumed only after a run was
// created or the start was deliberately skipped. A failed start leaves it stored, and
// the timer service tries it again after a bounded backoff.

// audit17Fixture is a published, switched-on flow with a manual trigger "start" and a
// Date/Time trigger "when" (2026-10-03 08:00 UTC), both leading to a web search on gate
// tools and a set node "done". The clock is a fake at 07:00 and MaxQueuedPerFlow is 1.
// The Service is not started: the tests drive the timer passes themselves, so no loop
// runs processDue at the same time.
type audit17Fixture struct {
	s      *Service
	clock  *fakeClock
	tools  *svcRunGateTools
	bridge *svcRunBridge
	logs   *svcLogs
	pub    *FlowRecord
	when   string
	due    time.Time
}

func newAudit17Fixture(t *testing.T, concurrency ConcurrencyPolicy) *audit17Fixture {
	t.Helper()
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	fx := &audit17Fixture{clock: clock, tools: newSvcRunGateTools(), bridge: newSvcRunBridge(), logs: &svcLogs{},
		due: time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)}
	fx.s = NewService(openTestStore(t), catalogRegistry(t, fullEnv()), &Services{Tools: fx.tools, Clock: clock, Location: time.UTC},
		fx.bridge, ServiceConfig{MaxQueuedPerFlow: 1}, slog.New(fx.logs))
	t.Cleanup(func() { _ = fx.s.Shutdown(context.Background()) })
	t.Cleanup(fx.tools.open) // runs before the Shutdown
	b := newFlow("Termin")
	start := b.node("start", TypeTriggerManual, nil)
	fx.when = b.node("when", TypeTriggerDateTime, map[string]any{"at": "2026-10-03 08:00"})
	search := b.node("search", TypeWebSearch, map[string]any{"query": "wetter"})
	done := b.node("done", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "v", "value": "done"}}})
	b.edge(start, PortOut, search)
	b.edge(fx.when, PortOut, search)
	b.edge(search, PortOut, done)
	doc := b.build()
	doc.Settings.Concurrency = concurrency
	fx.pub = svcRunPublish(t, fx.s, doc)
	fx.assertOccurrence(t, fx.due)
	return fx
}

// assertOccurrence checks that the flow's only timer is the Date/Time node at at.
func (fx *audit17Fixture) assertOccurrence(t *testing.T, at time.Time) {
	t.Helper()
	list, err := fx.s.store.ListTimers(context.Background())
	if err != nil {
		t.Fatalf("ListTimers: %v", err)
	}
	if len(list) != 1 || list[0].FlowID != fx.pub.ID || list[0].NodeID != fx.when || !list[0].FireAt.Equal(at) {
		t.Fatalf("stored timers = %+v, want the occurrence of %s at %v", list, fx.when, at)
	}
}

func (fx *audit17Fixture) assertNoTimers(t *testing.T) {
	t.Helper()
	list, err := fx.s.store.ListTimers(context.Background())
	if err != nil {
		t.Fatalf("ListTimers: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("stored timers = %+v, want the occurrence consumed", list)
	}
}

// timerRuns returns the flow's live runs that the Date/Time trigger started.
func (fx *audit17Fixture) timerRuns(t *testing.T) []RunRecord {
	t.Helper()
	runs, err := fx.s.Runs(context.Background(), fx.pub.ID, RunFilter{Mode: ModeLive})
	if err != nil {
		t.Fatalf("Runs: %v", err)
	}
	var out []RunRecord
	for _, r := range runs {
		if r.TriggerType == "datetime" {
			out = append(out, r)
		}
	}
	return out
}

// fillQueue starts a run that waits in its tool and queues a second one, so the flow's
// queue (MaxQueuedPerFlow 1) is full; it returns both.
func (fx *audit17Fixture) fillQueue(t *testing.T) (running, queued StartResult) {
	t.Helper()
	ctx := context.Background()
	running, err := fx.s.RunNow(ctx, fx.pub.ID)
	if err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	fx.tools.waitCalled(t, running.RunID)
	if queued, err = fx.s.RunNow(ctx, fx.pub.ID); err != nil || queued.Status != StartQueued {
		t.Fatalf("second RunNow = %+v, %v", queued, err)
	}
	if _, err := fx.s.RunNow(ctx, fx.pub.ID); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("third RunNow = %v, want ErrQueueFull (test setup)", err)
	}
	return running, queued
}

// The reproduction: the flow's queue is full when the occurrence is due. No run is
// created, so the occurrence must stay stored, and the retry after the backoff, once the
// queue has room, starts exactly one run for it and only then consumes it.
func TestAudit17FullQueueKeepsTheOccurrence(t *testing.T) {
	fx := newAudit17Fixture(t, ConcurrencyQueue)
	ctx := context.Background()
	running, queued := fx.fillQueue(t)

	fx.clock.Advance(time.Hour) // 08:00: the occurrence is due
	if err := fx.s.timers.processDue(ctx, false); err != nil {
		t.Fatalf("processDue: %v", err)
	}
	if got := fx.timerRuns(t); len(got) != 0 {
		t.Fatalf("the full queue admitted a timer run: %+v", got)
	}
	fx.assertOccurrence(t, fx.due)

	// The queue drains; the retry after the backoff starts the run.
	fx.tools.open()
	fx.bridge.waitInfo(t, running.RunID)
	fx.bridge.waitInfo(t, queued.RunID)
	fx.clock.Advance(fx.s.timers.retryDelay)
	if err := fx.s.timers.processDue(ctx, false); err != nil {
		t.Fatalf("processDue: %v", err)
	}
	got := fx.timerRuns(t)
	if len(got) != 1 {
		t.Fatalf("timer runs after the retry = %+v, want one", got)
	}
	fx.assertNoTimers(t)
	detail, err := fx.s.Run(ctx, got[0].ID, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if at := detail.Run.TriggerData["scheduled_for"]; at != "2026-10-03T08:00:00Z" {
		t.Fatalf("the retried run is scheduled for %v, want the original occurrence", at)
	}
	fx.bridge.waitInfo(t, got[0].ID)
	if warned := fx.logs.messages(slog.LevelWarn, "a flow timer was not handled"); len(warned) != 1 {
		t.Fatalf("the failed start must be logged once at Warn, got %v", warned)
	}
}

// A store error while the run is recorded is a failed start too: the occurrence stays.
func TestAudit17StoreErrorKeepsTheOccurrence(t *testing.T) {
	fx := newAudit17Fixture(t, ConcurrencyQueue)
	ctx := context.Background()
	exec := func(stmt string) {
		t.Helper()
		if _, err := fx.s.store.db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	exec(`CREATE TRIGGER audit17_block_runs BEFORE INSERT ON flow_runs BEGIN SELECT RAISE(ABORT, 'insert blocked'); END`)
	fx.clock.Advance(time.Hour)
	if err := fx.s.timers.processDue(ctx, false); err != nil {
		t.Fatalf("processDue: %v", err)
	}
	fx.assertOccurrence(t, fx.due)
	if warned := fx.logs.messages(slog.LevelWarn, "a flow timer was not handled"); len(warned) != 1 {
		t.Fatalf("the failed start must be logged at Warn, got %v", warned)
	}

	exec(`DROP TRIGGER audit17_block_runs`)
	fx.clock.Advance(fx.s.timers.retryDelay)
	if err := fx.s.timers.processDue(ctx, false); err != nil {
		t.Fatalf("processDue: %v", err)
	}
	fx.assertNoTimers(t)
	got := fx.timerRuns(t)
	if len(got) != 1 {
		t.Fatalf("timer runs after the store recovered = %+v, want one", got)
	}
	fx.tools.open()
	fx.bridge.waitInfo(t, got[0].ID)
}

// Deliberate skips consume the occurrence and say so in the log: the Skip concurrency
// policy, a flow that is switched off and a flow without a published revision. None of
// them is retried.
func TestAudit17DeliberateSkipsConsumeTheOccurrenceAndLog(t *testing.T) {
	ctx := context.Background()
	t.Run("skip policy", func(t *testing.T) {
		fx := newAudit17Fixture(t, ConcurrencySkip)
		running, err := fx.s.RunNow(ctx, fx.pub.ID)
		if err != nil {
			t.Fatal(err)
		}
		fx.tools.waitCalled(t, running.RunID)
		fx.clock.Advance(time.Hour)
		if err := fx.s.timers.processDue(ctx, false); err != nil {
			t.Fatal(err)
		}
		fx.assertNoTimers(t)
		if got := fx.timerRuns(t); len(got) != 0 {
			t.Fatalf("the skip policy admitted a timer run: %+v", got)
		}
		if info := fx.logs.messages(slog.LevelInfo, "concurrency: skip"); len(info) != 1 || !strings.HasPrefix(info[0], "INFO ") {
			t.Fatalf("the skipped trigger must be logged at Info, got %v", info)
		}
		audit17NoRetry(t, fx)
	})
	t.Run("switched off", func(t *testing.T) {
		fx := newAudit17Fixture(t, ConcurrencyQueue)
		if err := fx.s.SetEnabled(ctx, fx.pub.ID, false); err != nil {
			t.Fatal(err)
		}
		// Switching off clears the timers; one can still fire when the switch and the
		// timer cross, which is what this row stands for.
		if err := fx.s.store.ReplaceTimers(ctx, fx.pub.ID, []TimerRecord{{FlowID: fx.pub.ID, NodeID: fx.when, FireAt: fx.due}}); err != nil {
			t.Fatal(err)
		}
		fx.clock.Advance(time.Hour)
		if err := fx.s.timers.processDue(ctx, false); err != nil {
			t.Fatal(err)
		}
		fx.assertNoTimers(t)
		if got := fx.timerRuns(t); len(got) != 0 {
			t.Fatalf("a switched-off flow got a timer run: %+v", got)
		}
		if info := fx.logs.messages(slog.LevelInfo, "switched off"); len(info) != 1 || !strings.HasPrefix(info[0], "INFO ") {
			t.Fatalf("the switched-off flow must be logged at Info, got %v", info)
		}
		audit17NoRetry(t, fx)
	})
	t.Run("not published", func(t *testing.T) {
		fx := newAudit17Fixture(t, ConcurrencyQueue)
		draft, err := fx.s.CreateFlow(ctx, CreateRequest{Name: "Entwurf"})
		if err != nil {
			t.Fatal(err)
		}
		if err := fx.s.store.ReplaceTimers(ctx, fx.pub.ID, nil); err != nil {
			t.Fatal(err)
		}
		if err := fx.s.store.ReplaceTimers(ctx, draft.ID, []TimerRecord{{FlowID: draft.ID, NodeID: fx.when, FireAt: fx.due}}); err != nil {
			t.Fatal(err)
		}
		fx.clock.Advance(time.Hour)
		if err := fx.s.timers.processDue(ctx, false); err != nil {
			t.Fatal(err)
		}
		fx.assertNoTimers(t)
		if warned := fx.logs.messages(slog.LevelWarn, "without a published revision"); len(warned) != 1 {
			t.Fatalf("the unpublished flow must be logged at Warn, got %v", warned)
		}
		audit17NoRetry(t, fx)
	})
}

// Review M3: a flow that cannot be read is a failed start. The occurrence stays, and the
// failure is logged once, by the timer service with its retry, not also by the callback.
func TestAudit17UnreadableFlowIsRetriedAndLoggedOnce(t *testing.T) {
	fx := newAudit17Fixture(t, ConcurrencyQueue)
	ctx := context.Background()
	if _, err := fx.s.store.db.ExecContext(ctx, `UPDATE flows SET draft_json = 'x' WHERE id = ?`, fx.pub.ID); err != nil {
		t.Fatal(err)
	}
	fx.clock.Advance(time.Hour)
	if err := fx.s.timers.processDue(ctx, false); err != nil {
		t.Fatalf("processDue: %v", err)
	}
	fx.assertOccurrence(t, fx.due)
	if warned := fx.logs.messages(slog.LevelWarn, ""); len(warned) != 1 || !strings.Contains(warned[0], "a flow timer was not handled") {
		t.Fatalf("warnings = %v, want only the timer service's retry line", warned)
	}
}

// audit17ReasonOf returns the "reason" attribute of the records whose message contains part.
func audit17ReasonOf(l *svcLogs, part string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, r := range l.records {
		if !strings.Contains(r.Message, part) {
			continue
		}
		reason := ""
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "reason" {
				reason = a.Value.String()
			}
			return true
		})
		out = append(out, r.Level.String()+" "+reason)
	}
	return out
}

// Review M1: only a flow that is switched off is logged as switched off (Info). When its
// switch cannot be read, because Mission Control is not available or holds no mission for
// the flow, the skip is a Warn naming that reason. The occurrence is consumed either way.
func TestAudit17UnreadableSwitchIsAWarnWithItsReason(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name  string
		setup func(b *ff1ReconBridge, missionID string)
		want  []string // log lines: level and reason
		about string   // part of the expected message
	}{
		{"switched off", func(b *ff1ReconBridge, m string) { _ = b.fakeBridge.SetFlowMissionEnabled(m, false) },
			[]string{"INFO "}, "is switched off"},
		{"no Mission Control", func(b *ff1ReconBridge, m string) { _ = b.fakeBridge.SetFlowMissionEnabled(m, false); b.none = true },
			[]string{"WARN " + ErrMissionControlUnavailable.Error()}, "cannot tell whether its flow is switched on"},
		{"mission missing", func(b *ff1ReconBridge, m string) { _ = b.fakeBridge.DeleteFlowMission(m) },
			[]string{"WARN " + ErrFlowMissionMissing.Error()}, "cannot tell whether its flow is switched on"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
			bridge := &ff1ReconBridge{svcBridge: newSvcBridge()}
			logs := &svcLogs{}
			s := NewService(openTestStore(t), catalogRegistry(t, fullEnv()), &Services{Tools: &fakeTools{}, Clock: clock, Location: time.UTC},
				bridge, ServiceConfig{}, slog.New(logs))
			t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
			b := newFlow("Schalter")
			when := b.node("when", TypeTriggerDateTime, map[string]any{"at": "2026-10-03 08:00"})
			done := b.node("done", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "v", "value": "done"}}})
			b.edge(when, PortOut, done)
			pub := svcRunPublish(t, s, b.build())
			if list, err := s.store.ListTimers(ctx); err != nil || len(list) != 1 {
				t.Fatalf("armed timers = %+v, %v (test setup)", list, err)
			}
			tc.setup(bridge, pub.MissionID)
			clock.Advance(time.Hour)
			if err := s.timers.processDue(ctx, false); err != nil {
				t.Fatalf("processDue: %v", err)
			}
			if list, err := s.store.ListTimers(ctx); err != nil || len(list) != 0 {
				t.Fatalf("timers = %+v, %v; the occurrence must be consumed", list, err)
			}
			if got := audit17ReasonOf(logs, tc.about); len(got) != 1 || got[0] != tc.want[0] {
				t.Fatalf("log lines %q = %q, want %q", tc.about, got, tc.want)
			}
			if runs, err := s.Runs(ctx, pub.ID, RunFilter{}); err != nil || len(runs) != 0 {
				t.Fatalf("runs = %+v, %v; none may start", runs, err)
			}
			if len(s.timers.retrying) != 0 {
				t.Fatalf("a skip must not be retried: %v", s.timers.retrying)
			}
		})
	}
}

// audit17NoRetry checks that the timer service keeps no retry state and logged no failed start.
func audit17NoRetry(t *testing.T, fx *audit17Fixture) {
	t.Helper()
	if len(fx.s.timers.retrying) != 0 {
		t.Fatalf("a deliberate skip must not be retried, retry state %v", fx.s.timers.retrying)
	}
	if warned := fx.logs.messages(slog.LevelWarn, "a flow timer was not handled"); len(warned) != 0 {
		t.Fatalf("a deliberate skip was logged as a failed start: %v", warned)
	}
}
