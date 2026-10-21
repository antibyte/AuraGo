package config

import (
	"gopkg.in/yaml.v3"
	"testing"
)

func TestOCRDefaultsAndExplicitOptOut(t *testing.T) {
	cfg := DefaultOCRConfig()
	if cfg.Enabled || !cfg.AgentAccess || !cfg.AutomaticChat || !cfg.AutomaticIndexing {
		t.Fatal(cfg)
	}
	if err := yaml.Unmarshal([]byte("agent_access: false\nautomatic_chat: false\nautomatic_indexing: false\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.AgentAccess || cfg.AutomaticChat || cfg.AutomaticIndexing {
		t.Fatal("explicit denial lost")
	}
}
