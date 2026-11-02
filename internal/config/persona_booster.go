package config

import "strings"

// PersonaBoosterConfig controls temporary persona phases and the static voice
// amplifier. Defaults are set in Load before the YAML is decoded, so missing
// keys stay active.
type PersonaBoosterConfig struct {
	Enabled         bool   `yaml:"enabled"`          // persona phases and voice amplifier (default: true)
	Intensity       string `yaml:"intensity"`        // mild | strong | wild (default: strong)
	Spontaneous     bool   `yaml:"spontaneous"`      // allow random phases (default: true)
	CooldownMinutes int    `yaml:"cooldown_minutes"` // minutes without low-priority phases after a phase ends, 0-1440 (default: 20)
}

// DefaultPersonaBoosterConfig returns the on-by-default booster settings.
func DefaultPersonaBoosterConfig() PersonaBoosterConfig {
	return PersonaBoosterConfig{Enabled: true, Intensity: "strong", Spontaneous: true, CooldownMinutes: 20}
}

// NormalizePersonaBoosterConfig repairs decoded values. It reports whether a
// non-empty, unknown intensity had to be replaced by "strong".
func NormalizePersonaBoosterConfig(c *PersonaBoosterConfig) bool {
	invalid := false
	switch value := strings.ToLower(strings.TrimSpace(c.Intensity)); value {
	case "mild", "strong", "wild":
		c.Intensity = value
	case "":
		c.Intensity = "strong"
	default:
		c.Intensity = "strong"
		invalid = true
	}
	c.CooldownMinutes = min(max(c.CooldownMinutes, 0), 1440)
	return invalid
}
