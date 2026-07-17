// Package realtimespeech contains provider catalogs and the server-side
// lifecycle for AuraGo's realtime speech interface.
package realtimespeech

import (
	"fmt"
	"slices"
	"strings"

	"aurago/internal/config"
)

const CatalogVersion = "2026-07-17"

const (
	ProviderOpenAI = "openai"
	ProviderXAI    = "xai"
	ProviderGemini = "gemini"
)

// Model describes a realtime model exposed by AuraGo.
type Model struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Deprecated bool   `json:"deprecated"`
	Offered    bool   `json:"offered"`
	Note       string `json:"note,omitempty"`
}

// Voice describes a provider voice exposed by AuraGo.
type Voice struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// Capabilities documents transport and lifecycle features used by the browser
// adapter without exposing credentials.
type Capabilities struct {
	Transport             string `json:"transport"`
	ParkStrategy          string `json:"park_strategy"`
	SessionResumption     bool   `json:"session_resumption"`
	ManualTurnDetection   bool   `json:"manual_turn_detection"`
	FunctionCalling       bool   `json:"function_calling"`
	InputSampleRate       int    `json:"input_sample_rate"`
	OutputSampleRate      int    `json:"output_sample_rate"`
	DynamicVoiceDiscovery bool   `json:"dynamic_voice_discovery"`
}

// ProviderCatalog is the versioned public metadata for one provider.
type ProviderCatalog struct {
	ID           string       `json:"id"`
	Label        string       `json:"label"`
	DefaultModel string       `json:"default_model"`
	DefaultVoice string       `json:"default_voice"`
	Models       []Model      `json:"models"`
	Voices       []Voice      `json:"voices"`
	Capabilities Capabilities `json:"capabilities"`
}

