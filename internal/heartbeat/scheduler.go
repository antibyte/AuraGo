package heartbeat

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
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

type persistedState struct {
	LastRun string `json:"last_run"`
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

	if restored, ok := s.loadPersistedLastRunLocked(); ok {
		s.lastRun = restored
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

	if err := s.persistLastRun(now); err != nil {
		s.logger.Warn("Heartbeat: failed to persist state, not updating lastRun", "error", err)
		s.running.Store(false)
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

func (s *Scheduler) heartbeatStatePathLocked() string {
	if s.cfg == nil || s.cfg.Directories.DataDir == "" {
		return ""
	}
	return filepath.Join(s.cfg.Directories.DataDir, "heartbeat_state.json")
}

func (s *Scheduler) loadPersistedLastRunLocked() (time.Time, bool) {
	statePath := s.heartbeatStatePathLocked()
	if statePath == "" {
		return time.Time{}, false
	}

	raw, err := os.ReadFile(statePath)
	if err != nil {
		return time.Time{}, false
	}

	var state persistedState
	if err := json.Unmarshal(raw, &state); err != nil {
		s.logger.Warn("Heartbeat scheduler state unreadable, ignoring", "path", statePath, "error", err)
		return time.Time{}, false
	}
	if state.LastRun == "" {
		return time.Time{}, false
	}
	lastRun, err := time.Parse(time.RFC3339Nano, state.LastRun)
	if err != nil {
		s.logger.Warn("Heartbeat scheduler state has invalid timestamp, ignoring", "path", statePath, "error", err)
		return time.Time{}, false
	}
	return lastRun, true
}

func (s *Scheduler) persistLastRun(lastRun time.Time) error {
	s.mu.RLock()
	statePath := s.heartbeatStatePathLocked()
	s.mu.RUnlock()
	if statePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		s.logger.Warn("Failed to create heartbeat state directory", "path", filepath.Dir(statePath), "error", err)
		return err
	}

	payload, err := json.Marshal(persistedState{LastRun: lastRun.Format(time.RFC3339Nano)})
	if err != nil {
		s.logger.Warn("Failed to encode heartbeat scheduler state", "error", err)
		return err
	}

	tmpPath := statePath + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o600); err != nil {
		s.logger.Warn("Failed to write heartbeat scheduler state", "path", tmpPath, "error", err)
		return err
	}
	if err := os.Rename(tmpPath, statePath); err != nil {
		_ = os.Remove(tmpPath)
		s.logger.Warn("Failed to finalize heartbeat scheduler state", "path", statePath, "error", err)
		return err
	}
	return nil
}
