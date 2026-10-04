package flows

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestTimerServiceStartTwiceRunsOneLoop(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	calls := &timerCalls{}
	svc := h.service(t, calls.fire, calls.missed, nil)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow.Add(time.Hour)})
	for i := 1; i <= 2; i++ {
		if err := svc.Start(ctx); err != nil {
			t.Fatalf("Start #%d: %v", i, err)
		}
	}
	h.clock.WaitForWaiters(t, 1)
	time.Sleep(20 * time.Millisecond) // a second loop would have planned its own wait by now
	if got := h.clock.pending(); got != 1 {
		t.Fatalf("%d loops are waiting, want one", got)
	}
	h.clock.Advance(time.Hour)
	waitUntil(t, "the timer to be removed", func() bool { return len(h.timers(t)) == 0 })
	svc.Stop()
	svc.Stop()
	calls.assertCalls(t, "fire:n_aaaaaaab")
}

func TestTimerServiceConcurrentStartRunsOneLoop(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	svc := h.service(t, nil, nil, nil)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow.Add(time.Hour)})
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < cap(errs); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- svc.Start(ctx)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
	}
	h.clock.WaitForWaiters(t, 1)
	time.Sleep(20 * time.Millisecond)
	if got := h.clock.pending(); got != 1 {
		t.Fatalf("%d loops are waiting, want one", got)
	}
	svc.Stop()
}

func TestTimerServiceStartAfterStopIsRefused(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	calls := &timerCalls{}

	started := h.service(t, calls.fire, calls.missed, nil)
	if err := started.Start(ctx); err != nil {
		t.Fatal(err)
	}
	started.Stop()
	neverStarted := h.service(t, calls.fire, calls.missed, nil)
	neverStarted.Stop()

	// A due timer must neither fire nor be removed, and no loop may be left behind.
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow.Add(-time.Minute)})
	for name, svc := range map[string]*TimerService{"stopped after Start": started, "stopped before Start": neverStarted} {
		if err := svc.Start(ctx); !errors.Is(err, errTimerServiceStopped) {
			t.Fatalf("%s: Start = %v, want errTimerServiceStopped", name, err)
		}
	}
	calls.assertCalls(t)
	if list := h.timers(t); len(list) != 1 {
		t.Fatalf("the refused Start must leave the timer alone, got %+v", list)
	}
	if got := h.clock.afterCalls(); len(got) != 0 {
		t.Fatalf("a loop was started after Stop: After calls = %v", got)
	}
}

func TestTimerServiceStopDuringStartupKeepsLoopFromStarting(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	svc := h.service(t, func(string, string, time.Time) {
		entered <- struct{}{}
		<-release
	}, nil, nil)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow.Add(-time.Minute)})
	startErr := make(chan error, 1)
	go func() { startErr <- svc.Start(ctx) }()

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("the start-up pass did not reach the callback")
	}
	svc.Stop() // there is no loop yet, so this returns at once
	close(release)
	select {
	case err := <-startErr:
		if !errors.Is(err, errTimerServiceStopped) {
			t.Fatalf("Start = %v, want errTimerServiceStopped", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return")
	}
	if got := h.clock.afterCalls(); len(got) != 0 {
		t.Fatalf("a loop was started after Stop: After calls = %v", got)
	}
}

func TestTimerServiceFailedStartCanBeRetried(t *testing.T) {
	h := newHardTimers(t, storeNow)
	ctx := context.Background()
	svc := h.service(t, nil, nil, nil)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: storeNow.Add(time.Hour)})
	h.hideTimerTable(t)
	if err := svc.Start(ctx); err == nil {
		t.Fatal("Start must report that the timers cannot be read")
	}
	if got := h.clock.afterCalls(); len(got) != 0 {
		t.Fatalf("a failed Start must not leave a loop behind: After calls = %v", got)
	}
	h.restoreTimerTable(t)
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start after the failure: %v", err)
	}
	h.clock.WaitForWaiters(t, 1)
}

func TestTimerServiceNextYearly(t *testing.T) {
	utc := func(y int, m time.Month, d, hh, mm int) time.Time { return time.Date(y, m, d, hh, mm, 0, 0, time.UTC) }
	cet := time.FixedZone("CET", 3600)
	cases := []struct {
		name      string
		from, now time.Time
		want      time.Time
	}{
		{"an ordinary date moves one year", utc(2026, 10, 3, 7, 1), utc(2026, 10, 3, 7, 1), utc(2027, 10, 3, 7, 1)},
		{"skips every year it was overdue", utc(2020, 3, 5, 9, 0), utc(2026, 10, 4, 0, 0), utc(2027, 3, 5, 9, 0)},
		{"a time in the future is kept", utc(2027, 1, 1, 0, 0), utc(2026, 10, 4, 0, 0), utc(2027, 1, 1, 0, 0)},
		{"Feb 29 waits for the next leap day", utc(2028, 2, 29, 12, 0), utc(2028, 2, 29, 12, 0), utc(2032, 2, 29, 12, 0)},
		{"Feb 29 of a long overdue timer", utc(2024, 2, 29, 12, 0), utc(2026, 10, 4, 0, 0), utc(2028, 2, 29, 12, 0)},
		{"Feb 29 skips the century that is no leap year", utc(2096, 2, 29, 12, 0), utc(2096, 3, 1, 0, 0), utc(2104, 2, 29, 12, 0)},
		{"Feb 28 stays on Feb 28", utc(2027, 2, 28, 12, 0), utc(2027, 2, 28, 12, 0), utc(2028, 2, 28, 12, 0)},
		{"keeps the zone and the time of day", time.Date(2027, 3, 1, 0, 30, 0, 0, cet), utc(2027, 3, 1, 0, 0), time.Date(2028, 3, 1, 0, 30, 0, 0, cet)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nextYearly(tc.from, tc.now)
			if got.Format(time.RFC3339Nano) != tc.want.Format(time.RFC3339Nano) {
				t.Fatalf("nextYearly(%s, %s) = %s, want %s", tc.from.Format(time.RFC3339), tc.now.Format(time.RFC3339),
					got.Format(time.RFC3339), tc.want.Format(time.RFC3339))
			}
		})
	}
}

func TestTimerServiceLeapDayTimerStaysOnLeapDay(t *testing.T) {
	leapDay := time.Date(2028, 2, 29, 12, 0, 0, 0, time.UTC)
	h := newHardTimers(t, leapDay.Add(-time.Hour))
	calls := &timerCalls{}
	svc := h.service(t, calls.fire, calls.missed, nil)
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaab", FireAt: leapDay, Repeat: RepeatYearly})
	if err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.clock.WaitForWaiters(t, 1)
	h.clock.Advance(time.Hour)

	// AddDate would turn Feb 29 into Mar 1 and the timer would stay there for good.
	want := time.Date(2032, 2, 29, 12, 0, 0, 0, time.UTC)
	waitUntil(t, "the leap day timer to move to the next leap day", func() bool {
		list := h.timers(t)
		return len(list) == 1 && list[0].FireAt.Equal(want) && list[0].Repeat == RepeatYearly
	})
	calls.assertCalls(t, "fire:n_aaaaaaab")
}
