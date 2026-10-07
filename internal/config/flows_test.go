package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFlowsConfigEffectiveLimits(t *testing.T) {
	var c FlowsConfig
	if c.EffectiveMaxParallelRuns() != 8 || c.EffectiveMaxParallelNodes() != 4 ||
		c.EffectiveRunRetentionDays() != 30 || c.EffectiveMaxRunsPerFlow() != 200 {
		t.Fatalf("zero values must use the defaults: %+v", c)
	}
	c = FlowsConfig{MaxParallelRuns: 99, MaxParallelNodesPerRun: 99, RunRetentionDays: 9999, MaxRunsPerFlow: 1}
	if c.EffectiveMaxParallelRuns() != 32 || c.EffectiveMaxParallelNodes() != 16 ||
		c.EffectiveRunRetentionDays() != 365 || c.EffectiveMaxRunsPerFlow() != 10 {
		t.Fatalf("limits are not clamped: %+v", c)
	}
}

func TestLoadAppliesFlowsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("agent:\n  system_language: Deutsch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	f := cfg.Flows
	if !f.Enabled || f.MaxParallelRuns != 8 || f.MaxParallelNodesPerRun != 4 || f.RunRetentionDays != 30 ||
		f.MaxRunsPerFlow != 200 || f.AIProvider != "" || f.Agent.ReadOnly || f.Agent.AllowPublish {
		t.Fatalf("flows defaults = %+v", f)
	}
	path2 := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path2, []byte("flows:\n  enabled: false\n  max_parallel_runs: 3\n  ai_provider: fast\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(path2)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Flows.Enabled || cfg.Flows.MaxParallelRuns != 3 || cfg.Flows.AIProvider != "fast" || cfg.Flows.RunRetentionDays != 30 {
		t.Fatalf("flows from YAML = %+v", cfg.Flows)
	}
}
