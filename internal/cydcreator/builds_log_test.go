package cydcreator

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
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

func TestBuildLogNeverExceedsTheCap(t *testing.T) {
	var l buildLog
	room := MaxBuildLogBytes - len(buildLogTruncated)
	l.add(strings.Repeat("a", room-1)) // line plus newline fill the room exactly
	if text := l.String(); len(text) != room || strings.Contains(text, "truncated") {
		t.Fatalf("log of %d bytes after a fitting line, want %d without marker", len(text), room)
	}
	l.add("b")
	text := l.String()
	if len(text) != MaxBuildLogBytes || !strings.HasSuffix(text, buildLogTruncated) {
		t.Fatalf("log of %d bytes (cap %d), suffix %q", len(text), MaxBuildLogBytes, text[len(text)-40:])
	}
	l.add("c")
	if l.String() != text {
		t.Fatal("a line was stored after the truncation marker")
	}

	var full buildLog // the 1023-byte lines of TestBuildLogCapsTheStoredLog end exactly at the cap
	for i := 0; i < 3000; i++ {
		full.add(strings.Repeat("x", 1023))
	}
	if n := len(full.String()); n > MaxBuildLogBytes {
		t.Fatalf("log of %d bytes exceeds the cap %d", n, MaxBuildLogBytes)
	}
}

func TestClipLogLineKeepsRunesWhole(t *testing.T) {
	cases := []struct {
		name string
		line string
		keep int // bytes kept before "…"; -1 = unchanged
	}{
		{"short", "short line", -1},
		{"exact", strings.Repeat("x", maxBuildLogLine), -1},
		{"ascii", strings.Repeat("y", 5000), maxBuildLogLine},
		{"two-byte", "a" + strings.Repeat("é", 3000), maxBuildLogLine - 1},
		{"three-byte", "ab" + strings.Repeat("€", 2000), maxBuildLogLine - 2},
		{"four-byte", "a" + strings.Repeat("\U0001F600", 2000), maxBuildLogLine - 3},
		{"boundary", "a" + strings.Repeat("€", 2000), maxBuildLogLine},
		{"invalid", strings.Repeat("\x80", 5000), maxBuildLogLine - 3},
	}
	for _, c := range cases {
		got := clipLogLine(c.line)
		if c.keep < 0 {
			if got != c.line {
				t.Errorf("%s: changed a line that fits", c.name)
			}
			continue
		}
		if got != c.line[:c.keep]+"…" {
			t.Errorf("%s: kept %d bytes, want %d plus the marker", c.name, len(got)-len("…"), c.keep)
		}
		if c.name != "invalid" && !utf8.ValidString(got) {
			t.Errorf("%s: clipped line is not valid UTF-8", c.name)
		}
	}

	var got []string
	throttle := newLogThrottle(time.Hour, func(lines []string) { got = append(got, lines...) })
	throttle.add("ab" + strings.Repeat("€", 2000))
	throttle.stop()
	if len(got) != 1 || !utf8.ValidString(got[0]) || !strings.HasSuffix(got[0], "€…") {
		t.Fatalf("throttled long line = %d lines, valid=%v", len(got), len(got) == 1 && utf8.ValidString(got[0]))
	}
}

func TestLogThrottleBoundsPendingLines(t *testing.T) {
	var events [][]string
	throttle := newLogThrottle(time.Hour, func(lines []string) { events = append(events, lines) })
	for i := 0; i < 1000; i++ {
		throttle.add(fmt.Sprintf("line %d", i))
	}
	throttle.mu.Lock()
	pending, skipped := len(throttle.pending), throttle.skipped
	throttle.mu.Unlock()
	if pending != buildLogMaxPending || skipped != 1000-buildLogMaxPending {
		t.Fatalf("pending %d, skipped %d; want %d and %d", pending, skipped, buildLogMaxPending, 1000-buildLogMaxPending)
	}
	throttle.stop()
	if len(events) != 1 || len(events[0]) != buildLogMaxLines || events[0][0] != "[… 951 lines skipped, the full log is in build.log …]" ||
		events[0][1] != "line 951" || events[0][buildLogMaxLines-1] != "line 999" {
		t.Fatalf("events = %d, first %q", len(events), events)
	}
}

func TestLogThrottleStopTwice(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	throttle := newLogThrottle(time.Hour, func([]string) { mu.Lock(); calls++; mu.Unlock() })
	throttle.add("last line")
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); throttle.stop() }()
	}
	wg.Wait()
	throttle.stop()
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatalf("emit called %d times, want the final flush once", calls)
	}
}
