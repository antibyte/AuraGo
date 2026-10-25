package server

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestMissionUpdateDebouncerSurvivesAPanickingSend(t *testing.T) {
	var queued []func()
	d := &missionUpdateDebouncer{schedule: func(_ time.Duration, fn func()) { queued = append(queued, fn) }}
	var calls atomic.Int32
	send := func() {
		if calls.Add(1) == 2 {
			panic("boom")
		}
	}
	d.fire(send) // leading send
	d.fire(send) // folded into the trailing send
	queued[0]()  // the trailing send panics; the debouncer must not
	d.fire(send) // window state is consistent: a new call is folded again, not lost
	if len(queued) < 2 {
		t.Fatalf("no window armed after the panic: %d", len(queued))
	}
	queued[len(queued)-1]()
	if calls.Load() != 3 {
		t.Fatalf("sends = %d, want 3", calls.Load())
	}
}
