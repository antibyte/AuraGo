package tools

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// UptimeKumaTransition describes a state change detected by the poller.
type UptimeKumaTransition struct {
	Event          string                    `json:"event"`
	PreviousStatus string                    `json:"previous_status"`
	CurrentStatus  string                    `json:"current_status"`
	Monitor        UptimeKumaMonitorSnapshot `json:"monitor"`
}

// UptimeKumaPoller periodically scrapes the metrics endpoint and reports transitions.
type UptimeKumaPoller struct {
	logger       *slog.Logger
	interval     time.Duration
	fetch        func(context.Context) (UptimeKumaSnapshot, error)
	onTransition func(UptimeKumaTransition)

	mu           sync.Mutex
	cancel       context.CancelFunc
	done         chan struct{}
	closed       bool
	lastStatuses map[string]string
	baselined    bool
}

// UptimeKumaPollerConfig contains the dependencies for the poller.
type UptimeKumaPollerConfig struct {
	Logger       *slog.Logger
	Interval     time.Duration
	Fetch        func(context.Context) (UptimeKumaSnapshot, error)
	OnTransition func(UptimeKumaTransition)
}

// NewUptimeKumaPoller creates a new transition poller.
func NewUptimeKumaPoller(cfg UptimeKumaPollerConfig) *UptimeKumaPoller {
	interval := cfg.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &UptimeKumaPoller{
		logger:       cfg.Logger,
		interval:     interval,
		fetch:        cfg.Fetch,
		onTransition: cfg.OnTransition,
		lastStatuses: make(map[string]string),
	}
}

// Start begins the background polling loop.
func (p *UptimeKumaPoller) Start() { p.StartContext(context.Background()) }

// StartContext binds this single-use runtime to its owner.
func (p *UptimeKumaPoller) StartContext(parent context.Context) {
	if p == nil || p.fetch == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil || p.closed {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	p.cancel = cancel
	p.done = make(chan struct{})
	go func() { defer close(p.done); p.run(ctx) }()
}

// Stop terminates the polling loop.
func (p *UptimeKumaPoller) Stop() {
	if p == nil {
		return
	}
	p.mu.Lock()
	cancel := p.cancel
	done := p.done
	p.closed = true
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (p *UptimeKumaPoller) run(ctx context.Context) {
	p.pollOnce(ctx)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.pollOnce(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (p *UptimeKumaPoller) pollOnce(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	snapshot, err := p.fetch(ctx)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("Uptime Kuma poll failed", "error", err)
		}
		return
	}

	p.mu.Lock()

	if !p.baselined {
		for _, monitor := range snapshot.Monitors {
			p.lastStatuses[uptimeKumaMonitorKey(monitor.Labels)] = monitor.Status
		}
		p.baselined = true
		p.mu.Unlock()
		return
	}

	// Collect transitions while holding the lock, dispatch afterwards to
	// avoid calling user callbacks (which may acquire their own locks)
	// while the poller mutex is held.
	var transitions []UptimeKumaTransition
	activeKeys := make(map[string]struct{}, len(snapshot.Monitors))
	for _, monitor := range snapshot.Monitors {
		key := uptimeKumaMonitorKey(monitor.Labels)
		activeKeys[key] = struct{}{}
		previous := p.lastStatuses[key]
		current := monitor.Status
		p.lastStatuses[key] = current

		if shouldReportUptimeKumaTransition(previous, current) {
			transitions = append(transitions, UptimeKumaTransition{
				Event:          strings.ToUpper(current),
				PreviousStatus: previous,
				CurrentStatus:  current,
				Monitor:        monitor,
			})
		}
	}

	// Clean up stale monitors that no longer exist in the snapshot.
	for key := range p.lastStatuses {
		if _, ok := activeKeys[key]; !ok {
			delete(p.lastStatuses, key)
		}
	}

	p.mu.Unlock()

	// Dispatch transitions outside the lock.
	for _, event := range transitions {
		if ctx.Err() != nil {
			return
		}
		if p.onTransition != nil {
			p.onTransition(event)
		}
	}
}

func shouldReportUptimeKumaTransition(previous, current string) bool {
	if previous == "" || previous == current {
		return false
	}
	return (previous == "up" && current == "down") || (previous == "down" && current == "up")
}
