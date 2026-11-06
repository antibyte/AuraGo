package config

import "testing"

func TestStorymakerAgentConfigDefaultsAndRanges(t *testing.T) {
	cfg, err := loadStorymakerYAML(t, "{}\n")
	if err != nil {
		t.Fatal(err)
	}
	want := StorymakerAgentConfig{Enabled: true, JobTimeoutSeconds: 1800, MaxLengthSeconds: 180, MaxDailyJobs: 10}
	if cfg.Storymaker.Agent != want {
		t.Fatalf("defaults = %+v, want %+v", cfg.Storymaker.Agent, want)
	}

	cfg, err = loadStorymakerYAML(t, "storymaker:\n  agent:\n    enabled: false\n    provider: \" main \"\n    model: m1\n    max_length_seconds: 600\n")
	if err != nil {
		t.Fatal(err)
	}
	want = StorymakerAgentConfig{Provider: "main", Model: "m1", JobTimeoutSeconds: 1800, MaxLengthSeconds: 600, MaxDailyJobs: 10}
	if cfg.Storymaker.Agent != want {
		t.Fatalf("explicit values = %+v, want %+v", cfg.Storymaker.Agent, want)
	}

	for _, a := range []StorymakerAgentConfig{
		{JobTimeoutSeconds: 120}, {JobTimeoutSeconds: 7200}, {MaxLengthSeconds: 15}, {MaxLengthSeconds: 3600}, {MaxDailyJobs: 1}, {MaxDailyJobs: 1000},
	} {
		c := StorymakerConfig{Agent: a}
		if err := NormalizeStorymakerConfig(&c); err != nil {
			t.Errorf("%+v rejected: %v", a, err)
		}
		if c.Agent.Enabled {
			t.Errorf("%+v: normalize changed Enabled", a)
		}
	}
	for _, a := range []StorymakerAgentConfig{
		{JobTimeoutSeconds: 119}, {JobTimeoutSeconds: 7201}, {JobTimeoutSeconds: -1},
		{MaxLengthSeconds: 14}, {MaxLengthSeconds: 3601},
		{MaxDailyJobs: 1001}, {MaxDailyJobs: -1},
	} {
		c := StorymakerConfig{Agent: a}
		if NormalizeStorymakerConfig(&c) == nil {
			t.Errorf("%+v accepted", a)
		}
	}
	if _, err := loadStorymakerYAML(t, "storymaker:\n  agent:\n    max_daily_jobs: 1001\n"); err == nil {
		t.Error("Load accepted 1001 daily jobs")
	}
}
