package flows

import (
	"sync"
	"time"
)

const subscriberBuffer = 256

// leakedLogGrace is added to the longest possible run time (MaxRunSecondsLimit)
// before an open log counts as leaked, see Sweep.
const leakedLogGrace = time.Hour

// EventBus keeps each run's events in memory and fans them out to subscribers.
// A subscriber that falls behind by more than subscriberBuffer events is dropped
// (its channel closes) and must resubscribe with its last seen Seq.
//
// Memory: a run's log is naturally bounded, so the bus has no count cap. The
// engine refuses flows with more than MaxNodes nodes and every node yields at
// most a step_started and a step_finished event, which makes about
// 2*MaxNodes+2 (1002) events per run. A finished log stays for the retention and
// is then dropped by Sweep.
//
// Concurrency: every method is safe for concurrent use. The engine's coordinator
// goroutine calls Open, Publish and Finish; HTTP handlers call Subscribe and the
// cancel funcs; a janitor calls Sweep. All state changes, every send to a
// subscriber channel and every close of one happen under mu, and a channel is
// closed only together with its removal from the subscriber map of its log. So
// no channel is closed twice and none is sent to after it was closed.
type EventBus struct {
	mu     sync.Mutex
	runs   map[string]*runLog
	retain time.Duration
	now    func() time.Time
}

type runLog struct {
	events   []RunEvent
	subs     map[int]chan RunEvent
	nextID   int
	done     bool
	openedAt time.Time
	doneAt   time.Time
}

// NewEventBus keeps finished runs for retain so late subscribers still get the full log.
func NewEventBus(retain time.Duration, now func() time.Time) *EventBus {
	if now == nil {
		now = time.Now
	}
	return &EventBus{runs: map[string]*runLog{}, retain: retain, now: now}
}

// Open starts the log of a run and records when it was opened (see Sweep for what
// that is used for). Opening a run that is already known changes nothing.
func (b *EventBus) Open(runID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.runs[runID]; !ok {
		b.runs[runID] = &runLog{subs: map[int]chan RunEvent{}, openedAt: b.now()}
	}
}

// Publish appends ev to its run's log and delivers it to subscribers. Events for
// unknown or finished runs are ignored. The bus stores and hands out ev as it is
// and never modifies it.
func (b *EventBus) Publish(ev RunEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	log := b.runs[ev.RunID]
	if log == nil || log.done {
		return
	}
	log.events = append(log.events, ev)
	for id, ch := range log.subs {
		select {
		case ch <- ev:
		default:
			close(ch)
			delete(log.subs, id)
		}
	}
}

// Finish marks the run as done and closes all subscriber channels.
func (b *EventBus) Finish(runID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	log := b.runs[runID]
	if log == nil || log.done {
		return
	}
	log.done = true
	log.doneAt = b.now()
	closeSubscribers(log)
}

// closeSubscribers closes and forgets every subscriber of log. The caller holds b.mu.
func closeSubscribers(log *runLog) {
	for id, ch := range log.subs {
		close(ch)
		delete(log.subs, id)
	}
}

// Subscribe returns the events after afterSeq and a channel for new ones, plus a func
// that ends the subscription. ok is false for unknown runs. For finished runs the
// backlog holds the rest of the log and the channel is already closed. The backlog
// and the events on the channel are taken atomically, so they neither overlap nor
// leave a gap.
//
// A closed channel means one of three things: the run finished, the subscriber was
// dropped for falling more than subscriberBuffer events behind, or it was cancelled
// (by the cancel func, or because Sweep forgot a leaked log). The consumer tells
// them apart by whether it received an event of type run_finished. If it did not
// and it still wants the stream, it subscribes again with the Seq of the last event
// it saw. Events that were buffered before the channel closed can still be read.
//
// The events are read-only: they share their *StepRecord and *RunSummary values,
// and the maps inside them, with the run's result and with every other subscriber.
// A consumer must not modify them.
//
// The cancel func is idempotent and safe in every state: after a drop, after
// Finish, after Sweep removed the run, and while other goroutines publish.
func (b *EventBus) Subscribe(runID string, afterSeq int) ([]RunEvent, <-chan RunEvent, func(), bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	log := b.runs[runID]
	if log == nil {
		return nil, nil, func() {}, false
	}
	var backlog []RunEvent
	for _, ev := range log.events {
		if ev.Seq > afterSeq {
			backlog = append(backlog, ev)
		}
	}
	ch := make(chan RunEvent, subscriberBuffer)
	if log.done {
		close(ch)
		return backlog, ch, func() {}, true
	}
	id := log.nextID
	log.nextID++
	log.subs[id] = ch
	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if c, ok := log.subs[id]; ok {
			close(c)
			delete(log.subs, id)
		}
	}
	return backlog, ch, cancel, true
}

// Sweep forgets finished runs older than the retention and returns how many were removed.
//
// It also forgets leaked logs: a run whose Finish never came (a runner bug, a panic
// between Open and Finish) would otherwise stay in memory forever. A log still open
// after MaxRunSecondsLimit plus one hour plus the retention, counted from Open, is
// removed after its subscriber channels were closed, so their consumers are released.
// No run can legitimately live that long. Events published to a removed run are ignored.
func (b *EventBus) Sweep() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	leakedAfter := MaxRunSecondsLimit*time.Second + leakedLogGrace + b.retain
	removed := 0
	for id, log := range b.runs {
		expired := log.done && now.Sub(log.doneAt) > b.retain
		leaked := !log.done && now.Sub(log.openedAt) > leakedAfter
		if !expired && !leaked {
			continue
		}
		closeSubscribers(log) // finished logs have none left
		delete(b.runs, id)
		removed++
	}
	return removed
}

// Has reports whether the bus still knows the run.
func (b *EventBus) Has(runID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.runs[runID]
	return ok
}
