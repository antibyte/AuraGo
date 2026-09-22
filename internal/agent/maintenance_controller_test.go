package agent

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
)

type maintenanceControllerTestTimer struct {
	ch      chan time.Time
	stopped atomic.Bool
}

func (t *maintenanceControllerTestTimer) Chan() <-chan time.Time { return t.ch }
func (t *maintenanceControllerTestTimer) Stop() bool {
	return !t.stopped.Swap(true)
}

type maintenanceControllerTestClock struct {
	mu        sync.Mutex
	now       time.Time
	timers    []*maintenanceControllerTestTimer
	durations []time.Duration
}

func (c *maintenanceControllerTestClock) timer(duration time.Duration) maintenanceControllerTimer {
	t := &maintenanceControllerTestTimer{ch: make(chan time.Time, 1)}
	c.mu.Lock()
	c.timers = append(c.timers, t)
	c.durations = append(c.durations, duration)
	c.mu.Unlock()
	return t
}

func (c *maintenanceControllerTestClock) timerCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}

func (c *maintenanceControllerTestClock) fireLatest() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.timers) - 1; i >= 0; i-- {
		if !c.timers[i].stopped.Load() {
			c.timers[i].ch <- c.now
			return true
		}
	}
	return false
}

func maintenanceControllerTestConfig(enabled bool) *config.Config {
	cfg := &config.Config{}
	cfg.Maintenance.Enabled = enabled
	cfg.Maintenance.Time = "04:00"
	return cfg
}

func maintenanceControllerTestClaimDay(time.Time) (bool, error) {
	return true, nil
}

func TestCloneMaintenanceConfigPreservesNilAndRuntimeFields(t *testing.T) {
	if clone, err := cloneMaintenanceConfig(nil); err != nil || clone != nil {
		t.Fatalf("nil clone = (%v, %v), want (nil, nil)", clone, err)
	}
	source := &config.Config{}
	source.Runtime.IsDocker = true
	source.Runtime.ProtectSystemStrict = true
	clone, err := cloneMaintenanceConfig(source)
	if err != nil {
		t.Fatalf("clone config: %v", err)
	}
	if clone == source || !clone.Runtime.IsDocker || !clone.Runtime.ProtectSystemStrict {
		t.Fatalf("clone runtime = %#v, source=%p clone=%p", clone.Runtime, source, clone)
	}
}

func waitMaintenanceController(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("maintenance controller condition did not become true")
}

