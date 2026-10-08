package flows

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// Audit 2026-10-08, finding 1.7, on the TimerService itself: an occurrence whose fire
// callback did not handle it stays stored and is tried again after a bounded,
// per-occurrence backoff.

// audit17Handler is a TimerHandleFunc that records its calls like timerCalls and fails
// for the nodes in failing.
type audit17Handler struct {
	timerCalls
	failMu  sync.Mutex
	failing map[string]bool
	when    []time.Time // scheduledFor of every call, in order
}

func (h *audit17Handler) handle(_, nodeID string, scheduledFor time.Time) error {
	h.record("fire:" + nodeID)
	h.failMu.Lock()
	defer h.failMu.Unlock()
	h.when = append(h.when, scheduledFor)
	if h.failing[nodeID] {
		return errors.New("the flow's run queue is full")
	}
	return nil
}

func (h *audit17Handler) setFailing(nodeID string, fail bool) {
	h.failMu.Lock()
	defer h.failMu.Unlock()
	if h.failing == nil {
		h.failing = map[string]bool{}
	}
	h.failing[nodeID] = fail
}

// audit17Service builds a handling timer service with the test retry delay (7 s) and
// a 20 s cap, stopped at the end of the test.
func audit17Service(t *testing.T, h *hardTimers, fire TimerHandleFunc, logger *slog.Logger) *TimerService {
	t.Helper()
	if logger == nil {
		logger = discardLogger()
	}
	svc := newTimerService(h.store, h.clock, fire, nil, logger)
	svc.retryDelay = timerRetryTestDelay
	svc.maxRetryDelay = 20 * time.Second
	t.Cleanup(svc.Stop)
	return svc
}

// The backoff of an unhandled occurrence doubles from the retry delay up to the cap, the
// loop never spins on it, and a timer due in the meantime still fires on time.
func TestAudit17UnhandledOccurrenceBacksOffWithoutBlockingOthers(t *testing.T) {
	h := newHardTimers(t, storeNow)
	handler := &audit17Handler{}
	handler.setFailing("n_aaaaaaab", true)
	logs := &syncBuffer{}
	svc := audit17Service(t, h, handler.handle, slog.New(slog.NewTextHandler(logs, nil)))
	due := storeNow.Add(time.Hour)
	h.arm(t,
		TimerRecord{NodeID: "n_aaaaaaab", FireAt: due},
		TimerRecord{NodeID: "n_aaaaaaac", FireAt: due.Add(10 * time.Second)})
	if err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	step := func(d time.Duration) {
		t.Helper()
		h.clock.WaitForWaiters(t, 1)
		h.clock.Advance(d)
	}
	step(time.Hour)             // b fails: retry after 7 s, before c's time
	step(7 * time.Second)       // b fails again: retry after 14 s, so c (3 s away) is next
	step(3 * time.Second)       // c fires on time while b waits
	step(11 * time.Second)      // b fails a third time: 28 s, capped at 20 s
	h.clock.WaitForWaiters(t, 1) // the 20 s wait
	h.clock.assertWaitsStay(t, time.Hour, 7*time.Second, 3*time.Second, 11*time.Second, 20*time.Second)
	handler.assertCalls(t, "fire:n_aaaaaaab", "fire:n_aaaaaaab", "fire:n_aaaaaaac", "fire:n_aaaaaaab")
	if left := timerNodes(h.timers(t)); left != "n_aaaaaaab" {
		t.Fatalf("stored timers = %s, want only the unhandled occurrence", left)
	}
	out := logs.String()
	if n := strings.Count(out, "a flow timer was not handled"); n != 3 || !strings.Contains(out, "level=WARN") ||
		!strings.Contains(out, "attempt=3") || !strings.Contains(out, "retry_in=20s") {
		t.Fatalf("every failed attempt must be logged at Warn with its attempt and wait (%d lines):\n%s", n, out)
	}

	// The cause is gone: the next retry handles it, with its own time, and consumes it.
	handler.setFailing("n_aaaaaaab", false)
	step(20 * time.Second)
	waitUntil(t, "the occurrence to be consumed", func() bool { return len(h.timers(t)) == 0 })
	handler.assertCalls(t, "fire:n_aaaaaaab", "fire:n_aaaaaaab", "fire:n_aaaaaaac", "fire:n_aaaaaaab", "fire:n_aaaaaaab")
	handler.failMu.Lock()
	last := handler.when[len(handler.when)-1]
	handler.failMu.Unlock()
	if !last.Equal(due) {
		t.Fatalf("the retry was scheduled for %v, want the occurrence's own time %v", last, due)
	}
}

