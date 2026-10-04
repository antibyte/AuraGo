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

// timerRetryTestDelay is the retry delay of the services built by hardTimers. It
// differs from the production default so the tests can tell which delay was used.
const timerRetryTestDelay = 7 * time.Second

// syncBuffer is a log sink that can be written from the timer goroutine and read
// from the test.
type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// countingClock records every After call. fakeClock only keeps the waiters that
// are still pending, so the zero-length waits of a spinning loop leave no trace there.
type countingClock struct {
	*fakeClock
	callsMu sync.Mutex
	waits   []time.Duration
}

func (c *countingClock) After(d time.Duration) <-chan time.Time {
	c.callsMu.Lock()
	c.waits = append(c.waits, d)
	c.callsMu.Unlock()
	return c.fakeClock.After(d)
}

func (c *countingClock) afterCalls() []time.Duration {
	c.callsMu.Lock()
	defer c.callsMu.Unlock()
	return append([]time.Duration(nil), c.waits...)
}

func (c *countingClock) assertWaits(t *testing.T, want ...time.Duration) {
	t.Helper()
	got := c.afterCalls()
	if len(got) != len(want) {
		t.Fatalf("After calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("After calls = %v, want %v", got, want)
		}
	}
}

// assertWaitsStay checks the After calls so far and then, after a short real pause,
// that the loop made no further ones. A pause cannot prove a negative, but a
// spinning loop makes a call every few microseconds, so it cannot pass it.
func (c *countingClock) assertWaitsStay(t *testing.T, want ...time.Duration) {
	t.Helper()
	c.assertWaits(t, want...)
	time.Sleep(20 * time.Millisecond)
	c.assertWaits(t, want...)
}

// timerCalls records the callbacks of a service as "fire:<node>" / "missed:<node>"
// and panics in the ones listed in panicIn.
type timerCalls struct {
	mu      sync.Mutex
	log     []string
	panicIn map[string]bool
}

func (c *timerCalls) fire(_, nodeID string, _ time.Time)   { c.record("fire:" + nodeID) }
func (c *timerCalls) missed(_, nodeID string, _ time.Time) { c.record("missed:" + nodeID) }

func (c *timerCalls) record(call string) {
	c.mu.Lock()
	c.log = append(c.log, call)
	boom := c.panicIn[call]
	c.mu.Unlock()
	if boom {
		panic("boom in " + call)
	}
}

func (c *timerCalls) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.log...)
}

func (c *timerCalls) count(call string) int {
	n := 0
	for _, got := range c.all() {
		if got == call {
			n++
		}
	}
	return n
}

func (c *timerCalls) assertCalls(t *testing.T, want ...string) {
	t.Helper()
	if got := strings.Join(c.all(), ","); got != strings.Join(want, ",") {
		t.Fatalf("callbacks = %s, want %s", got, strings.Join(want, ","))
	}
}

// hardTimers is the store, clock and flow of a timer test that builds its own service.
type hardTimers struct {
	store *Store
	clock *countingClock
	flow  *Flow
}

func newHardTimers(t *testing.T, now time.Time) *hardTimers {
	t.Helper()
	h := &hardTimers{store: openTestStore(t), clock: &countingClock{fakeClock: newFakeClock(now)}}
	h.flow = createStoredFlow(t, h.store, "flow_aaaaaaaada")
	return h
}

// service builds a service with the short test retry delay that is stopped at the end of the test.
func (h *hardTimers) service(t *testing.T, fire, missed TimerFireFunc, logger *slog.Logger) *TimerService {
	t.Helper()
	if logger == nil {
		logger = discardLogger()
	}
	svc := NewTimerService(h.store, h.clock, fire, missed, logger)
	svc.retryDelay = timerRetryTestDelay
	t.Cleanup(svc.Stop)
	return svc
}

