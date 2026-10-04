package flows

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

// MissedTimerGrace is how late a timer may be at start-up and still fire.
const MissedTimerGrace = 10 * time.Minute

// timerRetryDelay is how long the service leaves the store alone after a failed
// attempt to read or settle timers. Without it a persistent database error would
// turn the loop into a busy loop: the due timer stays stored, so it is due again at once.
const timerRetryDelay = 30 * time.Second

// errTimerServiceStopped is returned by Start after Stop.
var errTimerServiceStopped = errors.New("the flow timer service was stopped")

// TimerFireFunc is called when a Date/Time trigger is due, or, for the missed
// callback, when it became due while AuraGo was off for longer than MissedTimerGrace.
//
// Callbacks run synchronously, one at a time: on the timer goroutine, except those of
// the start-up pass, which run on the goroutine that called Start. While one runs, no
// other timer fires, and Stop waits for the one on the timer goroutine, so a callback
// must return quickly: starting a run is fine (Runner.Start writes one database row),
// anything slower belongs in a goroutine that the callback starts. A callback must not
// call Stop, which would wait for the callback itself.
//
// A panic in a callback is recovered and logged at Error level. The timer then counts
// as handled like after a normal return: a one-off timer is deleted and a yearly one
// moves to its next date, so a crashing callback is never repeated in a loop. The
// recover covers the callback's own goroutine only, not goroutines it starts.
type TimerFireFunc func(flowID, nodeID string, scheduledFor time.Time)

// timerKey identifies one due occurrence of a timer.
type timerKey struct {
	flowID, nodeID string
	fireAt         int64 // UnixMicro: the store keeps microseconds
}

func keyOfTimer(tm TimerRecord) timerKey {
	return timerKey{flowID: tm.FlowID, nodeID: tm.NodeID, fireAt: tm.FireAt.UnixMicro()}
}

// TimerService fires persisted Date/Time triggers. One goroutine waits for the
// earliest timer; Replace wakes it up to re-plan.
//
// Delivery is at least once: a timer is deleted (or moved) only after its callback
// returned, so a crash in between fires it again at the next start (or reports it as
// missed). Within one process a callback is not repeated, even when the store fails
// in between and the timer has to be settled by a later attempt.
type TimerService struct {
	store  *Store
	clock  Clock
	fire   TimerFireFunc
	missed TimerFireFunc
	logger *slog.Logger
	grace  time.Duration
	// retryDelay is the pause after a store error, see timerRetryDelay. Tests shorten it.
	retryDelay time.Duration

	reload   chan struct{}
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once

	startMu     sync.Mutex // serializes Start
	mu          sync.Mutex // guards the two flags below
	loopStarted bool
	stopped     bool

	// unsettled holds the timers whose callback ran but whose delete or move failed.
	// It is touched only by processDue, which never runs concurrently with itself:
	// first from Start, and once Start succeeded only from the loop.
	unsettled map[timerKey]struct{}
}

// NewTimerService creates a timer service. missed may be nil; a timer that is too late
// at start-up is then just removed (or moved forward, when it repeats).
func NewTimerService(store *Store, clock Clock, fire, missed TimerFireFunc, logger *slog.Logger) *TimerService {
	if clock == nil {
		clock = RealClock()
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &TimerService{
		store: store, clock: clock, fire: fire, missed: missed, logger: logger,
		grace: MissedTimerGrace, retryDelay: timerRetryDelay,
		reload: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}),
		unsettled: make(map[timerKey]struct{}),
	}
}

// Start handles timers that became due while AuraGo was off, then starts waiting.
// ctx bounds that first pass only.
//
// Start is idempotent: once it succeeded, further calls return nil and do nothing, so
// there is never a second loop. Concurrent calls wait for each other. After a failed
// Start (the store could not be read or updated) no loop runs and Start may be called
// again. After Stop, Start returns an error and neither fires timers nor starts a loop;
// a stopped service cannot be restarted, create a new one.
func (t *TimerService) Start(ctx context.Context) error {
	t.startMu.Lock()
	defer t.startMu.Unlock()
	t.mu.Lock()
	stopped, running := t.stopped, t.loopStarted
	t.mu.Unlock()
	if stopped {
		return errTimerServiceStopped
	}
	if running {
		return nil
	}
	err := t.processDue(ctx, true)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped { // Stop was called while the start-up pass ran
		return errTimerServiceStopped
	}
	if err != nil {
		return err
	}
	t.loopStarted = true
	go t.loop()
	return nil
}

