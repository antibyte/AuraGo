package retronet

import (
	"io"
	"sync/atomic"
	"testing"
	"time"
)

// pumpBlockedWriter blocks every Write until release is closed, like an SSH channel whose
// window is exhausted.
type pumpBlockedWriter struct{ release chan struct{} }

func (b pumpBlockedWriter) Write([]byte) (int, error) {
	<-b.release
	return 0, io.EOF
}

func TestSessionWriterAbortsACallThatOutlastsTheWriteTimeout(t *testing.T) {
	previous := sessionWriteTimeout
	sessionWriteTimeout = 50 * time.Millisecond
	t.Cleanup(func() { sessionWriteTimeout = previous })

	for name, call := range map[string]func(w sessionWriter) error{
		"write":   func(w sessionWriter) error { return w.write([]byte("x")) },
		"bounded": func(w sessionWriter) error { return w.bounded(func() error { _, err := w.w.Write(nil); return err }) },
	} {
		t.Run(name, func(t *testing.T) {
			release := make(chan struct{})
			var aborts atomic.Int32
			w := sessionWriter{
				w:     pumpBlockedWriter{release: release},
				stats: &sessionStats{},
				abort: func() {
					aborts.Add(1)
					close(release)
				},
			}
			started := time.Now()
			if err := call(w); err != io.EOF {
				t.Fatalf("error = %v, want the error of the unblocked write", err)
			}
			if elapsed := time.Since(started); elapsed < 40*time.Millisecond || elapsed > 2*time.Second {
				t.Fatalf("blocked call returned after %v, want about the 50ms write timeout", elapsed)
			}
			if aborts.Load() != 1 {
				t.Fatalf("abort calls = %d, want 1", aborts.Load())
			}
		})
	}
}

func TestSessionWriterDoesNotAbortFastCalls(t *testing.T) {
	previous := sessionWriteTimeout
	sessionWriteTimeout = 30 * time.Millisecond
	t.Cleanup(func() { sessionWriteTimeout = previous })
	var aborts atomic.Int32
	stats := &sessionStats{}
	w := sessionWriter{w: io.Discard, stats: stats, abort: func() { aborts.Add(1) }}
	if err := w.write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := w.bounded(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * sessionWriteTimeout) // a stopped watchdog must stay quiet
	if aborts.Load() != 0 || stats.bytesOut != 5 {
		t.Fatalf("abort calls = %d, bytes out = %d, want 0 and 5", aborts.Load(), stats.bytesOut)
	}
}
