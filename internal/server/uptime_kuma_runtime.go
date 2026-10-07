package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// Each generation has one consumer and at most sixteen pending notifications.
// Overflow is reported, never turned into an unbounded set of agent runs.
type uptimeKumaRuntime struct {
	cancel  context.CancelFunc
	done    chan struct{}
	poller  *tools.UptimeKumaPoller
	initial config.UptimeKumaConfig
	eggMode bool
}

func (r *uptimeKumaRuntime) stop() { r.cancel(); r.poller.Stop(); <-r.done }

func consumeUptimeKumaNotifications(ctx context.Context, queue <-chan string, deliver func(context.Context, string)) {
	for {
		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case prompt := <-queue:
			if ctx.Err() != nil {
				return
			}
			turnCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			deliver(turnCtx, prompt)
			cancel()
		}
	}
}

func (s *Server) restartUptimeKumaPoller() {
	s.uptimeKumaMu.Lock()
	defer s.uptimeKumaMu.Unlock()
	if previous := s.uptimeKuma.Swap(nil); previous != nil {
		previous.stop()
	}
	if s.uptimeKumaClosed {
		return
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	cfg := s.ConfigSnapshot()
	if cfg == nil || cfg.EggMode.Enabled || !cfg.UptimeKuma.Enabled {
		return
	}
	parent := s.integrationCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	runtime := &uptimeKumaRuntime{cancel: cancel, done: make(chan struct{}), initial: cfg.UptimeKuma, eggMode: cfg.EggMode.Enabled}
	queue := make(chan string, 16)
	runtime.poller = tools.NewUptimeKumaPoller(tools.UptimeKumaPollerConfig{
		Logger:   s.Logger,
		Interval: time.Duration(cfg.UptimeKuma.PollIntervalSeconds) * time.Second,
		Fetch: func(ctx context.Context) (tools.UptimeKumaSnapshot, error) {
			c := runtime.initial
			return tools.FetchUptimeKumaSnapshot(ctx, tools.UptimeKumaConfig{BaseURL: c.BaseURL, APIKey: c.APIKey, InsecureSSL: c.InsecureSSL, RequestTimeout: c.RequestTimeout}, s.Logger)
		},
		OnTransition: func(event tools.UptimeKumaTransition) {
			if ctx.Err() != nil || !runtime.initial.RelayToAgent {
				return
			}
			prompt := formatUptimeKumaTransitionPrompt(event, runtime.initial.RelayInstruction)
			select {
			case <-ctx.Done():
			case queue <- prompt:
			default:
				if s.Logger != nil {
					s.Logger.Warn("Uptime Kuma notification queue full; transition omitted")
				}
			}
		},
	})
	s.uptimeKuma.Store(runtime)
	go func() {
		defer close(runtime.done)
		consumeUptimeKumaNotifications(ctx, queue, func(ctx context.Context, prompt string) {
			agent.LoopbackContext(ctx, s.buildUptimeKumaRunConfig(), prompt, agent.NoopBroker{})
		})
	}()
	runtime.poller.StartContext(ctx)
}

func (s *Server) stopUptimeKumaPoller() {
	s.uptimeKumaMu.Lock()
	defer s.uptimeKumaMu.Unlock()
	s.uptimeKumaClosed = true
	if previous := s.uptimeKuma.Swap(nil); previous != nil {
		previous.stop()
	}
}

func (s *Server) buildUptimeKumaRunConfig() agent.RunConfig {
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	cfg := s.ConfigSnapshot()
	return agent.RunConfig{
		Config:             cfg,
		Logger:             s.Logger,
		LLMClient:          s.LLMClient,
		ShortTermMem:       s.ShortTermMem,
		HistoryManager:     s.HistoryManager,
		LongTermMem:        s.LongTermMem,
		KG:                 s.KG,
		InventoryDB:        s.InventoryDB,
		InvasionDB:         s.InvasionDB,
		CheatsheetDB:       s.CheatsheetDB,
		ImageGalleryDB:     s.ImageGalleryDB,
		MediaRegistryDB:    s.MediaRegistryDB,
		HomepageRegistryDB: s.HomepageRegistryDB,
		ContactsDB:         s.ContactsDB,
		PlannerDB:          s.PlannerDB,
		SQLConnectionsDB:   s.SQLConnectionsDB,
		SQLConnectionPool:  s.SQLConnectionPool,
		RemoteHub:          s.RemoteHub,
		Vault:              s.Vault,
		Registry:           s.Registry,
		Manifest:           tools.NewManifest(cfg.Directories.ToolsDir),
		CronManager:        s.CronManager,
		MissionManagerV2:   s.MissionManagerV2,
		CoAgentRegistry:    s.CoAgentRegistry,
		BudgetTracker:      s.BudgetTracker,
		DaemonSupervisor:   s.DaemonSupervisor,
		LLMGuardian:        s.LLMGuardian,
		PreparationService: s.PreparationService,
		WorkspaceSearch:    s.WorkspaceSearch,
		SessionID:          "uptime_kuma",
		MessageSource:      "uptime_kuma",
	}
}

func formatUptimeKumaTransitionPrompt(event tools.UptimeKumaTransition, relayInstruction string) string {
	bounded := func(value string) string {
		value = strings.TrimSpace(value)
		if len(value) > 2048 {
			value = value[:2048] + "…"
		}
		return value
	}
	monitorName := bounded(event.Monitor.MonitorName)
	if monitorName == "" {
		monitorName = "Unnamed monitor"
	}
	lines := []string{
		fmt.Sprintf("[UPTIME KUMA EVENT: %s]", bounded(strings.ToUpper(event.Event))),
		fmt.Sprintf("Monitor: %s", monitorName),
	}
	if target := bounded(event.Monitor.Target()); target != "" {
		lines = append(lines, fmt.Sprintf("Target: %s", target))
	}
	if event.Monitor.MonitorType != "" {
		lines = append(lines, fmt.Sprintf("Type: %s", bounded(event.Monitor.MonitorType)))
	}
	lines = append(lines,
		fmt.Sprintf("Previous status: %s", bounded(event.PreviousStatus)),
		fmt.Sprintf("Current status: %s", bounded(event.CurrentStatus)),
	)
	if event.Monitor.ResponseTimeMS > 0 {
		lines = append(lines, fmt.Sprintf("Response time: %d ms", event.Monitor.ResponseTimeMS))
	}
	lines = []string{security.IsolateExternalData(strings.Join(lines, "\n"))}
	if relayInstruction = strings.TrimSpace(relayInstruction); relayInstruction != "" {
		lines = append(lines,
			"",
			"Configured outage instruction from the user:",
			relayInstruction,
		)
	}
	lines = append(lines, "Decide whether the user should be informed or whether a follow-up action is useful.")
	return strings.Join(lines, "\n")
}
