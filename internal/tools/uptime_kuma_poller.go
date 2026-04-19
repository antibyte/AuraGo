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
func (p *UptimeKumaPoller) Start() {
	if p == nil || p.fetch == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go p.run(ctx)
}

// Stop terminates the polling loop.
func (p *UptimeKumaPoller) Stop() {
	if p == nil {
		return
	}
	p.mu.Lock()
	cancel := p.cancel
	p.cancel = nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
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
	snapshot, err := p.fetch(ctx)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("Uptime Kuma poll failed", "error", err)
		}
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.baselined {
		for _, monitor := range snapshot.Monitors {
			p.lastStatuses[uptimeKumaMonitorKey(monitor.Labels)] = monitor.Status
		}
		p.baselined = true
		return
	}

	for _, monitor := range snapshot.Monitors {
		key := uptimeKumaMonitorKey(monitor.Labels)
		previous := p.lastStatuses[key]
		current := monitor.Status
		p.lastStatuses[key] = current

		if !shouldReportUptimeKumaTransition(previous, current) {
			continue
		}
		if p.onTransition != nil {
			p.onTransition(UptimeKumaTransition{
				Event:          strings.ToUpper(current),
				PreviousStatus: previous,
				CurrentStatus:  current,
				Monitor:        monitor,
			})
		}
	}
}

func shouldReportUptimeKumaTransition(previous, current string) bool {
	if previous == "" || previous == current {
		return false
	}
	return (previous == "up" && current == "down") || (previous == "down" && current == "up")
}
