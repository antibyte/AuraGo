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
// It is also the first wait before an occurrence that its callback did not handle is
// tried again (see timerHandleFunc).
const timerRetryDelay = 30 * time.Second

// timerMaxRetryDelay caps the wait before an unhandled occurrence is tried again: the
// wait starts at timerRetryDelay and doubles with every failed attempt up to this.
const timerMaxRetryDelay = 15 * time.Minute

// timerMaxRetryLateness ends the retries of an unhandled occurrence: one that is due again
// for a retry more than this after its own time is given up, with a Warn, and goes to the
// missed callback instead. A run a day late is rarely still wanted, and a failure that
// lasted a day is not a passing one.
const timerMaxRetryLateness = 24 * time.Hour

// errTimerServiceStopped is returned by Start after Stop.
var errTimerServiceStopped = errors.New("the flow timer service was stopped")

// TimerFireFunc is called when a Date/Time trigger is due, or, for the missed
// callback, when it became due while AuraGo was off for longer than MissedTimerGrace or
// its retries were given up (timerMaxRetryLateness). The occurrence counts as handled
// when it returns; the Service's fire callback, which can fail, is a timerHandleFunc.
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

// timerHandleFunc is a fire callback that reports whether it handled the occurrence.
// nil means handled: the occurrence is consumed (a one-off timer is deleted, a yearly one
// moves to its next date). An error means the occurrence was not handled (the start it
// wanted failed): it stays stored, the service logs the error at Warn and calls the
// callback again for the same occurrence once a backoff has passed. The backoff is per
// occurrence, starts at the retry delay (timerRetryDelay) and doubles up to
// timerMaxRetryDelay, so other timers keep firing meanwhile and a lasting failure never
// spins; past timerMaxRetryLateness the occurrence is given up. The rules of
// TimerFireFunc apply otherwise; a panic counts as handled. It is unexported like
// newTimerService, which takes it: only the Service's own callback reports failures.
type timerHandleFunc func(flowID, nodeID string, scheduledFor time.Time) error

// handled adapts a TimerFireFunc, which always handles its occurrence, to a timerHandleFunc.
func handled(fn TimerFireFunc) timerHandleFunc {
	if fn == nil {
		return nil
	}
	return func(flowID, nodeID string, scheduledFor time.Time) error {
		fn(flowID, nodeID, scheduledFor)
		return nil
	}
}

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
// returned and handled it, so a crash in between fires it again at the next start (or
// reports it as missed). Within one process a callback that handled its occurrence is
// not repeated, even when the store fails in between and the timer has to be settled by
// a later attempt.
//
// A fire callback that did not handle its occurrence (timerHandleFunc returned an error)
// leaves it stored and is called again for it after a per-occurrence backoff. The retry
// keeps the occurrence's own time (scheduledFor). Retries end once the occurrence is due
// for one more than timerMaxRetryLateness (24 h) after its own time: it is given up with a
// Warn, goes to the missed callback and is consumed (a one-off deleted, a yearly one moved
// to its first date after now), so a yearly occurrence is never retried into its next
// year. Replacing or removing the timer (a republish, switching the flow off, deleting it)
// drops a pending retry, logged at Info. The retry state lives in memory: after a restart
// the start-up pass treats the occurrence like any other, so beyond MissedTimerGrace it
// goes to the missed callback and is consumed.
type TimerService struct {
	store  *Store
	clock  Clock
	fire   timerHandleFunc
	missed timerHandleFunc
	logger *slog.Logger
	grace  time.Duration
	// retryDelay is the pause after a store error, see timerRetryDelay, and the first
	// wait before an unhandled occurrence is tried again. Tests shorten it.
	retryDelay time.Duration
	// maxRetryDelay caps the doubling wait of an unhandled occurrence, see timerMaxRetryDelay.
	maxRetryDelay time.Duration
	// maxRetryLateness ends the retries of an occurrence, see timerMaxRetryLateness.
	maxRetryLateness time.Duration

	reload   chan struct{}
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once

	startMu     sync.Mutex // serializes Start
	mu          sync.Mutex // guards the two flags and loc below
	loopStarted bool
	stopped     bool
	// loc is the zone yearly timers recur in; nil means UTC. See SetLocation.
	loc *time.Location

	// unsettled holds the timers whose callback ran but whose delete or move failed.
	// retrying holds the occurrences whose callback did not handle them, with their
	// backoff. Both are touched only by processDue and nextFire, which never run
	// concurrently with each other: processDue first from Start, and once Start succeeded
	// both only from the loop.
	unsettled map[timerKey]struct{}
	retrying  map[timerKey]timerRetry
}

// timerRetry is the backoff of an occurrence whose callback did not handle it.
type timerRetry struct {
	attempts  int       // failed attempts so far
	notBefore time.Time // the next attempt is not made before this
}

