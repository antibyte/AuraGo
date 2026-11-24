package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestVideoStudioPlanConfigDefaultsAndClamps(t *testing.T) {
	cfg, err := loadStorymakerYAML(t, "{}\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := (VideoStudioPlanConfig{Enabled: true, MaxDailyJobs: 20, JobTimeoutSeconds: 900}); cfg.VideoStudio.Plan != want {
		t.Fatalf("defaults = %+v, want %+v", cfg.VideoStudio.Plan, want)
	}
	cfg, err = loadStorymakerYAML(t, "video_studio:\n  plan:\n    enabled: false\n    agent_provider: \" main \"\n    agent_model: \" m1 \"\n    max_daily_jobs: 5000\n    job_timeout_seconds: 30\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := (VideoStudioPlanConfig{AgentProvider: "main", AgentModel: "m1", MaxDailyJobs: 1000, JobTimeoutSeconds: 120}); cfg.VideoStudio.Plan != want {
		t.Fatalf("explicit values = %+v, want %+v", cfg.VideoStudio.Plan, want)
	}
	for _, tc := range []struct{ in, want VideoStudioPlanConfig }{
		{VideoStudioPlanConfig{MaxDailyJobs: -3, JobTimeoutSeconds: 7200}, VideoStudioPlanConfig{MaxDailyJobs: 1, JobTimeoutSeconds: 3600}},
		{VideoStudioPlanConfig{MaxDailyJobs: 1000, JobTimeoutSeconds: 120}, VideoStudioPlanConfig{MaxDailyJobs: 1000, JobTimeoutSeconds: 120}},
		{VideoStudioPlanConfig{Enabled: true, MaxDailyJobs: 1, JobTimeoutSeconds: 3600}, VideoStudioPlanConfig{Enabled: true, MaxDailyJobs: 1, JobTimeoutSeconds: 3600}},
	} {
		c := VideoStudioConfig{Plan: tc.in}
		if err := NormalizeVideoStudioConfig(&c); err != nil {
			t.Fatalf("%+v rejected: %v", tc.in, err)
		}
		if c.Plan != tc.want {
			t.Errorf("normalize %+v = %+v, want %+v", tc.in, c.Plan, tc.want)
		}
	}
}

// The shipped template must describe the same defaults the loader applies, plan block included.
func TestVideoStudioTemplateMatchesLoadDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse template: %v", err)
	}
	node, ok := doc["video_studio"]
	if !ok {
		t.Fatal("config_template.yaml has no video_studio: block")
	}
	raw, err := yaml.Marshal(&node)
	if err != nil {
		t.Fatal(err)
	}
	var tpl VideoStudioConfig
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&tpl); err != nil {
		t.Fatalf("template block does not decode into VideoStudioConfig: %v", err)
	}
	if want := (VideoStudioPlanConfig{Enabled: true, MaxDailyJobs: 20, JobTimeoutSeconds: 900}); tpl.Plan != want {
		t.Fatalf("the template must write the plan defaults: %+v", tpl.Plan)
	}
	if err := NormalizeVideoStudioConfig(&tpl); err != nil {
		t.Fatalf("template values rejected: %v", err)
	}
	cfg, err := loadStorymakerYAML(t, "{}\n")
	if err != nil {
		t.Fatal(err)
	}
	if tpl != cfg.VideoStudio {
		t.Fatalf("template %+v differs from loader defaults %+v", tpl, cfg.VideoStudio)
	}
}

func TestVideoStudioPlanAgentProviderReferences(t *testing.T) {
	var c Config
	c.VideoStudio.Plan.AgentProvider = "p1"
	found := false
	for _, ref := range c.ProviderReferences() {
		if ref.Path == "video_studio.plan.agent_provider" && ref.Role == "video_studio_plan_agent" && ref.ProviderID == "p1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("video_studio.plan.agent_provider reference missing: %+v", c.ProviderReferences())
	}
	c.Providers = []ProviderEntry{{ID: "p1", Type: "openrouter", Usage: ProviderUsageDecisions}}
	if err := validateDecisionsOnlyProviderReferences(&c); err == nil || !strings.Contains(err.Error(), "video_studio.plan.agent_provider") {
		t.Fatalf("a decisions-only provider must not plan clips: %v", err)
	}
}
