package config

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// DefaultRealtimeSpeechParkAfterSeconds is the inactivity threshold used
	// when realtime speech is enabled without an explicit parking interval.
	DefaultRealtimeSpeechParkAfterSeconds = 5
	MinRealtimeSpeechParkAfterSeconds     = 5
	MaxRealtimeSpeechParkAfterSeconds     = 60
)

var realtimeSpeechProfileIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// RealtimeSpeechConfig is intentionally independent from the regular LLM
// provider list because realtime audio sessions use provider-specific
// transports, credentials, voices, and lifecycle rules.
type RealtimeSpeechConfig struct {
	Enabled          bool                    `yaml:"enabled" json:"enabled"`
	DefaultProfile   string                  `yaml:"default_profile" json:"default_profile"`
	ParkAfterSeconds int                     `yaml:"park_after_seconds" json:"park_after_seconds"`
	Profiles         []RealtimeSpeechProfile `yaml:"profiles" json:"profiles"`
}

// RealtimeSpeechProfile describes one realtime speech provider connection.
// APIKey is runtime-only and is hydrated exclusively from the encrypted vault.
type RealtimeSpeechProfile struct {
	ID       string `yaml:"id" json:"id"`
	Name     string `yaml:"name" json:"name"`
	Provider string `yaml:"provider" json:"provider"`
	Model    string `yaml:"model" json:"model"`
	Voice    string `yaml:"voice,omitempty" json:"voice"`
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	APIKey   string `yaml:"-" json:"-"`
}

// NormalizeRealtimeSpeechConfig applies safe runtime defaults without enabling
// the feature or changing existing profile identities.
func NormalizeRealtimeSpeechConfig(cfg *RealtimeSpeechConfig) {
	if cfg == nil {
		return
	}
	if cfg.ParkAfterSeconds == 0 {
		cfg.ParkAfterSeconds = DefaultRealtimeSpeechParkAfterSeconds
	}
	cfg.DefaultProfile = strings.TrimSpace(cfg.DefaultProfile)
	for i := range cfg.Profiles {
		profile := &cfg.Profiles[i]
		profile.ID = strings.TrimSpace(profile.ID)
		profile.Name = strings.TrimSpace(profile.Name)
		profile.Provider = strings.ToLower(strings.TrimSpace(profile.Provider))
		profile.Model = strings.TrimSpace(profile.Model)
		profile.Voice = strings.TrimSpace(profile.Voice)
	}
}

// ValidateRealtimeSpeechConfig checks transport-independent invariants during
// startup. Provider model catalogs are validated by the realtime speech API so
// a newer catalog can still load an older, deprecated profile safely.
func ValidateRealtimeSpeechConfig(cfg RealtimeSpeechConfig) error {
	if cfg.ParkAfterSeconds < MinRealtimeSpeechParkAfterSeconds ||
		cfg.ParkAfterSeconds > MaxRealtimeSpeechParkAfterSeconds {
		return fmt.Errorf("realtime_speech.park_after_seconds must be between %d and %d", MinRealtimeSpeechParkAfterSeconds, MaxRealtimeSpeechParkAfterSeconds)
	}
	seen := make(map[string]struct{}, len(cfg.Profiles))
	defaultFound := cfg.DefaultProfile == ""
	for _, profile := range cfg.Profiles {
		if err := ValidateRealtimeSpeechProfileID(profile.ID); err != nil {
			return err
		}
		if _, duplicate := seen[profile.ID]; duplicate {
			return fmt.Errorf("duplicate realtime speech profile ID %q", profile.ID)
		}
		seen[profile.ID] = struct{}{}
		if profile.ID == cfg.DefaultProfile {
			defaultFound = true
			if !profile.Enabled {
				return fmt.Errorf("default realtime speech profile %q is disabled", cfg.DefaultProfile)
			}
		}
	}
	if !defaultFound {
		return fmt.Errorf("default realtime speech profile %q does not exist", cfg.DefaultProfile)
	}
	return nil
}

// ValidateRealtimeSpeechProfileID validates profile IDs used in both YAML and
// vault key names.
func ValidateRealtimeSpeechProfileID(id string) error {
	id = strings.TrimSpace(id)
	if !realtimeSpeechProfileIDPattern.MatchString(id) {
		return fmt.Errorf("realtime speech profile ID must be 1-64 lowercase letters, numbers, dashes, or underscores")
	}
	return nil
}

// RealtimeSpeechProfileAPIKeyVaultKey returns the canonical vault key for a
// profile. Invalid IDs return an empty string so callers cannot construct
// ambiguous secret names.
func RealtimeSpeechProfileAPIKeyVaultKey(id string) string {
	id = strings.TrimSpace(id)
	if ValidateRealtimeSpeechProfileID(id) != nil {
		return ""
	}
	return "realtime_speech_profile_" + id + "_api_key"
}