func (h *hardTimers) arm(t *testing.T, timers ...TimerRecord) {
	t.Helper()
	if err := h.store.ReplaceTimers(context.Background(), h.flow.ID, timers); err != nil {
		t.Fatalf("ReplaceTimers: %v", err)
	}
}

// timers lists the stored timers. Call it only while the store is healthy.
func (h *hardTimers) timers(t *testing.T) []TimerRecord {
	t.Helper()
	list, err := h.store.ListTimers(context.Background())
	if err != nil {
		t.Fatalf("ListTimers: %v", err)
	}
	return list
}

// exec runs raw SQL on the store's database. The tests use it to inject store faults.
func (h *hardTimers) exec(t *testing.T, stmt string) {
	t.Helper()
	if _, err := h.store.db.ExecContext(context.Background(), stmt); err != nil {
		t.Fatalf("%s: %v", stmt, err)
	}
}

// Fault injection: deletes fail while reads and inserts keep working.
func (h *hardTimers) blockTimerDeletes(t *testing.T) {
	t.Helper()
	h.exec(t, `CREATE TRIGGER block_timer_delete BEFORE DELETE ON flow_timers BEGIN SELECT RAISE(ABORT, 'delete blocked'); END`)
}

func (h *hardTimers) unblockTimerDeletes(t *testing.T) {
	t.Helper()
	h.exec(t, `DROP TRIGGER block_timer_delete`)
}

// Fault injection: reads fail (the table is gone) until restoreTimerTable.
func (h *hardTimers) hideTimerTable(t *testing.T) {
	t.Helper()
	h.exec(t, `ALTER TABLE flow_timers RENAME TO flow_timers_off`)
}

func (h *hardTimers) restoreTimerTable(t *testing.T) {
	t.Helper()
	h.exec(t, `ALTER TABLE flow_timers_off RENAME TO flow_timers`)
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestTimerServiceFirePanicIsRecoveredAndTimersAreSettled(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	calls := &timerCalls{panicIn: map[string]bool{"fire:n_aaaaaaab": true}}
	logs := &syncBuffer{}
	svc := h.service(t, calls.fire, calls.missed, slog.New(slog.NewTextHandler(logs, nil)))
	due := storeNow.Add(time.Hour)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: due}, TimerRecord{NodeID: "n_aaaaaaac", FireAt: due})
	if err := svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	h.clock.WaitForWaiters(t, 1)
	h.clock.Advance(time.Hour)

	waitUntil(t, "both timers to be removed", func() bool { return len(h.timers(t)) == 0 })
	// The timer after the panicking one still fired in the same pass, and the panicking one was not repeated.
	calls.assertCalls(t, "fire:n_aaaaaaab", "fire:n_aaaaaaac")
	out := logs.String()
	for _, want := range []string{"level=ERROR", "n_aaaaaaab", "boom in fire:n_aaaaaaab"} {
		if !strings.Contains(out, want) {
			t.Fatalf("the recovered panic must be logged at Error level with %q, log:\n%s", want, out)
		}
	}

	// The loop survived: a timer armed afterwards still fires.
	if err := svc.Replace(ctx, h.flow.ID, []TimerRecord{{NodeID: "n_aaaaaaad", FireAt: storeNow.Add(70 * time.Minute)}}); err != nil {
		t.Fatal(err)
	}
	h.clock.WaitForWaiters(t, 1)
	h.clock.Advance(10 * time.Minute)
	waitUntil(t, "a timer armed after the panic to fire", func() bool { return calls.count("fire:n_aaaaaaad") == 1 })
	calls.assertCalls(t, "fire:n_aaaaaaab", "fire:n_aaaaaaac", "fire:n_aaaaaaad")
}

