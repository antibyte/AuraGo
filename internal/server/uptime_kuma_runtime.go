package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/tools"
)

func (s *Server) restartUptimeKumaPoller() {
	if s.UptimeKumaPoller != nil {
		s.UptimeKumaPoller.Stop()
		s.UptimeKumaPoller = nil
	}
	if s.Cfg == nil || !s.Cfg.UptimeKuma.Enabled {
		return
	}

	interval := time.Duration(s.Cfg.UptimeKuma.PollIntervalSeconds) * time.Second
	poller := tools.NewUptimeKumaPoller(tools.UptimeKumaPollerConfig{
		Logger:   s.Logger,
		Interval: interval,
		Fetch: func(ctx context.Context) (tools.UptimeKumaSnapshot, error) {
			s.CfgMu.RLock()
			cfg := uptimeKumaToolConfig(s)
			s.CfgMu.RUnlock()
			return tools.FetchUptimeKumaSnapshot(ctx, cfg, s.Logger)
		},
		OnTransition: func(event tools.UptimeKumaTransition) {
			s.CfgMu.RLock()
			relayEnabled := s.Cfg.UptimeKuma.RelayToAgent
			relayInstruction := s.Cfg.UptimeKuma.RelayInstruction
			s.CfgMu.RUnlock()
			if !relayEnabled {
				return
			}
			go agent.Loopback(s.buildUptimeKumaRunConfig(), formatUptimeKumaTransitionPrompt(event, relayInstruction), agent.NoopBroker{})
		},
	})
	poller.Start()
	s.UptimeKumaPoller = poller
}

func (s *Server) buildUptimeKumaRunConfig() agent.RunConfig {
	return agent.RunConfig{
		Config:             s.Cfg,
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
		Manifest:           tools.NewManifest(s.Cfg.Directories.ToolsDir),
		CronManager:        s.CronManager,
		MissionManagerV2:   s.MissionManagerV2,
		CoAgentRegistry:    s.CoAgentRegistry,
		BudgetTracker:      s.BudgetTracker,
		DaemonSupervisor:   s.DaemonSupervisor,
		LLMGuardian:        s.LLMGuardian,
		PreparationService: s.PreparationService,
		SessionID:          "default",
		MessageSource:      "uptime_kuma",
	}
}

func formatUptimeKumaTransitionPrompt(event tools.UptimeKumaTransition, relayInstruction string) string {
	monitorName := strings.TrimSpace(event.Monitor.MonitorName)
	if monitorName == "" {
		monitorName = "Unnamed monitor"
	}
	lines := []string{
		fmt.Sprintf("[UPTIME KUMA EVENT: %s]", strings.ToUpper(event.Event)),
		fmt.Sprintf("Monitor: %s", monitorName),
	}
	if target := strings.TrimSpace(event.Monitor.Target()); target != "" {
		lines = append(lines, fmt.Sprintf("Target: %s", target))
	}
	if event.Monitor.MonitorType != "" {
		lines = append(lines, fmt.Sprintf("Type: %s", event.Monitor.MonitorType))
	}
	lines = append(lines,
		fmt.Sprintf("Previous status: %s", event.PreviousStatus),
		fmt.Sprintf("Current status: %s", event.CurrentStatus),
	)
	if event.Monitor.ResponseTimeMS > 0 {
		lines = append(lines, fmt.Sprintf("Response time: %d ms", event.Monitor.ResponseTimeMS))
	}
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