// Replace sets the timers of one flow and re-plans.
func (t *TimerService) Replace(ctx context.Context, flowID string, timers []TimerRecord) error {
	if err := t.store.ReplaceTimers(ctx, flowID, timers); err != nil {
		return err
	}
	select {
	case t.reload <- struct{}{}:
	default:
	}
	return nil
}

// Stop ends the wait loop. A pass that is running does not start another timer: Stop
// waits until the loop has finished the callback it is in, then returns. Timers that
// were not processed yet stay stored for the next start (at-least-once delivery).
//
// A callback in the start-up pass runs on Start's goroutine, which Stop does not wait
// for; it stops there before its next timer, and Start then returns an error.
//
// Stop is safe to call more than once, concurrently and before Start. It must not be
// called from a timer callback.
func (t *TimerService) Stop() {
	t.mu.Lock()
	t.stopped = true
	running := t.loopStarted
	t.mu.Unlock()
	t.stopOnce.Do(func() { close(t.stop) })
	if running {
		<-t.done
	}
}

// loop sleeps until the earliest timer is due, settles what is due and re-plans when
// Replace says so.
//
// A store error never makes it spin. notBefore is the earliest time the loop may
// touch the store again after an error. When the timers cannot be read at all the loop
// retries after retryDelay (and so recovers on its own when the store works again,
// instead of waiting for a Replace that may never come). When they can be read but a
// due timer cannot be settled, that timer stays stored and is due, so its wait is
// stretched to notBefore. A Replace lifts that wait: it just wrote to the store and may
// have changed what is due, and each Replace allows only one extra attempt.
func (t *TimerService) loop() {
	defer close(t.done)
	var notBefore time.Time
	for {
		select {
		case <-t.stop: // also after a pass that Stop cut short
			return
		default:
		}
		next, ok, planErr := t.nextFire()
		if planErr != nil {
			notBefore = t.clock.Now().Add(t.retryDelay)
			next, ok = notBefore, true
		}
		var wait <-chan time.Time
		if ok {
			if next.Before(notBefore) {
				next = notBefore
			}
			d := next.Sub(t.clock.Now())
			if d < 0 {
				d = 0
			}
			wait = t.clock.After(d)
		}
		select {
		case <-t.stop:
			return
		case <-t.reload:
			notBefore = time.Time{}
		case <-wait:
			if planErr != nil {
				continue // only the retry delay ran out; read the timers again
			}
			if err := t.processDue(context.Background(), false); err != nil {
				t.logger.Warn("flow timers could not be processed", "error", err)
				notBefore = t.clock.Now().Add(t.retryDelay)
			}
		}
	}
}

// nextFire returns when the earliest timer is due; ok is false when there is none.
// A store error is logged and returned for the loop to back off.
func (t *TimerService) nextFire() (next time.Time, ok bool, err error) {
	timers, err := t.store.ListTimers(context.Background())
	if err != nil {
		t.logger.Warn("flow timers could not be loaded", "error", err)
		return time.Time{}, false, err
	}
	if len(timers) == 0 {
		return time.Time{}, false, nil
	}
	return timers[0].FireAt, true, nil
}

