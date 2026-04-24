package setup

import (
	"log/slog"
	"testing"
)

func TestLoadEmbeddedProfiles(t *testing.T) {
	t.Parallel()

	profiles := LoadProfiles("", slog.Default())
	if len(profiles) == 0 {
		t.Fatal("expected at least one embedded profile")
	}

	// Check that profiles are sorted by sort_order
	for i := 1; i < len(profiles); i++ {
		if profiles[i].SortOrder < profiles[i-1].SortOrder {
			t.Fatalf("profiles not sorted: %q (order %d) comes after %q (order %d)",
				profiles[i].ID, profiles[i].SortOrder,
				profiles[i-1].ID, profiles[i-1].SortOrder)
		}
	}
}

func TestEmbeddedProfilesContainOpenRouter(t *testing.T) {
	t.Parallel()

	profiles := LoadProfiles("", slog.Default())

	found := false
	for _, p := range profiles {
		if p.ID == "openrouter" {
			found = true
			if p.ProviderType != "openrouter" {
				t.Fatalf("openrouter profile has wrong type: %q", p.ProviderType)
			}
			if p.MainModel == "" {
				t.Fatal("openrouter profile has no main_model")
			}
			if !p.Features.Vision {
				t.Fatal("openrouter profile should have vision enabled")
			}
			if !p.Features.Embeddings {
				t.Fatal("openrouter profile should have embeddings enabled")
			}
			break
		}
	}
	if !found {
		t.Fatal("expected openrouter profile in embedded profiles")
	}
}

