package flows

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// receiveNow takes the next event of ch without blocking: ready is false when nothing is
// buffered and the channel is still open, open is false once it is closed and drained.
func receiveNow(ch <-chan RunEvent) (ev RunEvent, open, ready bool) {
	select {
	case ev, open = <-ch:
		return ev, open, true
	default:
		return RunEvent{}, true, false
	}
}

func TestEventBusBacklogAndLiveDelivery(t *testing.T) {
	now := time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)
	bus := NewEventBus(time.Minute, func() time.Time { return now })
	if _, _, _, ok := bus.Subscribe("run_x", 0); ok {
		t.Fatal("unknown runs cannot be subscribed")
	}
	bus.Open("run_x")
	bus.Publish(RunEvent{RunID: "run_x", Seq: 1, Type: EventRunStarted})
	bus.Publish(RunEvent{RunID: "run_x", Seq: 2, Type: EventStepStarted})
	backlog, ch, cancel, ok := bus.Subscribe("run_x", 1)
	if !ok || len(backlog) != 1 || backlog[0].Seq != 2 {
		t.Fatalf("backlog = %+v ok=%v", backlog, ok)
	}
	bus.Publish(RunEvent{RunID: "run_x", Seq: 3, Type: EventRunFinished})
	if ev := <-ch; ev.Seq != 3 {
		t.Fatalf("live event = %+v", ev)
	}
	bus.Finish("run_x")
	if _, open := <-ch; open {
		t.Fatal("Finish must close subscriber channels")
	}
	cancel()

	backlog, ch, _, ok = bus.Subscribe("run_x", 0)
	if !ok || len(backlog) != 3 {
		t.Fatalf("finished run backlog = %d ok=%v", len(backlog), ok)
	}
	if _, open := <-ch; open {
		t.Fatal("subscribing to a finished run returns a closed channel")
	}
	bus.Publish(RunEvent{RunID: "run_x", Seq: 4})
	if backlog, _, _, _ = bus.Subscribe("run_x", 0); len(backlog) != 3 {
		t.Fatal("events after Finish are ignored")
	}

	now = now.Add(2 * time.Minute)
	if removed := bus.Sweep(); removed != 1 || bus.Has("run_x") {
		t.Fatalf("Sweep removed %d", removed)
	}
}

func TestEventBusDropsSlowSubscribers(t *testing.T) {
	bus := NewEventBus(time.Minute, nil)
	bus.Open("run_y")
	_, ch, cancel, _ := bus.Subscribe("run_y", 0)
	defer cancel()
	for i := 1; i <= subscriberBuffer+1; i++ {
		bus.Publish(RunEvent{RunID: "run_y", Seq: i})
	}
	count := 0
	for range ch {
		count++
	}
	if count != subscriberBuffer {
		t.Fatalf("slow subscriber received %d events before being dropped, want %d", count, subscriberBuffer)
	}
}

// A subscriber that was dropped for being slow resubscribes with its last seen Seq and
// gets the rest of the log; the dropped channel carried no run_finished event.
func TestEventBusDroppedSubscriberResumesFromLastSeq(t *testing.T) {
	bus := NewEventBus(time.Minute, nil)
	bus.Open("run_r")
	_, ch, cancel, _ := bus.Subscribe("run_r", 0)
	defer cancel()
	total := subscriberBuffer + 10
	for i := 1; i < total; i++ {
		bus.Publish(RunEvent{RunID: "run_r", Seq: i, Type: EventStepStarted})
	}
	bus.Publish(RunEvent{RunID: "run_r", Seq: total, Type: EventRunFinished})
	last, sawFinished := 0, false
	for ev := range ch {
		last = ev.Seq
		sawFinished = sawFinished || ev.Type == EventRunFinished
	}
	if last != subscriberBuffer || sawFinished {
		t.Fatalf("dropped subscriber saw up to Seq %d (finished=%v), want %d without run_finished", last, sawFinished, subscriberBuffer)
	}
	backlog, _, _, ok := bus.Subscribe("run_r", last)
	if !ok || len(backlog) != total-subscriberBuffer || backlog[0].Seq != subscriberBuffer+1 || backlog[len(backlog)-1].Type != EventRunFinished {
		t.Fatalf("resumed backlog = %d events ok=%v, want %d ending in run_finished", len(backlog), ok, total-subscriberBuffer)
	}
}

