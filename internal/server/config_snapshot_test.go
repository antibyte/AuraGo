package server

import (
	"testing"

	"aurago/internal/config"
)

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
}
