package server

import (
	"bytes"
	"testing"

	"aurago/internal/flows"
)

func TestFB4SSECacheScrubsAnEventOnce(t *testing.T) {
	calls := 0
	orig := flowScrub
	flowScrub = func(s string) string { calls++; return orig(s) }
	t.Cleanup(func() { flowScrub = orig })
	var c flowSSECache
	ev := flows.RunEvent{Seq: 3, RunID: "run_fb4cache0001", Type: flows.EventStepFinished, NodeID: "n_aaaaaaaa",
		Step: &flows.StepRecord{NodeID: "n_aaaaaaaa", Output: map[string]any{"text": "a text long enough to scrub"}}}
	first, err := c.scrubbedEvent(ev)
	if err != nil || calls == 0 {
		t.Fatalf("first scrub: %v, %d calls", err, calls)
	}
	n := calls
	second, err := c.scrubbedEvent(ev)
	if err != nil || calls != n || !bytes.Equal(first, second) {
		t.Fatalf("second stream scrubbed again (%d -> %d calls) or got other bytes", n, calls)
	}
}

func TestFB4FlowStreamsAreCappedPerClient(t *testing.T) {
	var l flowStreamLimiter
	for i := 0; i < flowStreamsPerClient; i++ {
		if !l.acquireFor("run_fb4client"+string(rune('a'+i)), "192.0.2.1") {
			t.Fatalf("stream %d refused", i)
		}
	}
	if l.acquireFor("run_fb4clientz", "192.0.2.1") {
		t.Fatal("one client got more than flowStreamsPerClient streams")
	}
	if !l.acquireFor("run_fb4clientz", "192.0.2.2") {
		t.Fatal("another client was refused")
	}
	l.releaseFor("run_fb4clienta", "192.0.2.1")
	if !l.acquireFor("run_fb4clienty", "192.0.2.1") {
		t.Fatal("a released stream was not given back to its client")
	}
}
