package heartbeat

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
)

// Scheduler manages the periodic heartbeat wake-ups based on configured time windows.
type Scheduler struct {
	cfg     *config.Config
	logger  *slog.Logger
	runner  func(prompt string)
	mu      sync.RWMutex
	stopCh  chan struct{}
	wg      sync.WaitGroup
	lastRun time.Time
	running atomic.Bool // true while a heartbeat runner is executing
}

// New creates a new heartbeat scheduler.
// The runner callback is invoked with the heartbeat prompt when a wake-up is due.
func New(cfg *config.Config, logger *slog.Logger, runner func(prompt string)) *Scheduler {
	return &Scheduler{
		cfg:    cfg,
		logger: logger,
		runner: runner,
	}
}

// Start begins the heartbeat scheduler loop.
// If the scheduler is already running it is stopped cleanly first.
func (s *Scheduler) Start() {
	s.mu.Lock()

	// If a loop is already running, close its channel and wait for it to exit
	// before starting a new one.
	if s.stopCh != nil {
		ch := s.stopCh
		s.stopCh = nil
		s.mu.Unlock()

		select {
		case <-ch:
			// already closed
		default:
			close(ch)
		}
		s.wg.Wait()

		s.mu.Lock()
	}

	if !s.cfg.Heartbeat.Enabled {
		s.mu.Unlock()
		s.logger.Info("Heartbeat scheduler disabled")
		return
	}

	ch := make(chan struct{})
	s.stopCh = ch
	s.wg.Add(1)
	go s.loop(ch)

	// Copy log values while still holding the lock to avoid races.
	dayWindow := s.cfg.Heartbeat.DayTimeWindow.Start + "-" + s.cfg.Heartbeat.DayTimeWindow.End
	dayInterval := s.cfg.Heartbeat.DayTimeWindow.Interval
	nightWindow := s.cfg.Heartbeat.NightTimeWindow.Start + "-" + s.cfg.Heartbeat.NightTimeWindow.End
	nightInterval := s.cfg.Heartbeat.NightTimeWindow.Interval
	s.mu.Unlock()

	s.logger.Info("Heartbeat scheduler started",
		"day_window", dayWindow,
		"day_interval", dayInterval,
		"night_window", nightWindow,
		"night_interval", nightInterval,
	)
}

// Stop halts the heartbeat scheduler and waits for the loop to exit.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	ch := s.stopCh
	s.stopCh = nil
	s.mu.Unlock()

	if ch != nil {
		select {
		case <-ch:
			// already closed
		default:
			close(ch)
		}
		s.wg.Wait()
	}
}

// Restart updates the configuration and restarts the scheduler.
func (s *Scheduler) Restart(cfg *config.Config) {
	s.Stop()
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	s.Start()
}

func (s *Scheduler) loop(stopCh chan struct{}) {
	defer s.wg.Done()

	// Run immediately on start if within an active window
	s.checkAndRun()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			s.checkAndRun()
		}
	}
}

func (s *Scheduler) checkAndRun() {
	s.mu.RLock()
	cfg := s.cfg
	lastRun := s.lastRun
	s.mu.RUnlock()

	if !cfg.Heartbeat.Enabled {
		return
	}

	now := time.Now()
	window, interval := getActiveWindow(now, cfg.Heartbeat)
	if window == nil {
		return
	}

	intervalDur := parseInterval(interval)

	// Check if enough time has passed since last run
	if !lastRun.IsZero() && now.Sub(lastRun) < intervalDur {
		return
	}

	// Prevent overlapping executions.
	if !s.running.CompareAndSwap(false, true) {
		s.logger.Debug("Heartbeat skipped: previous run still active",
			"time", now.Format("15:04"),
		)
		return
	}

	s.mu.Lock()
	s.lastRun = now
	s.mu.Unlock()

	prompt := buildHeartbeatPrompt(cfg.Heartbeat, now)
	s.logger.Info("Heartbeat wake-up triggered",
		"time", now.Format("15:04"),
		"interval", interval,
		"window_start", window.Start,
		"window_end", window.End,
	)

	// Run asynchronously to avoid blocking the scheduler
	go func() {
		defer s.running.Store(false)
		s.runner(prompt)
	}()
}