func TestTimerServiceFirePanicStillMovesYearlyTimerForward(t *testing.T) {
	h := newHardTimers(t, storeNow)
	calls := &timerCalls{panicIn: map[string]bool{"fire:n_aaaaaaab": true}}
	svc := h.service(t, calls.fire, calls.missed, nil)
	at := storeNow.Add(time.Minute)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: at, Repeat: RepeatYearly})
	if err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.clock.WaitForWaiters(t, 1)
	h.clock.Advance(time.Minute)

	next := at.AddDate(1, 0, 0)
	waitUntil(t, "the yearly timer to move to next year", func() bool {
		list := h.timers(t)
		return len(list) == 1 && list[0].FireAt.Equal(next)
	})
	// The loop now waits a year for it instead of firing it again.
	h.clock.WaitForWaiters(t, 1)
	waits := h.clock.afterCalls()
	if last := waits[len(waits)-1]; last < 364*24*time.Hour {
		t.Fatalf("the loop waits %v for the moved timer, want about a year", last)
	}
	calls.assertCalls(t, "fire:n_aaaaaaab")
}

func TestTimerServiceMissedPanicDoesNotFailStart(t *testing.T) {
	h := newHardTimers(t, storeNow)
	calls := &timerCalls{panicIn: map[string]bool{"missed:n_aaaaaaab": true}}
	svc := h.service(t, calls.fire, calls.missed, nil)
	h.arm(t,
		TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow.Add(-time.Hour)},
		TimerRecord{NodeID: "n_aaaaaaac", FireAt: storeNow.Add(-5 * time.Minute)})
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("a panicking missed callback must not fail Start: %v", err)
	}
	calls.assertCalls(t, "missed:n_aaaaaaab", "fire:n_aaaaaaac")
	if list := h.timers(t); len(list) != 0 {
		t.Fatalf("both timers must be removed, got %+v", list)
	}
}

func TestTimerServiceWithoutCallbacksStillSettlesTimers(t *testing.T) {
	h := newHardTimers(t, storeNow)
	svc := h.service(t, nil, nil, nil)
	h.arm(t,
		TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow.Add(-time.Hour)},
		TimerRecord{NodeID: "n_aaaaaaac", FireAt: storeNow.Add(-time.Minute)})
	if err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if list := h.timers(t); len(list) != 0 {
		t.Fatalf("timers without callbacks must still be removed, got %+v", list)
	}
}

func TestTimerServiceStoreWriteErrorBacksOffAndDoesNotRefire(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	calls := &timerCalls{}
	svc := h.service(t, calls.fire, calls.missed, nil)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow.Add(time.Hour)})
	if err := svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	h.clock.WaitForWaiters(t, 1)
	h.blockTimerDeletes(t)
	h.clock.Advance(time.Hour)

	waitUntil(t, "the timer to fire", func() bool { return calls.count("fire:n_aaaaaaab") == 1 })
	// The delete failed, so the due timer is still stored. The loop must wait the
	// retry delay instead of firing it again at once.
	h.clock.WaitForWaiters(t, 1)
	h.clock.assertWaitsStay(t, time.Hour, timerRetryTestDelay)
	calls.assertCalls(t, "fire:n_aaaaaaab")

	// Once the store works again, the retry only repeats the delete.
	h.unblockTimerDeletes(t)
	h.clock.Advance(timerRetryTestDelay)
	waitUntil(t, "the timer to be removed", func() bool { return len(h.timers(t)) == 0 })
	calls.assertCalls(t, "fire:n_aaaaaaab")
}

func TestTimerServiceStoreReadErrorBacksOffAndRecovers(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	calls := &timerCalls{}
	svc := h.service(t, calls.fire, calls.missed, nil)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow.Add(time.Hour)})
	if err := svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	h.clock.WaitForWaiters(t, 1)
	h.hideTimerTable(t)
	h.clock.Advance(time.Hour)

	// The timers cannot be read, so the loop cannot plan. It must look again after
	// the retry delay: neither spin nor wait for a Replace that may never come.
	h.clock.WaitForWaiters(t, 1)
	h.clock.assertWaitsStay(t, time.Hour, timerRetryTestDelay)

	h.restoreTimerTable(t)
	h.clock.Advance(timerRetryTestDelay)
	waitUntil(t, "the timer to fire after the store recovered", func() bool { return calls.count("fire:n_aaaaaaab") == 1 })
	waitUntil(t, "the timer to be removed", func() bool { return len(h.timers(t)) == 0 })
	calls.assertCalls(t, "fire:n_aaaaaaab")
}

