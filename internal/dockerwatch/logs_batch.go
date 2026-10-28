package dockerwatch

import (
	"errors"
	"sync"
	"time"
)

// logTimer is what the batcher needs from a timer (*time.Timer, or a fake in tests).
type logTimer interface{ Stop() bool }

func logAfterFunc(d time.Duration, f func()) logTimer { return time.AfterFunc(d, f) }

var errLogBatcherClosed = errors.New("log stream closed")

// logBatcher turns single log entries into LogBatch messages: a batch leaves
// when it has max entries or interval after its first entry, whichever comes
// first. All calls to out and notice are serialized under one mutex and happen
// in the order the entries and notices were added; a slow receiver therefore
// slows the producers down (back pressure) instead of buffering without bound.
// The first error of out or notice is sticky: every later call returns it, and
// onFail is told once, so the follower stops reading instead of waiting for its
// next line.
type logBatcher struct {
	out      func(LogBatch) error
	notice   func(Notice) error
	onFail   func(error)
	max      int
	interval time.Duration
	after    func(d time.Duration, f func()) logTimer

	mu      sync.Mutex
	pending []LogEntry
	timer   logTimer
	err     error
	closed  bool
}

func newLogBatcher(out func(LogBatch) error, notice func(Notice) error, onFail func(error), max int, interval time.Duration, after func(time.Duration, func()) logTimer) *logBatcher {
	if notice == nil {
		notice = func(Notice) error { return nil }
	}
	if onFail == nil {
		onFail = func(error) {}
	}
	return &logBatcher{out: out, notice: notice, onFail: onFail, max: max, interval: interval, after: after}
}

// setErr records the first error.
func (b *logBatcher) setErr(err error) {
	if b.err == nil {
		b.err = err
		b.onFail(err)
	}
}

// add queues one entry.
func (b *logBatcher) add(e LogEntry) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.addLocked(e)
}

// addAll queues entries in order (the merged history).
func (b *logBatcher) addAll(entries []LogEntry) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, e := range entries {
		if err := b.addLocked(e); err != nil {
			return err
		}
	}
	return nil
}

func (b *logBatcher) addLocked(e LogEntry) error {
	if b.err != nil {
		return b.err
	}
	if b.closed {
		return errLogBatcherClosed
	}
	b.pending = append(b.pending, e)
	if len(b.pending) >= b.max {
		return b.flushLocked()
	}
	if b.timer == nil {
		b.timer = b.after(b.interval, b.onTimer)
	}
	return nil
}

func (b *logBatcher) onTimer() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.timer = nil
	_ = b.flushLocked()
}

// flushLocked sends what is pending.
func (b *logBatcher) flushLocked() error {
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
	if len(b.pending) == 0 || b.err != nil {
		return b.err
	}
	lines := b.pending
	b.pending = nil
	if err := b.out(LogBatch{Lines: lines}); err != nil {
		b.setErr(err)
	}
	return b.err
}

// send delivers a notice after the entries added before it.
func (b *logBatcher) send(n Notice) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.flushLocked(); err != nil {
		return err
	}
	if b.closed {
		return errLogBatcherClosed
	}
	if err := b.notice(n); err != nil {
		b.setErr(err)
	}
	return b.err
}

// failure returns the sticky error, if any.
func (b *logBatcher) failure() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

// close sends what is pending and refuses further entries.
func (b *logBatcher) close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	err := b.flushLocked()
	b.closed = true
	return err
}