// The cancel func closes its channel at most once, whatever ended the subscription before,
// and publishing after a cancel never sends on the closed channel.
func TestEventBusCancelIsIdempotentInEveryState(t *testing.T) {
	bus := NewEventBus(time.Minute, nil)
	bus.Open("run_c")
	bus.Publish(RunEvent{RunID: "run_c", Seq: 1})

	_, ch, cancel, _ := bus.Subscribe("run_c", 0)
	cancel()
	cancel()
	bus.Publish(RunEvent{RunID: "run_c", Seq: 2})
	if _, open, ready := receiveNow(ch); open || !ready {
		t.Fatalf("cancel must close the channel without delivering later events (open=%v ready=%v)", open, ready)
	}

	_, dropped, cancelDropped, _ := bus.Subscribe("run_c", 0)
	for i := 3; i <= 3+subscriberBuffer; i++ {
		bus.Publish(RunEvent{RunID: "run_c", Seq: i})
	}
	cancelDropped()
	cancelDropped()
	if n := len(dropped); n != subscriberBuffer {
		t.Fatalf("dropped channel holds %d events, want %d", n, subscriberBuffer)
	}

	_, finished, cancelFinished, _ := bus.Subscribe("run_c", 0)
	bus.Finish("run_c")
	bus.Finish("run_c")
	cancelFinished()
	cancelFinished()
	cancelDropped()
	if _, open, ready := receiveNow(finished); open || !ready {
		t.Fatalf("Finish must close the channel (open=%v ready=%v)", open, ready)
	}
}

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

func TestEventBusSubscribeUnknownRunReturnsNothing(t *testing.T) {
	bus := NewEventBus(time.Minute, nil)
	backlog, ch, cancel, ok := bus.Subscribe("run_missing", 0)
	if ok || backlog != nil || ch != nil || cancel == nil {
		t.Fatalf("unknown run: backlog=%v ch=%v cancel nil=%v ok=%v", backlog, ch, cancel == nil, ok)
	}
	cancel()
}

// The backlog is a copy of the log, cut at the first Seq after afterSeq; changing the returned
// slice must not change what later subscribers get.
func TestEventBusBacklogIsACopyCutAtAfterSeq(t *testing.T) {
	bus := NewEventBus(time.Minute, nil)
	bus.Open("run_b")
	for i := 1; i <= 5; i++ {
		bus.Publish(RunEvent{RunID: "run_b", Seq: i, Type: EventStepStarted})
	}
	for after, want := range map[int]int{-1: 5, 0: 5, 1: 4, 4: 1, 5: 0, 6: 0, 1000: 0} {
		backlog, _, cancel, _ := bus.Subscribe("run_b", after)
		cancel()
		if len(backlog) != want {
			t.Fatalf("after %d: %d events, want %d", after, len(backlog), want)
		}
		if want > 0 && backlog[0].Seq != 5-want+1 {
			t.Fatalf("after %d: backlog starts at Seq %d", after, backlog[0].Seq)
		}
	}
	backlog, _, cancel, _ := bus.Subscribe("run_b", 0)
	cancel()
	backlog[0].Type = "tampered"
	backlog[1] = RunEvent{}
	again, _, cancel, _ := bus.Subscribe("run_b", 0)
	cancel()
	if again[0].Type != EventStepStarted || again[1].Seq != 2 {
		t.Fatalf("the log changed through a backlog: %+v %+v", again[0], again[1])
	}
}

