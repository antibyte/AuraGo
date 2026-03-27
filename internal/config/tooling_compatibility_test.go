package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPreservesCriticalToolingCompatibilityFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
llm:
  use_native_functions: false
  structured_outputs: true
agent:
  max_tool_guides: 9
  adaptive_tools:
    enabled: true
    max_tools: 17
    decay_half_life_days: 3
    weight_success_rate: false
    always_include: [docker, homepage, filesystem]
memory_analysis:
  enabled: true
  real_time: true
  query_expansion: true
llm_guardian:
  enabled: true
  allow_clarification: true
homepage:
  allow_local_server: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.LLM.UseNativeFunctions {
		t.Fatal("expected llm.use_native_functions=false to be preserved")
	}
	if !cfg.LLM.StructuredOutputs {
		t.Fatal("expected llm.structured_outputs=true to be preserved")
	}
	if !cfg.Agent.AdaptiveTools.Enabled {
		t.Fatal("expected adaptive_tools.enabled=true to be preserved")
	}
	if cfg.Agent.AdaptiveTools.MaxTools != 17 {
		t.Fatalf("adaptive_tools.max_tools = %d, want 17", cfg.Agent.AdaptiveTools.MaxTools)
	}
	if cfg.Agent.AdaptiveTools.DecayHalfLifeDays != 3 {
		t.Fatalf("adaptive_tools.decay_half_life_days = %v, want 3", cfg.Agent.AdaptiveTools.DecayHalfLifeDays)
	}
	if cfg.Agent.AdaptiveTools.WeightSuccessRate {
		t.Fatal("expected adaptive_tools.weight_success_rate=false to be preserved")
	}
	if len(cfg.Agent.AdaptiveTools.AlwaysInclude) != 3 {
		t.Fatalf("adaptive_tools.always_include len = %d, want 3", len(cfg.Agent.AdaptiveTools.AlwaysInclude))
	}
	if cfg.Agent.MaxToolGuides != 9 {
		t.Fatalf("agent.max_tool_guides = %d, want 9", cfg.Agent.MaxToolGuides)
	}
	if !cfg.MemoryAnalysis.Enabled || !cfg.MemoryAnalysis.RealTime || !cfg.MemoryAnalysis.QueryExpansion {
		t.Fatal("expected memory_analysis compatibility fields to be preserved")
	}
	if !cfg.LLMGuardian.Enabled || !cfg.LLMGuardian.AllowClarification {
		t.Fatal("expected llm_guardian compatibility fields to be preserved")
	}
	if !cfg.Homepage.AllowLocalServer {
		t.Fatal("expected homepage.allow_local_server=true to be preserved")
	}
}

func TestLoadDefaultsAdaptiveWeightSuccessRateWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
agent:
  adaptive_tools:
    enabled: true
    max_tools: 11
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Agent.AdaptiveTools.WeightSuccessRate {
		t.Fatal("expected adaptive_tools.weight_success_rate to default to true when omitted")
	}
}
