package server

import (
	"testing"
	"time"
)

func TestFB4MissionUpdatesAreDebounced(t *testing.T) {
	var pending []func()
	d := missionUpdateDebouncer{schedule: func(_ time.Duration, fn func()) { pending = append(pending, fn) }}
	sent := 0
	send := func() { sent++ }
	d.fire(send)
	d.fire(send)
	d.fire(send)
	if sent != 1 || len(pending) != 1 {
		t.Fatalf("leading edge: sent %d, windows %d; want 1 and 1", sent, len(pending))
	}
	next := pending[0]
	pending = pending[1:]
	next()
	if sent != 2 || len(pending) != 1 {
		t.Fatalf("trailing edge: sent %d, windows %d; want 2 and a new window", sent, len(pending))
	}
	next = pending[0]
	pending = nil
	next()
	if sent != 2 || len(pending) != 0 {
		t.Fatalf("quiet window: sent %d, windows %d", sent, len(pending))
	}
	d.fire(send)
	if sent != 3 {
		t.Fatalf("after a quiet window the next call must send at once, sent %d", sent)
	}
}
