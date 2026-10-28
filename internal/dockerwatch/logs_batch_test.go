package dockerwatch

import (
	"errors"
	"testing"
	"time"
)

// logsFakeTimers is a timer source that fires only when the test says so.
type logsFakeTimers struct {
	armed   int
	stopped int
	fn      func()
}

type logsFakeTimer struct {
	owner *logsFakeTimers
	live  bool
}

func (t *logsFakeTimer) Stop() bool {
	was := t.live
	t.live = false
	if was {
		t.owner.stopped++
	}
	return was
}

func (f *logsFakeTimers) after(_ time.Duration, fn func()) logTimer {
	f.armed++
	t := &logsFakeTimer{owner: f, live: true}
	f.fn = func() {
		if t.live {
			t.live = false
			fn()
		}
	}
	return t
}

func (f *logsFakeTimers) fire() {
	if f.fn != nil {
		f.fn()
	}
}

// logsBase is the time of line 0 in the log fixtures.
var logsBase = time.Date(2026, 10, 28, 7, 0, 0, 0, time.UTC)

func logsEntry(text string) LogEntry { return LogEntry{T: logsBase, S: "out", Text: text} }

func TestLogBatcherFlushesOnTheTimerAndWhenFull(t *testing.T) {
	var batches []LogBatch
	timers := &logsFakeTimers{}
	b := newLogBatcher(func(b LogBatch) error { batches = append(batches, b); return nil }, nil, nil, 3, 100*time.Millisecond, timers.after)

	_ = b.add(logsEntry("a"))
	_ = b.add(logsEntry("b"))
	if len(batches) != 0 || timers.armed != 1 {
		t.Fatalf("after two lines: %d batches, %d timers armed; want none sent and one timer for the batch", len(batches), timers.armed)
	}
	timers.fire()
	if len(batches) != 1 || len(batches[0].Lines) != 2 || batches[0].Lines[0].Text != "a" || batches[0].Lines[1].Text != "b" {
		t.Fatalf("batches after the timer = %+v, want one batch a,b", batches)
	}

	_ = b.add(logsEntry("c"))
	_ = b.add(logsEntry("d"))
	_ = b.add(logsEntry("e")) // the third line fills the batch
	if len(batches) != 2 || len(batches[1].Lines) != 3 {
		t.Fatalf("batches after a full batch = %+v, want a second batch c,d,e at once", batches)
	}
	timers.fire() // the timer of that batch was stopped: nothing is left to send
	if len(batches) != 2 {
		t.Fatalf("a stopped timer sent %d batches", len(batches))
	}

	_ = b.add(logsEntry("f"))
	if err := b.close(); err != nil || len(batches) != 3 || batches[2].Lines[0].Text != "f" {
		t.Fatalf("close = %v, batches %+v; want the last line sent", err, batches)
	}
	if err := b.add(logsEntry("g")); err == nil {
		t.Fatal("a closed batcher must refuse lines")
	}
}

func TestLogBatcherSendsANoticeAfterTheLinesAddedBeforeIt(t *testing.T) {
	var calls []string
	timers := &logsFakeTimers{}
	b := newLogBatcher(
		func(b LogBatch) error { calls = append(calls, "lines"); return nil },
		func(n Notice) error { calls = append(calls, "notice:"+n.Code); return nil },
		nil, 200, 100*time.Millisecond, timers.after)
	_ = b.add(logsEntry("a"))
	if err := b.send(Notice{Code: NoticeDetached}); err != nil {
		t.Fatal(err)
	}
	_ = b.add(logsEntry("b"))
	_ = b.close()
	want := []string{"lines", "notice:detached", "lines"}
	if len(calls) != len(want) || calls[0] != want[0] || calls[1] != want[1] || calls[2] != want[2] {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestLogBatcherKeepsTheFirstError(t *testing.T) {
	boom := errors.New("receiver gone")
	calls := 0
	var told []error
	b := newLogBatcher(func(LogBatch) error { calls++; return boom }, nil, func(err error) { told = append(told, err) }, 1, time.Hour, (&logsFakeTimers{}).after)
	if err := b.add(logsEntry("a")); !errors.Is(err, boom) {
		t.Fatalf("add = %v, want the receiver's error", err)
	}
	if err := b.add(logsEntry("b")); !errors.Is(err, boom) || calls != 1 {
		t.Fatalf("second add = %v after %d out calls, want the same error without another call", err, calls)
	}
	if !errors.Is(b.failure(), boom) || !errors.Is(b.send(Notice{}), boom) {
		t.Fatal("failure and notices must report the sticky error")
	}
	if len(told) != 1 || !errors.Is(told[0], boom) {
		t.Fatalf("onFail was told %v, want the error once", told)
	}
}
