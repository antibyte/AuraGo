package server

import (
	"context"

	"aurago/internal/telegram"
)

func (s *Server) configureTelegramBot() {
	s.telegramLifecycleMu.Lock()
	defer s.telegramLifecycleMu.Unlock()
	if s.telegramClosed {
		return
	}
	s.CfgMu.RLock()
	current := s.telegramBot.Load()
	unchanged := current != nil && current.MatchesConfig(s.ConfigSnapshot())
	s.CfgMu.RUnlock()
	if unchanged {
		return
	}
	if previous := s.telegramBot.Swap(nil); previous != nil {
		previous.Stop()
	}
	// Publish the handle under CfgMu so a concurrent configuration change cannot
	// miss revoking it. Initialization and message processing run asynchronously.
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	parent := s.integrationCtx
	if parent == nil {
		parent = context.Background()
	}
	bot := telegram.StartLongPolling(parent, s.ConfigSnapshot(), s.Logger, s.LLMClient, s.ShortTermMem, s.LongTermMem, s.Vault, s.Registry, s.CronManager, s.HistoryManager, s.KG, s.InventoryDB, s.PlannerDB, s.MissionManagerV2, s.RemoteHub, s.Guardian, s.budgetTrackerSnapshot, s.WebhookManager, s.chatRuntimeSnapshot, taskDecisionService{s}, &s.telegramOffsets)
	s.telegramBot.Store(bot)
}

func (s *Server) stopTelegramBot() {
	s.telegramLifecycleMu.Lock()
	defer s.telegramLifecycleMu.Unlock()
	s.telegramClosed = true
	if previous := s.telegramBot.Swap(nil); previous != nil {
		previous.Stop()
	}
}
