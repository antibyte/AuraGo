package heartbeat

import (
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestSchedulerStartStop(t *testing.T) {
	var called atomic.Int32
	runner := func(prompt string) {
		called.Add(1)
	}

	cfg := &config.Config{}
	cfg.Heartbeat.Enabled = true
	cfg.Heartbeat.DayTimeWindow = config.HeartbeatTimeWindow{Start: "00:00", End: "23:59", Interval: "15m"}
	cfg.Heartbeat.NightTimeWindow = config.HeartbeatTimeWindow{Start: "22:00", End: "08:00", Interval: "4h"}
	cfg.Heartbeat.CheckTasks = true

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(cfg, logger, runner)

	s.Start()
	time.Sleep(50 * time.Millisecond)
	s.Stop()

	// checkAndRun is executed immediately in loop(); because we are inside the
	// active window the runner should have been invoked once.
	if called.Load() != 1 {
		t.Errorf("runner called %d times, want 1", called.Load())
	}
}

func TestSchedulerDisabled(t *testing.T) {
	var called atomic.Int32
	runner := func(prompt string) {
		called.Add(1)
	}

	cfg := &config.Config{}
	cfg.Heartbeat.Enabled = false

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(cfg, logger, runner)

	s.Start()
	time.Sleep(50 * time.Millisecond)
	s.Stop()

	if called.Load() != 0 {
		t.Errorf("runner called %d times, want 0", called.Load())
	}
}

func TestSchedulerIntervalRespected(t *testing.T) {
	var called atomic.Int32
	runner := func(prompt string) {
		called.Add(1)
		time.Sleep(10 * time.Millisecond)
	}

	cfg := &config.Config{}
	cfg.Heartbeat.Enabled = true
	cfg.Heartbeat.DayTimeWindow = config.HeartbeatTimeWindow{Start: "00:00", End: "23:59", Interval: "1h"}
	cfg.Heartbeat.NightTimeWindow = config.HeartbeatTimeWindow{Start: "22:00", End: "08:00", Interval: "4h"}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(cfg, logger, runner)

	s.Start()
	time.Sleep(100 * time.Millisecond)
	s.Stop()

	// Only one call expected because the interval is 1h and we wait only 100ms.
	if called.Load() != 1 {
		t.Errorf("runner called %d times, want 1", called.Load())
	}
}

func TestSchedulerOverlappingRunsPrevented(t *testing.T) {
	var called atomic.Int32
	runner := func(prompt string) {
		called.Add(1)
		time.Sleep(200 * time.Millisecond)
	}

	cfg := &config.Config{}
	cfg.Heartbeat.Enabled = true
	cfg.Heartbeat.DayTimeWindow = config.HeartbeatTimeWindow{Start: "00:00", End: "23:59", Interval: "15m"}
	cfg.Heartbeat.NightTimeWindow = config.HeartbeatTimeWindow{Start: "22:00", End: "08:00", Interval: "4h"}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(cfg, logger, runner)

	s.Start()
	// The first run is executing. Trigger additional checks while it is active.
	time.Sleep(50 * time.Millisecond)
	s.checkAndRun()
	time.Sleep(50 * time.Millisecond)
	s.checkAndRun()
	s.Stop()

	// Only one call expected because the first runner is still active.
	if called.Load() != 1 {
		t.Errorf("runner called %d times, want 1", called.Load())
	}
}

func TestSchedulerRestart(t *testing.T) {
	var called atomic.Int32
	runner := func(prompt string) {
		called.Add(1)
	}

	cfg := &config.Config{}
	cfg.Heartbeat.Enabled = true
	cfg.Heartbeat.DayTimeWindow = config.HeartbeatTimeWindow{Start: "00:00", End: "23:59", Interval: "15m"}
	cfg.Heartbeat.NightTimeWindow = config.HeartbeatTimeWindow{Start: "22:00", End: "08:00", Interval: "4h"}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(cfg, logger, runner)

	s.Start()
	time.Sleep(50 * time.Millisecond)

	newCfg := &config.Config{}
	*newCfg = *cfg
	newCfg.Heartbeat.Enabled = false

	s.Restart(newCfg)
	time.Sleep(50 * time.Millisecond)
	s.Stop()

	// After restart with Enabled=false no additional run should occur.
	if called.Load() != 1 {
		t.Errorf("runner called %d times, want 1", called.Load())
	}
}

func TestSchedulerDoubleStart(t *testing.T) {
	var called atomic.Int32
	runner := func(prompt string) {
		called.Add(1)
		time.Sleep(20 * time.Millisecond)
	}

	cfg := &config.Config{}
	cfg.Heartbeat.Enabled = true
	cfg.Heartbeat.DayTimeWindow = config.HeartbeatTimeWindow{Start: "00:00", End: "23:59", Interval: "15m"}
	cfg.Heartbeat.NightTimeWindow = config.HeartbeatTimeWindow{Start: "22:00", End: "08:00", Interval: "4h"}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(cfg, logger, runner)

	s.Start()
	time.Sleep(10 * time.Millisecond)
	// Start again while the first loop is still running.
	s.Start()
	time.Sleep(50 * time.Millisecond)
	s.Stop()

	// The second Start cleanly stops the first loop before starting a new one.
	// Because of timing the first run may or may not have fired; the important
	// thing is that the scheduler did not panic or deadlock and at least one
	// run executed.
	if called.Load() < 1 {
		t.Errorf("runner called %d times, want at least 1", called.Load())
	}
}

func TestSchedulerStopWithoutStart(t *testing.T) {
	cfg := &config.Config{}
	cfg.Heartbeat.Enabled = true

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(cfg, logger, func(string) {})

	// Should not panic or block.
	s.Stop()
}

func TestSchedulerRestartRace(t *testing.T) {
	var called atomic.Int32
	runner := func(prompt string) {
		called.Add(1)
	}

	cfg := &config.Config{}
	cfg.Heartbeat.Enabled = true
	cfg.Heartbeat.DayTimeWindow = config.HeartbeatTimeWindow{Start: "00:00", End: "23:59", Interval: "15m"}
	cfg.Heartbeat.NightTimeWindow = config.HeartbeatTimeWindow{Start: "22:00", End: "08:00", Interval: "4h"}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(cfg, logger, runner)

	s.Start()
	time.Sleep(20 * time.Millisecond)

	// Rapid restarts should not deadlock or panic.
	for i := 0; i < 5; i++ {
		newCfg := &config.Config{}
		*newCfg = *cfg
		s.Restart(newCfg)
	}

	s.Stop()

	// At least the initial run plus some restart runs should have executed.
	if called.Load() == 0 {
		t.Error("runner was never called")
	}
}