// Publishers, subscribers, cancels, Sweep and Finish run from separate goroutines. Whatever
// the interleaving, every subscriber receives the events of its run as one gap-free,
// in-order stretch of the Seq sequence, and nothing panics (a double close or a send on a
// closed channel would). It is also a target for the -race run on the test server.
func TestEventBusConcurrentPublishSubscribeCancel(t *testing.T) {
	const (
		runCount         = 3
		publishersPerRun = 2
		stepsPerWorker   = 200
		cancellersPerRun = 4
		total            = 1 + publishersPerRun*stepsPerWorker + 1 // run_started, steps, run_finished
	)
	if total <= subscriberBuffer {
		t.Fatalf("the test needs more than subscriberBuffer events per run, has %d", total)
	}
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	retain := time.Minute
	bus := NewEventBus(retain, clock.Now)

	var wg sync.WaitGroup
	spawn := func(who string, fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if v := recover(); v != nil {
					t.Errorf("%s panicked: %v", who, v)
				}
			}()
			fn()
		}()
	}
	expectRange := func(who string, seqs []int, first, last int) {
		if len(seqs) != last-first+1 {
			t.Errorf("%s received %d events, want %d (Seq %d..%d)", who, len(seqs), last-first+1, first, last)
			return
		}
		for i, seq := range seqs {
			if seq != first+i {
				t.Errorf("%s: event %d has Seq %d, want %d", who, i, seq, first+i)
				return
			}
		}
	}

	runIDs := make([]string, runCount)
	finished := make([]chan struct{}, runCount)
	var cancelMu sync.Mutex
	var cancels []func()
	keepCancel := func(c func()) {
		cancelMu.Lock()
		cancels = append(cancels, c)
		cancelMu.Unlock()
	}

	// Slow consumers subscribe before the first event, so their drop is certain.
	type stalled struct {
		ch     <-chan RunEvent
		cancel func()
	}
	stalledSubs := make([]stalled, runCount)
	for r := range runIDs {
		runIDs[r] = "run_conc_" + string(rune('a'+r))
		finished[r] = make(chan struct{})
		bus.Open(runIDs[r])
		_, ch, cancel, ok := bus.Subscribe(runIDs[r], 0)
		if !ok {
			t.Fatalf("Subscribe %s", runIDs[r])
		}
		stalledSubs[r] = stalled{ch: ch, cancel: cancel}
		keepCancel(cancel)
	}

	stopSweep := make(chan struct{})
	var stopOnce sync.Once
	stopSweeping := func() { stopOnce.Do(func() { close(stopSweep) }) }
	var sweeper sync.WaitGroup
	t.Cleanup(func() { // also on the failure path below
		stopSweeping()
		sweeper.Wait()
	})
	sweeper.Add(1)
	go func() {
		defer sweeper.Done()
		for {
			select {
			case <-stopSweep:
				return
			default:
			}
			// The clock stands still, so nothing is old enough to be forgotten.
			if n := bus.Sweep(); n != 0 {
				t.Errorf("Sweep removed %d logs while every run is young", n)
				return
			}
			runtime.Gosched()
		}
	}()

	for r := range runIDs {
		runID, done, slow := runIDs[r], finished[r], stalledSubs[r]

		spawn("coordinator "+runID, func() {
			var mu sync.Mutex // gives the publishers' Seq numbers the order of their Publish calls
			seq := 0
			publish := func(typ string) {
				mu.Lock()
				defer mu.Unlock()
				seq++
				bus.Publish(RunEvent{RunID: runID, Seq: seq, Type: typ})
			}
			publish(EventRunStarted)
			var workers sync.WaitGroup
			for p := 0; p < publishersPerRun; p++ {
				workers.Add(1)
				go func() {
					defer workers.Done()
					for i := 0; i < stepsPerWorker; i++ {
						publish(EventStepStarted)
					}
				}()
			}
			workers.Wait()
			publish(EventRunFinished)
			bus.Finish(runID)
			close(done)
		})

		spawn("slow subscriber "+runID, func() {
			<-done // reads nothing while the run publishes, so the bus must drop it
			var seqs []int
			sawFinished := false
			for ev := range slow.ch {
				seqs = append(seqs, ev.Seq)
				sawFinished = sawFinished || ev.Type == EventRunFinished
			}
			expectRange("slow subscriber "+runID, seqs, 1, subscriberBuffer)
			if sawFinished {
				t.Errorf("slow subscriber %s was dropped yet saw run_finished", runID)
			}
			slow.cancel() // after the drop and after Finish
			slow.cancel()
			backlog, ch, cancel, ok := bus.Subscribe(runID, subscriberBuffer)
			if !ok {
				t.Errorf("slow subscriber %s cannot resume", runID)
				return
			}
			defer cancel()
			rest := make([]int, 0, len(backlog))
			for _, ev := range backlog {
				rest = append(rest, ev.Seq)
			}
			expectRange("resumed slow subscriber "+runID, rest, subscriberBuffer+1, total)
			if len(backlog) == 0 || backlog[len(backlog)-1].Type != EventRunFinished {
				t.Errorf("resumed backlog of %s does not end in run_finished", runID)
			}
			if _, open, ready := receiveNow(ch); open || !ready {
				t.Errorf("a finished run must hand out a closed channel (%s)", runID)
			}
		})

		for f := 0; f < 2; f++ {
			who := "follower " + runID + string(rune('0'+f))
			spawn(who, func() {
				var seqs []int
				last, sawFinished := 0, false
				for !sawFinished {
					backlog, ch, cancel, ok := bus.Subscribe(runID, last)
					if !ok {
						t.Errorf("%s: the run vanished", who)
						return
					}
					keepCancel(cancel)
					take := func(ev RunEvent) {
						seqs = append(seqs, ev.Seq)
						last = ev.Seq
						sawFinished = sawFinished || ev.Type == EventRunFinished
					}
					for _, ev := range backlog {
						take(ev)
					}
					for ev := range ch { // closed by a drop, or by Finish
						take(ev)
					}
				}
				expectRange(who, seqs, 1, total)
			})
		}

		for c := 0; c < cancellersPerRun; c++ {
			who := "canceller " + runID + string(rune('0'+c))
			want := (c + 1) * 60 // events read before leaving; Finish may end the subscription first
			spawn(who, func() {
				backlog, ch, cancel, ok := bus.Subscribe(runID, 0)
				if !ok {
					t.Errorf("%s: the run vanished", who)
					return
				}
				keepCancel(cancel)
				var seqs []int
				for _, ev := range backlog {
					seqs = append(seqs, ev.Seq)
				}
				for len(seqs) < want {
					ev, open := <-ch
					if !open {
						break
					}
					seqs = append(seqs, ev.Seq)
				}
				cancel()
				cancel()
				for ev := range ch { // what was buffered before the cancel
					seqs = append(seqs, ev.Seq)
				}
				if len(seqs) > 0 {
					expectRange(who, seqs, 1, len(seqs))
				}
			})
		}
	}

	allDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(allDone)
	}()
	select {
	case <-allDone:
	case <-time.After(30 * time.Second):
		t.Fatal("concurrent subscribers did not finish; a channel was probably never closed")
	}
	stopSweeping()
	sweeper.Wait()

	for _, runID := range runIDs {
		backlog, ch, cancel, ok := bus.Subscribe(runID, 0)
		if !ok || len(backlog) != total {
			t.Fatalf("late subscriber of %s: ok=%v events=%d, want %d", runID, ok, len(backlog), total)
		}
		if _, open, ready := receiveNow(ch); open || !ready {
			t.Fatalf("late subscriber of %s got an open channel", runID)
		}
		cancel()
		bus.Publish(RunEvent{RunID: runID, Seq: total + 1})
		if after, _, _, _ := bus.Subscribe(runID, total); len(after) != 0 {
			t.Fatalf("%s accepted an event after Finish", runID)
		}
	}

	clock.Advance(retain + time.Nanosecond)
	if removed := bus.Sweep(); removed != runCount {
		t.Fatalf("Sweep removed %d finished runs, want %d", removed, runCount)
	}
	cancelMu.Lock()
	defer cancelMu.Unlock()
	for _, c := range cancels {
		c() // every subscription is long over, and its run is gone
	}
	for _, runID := range runIDs {
		if bus.Has(runID) {
			t.Fatalf("%s survived Sweep", runID)
		}
	}
}
