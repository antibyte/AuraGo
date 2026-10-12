package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecisionsConsentPersistenceAndSafeLegacyDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 8088\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Decisions.Enabled || cfg.Decisions.CPUFallbackAllowed || cfg.Decisions.Deployment != "auto" {
		t.Fatalf("unsafe legacy defaults: %+v", cfg.Decisions)
	}
	for _, allowed := range []bool{true, false} {
		cfg.Decisions = DecisionsConfig{Enabled: true, Deployment: "native", CPUFallbackAllowed: allowed}
		if err := cfg.Save(path); err != nil {
			t.Fatal(err)
		}
		reloaded, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if reloaded.Decisions != cfg.Decisions {
			t.Fatalf("permission changed across restart: %+v", reloaded.Decisions)
		}
		cfg = reloaded
	}
	if err := os.WriteFile(path, []byte("decisions:\n  deployment: remote\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("unsupported deployment accepted")
	}
}
