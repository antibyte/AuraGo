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

// xterm answers some service queries on its own (cursor position, status, device attributes,
// mode reports, focus changes). Such replies are not user activity; everything else is.
func TestTerminalReportsOnly(t *testing.T) {
	cases := []struct {
		name string
		data string
		want bool
	}{
		{"cursor position report", "\x1b[12;40R", true},
		{"status report", "\x1b[0n", true},
		{"primary device attributes", "\x1b[?1;2c", true},
		{"primary device attributes, many", "\x1b[?62;1;2;6;7;8;9;15;22c", true},
		{"secondary device attributes", "\x1b[>0;276;0c", true},
		{"private mode report", "\x1b[?2004;2$y", true},
		{"ansi mode report", "\x1b[4;2$y", true},
		{"focus in", "\x1b[I", true},
		{"focus out", "\x1b[O", true},
		{"several reports in one message", "\x1b[I\x1b[24;80R\x1b[?1;2c\x1b[O", true},

		{"empty", "", false},
		{"a key", "a", false},
		{"enter", "\r", false},
		{"escape", "\x1b", false},
		{"arrow up", "\x1b[A", false},
		{"function key", "\x1b[15~", false},
		{"sgr mouse press", "\x1b[<0;10;5M", false},
		{"x10 mouse press", "\x1b[M !!", false},
		{"report followed by a key", "\x1b[12;40Rx", false},
		{"key followed by a report", "x\x1b[12;40R", false},
		{"cursor report with one number", "\x1b[12R", false},
		{"cursor report with three numbers", "\x1b[1;2;3R", false},
		{"status report without a number", "\x1b[n", false},
		{"device attributes without a marker", "\x1b[1;2c", false},
		{"device attributes without parameters", "\x1b[?c", false},
		{"mode report with one number", "\x1b[?2004$y", false},
		{"mode report without the y", "\x1b[?2004;2$", false},
		{"empty parameter", "\x1b[;40R", false},
		{"truncated report", "\x1b[12;40", false},
		{"bracketed paste start", "\x1b[200~", false},
	}
	for _, tc := range cases {
		if got := terminalReportsOnly([]byte(tc.data)); got != tc.want {
			t.Errorf("%s: terminalReportsOnly(%q) = %v, want %v", tc.name, tc.data, got, tc.want)
		}
	}
}