func TestEmbeddedProfilesContainCustom(t *testing.T) {
	t.Parallel()

	profiles := LoadProfiles("", slog.Default())

	found := false
	for _, p := range profiles {
		if p.ID == "custom" {
			found = true
			if p.SortOrder != 99 {
				t.Fatalf("custom profile sort_order = %d, want 99", p.SortOrder)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected custom profile in embedded profiles")
	}
}

func TestMiniMaxProfileHasTTS(t *testing.T) {
	t.Parallel()

	profiles := LoadProfiles("", slog.Default())

	for _, p := range profiles {
		if p.ID == "minimax_coding" {
			if !p.Features.TTS {
				t.Fatal("minimax profile should have TTS enabled")
			}
			if p.TTS == nil {
				t.Fatal("minimax profile should have TTS config")
			}
			if p.TTS.Provider != "minimax" {
				t.Fatalf("minimax TTS provider = %q, want minimax", p.TTS.Provider)
			}
			if p.TTS.ModelID == "" {
				t.Fatal("minimax TTS should have model_id")
			}
			if p.TTS.VoiceID == "" {
				t.Fatal("minimax TTS should have voice_id")
			}
			if p.KeyPlaceholder != "sk-..." {
				t.Fatalf("minimax key placeholder = %q, want sk-...", p.KeyPlaceholder)
			}
			if p.BaseURL != "https://api.minimax.io/v1" {
				t.Fatalf("minimax base_url = %q, want international endpoint", p.BaseURL)
			}
			if p.AltBaseURL != "https://api.minimaxi.com/v1" {
				t.Fatalf("minimax alt_base_url = %q, want China endpoint", p.AltBaseURL)
			}
			if p.MainModel != "MiniMax-M2.7" {
				t.Fatalf("minimax main_model = %q, want MiniMax-M2.7", p.MainModel)
			}
			if p.HighspeedModel != "MiniMax-M2.7-highspeed" {
				t.Fatalf("minimax highspeed_model = %q, want MiniMax-M2.7-highspeed", p.HighspeedModel)
			}
			if p.Features.Embeddings {
				t.Fatal("minimax profile should have embeddings disabled")
			}
			if p.Features.Vision {
				t.Fatal("minimax profile should not preconfigure vision provider")
			}
			if p.Models.Embeddings != nil {
				t.Fatal("minimax profile should not define an embeddings model")
			}
			if p.Models.Vision != nil {
				t.Fatal("minimax profile should not define a vision provider model")
			}
			if p.Models.Helper == nil || p.Models.Helper.Model != "MiniMax-M2.5" {
				t.Fatalf("minimax helper model = %v, want MiniMax-M2.5", p.Models.Helper)
			}
			if p.Models.ImageGeneration == nil {
				t.Fatal("minimax image_generation should be configured")
			}
			if p.Models.ImageGeneration.ProviderType != "minimax" {
				t.Fatalf("minimax image_generation provider_type = %q, want minimax", p.Models.ImageGeneration.ProviderType)
			}
			if p.Models.ImageGeneration.BaseURL != "https://api.minimax.io/v1/image_generation" {
				t.Fatalf("minimax image_generation base_url = %q, want international image endpoint", p.Models.ImageGeneration.BaseURL)
			}
			if p.Models.ImageGeneration.AltBaseURL != "https://api.minimaxi.com/v1/image_generation" {
				t.Fatalf("minimax image_generation alt_base_url = %q, want China image endpoint", p.Models.ImageGeneration.AltBaseURL)
			}
			if p.Models.ImageGeneration.Model != "image-01" {
				t.Fatalf("minimax image_generation model = %q, want image-01", p.Models.ImageGeneration.Model)
			}
			if p.Models.MusicGeneration == nil {
				t.Fatal("minimax music_generation should be configured")
			}
			if p.Models.MusicGeneration.ProviderType != "minimax" {
				t.Fatalf("minimax music_generation provider_type = %q, want minimax", p.Models.MusicGeneration.ProviderType)
			}
			if p.Models.MusicGeneration.BaseURL != "https://api.minimax.io/v1/music_generation" {
				t.Fatalf("minimax music_generation base_url = %q, want international music endpoint", p.Models.MusicGeneration.BaseURL)
			}
			if p.Models.MusicGeneration.AltBaseURL != "https://api.minimaxi.com/v1/music_generation" {
				t.Fatalf("minimax music_generation alt_base_url = %q, want China music endpoint", p.Models.MusicGeneration.AltBaseURL)
			}
			if p.Models.MusicGeneration.Model != "music-2.6" {
				t.Fatalf("minimax music_generation model = %q, want music-2.6", p.Models.MusicGeneration.Model)
			}
			if p.Models.VideoGeneration == nil {
				t.Fatal("minimax video_generation should be configured")
			}
			if p.Models.VideoGeneration.ProviderType != "minimax" {
				t.Fatalf("minimax video_generation provider_type = %q, want minimax", p.Models.VideoGeneration.ProviderType)
			}
			if p.Models.VideoGeneration.BaseURL != "https://api.minimax.io/v1" {
				t.Fatalf("minimax video_generation base_url = %q, want international video endpoint", p.Models.VideoGeneration.BaseURL)
			}
			if p.Models.VideoGeneration.AltBaseURL != "https://api.minimaxi.com/v1" {
				t.Fatalf("minimax video_generation alt_base_url = %q, want China video endpoint", p.Models.VideoGeneration.AltBaseURL)
			}
			if p.Models.VideoGeneration.Model != "MiniMax-Hailuo-2.3" {
				t.Fatalf("minimax video_generation model = %q, want MiniMax-Hailuo-2.3", p.Models.VideoGeneration.Model)
			}
			if p.TTS.ModelID != "speech-02-hd" {
				t.Fatalf("minimax TTS model_id = %q, want speech-02-hd", p.TTS.ModelID)
			}
			if p.TTS.VoiceID != "English_PlayfulGirl" {
				t.Fatalf("minimax TTS voice_id = %q, want English_PlayfulGirl", p.TTS.VoiceID)
			}
			if got := p.ConfigPatch["llm"].(map[string]any)["structured_outputs"]; got != true {
				t.Fatalf("minimax llm.structured_outputs = %v, want true", got)
			}
			return
		}
	}
	t.Fatal("minimax_coding profile not found")
}

func TestAlibabaProfileHasNoTTS(t *testing.T) {
	t.Parallel()

	profiles := LoadProfiles("", slog.Default())

	for _, p := range profiles {
		if p.ID == "alibaba_coding" {
			if p.Features.TTS {
				t.Fatal("alibaba profile should NOT have TTS (cosyvoice not implemented)")
			}
			return
		}
	}
	t.Fatal("alibaba_coding profile not found")
}

func TestParseProfilesSkipsInvalid(t *testing.T) {
	t.Parallel()

	yaml := []byte(`
profiles:
  - id: ""
    name: "NoID"
    provider_type: openai
    base_url: "https://example.com"
    main_model: "test"
  - id: "valid"
    name: "Valid"
    provider_type: openai
    base_url: "https://example.com"
    main_model: "test"
    sort_order: 1
  - id: "no_provider"
    name: "NoProvider"
    provider_type: ""
    base_url: "https://example.com"
    main_model: "test"
`)

	profiles, err := parseProfiles(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("expected 1 valid profile, got %d", len(profiles))
	}
	if profiles[0].ID != "valid" {
		t.Fatalf("expected valid profile, got %q", profiles[0].ID)
	}
}

func TestLoadProfilesFallsBackOnMissingFile(t *testing.T) {
	t.Parallel()

	profiles := LoadProfiles("/nonexistent/path.yaml", slog.Default())
	if len(profiles) == 0 {
		t.Fatal("expected fallback to embedded profiles when file is missing")
	}
}