// NewTimerService creates a timer service whose fire callback always handles its
// occurrence. missed may be nil; a timer that is too late at start-up is then just
// removed (or moved forward, when it repeats).
func NewTimerService(store *Store, clock Clock, fire, missed TimerFireFunc, logger *slog.Logger) *TimerService {
	return newTimerService(store, clock, handled(fire), missed, logger)
}

// newTimerService creates a timer service whose fire callback reports whether it handled
// the occurrence (see timerHandleFunc). The missed callback always handles it.
func newTimerService(store *Store, clock Clock, fire timerHandleFunc, missed TimerFireFunc, logger *slog.Logger) *TimerService {
	if clock == nil {
		clock = RealClock()
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &TimerService{
		store: store, clock: clock, fire: fire, missed: handled(missed), logger: logger,
		grace: MissedTimerGrace, retryDelay: timerRetryDelay, maxRetryDelay: timerMaxRetryDelay,
		maxRetryLateness: timerMaxRetryLateness,
		reload: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}),
		unsettled: make(map[timerKey]struct{}), retrying: make(map[timerKey]timerRetry),
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

// SetLocation sets the zone in which yearly timers recur; nil means UTC, the default.
// The store keeps fire times in UTC, so without the zone a yearly timer keeps its UTC
// time of day: a date in the weeks where the daylight saving switch moves from year to
// year (end of March and of October in Europe) then comes out an hour off its local
// time, and a local Feb 29 whose UTC date is Feb 28 drifts to Mar 1. In the zone both
// keep their local date and time, with one limit: a time in the spring-forward gap
// (02:30 on the switch day) does not exist, time.Date moves it to 03:30, and since the
// stored time is all the timer remembers it stays at 03:30 in the following years until
// the flow is armed again from its document (Publish or SetEnabled). A time in the
// autumn overlap keeps its wall-clock time; which of the two instants is not defined.
// SetLocation may be called at any time; the zone applies to the timers settled afterwards.
func (t *TimerService) SetLocation(loc *time.Location) {
	t.mu.Lock()
	t.loc = loc
	t.mu.Unlock()
}

// location returns the zone set by SetLocation, or UTC.
func (t *TimerService) location() *time.Location {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.loc == nil {
		return time.UTC
	}
	return t.loc
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
// have changed what is due, and each Replace allows only one extra attempt. An
// occurrence whose callback did not handle it waits for its own backoff instead (see
// nextFire), which a Replace does not lift.
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

// nextFire returns when processDue has the next thing to do; ok is false when there is
// no timer. That is the earliest due occurrence that is not waiting for its retry
// backoff, else the end of the earliest backoff or the first timer still to come,
// whichever is earlier. It reads the timers in processDue's order and stops where
// processDue stops (at the first timer still to come), so a damaged row behind that
// timer cannot make the loop spin. A store error is logged and returned for the loop to
// back off.
func (t *TimerService) nextFire() (next time.Time, ok bool, err error) {
	timers, err := t.store.ListTimers(context.Background())
	if err != nil {
		t.logger.Warn("flow timers could not be loaded", "error", err)
		return time.Time{}, false, err
	}
	now := t.clock.Now()
	for _, tm := range timers {
		at := tm.FireAt // zero for a damaged row, which processDue drops at once
		upcoming := !at.IsZero() && at.After(now)
		if retry, waiting := t.retrying[keyOfTimer(tm)]; !upcoming && waiting && retry.notBefore.After(at) {
			at = retry.notBefore
		}
		if !ok || at.Before(next) {
			next, ok = at, true
		}
		if upcoming {
			break // processDue stops here too; the timers after it are not earlier
		}
	}
	return next, ok, nil
}

// processDue fires (or, at start-up beyond the grace period, reports as missed)
// every timer that is due, then deletes one-off timers and moves yearly ones forward.
// It stops at the first store error; the timers not settled yet stay stored and the
// next pass handles them, calling a callback only for those that have not run it. It
// also stops, without error, before the next timer once Stop was called: the rest stays
// stored for the next start, so a callback never runs after Stop in the middle of a
// shutdown.
//
// An occurrence whose fire callback did not handle it (an error) is neither settled nor
// remembered as fired: it stays stored and gets a backoff (retryLater), and processDue
// passes over it until the backoff has run out, then calls the callback again. The other
// timers of the pass go on. A retry that comes due more than maxRetryLateness after the
// occurrence's own time is given up instead: Warn, the missed callback, consumed.
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
		if tm.FireAt.IsZero() {
			// ListTimers yields a zero time for a row whose fire_at cannot be read. Firing it
			// would start a run "scheduled" for year 1, so drop it without a callback.
			t.logger.Warn("flow timer has no readable fire time and is dropped without firing",
				"flow", quoteForError(tm.FlowID), "node", quoteForError(tm.NodeID))
			if err := t.settle(ctx, tm, now); err != nil {
				return err
			}
			continue
		}
		if tm.FireAt.After(now) {
			break
		}
		key := keyOfTimer(tm)
		if _, ran := t.unsettled[key]; !ran {
			retry, waiting := t.retrying[key]
			if waiting && now.Before(retry.notBefore) {
				continue // its backoff runs; nextFire plans the next attempt
			}
			kind, fn := "fire", t.fire
			switch late := now.Sub(tm.FireAt); {
			case startup && late > t.grace:
				kind, fn = "missed", t.missed
			case waiting && late > t.maxRetryLateness:
				t.logger.Warn("a flow timer was not handled for too long; its retries end and it counts as missed",
					"flow_id", tm.FlowID, "node_id", tm.NodeID, "scheduled_for", tm.FireAt,
					"attempts", retry.attempts, "late", late.Round(time.Second).String())
				kind, fn = "missed", t.missed
			}
			if err := t.call(kind, fn, tm); err != nil {
				t.retryLater(key, tm, now, err)
				continue
			}
			delete(t.retrying, key)
			t.unsettled[key] = struct{}{}
		}
		if err := t.settle(ctx, tm, now); err != nil {
			return err
		}
		delete(t.unsettled, key)
	}
	return nil
}

