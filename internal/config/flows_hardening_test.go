package config

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

// TestFlowsTemplateMatchesLoadDefaultsAndGetters pins the flows block of the
// repository's config_template.yaml against Load's defaults and the Effective*
// getters, so none of the three can drift on its own.
func TestFlowsTemplateMatchesLoadDefaultsAndGetters(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse config_template.yaml: %v", err)
	}
	node, ok := doc["flows"]
	if !ok {
		t.Fatal("config_template.yaml has no flows: block")
	}
	raw, err := yaml.Marshal(&node)
	if err != nil {
		t.Fatalf("re-encode flows block: %v", err)
	}

	// Unknown keys (typos) are an error.
	var tpl FlowsConfig
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&tpl); err != nil {
		t.Fatalf("decode flows block: %v", err)
	}

	// The template documents every key, including the nested agent block.
	var keys map[string]interface{}
	if err := yaml.Unmarshal(raw, &keys); err != nil {
		t.Fatalf("decode flows keys: %v", err)
	}
	got := make([]string, 0, len(keys))
	for k := range keys {
		got = append(got, k)
	}
	sort.Strings(got)
	wantKeys := []string{"agent", "ai_provider", "enabled", "max_parallel_nodes_per_run", "max_parallel_runs", "max_runs_per_flow", "run_retention_days"}
	if strings.Join(got, ",") != strings.Join(wantKeys, ",") {
		t.Fatalf("template flows keys = %v, want %v", got, wantKeys)
	}
	agent, _ := keys["agent"].(map[string]interface{})
	if _, ok := agent["read_only"]; !ok {
		t.Fatalf("template flows.agent.read_only missing: %v", agent)
	}
	if _, ok := agent["allow_publish"]; !ok {
		t.Fatalf("template flows.agent.allow_publish missing: %v", agent)
	}

	_, cfg := flowsHardeningLoad(t, "agent:\n  system_language: Deutsch\n")
	if tpl != cfg.Flows {
		t.Fatalf("template flows = %+v, Load defaults = %+v", tpl, cfg.Flows)
	}

	// The getters report the template values unchanged: they are inside the clamp range.
	if tpl.EffectiveMaxParallelRuns() != tpl.MaxParallelRuns ||
		tpl.EffectiveMaxParallelNodes() != tpl.MaxParallelNodesPerRun ||
		tpl.EffectiveRunRetentionDays() != tpl.RunRetentionDays ||
		tpl.EffectiveMaxRunsPerFlow() != tpl.MaxRunsPerFlow {
		t.Fatalf("template values differ from the getters: %+v", tpl)
	}
	// An all-zero config reports the same values, so the getters' fallbacks are the template defaults.
	var zero FlowsConfig
	if zero.EffectiveMaxParallelRuns() != tpl.MaxParallelRuns ||
		zero.EffectiveMaxParallelNodes() != tpl.MaxParallelNodesPerRun ||
		zero.EffectiveRunRetentionDays() != tpl.RunRetentionDays ||
		zero.EffectiveMaxRunsPerFlow() != tpl.MaxRunsPerFlow {
		t.Fatalf("getter fallbacks differ from the template: %+v", tpl)
	}
}

// TestFlowsConfigGetterEdges pins the clamp edges of the four getters with literal values.
func TestFlowsConfigGetterEdges(t *testing.T) {
	getters := []struct {
		name  string
		get   func(v int) int
		cases [][2]int // {input, want}
	}{
		{
			name:  "max_parallel_runs (default 8, 1-32)",
			get:   func(v int) int { return FlowsConfig{MaxParallelRuns: v}.EffectiveMaxParallelRuns() },
			cases: [][2]int{{-100, 8}, {-1, 8}, {0, 8}, {1, 1}, {31, 31}, {32, 32}, {33, 32}, {1 << 30, 32}},
		},
		{
			name:  "max_parallel_nodes_per_run (default 4, 1-16)",
			get:   func(v int) int { return FlowsConfig{MaxParallelNodesPerRun: v}.EffectiveMaxParallelNodes() },
			cases: [][2]int{{-100, 4}, {-1, 4}, {0, 4}, {1, 1}, {15, 15}, {16, 16}, {17, 16}, {1 << 30, 16}},
		},
		{
			name:  "run_retention_days (default 30, 1-365)",
			get:   func(v int) int { return FlowsConfig{RunRetentionDays: v}.EffectiveRunRetentionDays() },
			cases: [][2]int{{-100, 30}, {-1, 30}, {0, 30}, {1, 1}, {364, 364}, {365, 365}, {366, 365}, {1 << 30, 365}},
		},
		{
			name:  "max_runs_per_flow (default 200, 10-5000)",
			get:   func(v int) int { return FlowsConfig{MaxRunsPerFlow: v}.EffectiveMaxRunsPerFlow() },
			cases: [][2]int{{-100, 200}, {-1, 200}, {0, 200}, {1, 10}, {9, 10}, {10, 10}, {11, 11}, {4999, 4999}, {5000, 5000}, {5001, 5000}, {1 << 30, 5000}},
		},
	}
	for _, g := range getters {
		for _, c := range g.cases {
			if got := g.get(c[0]); got != c[1] {
				t.Errorf("%s: input %d => %d, want %d", g.name, c[0], got, c[1])
			}
		}
	}
}
