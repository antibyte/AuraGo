package cydcreator

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBuildLogCapsTheStoredLog(t *testing.T) {
	var l buildLog
	line := strings.Repeat("x", 1023)
	for i := 0; i < 3000; i++ {
		l.add(line)
	}
	text := l.String()
	if len(text) > MaxBuildLogBytes+64 || !strings.HasSuffix(text, "[cyd-creator] log truncated\n") {
		t.Fatalf("log of %d bytes, suffix %q", len(text), text[len(text)-40:])
	}
}

func TestLogThrottleBatchesAndMarksSkippedLines(t *testing.T) {
	var mu sync.Mutex
	var events [][]string
	var times []time.Time
	throttle := newLogThrottle(100*time.Millisecond, func(lines []string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, lines)
		times = append(times, time.Now())
	})
	for i := 0; i < 10; i++ {
		throttle.add(fmt.Sprintf("early %d", i))
	}
	time.Sleep(130 * time.Millisecond) // the first event leaves at 100 ms
	for i := 0; i < 1200; i++ {
		throttle.add(fmt.Sprintf("burst %d", i))
	}
	throttle.add(strings.Repeat("y", 5000))
	throttle.stop()

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2 (one per interval with lines)", len(events))
	}
	if len(events[0]) != 10 || events[0][0] != "early 0" {
		t.Fatalf("first event = %q", events[0])
	}
	last := events[1]
	if len(last) != buildLogMaxLines || !strings.HasPrefix(last[0], "[… 1152 lines skipped") {
		t.Fatalf("burst event has %d lines, first %q", len(last), last[0])
	}
	if tail := last[len(last)-1]; len(tail) != maxBuildLogLine+len("…") || last[len(last)-2] != "burst 1199" {
		t.Fatalf("burst event ends with %q / %q", last[len(last)-2], tail[:20])
	}
	if gap := times[1].Sub(times[0]); gap < 95*time.Millisecond { // stop keeps the gap
		t.Fatalf("events only %v apart", gap)
	}
}
