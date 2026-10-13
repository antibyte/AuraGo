package server

import (
	"testing"

	"aurago/internal/config"
)

func TestLLMRouterChannelSnapshotFollowsReload(t *testing.T) {
	s, _, _, remote := routedDecisionFixture(t)
	s.Cfg.LLMRouter = config.DefaultLLMRouterConfig()
	s.Cfg.LLMRouter.Enabled = true
	s.LLMClient = &agodeskLocalTestChatClient{}
	startup, startupClient := s.chatRuntimeSnapshot()
	startupCapability := (taskDecisionService{s}).RoutingSnapshot(startup)
	if !startupCapability.Available {
		t.Fatal("initial Decisions route unavailable")
	}
	for _, change := range []struct {
		name      string
		apply     func(*config.Config)
		available bool
	}{
		{"model", func(c *config.Config) { c.Decisions.Model = "experimental/jeff-2b" }, true},
		{"disabled", func(c *config.Config) { c.LLMRouter.DecisionsFallback = false }, false},
		{"reenabled", func(c *config.Config) { c.LLMRouter.DecisionsFallback = true }, true},
		{"local", func(c *config.Config) {
			c.Decisions.Backend = "local"
			c.Decisions.Provider = ""
			c.Decisions.Model = ""
		}, true},
	} {
		t.Run(change.name, func(t *testing.T) {
			next := s.ConfigSnapshot().Clone()
			change.apply(next)
			next.LLM.Model = "next-" + change.name
			nextClient := &agodeskLocalTestChatClient{}
			s.CfgMu.Lock()
			s.replaceConfigSnapshot(next)
			s.LLMClient = nextClient
			s.CfgMu.Unlock()
			current, client := s.chatRuntimeSnapshot()
			if current == next || current.LLM.Model != next.LLM.Model || client != nextClient {
				t.Fatal("new turn did not receive a detached, matching config/client pair")
			}
			capability := (taskDecisionService{s}).RoutingSnapshot(current)
			if capability.Available != change.available || startupCapability.Current() {
				t.Fatalf("reload availability: available=%v code=%s", capability.Available, capability.Code)
			}
			current.Providers[0].Model = "private-turn-change"
			if s.ConfigSnapshot().Providers[0].Model == "private-turn-change" || startup.LLM.Model == current.LLM.Model || startupClient == client {
				t.Fatal("new turn changed a published or accepted snapshot")
			}
		})
	}
	if remote.calls.Load() != 0 || remote.catalogCalls.Load() != 0 {
		t.Fatal("runtime snapshot performed provider work")
	}
}
