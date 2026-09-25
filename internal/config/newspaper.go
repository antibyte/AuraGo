package config

// NewspaperConfig controls the optional, server-owned daily publication.
// Interests and destinations live in the Newspaper profile, never in YAML.
type NewspaperConfig struct {
	Enabled       bool `yaml:"enabled" json:"enabled"`
	ReadOnly      bool `yaml:"readonly" json:"readonly"`
	MaxMinutes    int  `yaml:"max_minutes" json:"max_minutes"`
	MaxPages      int  `yaml:"max_pages" json:"max_pages"`
	MaxEditions   int  `yaml:"max_editions" json:"max_editions"`
	AllowEmail    bool `yaml:"allow_email" json:"allow_email"`
	AllowTelegram bool `yaml:"allow_telegram" json:"allow_telegram"`
}
