package server

import (
	"context"
	"reflect"

	"aurago/internal/config"
	"aurago/internal/tools"
)

type homeAssistantPollerRuntime struct {
	cfg    *config.Config
	cancel context.CancelFunc
	done   chan struct{}
}

func (p *homeAssistantPollerRuntime) cancelIfChanged(cfg *config.Config) {
	if cfg.EggMode.Enabled || !reflect.DeepEqual(p.cfg.HomeAssistant, cfg.HomeAssistant) {
		p.cancel()
	}
}

func (p *homeAssistantPollerRuntime) stop() {
	if p != nil {
		p.cancel()
		<-p.done
	}
}

func (s *Server) configureHomeAssistantPoller() {
	s.haPollerMu.Lock()
	defer s.haPollerMu.Unlock()
	s.haPoller.Swap(nil).stop()
	if s.haPollerClosed || s.MissionManagerV2 == nil {
		return
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.HomeAssistant.Enabled || cfg.EggMode.Enabled || cfg.HomeAssistant.URL == "" || cfg.HomeAssistant.AccessToken == "" {
		return
	}
	parent := s.integrationCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	poller := &homeAssistantPollerRuntime{cfg: cfg.Clone(), cancel: cancel, done: make(chan struct{})}
	s.haPoller.Store(poller)
	haCfg := tools.HAConfig{URL: cfg.HomeAssistant.URL, AccessToken: cfg.HomeAssistant.AccessToken}
	go func() {
		defer close(poller.done)
		tools.StartHomeAssistantPoller(ctx, haCfg, s.MissionManagerV2, s.Logger)
	}()
}

func (s *Server) stopHomeAssistantPoller() {
	s.haPollerMu.Lock()
	defer s.haPollerMu.Unlock()
	s.haPollerClosed = true
	s.haPoller.Swap(nil).stop()
}