func TestTimerServiceFailedStoreUpdateDoesNotRepeatCallbacks(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	calls := &timerCalls{}
	svc := h.service(t, calls.fire, calls.missed, nil)
	at := storeNow.Add(-time.Minute)
	h.arm(t,
		TimerRecord{NodeID: "n_aaaaaaab", FireAt: at},
		TimerRecord{NodeID: "n_aaaaaaac", FireAt: at, Repeat: RepeatYearly})
	h.blockTimerDeletes(t)

	for i := 1; i <= 3; i++ {
		if err := svc.processDue(ctx, false); err == nil {
			t.Fatalf("pass %d: processDue must report the failed delete", i)
		}
	}
	calls.assertCalls(t, "fire:n_aaaaaaab")

	h.unblockTimerDeletes(t)
	if err := svc.processDue(ctx, false); err != nil {
		t.Fatalf("processDue after recovery: %v", err)
	}
	calls.assertCalls(t, "fire:n_aaaaaaab", "fire:n_aaaaaaac")
	list := h.timers(t)
	if len(list) != 1 || list[0].NodeID != "n_aaaaaaac" || !list[0].FireAt.Equal(at.AddDate(1, 0, 0)) {
		t.Fatalf("timers after recovery = %+v, want only the yearly one a year later", list)
	}
	if len(svc.unsettled) != 0 {
		t.Fatalf("settled timers must not be remembered, have %v", svc.unsettled)
	}
}

func TestTimerServiceForgetsUnsettledTimerThatWasRemoved(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	calls := &timerCalls{}
	svc := h.service(t, calls.fire, calls.missed, nil)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow.Add(-time.Minute)})
	h.blockTimerDeletes(t)
	if err := svc.processDue(ctx, false); err == nil {
		t.Fatal("processDue must report the failed delete")
	}
	if len(svc.unsettled) != 1 {
		t.Fatalf("the fired but unsettled timer must be remembered, have %v", svc.unsettled)
	}

	// The flow is republished before the store recovers: the timer is gone.
	h.unblockTimerDeletes(t)
	h.arm(t)
	if err := svc.processDue(ctx, false); err != nil {
		t.Fatal(err)
	}
	if len(svc.unsettled) != 0 {
		t.Fatalf("a removed timer must be forgotten, have %v", svc.unsettled)
	}
}

func TestTimerServiceYearlyTimerOfDeletedFlowIsNotAnError(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	calls := &timerCalls{}
	// The flow is deleted while its callback runs; the cascade takes the timer along,
	// so moving it forward finds no flow. That must not count as a store failure.
	svc := h.service(t, func(flowID, nodeID string, at time.Time) {
		calls.fire(flowID, nodeID, at)
		if err := h.store.DeleteFlow(ctx, flowID); err != nil {
			t.Errorf("DeleteFlow: %v", err)
		}
	}, nil, nil)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow.Add(-time.Minute), Repeat: RepeatYearly})
	if err := svc.processDue(ctx, false); err != nil {
		t.Fatalf("processDue: %v", err)
	}
	calls.assertCalls(t, "fire:n_aaaaaaab")
	if list := h.timers(t); len(list) != 0 {
		t.Fatalf("the deleted flow's timer must be gone, got %+v", list)
	}
	if !errors.Is(h.store.UpsertTimer(ctx, TimerRecord{FlowID: h.flow.ID, NodeID: "n_aaaaaaab", FireAt: storeNow}), ErrNotFound) {
		t.Fatal("test setup: the flow should be deleted")
	}
}