var providerCatalogs = []ProviderCatalog{
	{
		ID:           ProviderOpenAI,
		Label:        "OpenAI",
		DefaultModel: "gpt-realtime-2.1",
		DefaultVoice: "marin",
		Models: []Model{
			{ID: "gpt-realtime-2.1", Label: "GPT Realtime 2.1", Offered: true},
			{ID: "gpt-realtime-2.1-mini", Label: "GPT Realtime 2.1 Mini", Offered: true},
			{ID: "gpt-realtime-2", Label: "GPT Realtime 2", Offered: true},
			{ID: "gpt-realtime-1.5", Label: "GPT Realtime 1.5", Offered: true},
			{ID: "gpt-realtime", Label: "GPT Realtime", Offered: true},
			{ID: "gpt-realtime-mini", Label: "GPT Realtime Mini", Offered: true},
		},
		Voices: []Voice{
			{ID: "marin", Label: "Marin", Description: "Natural and expressive"},
			{ID: "cedar", Label: "Cedar", Description: "Natural and balanced"},
			{ID: "alloy", Label: "Alloy"},
			{ID: "ash", Label: "Ash"},
			{ID: "ballad", Label: "Ballad"},
			{ID: "coral", Label: "Coral"},
			{ID: "echo", Label: "Echo"},
			{ID: "sage", Label: "Sage"},
			{ID: "shimmer", Label: "Shimmer"},
			{ID: "verse", Label: "Verse"},
		},
		Capabilities: Capabilities{
			Transport:           "webrtc",
			ParkStrategy:        "warm_audio_gate",
			SessionResumption:   false,
			ManualTurnDetection: true,
			FunctionCalling:     true,
			InputSampleRate:     16000,
			OutputSampleRate:    24000,
		},
	},
	{
		ID:           ProviderXAI,
		Label:        "xAI",
		DefaultModel: "grok-voice-think-fast-1.0",
		DefaultVoice: "ara",
		Models: []Model{
			{ID: "grok-voice-think-fast-1.0", Label: "Grok Voice Think Fast 1.0", Offered: true},
			{ID: "grok-voice-latest", Label: "Grok Voice Latest", Offered: true, Note: "Moving alias; pin a version for production."},
			{ID: "grok-voice-fast-1.0", Label: "Grok Voice Fast 1.0", Deprecated: true, Offered: false, Note: "Accepted only when loading an existing profile."},
		},
		Voices: []Voice{
			{ID: "ara", Label: "Ara"},
			{ID: "eve", Label: "Eve"},
			{ID: "leo", Label: "Leo"},
			{ID: "rex", Label: "Rex"},
			{ID: "sal", Label: "Sal"},
		},
		Capabilities: Capabilities{
			Transport:             "websocket",
			ParkStrategy:          "conversation_resume",
			SessionResumption:     true,
			ManualTurnDetection:   true,
			FunctionCalling:       true,
			InputSampleRate:       16000,
			OutputSampleRate:      24000,
			DynamicVoiceDiscovery: true,
		},
	},
	{
		ID:           ProviderGemini,
		Label:        "Gemini",
		DefaultModel: "gemini-3.1-flash-live-preview",
		DefaultVoice: "Kore",
		Models: []Model{
			{ID: "gemini-3.1-flash-live-preview", Label: "Gemini 3.1 Flash Live Preview", Offered: true},
			{ID: "gemini-2.5-flash-native-audio-preview-12-2025", Label: "Gemini 2.5 Flash Native Audio Preview 12-2025", Offered: true},
			{ID: "gemini-live-2.5-flash-preview", Label: "Gemini Live 2.5 Flash Preview", Deprecated: true, Offered: false, Note: "No longer offered for new profiles."},
			{ID: "gemini-2.0-flash-live-001", Label: "Gemini 2.0 Flash Live 001", Deprecated: true, Offered: false, Note: "No longer offered for new profiles."},
		},
		Voices: []Voice{
			{ID: "Zephyr", Label: "Zephyr", Description: "Bright"},
			{ID: "Puck", Label: "Puck", Description: "Upbeat"},
			{ID: "Charon", Label: "Charon", Description: "Informative"},
			{ID: "Kore", Label: "Kore", Description: "Firm"},
			{ID: "Fenrir", Label: "Fenrir", Description: "Excitable"},
			{ID: "Leda", Label: "Leda", Description: "Youthful"},
			{ID: "Orus", Label: "Orus", Description: "Firm"},
			{ID: "Aoede", Label: "Aoede", Description: "Breezy"},
			{ID: "Callirrhoe", Label: "Callirrhoe", Description: "Easy-going"},
			{ID: "Autonoe", Label: "Autonoe", Description: "Bright"},
			{ID: "Enceladus", Label: "Enceladus", Description: "Breathy"},
			{ID: "Iapetus", Label: "Iapetus", Description: "Clear"},
			{ID: "Umbriel", Label: "Umbriel", Description: "Easy-going"},
			{ID: "Algieba", Label: "Algieba", Description: "Smooth"},
			{ID: "Despina", Label: "Despina", Description: "Smooth"},
			{ID: "Erinome", Label: "Erinome", Description: "Clear"},
			{ID: "Algenib", Label: "Algenib", Description: "Gravelly"},
			{ID: "Rasalgethi", Label: "Rasalgethi", Description: "Informative"},
			{ID: "Laomedeia", Label: "Laomedeia", Description: "Upbeat"},
			{ID: "Achernar", Label: "Achernar", Description: "Soft"},
			{ID: "Alnilam", Label: "Alnilam", Description: "Firm"},
			{ID: "Schedar", Label: "Schedar", Description: "Even"},
			{ID: "Gacrux", Label: "Gacrux", Description: "Mature"},
			{ID: "Pulcherrima", Label: "Pulcherrima", Description: "Forward"},
			{ID: "Achird", Label: "Achird", Description: "Friendly"},
			{ID: "Zubenelgenubi", Label: "Zubenelgenubi", Description: "Casual"},
			{ID: "Vindemiatrix", Label: "Vindemiatrix", Description: "Gentle"},
			{ID: "Sadachbia", Label: "Sadachbia", Description: "Lively"},
			{ID: "Sadaltager", Label: "Sadaltager", Description: "Knowledgeable"},
			{ID: "Sulafat", Label: "Sulafat", Description: "Warm"},
		},
		Capabilities: Capabilities{
			Transport:           "websocket",
			ParkStrategy:        "resumption_handle",
			SessionResumption:   true,
			ManualTurnDetection: true,
			FunctionCalling:     true,
			InputSampleRate:     16000,
			OutputSampleRate:    24000,
		},
	},
}

