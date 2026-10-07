package server

import (
	"context"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/rocketchat"
)

// budgetTrackerSnapshot lets long-lived channel runtimes observe a tracker
// created by a later budget.enable config reload.
func (s *Server) budgetTrackerSnapshot() *budget.Tracker {
	if s == nil {
		return nil
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	return s.BudgetTracker
}

// configureRocketChatBot drains the previous generation outside the config lock.
// Publication and starting the replacement share CfgMu so a concurrent update
// cannot miss revoking a newly started bot.
func (s *Server) configureRocketChatBot() {
	s.rocketChatLifecycleMu.Lock()
	defer s.rocketChatLifecycleMu.Unlock()
	if previous := s.rocketChatBot.Swap(nil); previous != nil {
		previous.Stop()
	}
	if s.rocketChatClosed {
		return
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return
	}
	parent := s.integrationCtx
	if parent == nil {
		parent = context.Background()
	}
	bot := rocketchat.StartBot(parent, cfg, s.Logger, s.LLMClient, s.ShortTermMem, s.LongTermMem, s.Vault, s.Registry, s.CronManager, s.HistoryManager, s.KG, s.InventoryDB, s.MissionManagerV2, s.RemoteHub, s.Guardian, s.budgetTrackerSnapshot, func() (*config.Config, llm.ChatClient) {
		s.CfgMu.RLock()
		defer s.CfgMu.RUnlock()
		current := s.ConfigSnapshot()
		if current == nil {
			return nil, nil
		}
		return current.Clone(), s.LLMClient
	})
	s.rocketChatBot.Store(bot)
}

func (s *Server) stopRocketChatBot() {
	s.rocketChatLifecycleMu.Lock()
	defer s.rocketChatLifecycleMu.Unlock()
	s.rocketChatClosed = true
	if previous := s.rocketChatBot.Swap(nil); previous != nil {
		previous.Stop()
	}
}