func TestMaintenanceControllerStartsDisabledAndHotEnables(t *testing.T) {
	clock := &maintenanceControllerTestClock{now: time.Date(2026, 6, 10, 3, 59, 0, 0, time.UTC)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var runs atomic.Int32
	controller := newMaintenanceController(ctx, maintenanceControllerTestConfig(false), MaintenanceControllerDependencies{}, maintenanceControllerOptions{
		now:          func() time.Time { return clock.now },
		timerFactory: clock.timer,
		claimDay:     maintenanceControllerTestClaimDay,
		run: func(context.Context, *config.Config) {
			runs.Add(1)
		},
	})
	defer controller.Stop(context.Background())

	time.Sleep(5 * time.Millisecond)
	if got := clock.timerCount(); got != 0 {
		t.Fatalf("disabled controller created %d timers", got)
	}
	controller.UpdateConfig(maintenanceControllerTestConfig(true))
	waitMaintenanceController(t, func() bool { return clock.timerCount() == 1 })
	if !clock.fireLatest() {
		t.Fatal("failed to fire enabled maintenance timer")
	}
	waitMaintenanceController(t, func() bool { return runs.Load() == 1 })
}

func TestMaintenanceControllerDisableCancelsActiveRunAndPreventsOverlap(t *testing.T) {
	clock := &maintenanceControllerTestClock{now: time.Date(2026, 6, 10, 3, 59, 0, 0, time.UTC)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	cancelled := make(chan struct{})
	var runs atomic.Int32
	controller := newMaintenanceController(ctx, maintenanceControllerTestConfig(true), MaintenanceControllerDependencies{}, maintenanceControllerOptions{
		now:          func() time.Time { return clock.now },
		timerFactory: clock.timer,
		claimDay:     maintenanceControllerTestClaimDay,
		run: func(runCtx context.Context, _ *config.Config) {
			runs.Add(1)
			close(started)
			<-runCtx.Done()
			close(cancelled)
		},
	})
	defer controller.Stop(context.Background())

	waitMaintenanceController(t, func() bool { return clock.timerCount() == 1 })
	if !clock.fireLatest() {
		t.Fatal("failed to fire maintenance timer")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("maintenance run did not start")
	}
	controller.UpdateConfig(maintenanceControllerTestConfig(false))
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("disabling maintenance did not cancel the active run")
	}
	waitMaintenanceController(t, func() bool {
		status := controller.Status()
		return !status.Running && status.NextRun.IsZero()
	})
	if got := runs.Load(); got != 1 {
		t.Fatalf("run count after disable = %d, want 1", got)
	}
}

func TestMaintenanceControllerHotReloadRecomputesNextRun(t *testing.T) {
	clock := &maintenanceControllerTestClock{now: time.Date(2026, 6, 10, 3, 59, 0, 0, time.UTC)}
	controller := newMaintenanceController(context.Background(), maintenanceControllerTestConfig(true), MaintenanceControllerDependencies{}, maintenanceControllerOptions{
		now:          func() time.Time { return clock.now },
		timerFactory: clock.timer,
		claimDay:     maintenanceControllerTestClaimDay,
		run:          func(context.Context, *config.Config) {},
	})
	defer controller.Stop(context.Background())
	waitMaintenanceController(t, func() bool { return clock.timerCount() == 1 })
	first := controller.Status()
	if !first.NextRun.Equal(time.Date(2026, 6, 10, 4, 0, 0, 0, time.UTC)) {
		t.Fatalf("initial next run = %v", first.NextRun)
	}
	reloaded := maintenanceControllerTestConfig(true)
	reloaded.Maintenance.Time = "05:00"
	controller.UpdateConfig(reloaded)
	waitMaintenanceController(t, func() bool { return clock.timerCount() == 2 })
	status := controller.Status()
	if !status.NextRun.Equal(time.Date(2026, 6, 10, 5, 0, 0, 0, time.UTC)) {
		t.Fatalf("reloaded next run = %v, want 05:00", status.NextRun)
	}
	clock.mu.Lock()
	defer clock.mu.Unlock()
	if len(clock.durations) != 2 || clock.durations[0] != time.Minute || clock.durations[1] != time.Hour+time.Minute {
		t.Fatalf("timer durations = %v", clock.durations)
	}
}

func TestMaintenanceControllerRejectsStaleTimerAndSchedulePublication(t *testing.T) {
	clock := &maintenanceControllerTestClock{now: time.Date(2026, 6, 10, 3, 59, 0, 0, time.UTC)}
	var runs atomic.Int32
	controller := newMaintenanceController(context.Background(), maintenanceControllerTestConfig(true), MaintenanceControllerDependencies{}, maintenanceControllerOptions{
		now: func() time.Time { return clock.now }, timerFactory: clock.timer,
		claimDay: maintenanceControllerTestClaimDay,
		run:      func(context.Context, *config.Config) { runs.Add(1) },
	})
	defer controller.Stop(context.Background())
	waitMaintenanceController(t, func() bool { return clock.timerCount() == 1 })
	controller.inner.mu.RLock()
	old := controller.inner.cfg
	controller.inner.mu.RUnlock()
	updated := maintenanceControllerTestConfig(true)
	updated.Maintenance.Time = "05:00"
	controller.UpdateConfig(updated)
	if controller.inner.startRun(old) || controller.inner.setNextRun(old, clock.now) || runs.Load() != 0 {
		t.Fatal("a stale timer started or published the old schedule")
	}
	if next := controller.Status().NextRun; next.Hour() != 5 {
		t.Fatalf("next run=%v", next)
	}
	controller.UpdateConfig(maintenanceControllerTestConfig(false))
	controller.inner.setNextRun(old, clock.now)
	if !controller.Status().NextRun.IsZero() {
		t.Fatal("stale schedule overwrote disablement")
	}
}

func TestMaintenanceControllerSkipsClaimedDayAfterReschedule(t *testing.T) {
	clock := &maintenanceControllerTestClock{now: time.Date(2026, 6, 10, 3, 59, 0, 0, time.UTC)}
	currentNow := func() time.Time {
		clock.mu.Lock()
		defer clock.mu.Unlock()
		return clock.now
	}
	setNow := func(now time.Time) {
		clock.mu.Lock()
		clock.now = now
		clock.mu.Unlock()
	}
	var claimMu sync.Mutex
	claimedDays := make(map[string]bool)
	claimDay := func(startedAt time.Time) (bool, error) {
		day := startedAt.Format("2006-01-02")
		claimMu.Lock()
		defer claimMu.Unlock()
		if claimedDays[day] {
			return false, nil
		}
		claimedDays[day] = true
		return true, nil
	}
	dayClaimed := func(startedAt time.Time) (bool, error) {
		claimMu.Lock()
		defer claimMu.Unlock()
		return claimedDays[startedAt.Format("2006-01-02")], nil
	}
	var runs atomic.Int32
	controller := newMaintenanceController(context.Background(), maintenanceControllerTestConfig(true), MaintenanceControllerDependencies{}, maintenanceControllerOptions{
		now:          currentNow,
		timerFactory: clock.timer,
		claimDay:     claimDay,
		dayClaimed:   dayClaimed,
		run: func(context.Context, *config.Config) {
			runs.Add(1)
		},
	})
	defer controller.Stop(context.Background())

	waitMaintenanceController(t, func() bool { return clock.timerCount() == 1 })
	setNow(time.Date(2026, 6, 10, 4, 1, 0, 0, time.UTC))
	if !clock.fireLatest() {
		t.Fatal("failed to fire first maintenance timer")
	}
	waitMaintenanceController(t, func() bool { return runs.Load() == 1 })
	reloaded := maintenanceControllerTestConfig(true)
	reloaded.Maintenance.Time = "05:00"
	controller.UpdateConfig(reloaded)
	waitMaintenanceController(t, func() bool {
		return controller.Status().NextRun.Equal(time.Date(2026, 6, 11, 5, 0, 0, 0, time.UTC))
	})
	if !clock.fireLatest() {
		t.Fatal("failed to fire rescheduled maintenance timer")
	}
	time.Sleep(20 * time.Millisecond)
	if got := runs.Load(); got != 1 {
		t.Fatalf("same-day reschedule started %d runs, want 1", got)
	}
	status := controller.Status()
	if !status.NextRun.Equal(time.Date(2026, 6, 11, 5, 0, 0, 0, time.UTC)) {
		t.Fatalf("same-day reschedule next run = %v, want next day", status.NextRun)
	}
}

func TestMaintenanceControllerDayClaimFailurePreventsRun(t *testing.T) {
	clock := &maintenanceControllerTestClock{now: time.Date(2026, 6, 10, 3, 59, 0, 0, time.UTC)}
	var runs atomic.Int32
	controller := newMaintenanceController(context.Background(), maintenanceControllerTestConfig(true), MaintenanceControllerDependencies{}, maintenanceControllerOptions{
		now:          func() time.Time { return clock.now },
		timerFactory: clock.timer,
		claimDay: func(time.Time) (bool, error) {
			return false, fmt.Errorf("test persistence failure")
		},
		run: func(context.Context, *config.Config) {
			runs.Add(1)
		},
	})
	defer controller.Stop(context.Background())

	waitMaintenanceController(t, func() bool { return clock.timerCount() == 1 })
	if !clock.fireLatest() {
		t.Fatal("failed to fire maintenance timer")
	}
	time.Sleep(20 * time.Millisecond)
	if got := runs.Load(); got != 0 {
		t.Fatalf("run count after claim failure = %d, want 0", got)
	}
}

func TestMaintenanceControllerKeepsRunConfigSnapshotImmutable(t *testing.T) {
	clock := &maintenanceControllerTestClock{now: time.Date(2026, 6, 10, 3, 59, 0, 0, time.UTC)}
	initial := maintenanceControllerTestConfig(true)
	initial.Tools.Journal.Enabled = true
	initial.Journal.DailySummary = true
	initial.Tools.PythonToolBridge.AllowedTools = []string{"read_file"}
	resolverCurrent := &config.Config{}
	initial.AuthorizationSnapshots = func() (*config.Config, *config.Config) {
		return initial, resolverCurrent
	}
	started := make(chan *config.Config, 1)
	controller := newMaintenanceController(context.Background(), initial, MaintenanceControllerDependencies{}, maintenanceControllerOptions{
		now:          func() time.Time { return clock.now },
		timerFactory: clock.timer,
		claimDay:     maintenanceControllerTestClaimDay,
		run: func(runCtx context.Context, runCfg *config.Config) {
			started <- runCfg
			<-runCtx.Done()
		},
	})

	waitMaintenanceController(t, func() bool { return clock.timerCount() == 1 })
	clock.fireLatest()
	select {
	case got := <-started:
		if got == initial {
			t.Fatalf("run config reused published pointer %p", got)
		}
		if got.Tools.Journal.Enabled != initial.Tools.Journal.Enabled ||
			got.Journal.DailySummary != initial.Journal.DailySummary ||
			len(got.Tools.PythonToolBridge.AllowedTools) != 1 || got.Tools.PythonToolBridge.AllowedTools[0] != "read_file" {
			t.Fatalf("run config lost copied values: %#v", got)
		}
		if got.AuthorizationSnapshots == nil {
			t.Fatal("run config lost authorization snapshot resolver")
		}
		baseline, current := got.AuthorizationSnapshots()
		if baseline != initial || current != resolverCurrent {
			t.Fatalf("authorization resolver = (%p, %p), want (%p, %p)", baseline, current, initial, resolverCurrent)
		}
		initial.Tools.PythonToolBridge.AllowedTools[0] = "mutated-after-publication"
		if got.Tools.PythonToolBridge.AllowedTools[0] != "read_file" {
			t.Fatal("run config shares mutable nested slices with published config")
		}
	case <-time.After(time.Second):
		t.Fatal("maintenance run did not start")
	}
	reloaded := maintenanceControllerTestConfig(true)
	reloaded.Maintenance.Time = "05:00"
	controller.UpdateConfig(reloaded)
	if err := controller.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestMaintenanceControllerStopWaitsForActiveRun(t *testing.T) {
	clock := &maintenanceControllerTestClock{now: time.Date(2026, 6, 10, 3, 59, 0, 0, time.UTC)}
	ctx := context.Background()
	started := make(chan struct{})
	var stopped atomic.Bool
	controller := newMaintenanceController(ctx, maintenanceControllerTestConfig(true), MaintenanceControllerDependencies{}, maintenanceControllerOptions{
		now:          func() time.Time { return clock.now },
		timerFactory: clock.timer,
		claimDay:     maintenanceControllerTestClaimDay,
		run: func(runCtx context.Context, _ *config.Config) {
			close(started)
			<-runCtx.Done()
			stopped.Store(true)
		},
	})

	waitMaintenanceController(t, func() bool { return clock.timerCount() == 1 })
	clock.fireLatest()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("maintenance run did not start")
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := controller.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if !stopped.Load() {
		t.Fatal("Stop returned before active maintenance run exited")
	}
	status := controller.Status()
	if status.Enabled || !status.NextRun.IsZero() || status.Running {
		t.Fatalf("stopped controller status = %+v, want disabled, idle, and no next run", status)
	}
}