// A yearly occurrence that is still retried when its next date has passed fires once,
// when a retry succeeds, and then moves to its first date after that moment: the missed
// year is not caught up.
func TestAudit17YearlyOccurrenceRetriedPastItsNextDateFiresOnce(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	handler := &audit17Handler{}
	handler.setFailing("n_aaaaaaab", true)
	svc := audit17Service(t, h, handler.handle, nil)
	at := storeNow.Add(-time.Minute)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: at, Repeat: RepeatYearly})
	if err := svc.processDue(ctx, false); err != nil {
		t.Fatal(err)
	}
	if list := h.timers(t); len(list) != 1 || !list[0].FireAt.Equal(at) {
		t.Fatalf("the unhandled yearly occurrence must stay where it is, got %+v", list)
	}

	h.clock.Advance(400 * 24 * time.Hour) // past the next yearly date
	handler.setFailing("n_aaaaaaab", false)
	if err := svc.processDue(ctx, false); err != nil {
		t.Fatal(err)
	}
	handler.assertCalls(t, "fire:n_aaaaaaab", "fire:n_aaaaaaab")
	handler.failMu.Lock()
	when := append([]time.Time(nil), handler.when...)
	handler.failMu.Unlock()
	for _, got := range when {
		if !got.Equal(at) {
			t.Fatalf("calls were scheduled for %v, want the occurrence's own time %v every time", when, at)
		}
	}
	want := at.AddDate(2, 0, 0) // the first yearly date after now; the one a year after at was missed
	if list := h.timers(t); len(list) != 1 || !list[0].FireAt.Equal(want) {
		t.Fatalf("the yearly timer must move to %v, got %+v", want, list)
	}
	if err := svc.processDue(ctx, false); err != nil {
		t.Fatal(err)
	}
	handler.assertCalls(t, "fire:n_aaaaaaab", "fire:n_aaaaaaab")
	if len(svc.retrying) != 0 || len(svc.unsettled) != 0 {
		t.Fatalf("retry state left after the occurrence was handled: %v %v", svc.retrying, svc.unsettled)
	}
}

// The retry state lives in memory. After a restart the start-up pass treats a retried
// occurrence like any other: beyond MissedTimerGrace it is reported as missed and consumed.
func TestAudit17RetriedOccurrenceAfterARestartFollowsTheGraceRule(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	handler := &audit17Handler{}
	handler.setFailing("n_aaaaaaab", true)
	before := audit17Service(t, h, handler.handle, nil)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow.Add(-time.Minute)})
	if err := before.processDue(ctx, false); err != nil {
		t.Fatal(err)
	}
	before.Stop()

	h.clock.Advance(time.Hour)
	calls := &timerCalls{}
	after := h.service(t, calls.fire, calls.missed, nil)
	if err := after.Start(ctx); err != nil {
		t.Fatal(err)
	}
	calls.assertCalls(t, "missed:n_aaaaaaab")
	if list := h.timers(t); len(list) != 0 {
		t.Fatalf("the missed occurrence must be consumed, got %+v", list)
	}
}

// A timer that is gone (the flow was switched off, republished or deleted) takes its
// retry state with it.
func TestAudit17RetryStateOfARemovedTimerIsForgotten(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	handler := &audit17Handler{}
	handler.setFailing("n_aaaaaaab", true)
	svc := audit17Service(t, h, handler.handle, nil)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow.Add(-time.Minute)})
	if err := svc.processDue(ctx, false); err != nil {
		t.Fatal(err)
	}
	if len(svc.retrying) != 1 {
		t.Fatalf("the unhandled occurrence must have a retry state, have %v", svc.retrying)
	}
	h.arm(t)
	if err := svc.processDue(ctx, false); err != nil {
		t.Fatal(err)
	}
	if len(svc.retrying) != 0 {
		t.Fatalf("a removed timer's retry state must be forgotten, have %v", svc.retrying)
	}
}
