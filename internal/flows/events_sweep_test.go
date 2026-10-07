package flows

import (
	"testing"
	"time"
)

// An open log whose run never called Finish is forgotten once it has outlived the longest
// possible run plus the grace and the retention. Its subscribers are closed first, and the
// cancel funcs stay safe, also against a fresh log that reuses the run id.
func TestEventBusSweepRemovesLeakedOpenLogs(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	retain := time.Minute
	leakedAfter := MaxRunSecondsLimit*time.Second + time.Hour + retain
	bus := NewEventBus(retain, clock.Now)

	bus.Open("run_leak")
	_, ch, cancel, ok := bus.Subscribe("run_leak", 0)
	if !ok {
		t.Fatal("Subscribe on an open run")
	}
	bus.Publish(RunEvent{RunID: "run_leak", Seq: 1, Type: EventRunStarted})

	clock.Advance(retain + time.Hour)
	if removed := bus.Sweep(); removed != 0 || !bus.Has("run_leak") {
		t.Fatalf("a run within its time limit was swept: removed=%d", removed)
	}
	clock.Advance(leakedAfter - retain - time.Hour)
	if removed := bus.Sweep(); removed != 0 || !bus.Has("run_leak") {
		t.Fatalf("a log exactly at the leak limit was swept: removed=%d", removed)
	}
	bus.Open("run_young")
	bus.Open("run_leak") // a duplicate Open must not restart the leak clock

	clock.Advance(time.Nanosecond)
	if removed := bus.Sweep(); removed != 1 || bus.Has("run_leak") || !bus.Has("run_young") {
		t.Fatalf("Sweep removed %d, leaked run present=%v, young run present=%v", removed, bus.Has("run_leak"), bus.Has("run_young"))
	}
	if ev, open, ready := receiveNow(ch); !open || !ready || ev.Seq != 1 {
		t.Fatalf("buffered event of the swept run = %+v open=%v ready=%v", ev, open, ready)
	}
	if _, open, ready := receiveNow(ch); open || !ready {
		t.Fatalf("Sweep must close the subscriber channels of a leaked log (open=%v ready=%v)", open, ready)
	}

	cancel()
	cancel()
	bus.Publish(RunEvent{RunID: "run_leak", Seq: 2})
	bus.Finish("run_leak")
	if _, _, _, known := bus.Subscribe("run_leak", 0); known {
		t.Fatal("a swept run is unknown")
	}

	bus.Open("run_leak")
	_, fresh, _, _ := bus.Subscribe("run_leak", 0)
	cancel() // belongs to the old log, so it must leave the new subscriber alone
	bus.Publish(RunEvent{RunID: "run_leak", Seq: 1, Type: EventRunStarted})
	if ev, open, ready := receiveNow(fresh); !open || !ready || ev.Seq != 1 {
		t.Fatalf("fresh subscriber = %+v open=%v ready=%v", ev, open, ready)
	}
}

