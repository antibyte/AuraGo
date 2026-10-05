package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// flowsHardeningLoad writes yamlText to a fresh config file and loads it.
func flowsHardeningLoad(t *testing.T, yamlText string) (string, *Config) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yamlText), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return path, cfg
}

func TestFlowsConfigSaveRoundTrip(t *testing.T) {
	want := FlowsConfig{
		Enabled:                false,
		MaxParallelRuns:        3,
		MaxParallelNodesPerRun: 2,
		RunRetentionDays:       7,
		MaxRunsPerFlow:         50,
		AIProvider:             "fast",
		Agent:                  FlowsAgentConfig{ReadOnly: true, AllowPublish: true},
	}
	path, cfg := flowsHardeningLoad(t, "flows:\n"+
		"  enabled: false\n"+
		"  max_parallel_runs: 3\n"+
		"  max_parallel_nodes_per_run: 2\n"+
		"  run_retention_days: 7\n"+
		"  max_runs_per_flow: 50\n"+
		"  ai_provider: fast\n"+
		"  agent:\n"+
		"    read_only: true\n"+
		"    allow_publish: true\n")
	if cfg.Flows != want {
		t.Fatalf("flows from YAML = %+v, want %+v", cfg.Flows, want)
	}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if reloaded.Flows != want {
		t.Fatalf("flows after Save/Load = %+v, want %+v", reloaded.Flows, want)
	}
}

func TestFlowsConfigSavePersistsChangesAndFalseValues(t *testing.T) {
	// Defaults are Enabled=true and AllowPublish=false. Saving must keep an
	// explicit false for a default-true field and a true for a default-false field.
	path, cfg := flowsHardeningLoad(t, "agent:\n  system_language: Deutsch\n")
	cfg.Flows.Enabled = false
	cfg.Flows.MaxParallelRuns = 5
	cfg.Flows.AIProvider = "cheap"
	cfg.Flows.Agent.AllowPublish = true
	want := cfg.Flows
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if reloaded.Flows != want {
		t.Fatalf("flows after Save/Load = %+v, want %+v", reloaded.Flows, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\nflows:") {
		t.Fatalf("Save did not write a flows section:\n%s", data)
	}
	if reloaded.Agent.SystemLanguage != "Deutsch" {
		t.Fatalf("Save lost an unrelated section: system_language=%q", reloaded.Agent.SystemLanguage)
	}
}

func TestFlowsConfigDefaultsSurviveSaveRoundTrip(t *testing.T) {
	path, cfg := flowsHardeningLoad(t, "agent:\n  system_language: Deutsch\n")
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if reloaded.Flows != cfg.Flows {
		t.Fatalf("defaults changed across Save/Load: %+v vs %+v", reloaded.Flows, cfg.Flows)
	}
	if !reloaded.Flows.Enabled || reloaded.Flows.MaxParallelRuns != 8 || reloaded.Flows.MaxRunsPerFlow != 200 {
		t.Fatalf("defaults lost: %+v", reloaded.Flows)
	}
}
