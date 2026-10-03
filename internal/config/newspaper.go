package config

// NewspaperConfig controls the optional, server-owned daily publication.
// Interests and destinations live in the Newspaper profile, never in YAML.
type NewspaperConfig struct {
	Enabled       bool `yaml:"enabled" json:"enabled"`
	ReadOnly      bool `yaml:"readonly" json:"readonly"`
	MaxMinutes    int  `yaml:"max_minutes" json:"max_minutes"`
	MaxPages      int  `yaml:"max_pages" json:"max_pages"`
	MaxSearches   int  `yaml:"max_searches" json:"max_searches"`
	MaxEditions   int  `yaml:"max_editions" json:"max_editions"`
	AllowEmail    bool `yaml:"allow_email" json:"allow_email"`
	AllowTelegram bool `yaml:"allow_telegram" json:"allow_telegram"`
}

// EffectiveMaxSearches bounds discovery requests, including retries and fallbacks.
func (c NewspaperConfig) EffectiveMaxSearches() int {
	if c.MaxSearches == 0 {
		return 32
	}
	return max(1, min(c.MaxSearches, 64))
}
