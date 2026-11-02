package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func loadStorymakerYAML(t *testing.T, content string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return Load(path)
}

func TestStorymakerDefaultsOn(t *testing.T) {
	want := StorymakerConfig{Enabled: true, Render: StorymakerRenderConfig{TimeoutSeconds: 3600, MaxDurationSeconds: 900}}
	for name, content := range map[string]string{
		"empty file":   "{}\n",
		"null section": "storymaker:\n",
		"render only":  "storymaker:\n  render:\n    transport: png\n",
	} {
		cfg, err := loadStorymakerYAML(t, content)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := cfg.Storymaker
		wantHere := want
		if name == "render only" {
			wantHere.Render.Transport = "png"
		}
		if got != wantHere {
			t.Fatalf("%s: storymaker = %+v, want %+v", name, got, wantHere)
		}
	}
	cfg, err := loadStorymakerYAML(t, "storymaker:\n  enabled: false\n  render:\n    timeout_seconds: 120\n    max_duration_seconds: 60\n    browser_path: /usr/bin/chromium\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storymaker.Enabled || cfg.Storymaker.Render.TimeoutSeconds != 120 || cfg.Storymaker.Render.MaxDurationSeconds != 60 || cfg.Storymaker.Render.BrowserPath != "/usr/bin/chromium" {
		t.Fatalf("explicit values lost: %+v", cfg.Storymaker)
	}
}

func TestNormalizeStorymakerConfigRanges(t *testing.T) {
	ok := []StorymakerRenderConfig{
		{}, {TimeoutSeconds: 60}, {TimeoutSeconds: 14400}, {MaxDurationSeconds: 10}, {MaxDurationSeconds: 3600},
		{Transport: " WebP "}, {Transport: "raw"}, {Transport: "screenshot"}, {BrowserPath: "/usr/bin/chromium"},
	}
	for _, r := range ok {
		c := StorymakerConfig{Render: r}
		if err := NormalizeStorymakerConfig(&c); err != nil {
			t.Errorf("%+v rejected: %v", r, err)
		}
		if c.Render.TimeoutSeconds == 0 || c.Render.MaxDurationSeconds == 0 {
			t.Errorf("%+v: defaults not applied: %+v", r, c.Render)
		}
	}
	bad := []StorymakerRenderConfig{
		{TimeoutSeconds: -1}, {TimeoutSeconds: 59}, {TimeoutSeconds: 14401},
		{MaxDurationSeconds: -5}, {MaxDurationSeconds: 9}, {MaxDurationSeconds: 3601},
		{Transport: "websocket"}, {BrowserPath: "chromium"}, {BrowserPath: "./chrome"}, {BrowserPath: "/usr/bin/a\nb"},
	}
	for _, r := range bad {
		c := StorymakerConfig{Render: r}
		if NormalizeStorymakerConfig(&c) == nil {
			t.Errorf("%+v accepted", r)
		}
	}
	c := StorymakerConfig{Render: StorymakerRenderConfig{Transport: " WebP ", BrowserPath: " /usr/bin/chromium "}}
	if err := NormalizeStorymakerConfig(&c); err != nil || c.Render.Transport != "webp" || c.Render.BrowserPath != "/usr/bin/chromium" {
		t.Errorf("normalisation: %+v (%v)", c.Render, err)
	}
	if _, err := loadStorymakerYAML(t, "storymaker:\n  render:\n    timeout_seconds: 5\n"); err == nil {
		t.Error("Load accepted a render timeout below 60 seconds")
	}
}

// The shipped template must describe the same defaults the loader applies.
func TestStorymakerTemplateMatchesLoadDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse template: %v", err)
	}
	node, ok := doc["storymaker"]
	if !ok {
		t.Fatal("config_template.yaml has no storymaker: block")
	}
	raw, err := yaml.Marshal(&node)
	if err != nil {
		t.Fatal(err)
	}
	var tpl StorymakerConfig
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&tpl); err != nil {
		t.Fatalf("template block does not decode into StorymakerConfig: %v", err)
	}
	if err := NormalizeStorymakerConfig(&tpl); err != nil {
		t.Fatalf("template values rejected: %v", err)
	}
	cfg, err := loadStorymakerYAML(t, "{}\n")
	if err != nil {
		t.Fatal(err)
	}
	if tpl != cfg.Storymaker {
		t.Fatalf("template %+v differs from loader defaults %+v", tpl, cfg.Storymaker)
	}
}