// Rearm restarts the leak clock of an open log, so a run whose log was opened when it was
// queued keeps the log for the full horizon after it starts. Rearm also reopens a log a
// sweep forgot, and it leaves finished logs alone.
func TestEventBusRearm(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	retain := time.Minute
	leakedAfter := MaxRunSecondsLimit*time.Second + time.Hour + retain
	bus := NewEventBus(retain, clock.Now)

	bus.Open("run_waited")
	clock.Advance(20 * time.Hour) // the run waited in a queue
	bus.Rearm("run_waited")
	clock.Advance(leakedAfter) // the whole horizon after Rearm, far beyond it after Open
	if removed := bus.Sweep(); removed != 0 || !bus.Has("run_waited") {
		t.Fatalf("a rearmed log within its horizon was swept: removed=%d", removed)
	}
	clock.Advance(time.Nanosecond)
	if removed := bus.Sweep(); removed != 1 || bus.Has("run_waited") {
		t.Fatalf("a rearmed log past its horizon survived: removed=%d", removed)
	}

	bus.Rearm("run_waited") // forgotten by the sweep, so Rearm opens it again
	_, ch, cancel, ok := bus.Subscribe("run_waited", 0)
	defer cancel()
	if !ok {
		t.Fatal("Rearm must reopen a log that a sweep forgot")
	}
	bus.Publish(RunEvent{RunID: "run_waited", Seq: 1, Type: EventRunStarted})
	if ev, open, ready := receiveNow(ch); !open || !ready || ev.Seq != 1 {
		t.Fatalf("event on the reopened log = %+v open=%v ready=%v", ev, open, ready)
	}

	bus.Open("run_done")
	bus.Publish(RunEvent{RunID: "run_done", Seq: 1, Type: EventRunFinished})
	bus.Finish("run_done")
	clock.Advance(retain)
	bus.Rearm("run_done") // must neither reopen the log nor delay its expiry
	backlog, finished, _, _ := bus.Subscribe("run_done", 0)
	if _, open, ready := receiveNow(finished); open || !ready || len(backlog) != 1 {
		t.Fatalf("finished log after Rearm: %d events, open=%v ready=%v", len(backlog), open, ready)
	}
	clock.Advance(time.Nanosecond)
	if removed := bus.Sweep(); removed != 1 || bus.Has("run_done") || !bus.Has("run_waited") {
		t.Fatalf("Sweep removed %d; finished log present=%v, reopened log present=%v",
			removed, bus.Has("run_done"), bus.Has("run_waited"))
	}
}

// The bus has no goroutine, so Open sweeps when at least the retention has passed since the
// last sweep, and not before.
func TestEventBusOpenSweepsLazily(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	retain := time.Minute
	bus := NewEventBus(retain, clock.Now)

	bus.Open("run_a") // the first Open sweeps nothing and starts the interval
	bus.Publish(RunEvent{RunID: "run_a", Seq: 1, Type: EventRunFinished})
	bus.Finish("run_a")

	clock.Advance(retain - time.Nanosecond)
	bus.Open("run_b")
	if !bus.Has("run_a") {
		t.Fatal("a finished log inside the retention was swept")
	}

	clock.Advance(time.Nanosecond) // a sweep is due now, but the log is only as old as the retention
	bus.Open("run_c")
	if !bus.Has("run_a") {
		t.Fatal("a log exactly as old as the retention was swept")
	}

	clock.Advance(time.Nanosecond) // the log has expired, but the last sweep was a moment ago
	bus.Open("run_d")
	if !bus.Has("run_a") {
		t.Fatal("Open swept again before the retention passed since the last sweep")
	}

	clock.Advance(retain - time.Nanosecond) // a full retention since the last sweep
	bus.Open("run_e")
	if bus.Has("run_a") {
		t.Fatal("Open must forget a finished log once the retention has passed")
	}
	if _, _, _, ok := bus.Subscribe("run_a", 0); ok {
		t.Fatal("a swept run is unknown")
	}
	for _, id := range []string{"run_b", "run_c", "run_d", "run_e"} {
		if !bus.Has(id) {
			t.Fatalf("%s is open and young, but it was forgotten", id)
		}
	}

	// An explicit Sweep counts as a sweep for the interval.
	bus.Finish("run_d")
	clock.Advance(retain)
	if removed := bus.Sweep(); removed != 0 {
		t.Fatalf("Sweep removed %d logs that are not older than the retention", removed)
	}
	clock.Advance(time.Nanosecond) // run_d has expired, but a sweep ran a moment ago
	bus.Open("run_f")
	if !bus.Has("run_d") {
		t.Fatal("Open swept again right after an explicit Sweep")
	}
	clock.Advance(retain)
	bus.Open("run_g")
	if bus.Has("run_d") {
		t.Fatal("Open must forget the expired log once the retention has passed since the explicit Sweep")
	}
}
