package setup

import (
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

//go:embed setup_profiles.yaml
var embeddedProfilesYAML []byte

// SetupProfile defines a pre-configured provider profile for the setup wizard.
type SetupProfile struct {
	ID                    string          `yaml:"id"                     json:"id"`
	Name                  string          `yaml:"name"                   json:"name"`
	Description           string          `yaml:"description"            json:"description"`
	Icon                  string          `yaml:"icon"                   json:"icon"`
	SortOrder             int             `yaml:"sort_order"             json:"sort_order"`
	PricingLabel          string          `yaml:"pricing_label"          json:"pricing_label"`
	PricingURL            string          `yaml:"pricing_url"            json:"pricing_url"`
	KeyURL                string          `yaml:"key_url"                json:"key_url"`
	KeyPlaceholder        string          `yaml:"key_placeholder"        json:"key_placeholder"`
	ProviderType          string          `yaml:"provider_type"          json:"provider_type"`
	BaseURL               string          `yaml:"base_url"               json:"base_url"`
	MainModel             string          `yaml:"main_model"             json:"main_model"`
	NativeFunctionCalling bool            `yaml:"native_function_calling" json:"native_function_calling"`
	Features              ProfileFeatures `yaml:"features"               json:"features"`
	Models                ProfileModels   `yaml:"models"                 json:"models"`
	DefaultTrustLevel     int             `yaml:"default_trust_level"    json:"default_trust_level"`
	TTS                   *ProfileTTS     `yaml:"tts,omitempty"          json:"tts,omitempty"`
	ConfigPatch           map[string]any  `yaml:"config_patch,omitempty" json:"config_patch,omitempty"`
}

// ProfileFeatures describes which subsystems a profile supports.
type ProfileFeatures struct {
	Vision          bool `yaml:"vision"           json:"vision"`
	Whisper         bool `yaml:"whisper"           json:"whisper"`
	Embeddings      bool `yaml:"embeddings"        json:"embeddings"`
	Helper          bool `yaml:"helper"            json:"helper"`
	TTS             bool `yaml:"tts"               json:"tts"`
	ImageGeneration bool `yaml:"image_generation"  json:"image_generation"`
	MusicGeneration bool `yaml:"music_generation"  json:"music_generation"`
	VideoGeneration bool `yaml:"video_generation"  json:"video_generation"`
}

// ProfileSubsystemModel holds the model and optional mode for a subsystem.
type ProfileSubsystemModel struct {
	Model string `yaml:"model"    json:"model"`
	Mode  string `yaml:"mode"     json:"mode,omitempty"`
}

// ProfileModels maps subsystem names to their model configuration.
type ProfileModels struct {
	Vision          *ProfileSubsystemModel `yaml:"vision,omitempty"           json:"vision,omitempty"`
	Whisper         *ProfileSubsystemModel `yaml:"whisper,omitempty"          json:"whisper,omitempty"`
	Embeddings      *ProfileSubsystemModel `yaml:"embeddings,omitempty"       json:"embeddings,omitempty"`
	Helper          *ProfileSubsystemModel `yaml:"helper,omitempty"           json:"helper,omitempty"`
	ImageGeneration *ProfileSubsystemModel `yaml:"image_generation,omitempty" json:"image_generation,omitempty"`
	MusicGeneration *ProfileSubsystemModel `yaml:"music_generation,omitempty" json:"music_generation,omitempty"`
}

// ProfileTTS holds TTS-specific config. TTS does NOT use the provider system —
// it has its own vault keys and config structure.
type ProfileTTS struct {
	Provider string  `yaml:"provider" json:"provider"` // "minimax", "elevenlabs", "google", "piper"
	ModelID  string  `yaml:"model_id" json:"model_id"`
	VoiceID  string  `yaml:"voice_id" json:"voice_id"`
	Speed    float64 `yaml:"speed"    json:"speed,omitempty"`
}

type profilesFile struct {
	Profiles []SetupProfile `yaml:"profiles"`
}

// LoadProfiles loads setup profiles from the given YAML file path.
// If the file does not exist or is unreadable, it falls back to the
// embedded defaults.
func LoadProfiles(path string, logger *slog.Logger) []SetupProfile {
	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			profiles, parseErr := parseProfiles(data)
			if parseErr == nil && len(profiles) > 0 {
				logger.Info("Setup profiles loaded from file", "path", path, "count", len(profiles))
				return profiles
			}
			if parseErr != nil {
				logger.Warn("Failed to parse setup profiles file, using embedded defaults", "path", path, "error", parseErr)
			}
		}
	}

	profiles, err := parseProfiles(embeddedProfilesYAML)
	if err != nil {
		logger.Error("Failed to parse embedded setup profiles", "error", err)
		return nil
	}
	logger.Info("Setup profiles loaded from embedded defaults", "count", len(profiles))
	return profiles
}

// parseProfiles unmarshals a YAML document, validates profiles, and returns
// them sorted by sort_order.
func parseProfiles(data []byte) ([]SetupProfile, error) {
	var pf profilesFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("unmarshal profiles: %w", err)
	}

	var valid []SetupProfile
	seen := make(map[string]bool)
	for _, p := range pf.Profiles {
		if p.ID == "" || p.Name == "" {
			continue
		}
		if seen[p.ID] {
			continue
		}
		// "custom" profile doesn't need provider/base_url/model
		if p.ID != "custom" && (p.ProviderType == "" || p.BaseURL == "" || p.MainModel == "") {
			continue
		}
		seen[p.ID] = true
		valid = append(valid, p)
	}

	sort.Slice(valid, func(i, j int) bool {
		return valid[i].SortOrder < valid[j].SortOrder
	})

	return valid, nil
}