// processDue fires (or, at start-up beyond the grace period, reports as missed)
// every timer that is due, then deletes one-off timers and moves yearly ones forward.
// It stops at the first store error; the timers not settled yet stay stored and the
// next pass handles them, calling a callback only for those that have not run it. It
// also stops, without error, before the next timer once Stop was called: the rest stays
// stored for the next start, so a callback never runs after Stop in the middle of a
// shutdown.
func (t *TimerService) processDue(ctx context.Context, startup bool) error {
	timers, err := t.store.ListTimers(ctx)
	if err != nil {
		return err
	}
	t.forgetGone(timers)
	now := t.clock.Now()
	for _, tm := range timers {
		select {
		case <-t.stop:
			return nil
		default:
		}
		if tm.FireAt.After(now) {
			break
		}
		key := keyOfTimer(tm)
		if _, ran := t.unsettled[key]; !ran {
			if startup && now.Sub(tm.FireAt) > t.grace {
				t.call("missed", t.missed, tm)
			} else {
				t.call("fire", t.fire, tm)
			}
			t.unsettled[key] = struct{}{}
		}
		if err := t.settle(ctx, tm, now); err != nil {
			return err
		}
		delete(t.unsettled, key)
	}
	return nil
}

// settle removes a one-off timer or moves a yearly one to its next date after now.
//
// Both steps are conditional on the occurrence that fired. processDue works on a
// snapshot, and a Replace (a republished flow) or a deleted flow may have changed or
// removed the row while the callback ran; then there is nothing left to settle, and the
// newly armed timer must survive.
func (t *TimerService) settle(ctx context.Context, tm TimerRecord, now time.Time) error {
	if tm.FireAt.IsZero() {
		// ListTimers yields a zero time for a row whose fire_at cannot be read. The
		// conditional forms below could never match it, so the row would be due and
		// fired again and again.
		return t.store.DeleteTimer(ctx, tm.FlowID, tm.NodeID)
	}
	if tm.Repeat != RepeatYearly {
		return t.store.DeleteTimerAt(ctx, tm.FlowID, tm.NodeID, tm.FireAt)
	}
	return t.store.MoveTimer(ctx, tm.FlowID, tm.NodeID, tm.FireAt, nextYearly(tm.FireAt, now))
}

// forgetGone drops unsettled entries whose timer is no longer stored (the flow was
// deleted or republished before the store recovered).
func (t *TimerService) forgetGone(stored []TimerRecord) {
	if len(t.unsettled) == 0 {
		return
	}
	live := make(map[timerKey]struct{}, len(stored))
	for _, tm := range stored {
		live[keyOfTimer(tm)] = struct{}{}
	}
	for key := range t.unsettled {
		if _, ok := live[key]; !ok {
			delete(t.unsettled, key)
		}
	}
}

// call runs one callback and recovers from a panic so a faulty trigger handler can
// neither kill the process nor the timer goroutine.
func (t *TimerService) call(kind string, fn TimerFireFunc, tm TimerRecord) {
	if fn == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			t.logger.Error("flow timer callback panicked; the timer counts as handled",
				"callback", kind, "flow_id", tm.FlowID, "node_id", tm.NodeID, "scheduled_for", tm.FireAt,
				"panic", truncateRunes(fmt.Sprint(r), maxErrorMessageRunes), "stack", string(debug.Stack()))
		}
	}()
	fn(tm.FlowID, tm.NodeID, tm.FireAt)
}

// nextYearly returns the first yearly recurrence of from that is after now, or from
// itself when it already is. It keeps the zone and the time of day.
//
// A timer on Feb 29 recurs only on Feb 29, that is every leap year, the way RFC 5545
// skips recurrences that do not exist. Time.AddDate would normalize Feb 29 to Mar 1
// and, as the stored date is the only memory of the original day, the timer would
// stay on Mar 1 for good. Clamping to Feb 28 instead would lose the original day just
// the same (the next leap year would fire on Feb 28), and remembering it needs a schema
// change, so the timer waits for the next real Feb 29.
func nextYearly(from, now time.Time) time.Time {
	year, month, day := from.Date()
	hour, minute, sec := from.Clock()
	next := from
	for y := year + 1; !next.After(now); y++ {
		candidate := time.Date(y, month, day, hour, minute, sec, from.Nanosecond(), from.Location())
		if candidate.Month() != month { // Feb 29 in a year without one
			continue
		}
		next = candidate
	}
	return next
}
