package heartbeat

import (
	"log/slog"
	"sync"
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
}

// New creates a new heartbeat scheduler.
// The runner callback is invoked with the heartbeat prompt when a wake-up is due.
func New(cfg *config.Config, logger *slog.Logger, runner func(prompt string)) *Scheduler {
	return &Scheduler{
		cfg:    cfg,
		logger: logger,
		runner: runner,
		stopCh: make(chan struct{}),
	}
}

// Start begins the heartbeat scheduler loop.
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopCh != nil {
		select {
		case <-s.stopCh:
			// already closed
		default:
			close(s.stopCh)
		}
	}
	s.stopCh = make(chan struct{})

	if !s.cfg.Heartbeat.Enabled {
		s.logger.Info("Heartbeat scheduler disabled")
		return
	}

	s.wg.Add(1)
	go s.loop()
	s.logger.Info("Heartbeat scheduler started",
		"day_window", s.cfg.Heartbeat.DayTimeWindow.Start+"-"+s.cfg.Heartbeat.DayTimeWindow.End,
		"day_interval", s.cfg.Heartbeat.DayTimeWindow.Interval,
		"night_window", s.cfg.Heartbeat.NightTimeWindow.Start+"-"+s.cfg.Heartbeat.NightTimeWindow.End,
		"night_interval", s.cfg.Heartbeat.NightTimeWindow.Interval,
	)
}

// Stop halts the heartbeat scheduler and waits for the loop to exit.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	ch := s.stopCh
	s.mu.Unlock()

	if ch != nil {
		select {
		case <-ch:
		default:
			close(ch)
		}
	}
	s.wg.Wait()
}

// Restart updates the configuration and restarts the scheduler.
func (s *Scheduler) Restart(cfg *config.Config) {
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	s.Stop()
	s.Start()
}

func (s *Scheduler) loop() {
	defer s.wg.Done()

	// Run immediately on start if within an active window
	s.checkAndRun()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAndRun()
		}
	}
}

func (s *Scheduler) checkAndRun() {
	s.mu.RLock()
	cfg := s.cfg
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
	if !s.lastRun.IsZero() && now.Sub(s.lastRun) < intervalDur {
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
	go s.runner(prompt)
}
