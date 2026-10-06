package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNewspaperSearchBudgetDefaultsAndBounds(t *testing.T) {
	for _, tc := range []struct{ value, want int }{{0, 32}, {-1, 1}, {1, 1}, {32, 32}, {64, 64}, {1000, 64}} {
		if got := (NewspaperConfig{MaxSearches: tc.value}).EffectiveMaxSearches(); got != tc.want {
			t.Fatalf("%d => %d, want %d", tc.value, got, tc.want)
		}
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("newspaper:\n  enabled: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Newspaper.MaxSearches != 32 || cfg.Newspaper.BudgetMode != "fixed" || len(cfg.Newspaper.OverviewSources) != 0 {
		t.Fatalf("legacy defaults: searches=%d mode=%q overview_sources=%v", cfg.Newspaper.MaxSearches, cfg.Newspaper.BudgetMode, cfg.Newspaper.OverviewSources)
	}
}

func TestNormalizeNewspaperConfigValidatesAndDeduplicatesOverviewSources(t *testing.T) {
	cfg := NewspaperConfig{
		BudgetMode:      " AUTO ",
		OverviewSources: []string{" google_news ", "techmeme", "GOOGLE_NEWS", " ", "hacker_news"},
	}
	if err := NormalizeNewspaperConfig(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.BudgetMode != NewspaperBudgetAuto {
		t.Fatalf("budget mode=%q", cfg.BudgetMode)
	}
	want := []string{NewspaperOverviewGoogleNews, NewspaperOverviewTechmeme, NewspaperOverviewHackerNews}
	if !reflect.DeepEqual(cfg.OverviewSources, want) {
		t.Fatalf("overview sources=%v, want %v", cfg.OverviewSources, want)
	}

	for _, invalid := range []NewspaperConfig{
		{BudgetMode: "automatic"},
		{OverviewSources: []string{"unknown_feed"}},
	} {
		if err := NormalizeNewspaperConfig(&invalid); err == nil {
			t.Fatalf("invalid config accepted: %+v", invalid)
		}
	}
}

func TestNewspaperLoadRejectsUnknownBudgetAndOverviewIDs(t *testing.T) {
	for _, body := range []string{
		"newspaper:\n  budget_mode: automatic\n",
		"newspaper:\n  overview_sources: [unknown_feed]\n",
	} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("Load accepted invalid newspaper config: %s", body)
		}
	}
}

func TestConfigTemplateOptsIntoAutoNewspaperBudget(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Newspaper NewspaperConfig `yaml:"newspaper"`
	}
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Newspaper.BudgetMode != NewspaperBudgetAuto || len(cfg.Newspaper.OverviewSources) != 0 {
		t.Fatalf("fresh template: mode=%q overview_sources=%v", cfg.Newspaper.BudgetMode, cfg.Newspaper.OverviewSources)
	}
}
