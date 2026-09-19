package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDisclosureMigrationCanonicalPrecedenceAndIdempotence(t *testing.T) {
	input := []byte(`# preserved comment
agent:
  max_tool_calls: 15
  discover_tools_snapshot_ttl_minutes: 12
  budget:
    enabled: true
    daily_limit_usd: 5
  output_compression:
    repetitive_substitution:
      ltsc_lite_enabled: true
circuit_breaker:
  max_tool_calls: 20
budget:
  enabled: false
composio:
  preferred_capabilities:
    web_search: {server: test, tool: search}
memory_analysis:
  real_time: false
  auto_confirm_threshold: 0.8
`)
	out, err := NormalizeToolDisclosureConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := NormalizeToolDisclosureConfig(out)
	if err != nil || string(again) != string(out) {
		t.Fatalf("non-idempotent migration: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(out, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.CircuitBreaker.MaxToolCalls != 20 || cfg.Budget.Enabled || cfg.Budget.DailyLimitUSD != 5 {
		t.Fatalf("canonical values or missing fields lost: %s", out)
	}
	if !strings.Contains(string(out), "# preserved comment") || !strings.Contains(string(out), "mcp:") {
		t.Fatalf("comment or destination missing: %s", out)
	}
	for _, key := range []string{"snapshot_ttl", "ltsc_lite", "real_time"} {
		if strings.Contains(string(out), key) {
			t.Errorf("obsolete key retained: %s", key)
		}
	}
	if _, err := NormalizeToolDisclosureConfig([]byte("agent: {max_tool_calls: 15}\ncircuit_breaker: false\n")); err == nil {
		t.Fatal("invalid destination accepted")
	}
}

func TestDisclosureExplicitDisabledAndEmptyPreferencesSurviveLoadSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	input := []byte(`agent:
  tool_output_limit: 0
  adaptive_tools:
    always_include: []
  output_compression:
    reversible: {enabled: false}
    smart_crusher: {enabled: false}
  importance_scoring: {enabled: false}
  auto_learning: {enabled: false}
`)
	if err := os.WriteFile(path, input, 0600); err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 2; round++ {
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Agent.ToolOutputLimit != 50000 || len(cfg.Agent.AdaptiveTools.AlwaysInclude) != 0 {
			t.Fatal("automatic output limit or explicit empty preference lost")
		}
		if cfg.Agent.OutputCompression.Reversible.Enabled || cfg.Agent.OutputCompression.SmartCrusher.Enabled || cfg.Agent.ImportanceScoring.Enabled || cfg.Agent.AutoLearning.Enabled {
			t.Fatal("explicit false ignored")
		}
		if err := cfg.Save(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, []byte("agent: {tool_output_limit: -1}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("negative limit accepted")
	}
}
