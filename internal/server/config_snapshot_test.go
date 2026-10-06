package server

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
	"aurago/internal/proxy"
)

func TestReplaceConfigSnapshotUpdatesSecurityProxyManager(t *testing.T) {
	startup := &config.Config{}
	startup.SecurityProxy.Domain = "old.example.com"
	s := &Server{Cfg: startup, ProxyManager: proxy.NewManager(startup, slog.New(slog.NewTextHandler(io.Discard, nil)))}
	s.initConfigSnapshot()

	saved := &config.Config{}
	saved.SecurityProxy.Domain = "new.example.com"
	saved.SecurityProxy.HTTPSPort = 8443
	s.replaceConfigSnapshot(saved)

	if got := s.ProxyManager.Config(); got != saved {
		t.Fatalf("security proxy manager config = %p, want the published snapshot %p", got, saved)
	}
}

func TestReplaceConfigStoresNewSnapshotWithoutMutatingOldConfig(t *testing.T) {
	oldCfg := &config.Config{}
	oldCfg.Server.Port = 1111
	s := &Server{Cfg: oldCfg}
	s.initConfigSnapshot()

	newCfg := &config.Config{}
	newCfg.Server.Port = 2222
	s.replaceConfigSnapshot(newCfg)

	if got := s.ConfigSnapshot(); got != newCfg {
		t.Fatalf("ConfigSnapshot returned %p, want new config %p", got, newCfg)
	}
	if s.Cfg != newCfg {
		t.Fatalf("compat Cfg pointer = %p, want %p", s.Cfg, newCfg)
	}
	if oldCfg.Server.Port != 1111 {
		t.Fatalf("old config was mutated: port=%d", oldCfg.Server.Port)
	}
	resolver := s.ConfigSnapshot().AuthorizationSnapshots
	if resolver == nil {
		t.Fatal("published config lost authorization snapshot resolver")
	}
	baseline, current := resolver()
	if baseline != newCfg || current != newCfg {
		t.Fatalf("authorization resolver = (%p, %p), want (%p, %p)", baseline, current, newCfg, newCfg)
	}
}
