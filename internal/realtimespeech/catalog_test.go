package realtimespeech

import (
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestCatalogContainsOnlySupportedAgentModels(t *testing.T) {
	providers := Catalog()
	if len(providers) != 3 {
		t.Fatalf("Catalog provider count = %d, want 3", len(providers))
	}
	expectedDefaults := map[string]string{
		ProviderOpenAI: "gpt-realtime-2.1",
		ProviderXAI:    "grok-voice-think-fast-1.0",
		ProviderGemini: "gemini-3.1-flash-live-preview",
	}
	for _, provider := range providers {
		if provider.DefaultModel != expectedDefaults[provider.ID] {
			t.Fatalf("%s default model = %q, want %q", provider.ID, provider.DefaultModel, expectedDefaults[provider.ID])
		}
		if provider.Capabilities.InputSampleRate != 16000 || provider.Capabilities.OutputSampleRate != 24000 {
			t.Fatalf("%s audio rates = %d/%d, want 16000/24000", provider.ID, provider.Capabilities.InputSampleRate, provider.Capabilities.OutputSampleRate)
		}
		for _, model := range provider.Models {
			if strings.Contains(model.ID, "translate") || strings.Contains(model.ID, "whisper") {
				t.Fatalf("special-purpose model %q must not be offered as an agent", model.ID)
			}
		}
	}
}

func TestNormalizeAndValidateConfigRejectsDeprecatedModelForNewProfile(t *testing.T) {
	input := config.RealtimeSpeechConfig{
		ParkAfterSeconds: 5,
		DefaultProfile:   "legacy",
		Profiles: []config.RealtimeSpeechProfile{{
			ID:       "legacy",
			Name:     "Legacy",
			Provider: ProviderXAI,
			Model:    "grok-voice-fast-1.0",
			Voice:    "ara",
			Enabled:  true,
		}},
	}
	if _, err := NormalizeAndValidateConfig(input, nil); err == nil {
		t.Fatal("expected a deprecated model to be rejected for a new profile")
	}

	existing := map[string]config.RealtimeSpeechProfile{"legacy": input.Profiles[0]}
	got, err := NormalizeAndValidateConfig(input, existing)
	if err != nil {
		t.Fatalf("existing deprecated profile should remain loadable: %v", err)
	}
	if got.Profiles[0].Model != "grok-voice-fast-1.0" {
		t.Fatalf("model changed to %q", got.Profiles[0].Model)
	}
}

func TestXAICatalogUsesCanonicalVoiceIDs(t *testing.T) {
	provider, ok := Provider(ProviderXAI)
	if !ok {
		t.Fatal("xAI provider missing")
	}
	if provider.DefaultVoice != "ara" {
		t.Fatalf("xAI default voice = %q, want ara", provider.DefaultVoice)
	}
	want := []string{"ara", "eve", "leo", "rex", "sal"}
	if len(provider.Voices) != len(want) {
		t.Fatalf("xAI voice count = %d, want %d", len(provider.Voices), len(want))
	}
	for index, voice := range provider.Voices {
		if voice.ID != want[index] {
			t.Fatalf("xAI voice %d ID = %q, want %q", index, voice.ID, want[index])
		}
	}
}

func TestNormalizeAndValidateConfigAppliesProviderDefaults(t *testing.T) {
	input := config.RealtimeSpeechConfig{
		ParkAfterSeconds: 5,
		DefaultProfile:   "main",
		Profiles: []config.RealtimeSpeechProfile{{
			ID:       "main",
			Name:     "Main",
			Provider: ProviderGemini,
			Enabled:  true,
		}},
	}
	got, err := NormalizeAndValidateConfig(input, nil)
	if err != nil {
		t.Fatalf("NormalizeAndValidateConfig: %v", err)
	}
	if got.Profiles[0].Model != "gemini-3.1-flash-live-preview" || got.Profiles[0].Voice != "Kore" {
		t.Fatalf("unexpected defaults: %+v", got.Profiles[0])
	}
}

func TestPrivateToolsExposeOnlyAuraGoBridge(t *testing.T) {
	tools := PrivateTools()
	if len(tools) != 2 {
		t.Fatalf("PrivateTools count = %d, want 2", len(tools))
	}
	names := []string{tools[0]["name"].(string), tools[1]["name"].(string)}
	if names[0] != "aurago_execute" || names[1] != "aurago_cancel_current_task" {
		t.Fatalf("unexpected private tools: %v", names)
	}
	if strings.Contains(strings.ToLower(AuraGoSystemContract), "forward it") {
		t.Fatal("system contract must not instruct delegation language")
	}
}