// Catalog returns a deep copy so callers can safely attach dynamic metadata.
func Catalog() []ProviderCatalog {
	out := make([]ProviderCatalog, len(providerCatalogs))
	for i, provider := range providerCatalogs {
		out[i] = provider
		out[i].Models = append([]Model(nil), provider.Models...)
		out[i].Voices = append([]Voice(nil), provider.Voices...)
	}
	return out
}

// Provider returns a copied provider catalog.
func Provider(id string) (ProviderCatalog, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, provider := range providerCatalogs {
		if provider.ID == id {
			provider.Models = append([]Model(nil), provider.Models...)
			provider.Voices = append([]Voice(nil), provider.Voices...)
			return provider, true
		}
	}
	return ProviderCatalog{}, false
}

// NormalizeAndValidateConfig returns a normalized copy suitable for
// persistence. Deprecated models may remain on an existing profile but cannot
// be selected for a newly created profile.
func NormalizeAndValidateConfig(input config.RealtimeSpeechConfig, existing map[string]config.RealtimeSpeechProfile) (config.RealtimeSpeechConfig, error) {
	config.NormalizeRealtimeSpeechConfig(&input)
	if input.ParkAfterSeconds < config.MinRealtimeSpeechParkAfterSeconds ||
		input.ParkAfterSeconds > config.MaxRealtimeSpeechParkAfterSeconds {
		return input, fmt.Errorf("park_after_seconds must be between %d and %d", config.MinRealtimeSpeechParkAfterSeconds, config.MaxRealtimeSpeechParkAfterSeconds)
	}

	seen := make(map[string]struct{}, len(input.Profiles))
	for i := range input.Profiles {
		profile := &input.Profiles[i]
		if err := config.ValidateRealtimeSpeechProfileID(profile.ID); err != nil {
			return input, err
		}
		if _, ok := seen[profile.ID]; ok {
			return input, fmt.Errorf("duplicate realtime speech profile ID %q", profile.ID)
		}
		seen[profile.ID] = struct{}{}
		if profile.Name == "" {
			return input, fmt.Errorf("realtime speech profile %q requires a name", profile.ID)
		}

		catalog, ok := Provider(profile.Provider)
		if !ok {
			return input, fmt.Errorf("unsupported realtime speech provider %q", profile.Provider)
		}
		if profile.Model == "" {
			profile.Model = catalog.DefaultModel
		}
		model, ok := modelByID(catalog.Models, profile.Model)
		if !ok {
			return input, fmt.Errorf("model %q is not in the %s realtime catalog", profile.Model, catalog.Label)
		}
		if !model.Offered {
			old, existed := existing[profile.ID]
			if !existed || old.Provider != profile.Provider || old.Model != profile.Model {
				return input, fmt.Errorf("model %q is deprecated and cannot be selected for a new profile", profile.Model)
			}
		}
		if profile.Voice == "" {
			profile.Voice = catalog.DefaultVoice
		}
		if profile.Provider != ProviderXAI && !voiceExists(catalog.Voices, profile.Voice) {
			return input, fmt.Errorf("voice %q is not in the %s realtime catalog", profile.Voice, catalog.Label)
		}
	}

	if input.DefaultProfile != "" {
		index := slices.IndexFunc(input.Profiles, func(profile config.RealtimeSpeechProfile) bool {
			return profile.ID == input.DefaultProfile
		})
		if index < 0 {
			return input, fmt.Errorf("default realtime speech profile %q does not exist", input.DefaultProfile)
		}
		if !input.Profiles[index].Enabled {
			return input, fmt.Errorf("default realtime speech profile %q is disabled", input.DefaultProfile)
		}
	}
	return input, nil
}

func modelByID(models []Model, id string) (Model, bool) {
	for _, model := range models {
		if model.ID == id {
			return model, true
		}
	}
	return Model{}, false
}

func voiceExists(voices []Voice, id string) bool {
	return slices.ContainsFunc(voices, func(voice Voice) bool {
		return strings.EqualFold(voice.ID, id)
	})
}
