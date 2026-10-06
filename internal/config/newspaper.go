package config

import (
	"fmt"
	"strings"
)

const (
	NewspaperBudgetAuto  = "auto"
	NewspaperBudgetFixed = "fixed"

	NewspaperOverviewGoogleNews = "google_news"
	NewspaperOverviewHackerNews = "hacker_news"
	NewspaperOverviewTechmeme   = "techmeme"
)

// NewspaperConfig controls the optional, server-owned daily publication.
// Interests and destinations live in the Newspaper profile, never in YAML.
type NewspaperConfig struct {
	Enabled         bool     `yaml:"enabled" json:"enabled"`
	ReadOnly        bool     `yaml:"readonly" json:"readonly"`
	BudgetMode      string   `yaml:"budget_mode" json:"budget_mode"`
	OverviewSources []string `yaml:"overview_sources" json:"overview_sources"`
	MaxMinutes      int      `yaml:"max_minutes" json:"max_minutes"`
	MaxPages        int      `yaml:"max_pages" json:"max_pages"`
	MaxSearches     int      `yaml:"max_searches" json:"max_searches"`
	MaxEditions     int      `yaml:"max_editions" json:"max_editions"`
	AllowEmail      bool     `yaml:"allow_email" json:"allow_email"`
	AllowTelegram   bool     `yaml:"allow_telegram" json:"allow_telegram"`
}

// NormalizeNewspaperConfig validates the budget mode and built-in overview
// source identifiers while preserving source selection order.
func NormalizeNewspaperConfig(cfg *NewspaperConfig) error {
	if cfg == nil {
		return nil
	}
	switch mode := strings.ToLower(strings.TrimSpace(cfg.BudgetMode)); mode {
	case "", NewspaperBudgetFixed:
		cfg.BudgetMode = NewspaperBudgetFixed
	case NewspaperBudgetAuto:
		cfg.BudgetMode = NewspaperBudgetAuto
	default:
		return fmt.Errorf("newspaper.budget_mode must be auto or fixed")
	}
	allowed := map[string]struct{}{
		NewspaperOverviewGoogleNews: {},
		NewspaperOverviewHackerNews: {},
		NewspaperOverviewTechmeme:   {},
	}
	seen := make(map[string]struct{}, len(cfg.OverviewSources))
	sources := make([]string, 0, len(cfg.OverviewSources))
	for _, raw := range cfg.OverviewSources {
		source := strings.ToLower(strings.TrimSpace(raw))
		if source == "" {
			continue
		}
		if _, ok := allowed[source]; !ok {
			return fmt.Errorf("newspaper.overview_sources contains unsupported source %q", raw)
		}
		if _, ok := seen[source]; ok {
			continue
		}
		seen[source] = struct{}{}
		sources = append(sources, source)
	}
	cfg.OverviewSources = sources
	return nil
}

// EffectiveMaxSearches bounds discovery requests, including retries and fallbacks.
func (c NewspaperConfig) EffectiveMaxSearches() int {
	if c.MaxSearches == 0 {
		return 32
	}
	return max(1, min(c.MaxSearches, 64))
}
