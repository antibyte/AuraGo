package cydcreator

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// buildLogMaxLines is the most lines one build_log event carries.
	buildLogMaxLines = 50
	// buildLogMaxPending bounds lines waiting for the next event; older ones
	// are counted as skipped (the full log stays in build.log).
	buildLogMaxPending = 1000
	maxBuildLogLine    = 4096
)

// buildLogInterval is the minimum gap between two build_log events of a build.
var buildLogInterval = 200 * time.Millisecond

// buildLog collects the full log of the running build, capped at
// MaxBuildLogBytes.
type buildLog struct {
	mu        sync.Mutex
	buf       strings.Builder
	truncated bool
}

func (l *buildLog) add(line string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.truncated {
		return
	}
	if l.buf.Len()+len(line)+1 > MaxBuildLogBytes {
		l.buf.WriteString("[cyd-creator] log truncated\n")
		l.truncated = true
		return
	}
	l.buf.WriteString(line)
	l.buf.WriteByte('\n')
}

func (l *buildLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// logThrottle turns a stream of log lines into events of at most
// buildLogMaxLines lines, at least interval apart. When more lines arrive than
// fit, the event starts with a "lines skipped" marker followed by the newest
// lines.
type logThrottle struct {
	mu      sync.Mutex
	pending []string
	skipped int
	last    time.Time // time of the last event
	emit    func(lines []string)
	quit    chan struct{}
	done    chan struct{}
}

func newLogThrottle(interval time.Duration, emit func(lines []string)) *logThrottle {
	t := &logThrottle{emit: emit, quit: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(t.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.flush()
			case <-t.quit:
				t.mu.Lock()
				wait := len(t.pending) > 0 || t.skipped > 0
				gap := interval - time.Since(t.last)
				t.mu.Unlock()
				if wait && gap > 0 {
					time.Sleep(gap) // keep the minimum gap for the last event too
				}
				t.flush()
				return
			}
		}
	}()
	return t
}

func (t *logThrottle) add(line string) {
	if len(line) > maxBuildLogLine {
		line = line[:maxBuildLogLine] + "…"
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pending = append(t.pending, line)
	if len(t.pending) > buildLogMaxPending {
		drop := len(t.pending) - buildLogMaxPending
		t.pending = append(t.pending[:0:0], t.pending[drop:]...)
		t.skipped += drop
	}
}

func (t *logThrottle) flush() {
	t.mu.Lock()
	lines, skipped := t.pending, t.skipped
	t.pending, t.skipped = nil, 0
	t.mu.Unlock()
	if len(lines) == 0 && skipped == 0 {
		return
	}
	t.mu.Lock()
	t.last = time.Now()
	t.mu.Unlock()
	if len(lines)+min(skipped, 1) > buildLogMaxLines {
		keep := buildLogMaxLines - 1
		skipped += len(lines) - keep
		lines = lines[len(lines)-keep:]
	}
	if skipped > 0 {
		lines = append([]string{fmt.Sprintf("[… %d lines skipped, the full log is in build.log …]", skipped)}, lines...)
	}
	t.emit(lines)
}

// stop flushes the remaining lines and waits for the goroutine.
func (t *logThrottle) stop() {
	close(t.quit)
	<-t.done
}