// settle removes a one-off timer or moves a yearly one to its next date after now,
// computed in the service's zone (SetLocation) and stored as UTC.
//
// Both steps are conditional on the occurrence that fired. processDue works on a
// snapshot, and a Replace (a republished flow) or a deleted flow may have changed or
// removed the row while the callback ran; then there is nothing left to settle, and the
// newly armed timer must survive.
func (t *TimerService) settle(ctx context.Context, tm TimerRecord, now time.Time) error {
	if tm.FireAt.IsZero() {
		// A damaged row (processDue never fires it). The conditional forms below could
		// never match it, so it would stay and be found again and again; it is deleted
		// by flow and node instead. That reopens the Replace race of the comment above,
		// but only for a row that was never valid, which is accepted.
		return t.store.DeleteTimer(ctx, tm.FlowID, tm.NodeID)
	}
	if tm.Repeat != RepeatYearly {
		return t.store.DeleteTimerAt(ctx, tm.FlowID, tm.NodeID, tm.FireAt)
	}
	return t.store.MoveTimer(ctx, tm.FlowID, tm.NodeID, tm.FireAt, nextYearly(tm.FireAt.In(t.location()), now).UTC())
}

// forgetGone drops unsettled and retrying entries whose timer is no longer stored (the
// flow was deleted, switched off or republished meanwhile). A dropped retry is logged at
// Info: that occurrence will not run.
func (t *TimerService) forgetGone(stored []TimerRecord) {
	if len(t.unsettled) == 0 && len(t.retrying) == 0 {
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
	for key, retry := range t.retrying {
		if _, ok := live[key]; !ok {
			delete(t.retrying, key)
			t.logger.Info("a pending retry of a flow timer was dropped: the timer was replaced or removed (republished, switched off or deleted)",
				"flow_id", key.flowID, "node_id", key.nodeID, "scheduled_for", time.UnixMicro(key.fireAt).UTC(),
				"attempts", retry.attempts)
		}
	}
}

// retryLater gives an occurrence whose callback did not handle it its next backoff: the
// retry delay, doubled for every earlier failed attempt, at most maxRetryDelay. The
// failure is logged at Warn with the attempt and the wait.
func (t *TimerService) retryLater(key timerKey, tm TimerRecord, now time.Time, err error) {
	retry := t.retrying[key]
	retry.attempts++
	delay := t.retryDelay
	for i := 1; i < retry.attempts && delay < t.maxRetryDelay; i++ {
		delay *= 2
	}
	if delay > t.maxRetryDelay {
		delay = t.maxRetryDelay
	}
	retry.notBefore = now.Add(delay)
	t.retrying[key] = retry
	t.logger.Warn("a flow timer was not handled; it stays due and is tried again",
		"flow_id", tm.FlowID, "node_id", tm.NodeID, "scheduled_for", tm.FireAt,
		"attempt", retry.attempts, "retry_in", delay.String(),
		"error", truncateRunes(fmt.Sprint(err), maxErrorMessageRunes))
}

// call runs one callback and recovers from a panic so a faulty trigger handler can
// neither kill the process nor the timer goroutine. It returns the callback's error; a
// panic counts as handled (nil).
func (t *TimerService) call(kind string, fn timerHandleFunc, tm TimerRecord) (err error) {
	if fn == nil {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			t.logger.Error("flow timer callback panicked; the timer counts as handled",
				"callback", kind, "flow_id", tm.FlowID, "node_id", tm.NodeID, "scheduled_for", tm.FireAt,
				"panic", truncateRunes(fmt.Sprint(r), maxErrorMessageRunes), "stack", string(debug.Stack()))
			err = nil
		}
	}()
	return fn(tm.FlowID, tm.NodeID, tm.FireAt)
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
