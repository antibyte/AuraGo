package tools

import (
	"context"
	"fmt"
	"strings"

	"aurago/internal/phonon"
)

// TTSSynthesizeInMemory uses the configured TTS provider without creating a
// cache file. Telephone audio uses this path so synthesized call audio remains
// transient and never becomes a general chat-media artifact.
func TTSSynthesizeInMemory(cfg TTSConfig, text string) ([]byte, string, error) {
	return TTSSynthesizeInMemoryContext(context.Background(), cfg, text)
}

// TTSSynthesizeInMemoryContext is the cancellable variant used by live voice
// sessions so barge-in terminates in-flight provider work.
func TTSSynthesizeInMemoryContext(ctx context.Context, cfg TTSConfig, text string) ([]byte, string, error) {
	if text == "" {
		return nil, "", fmt.Errorf("text is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cfg.Context = ctx
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Provider), "phonon") {
		if err := phonon.ValidateText(text); err != nil {
			return nil, "", err
		}
	}
	cfg = resolvePhononTTSConfig(cfg)
	var (
		data []byte
		err  error
	)
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "phonon":
		data, err = phonon.Synthesize(ctx, cfg.Phonon.Root, cfg.DataDir, cfg.Language, cfg.Phonon.VoiceID, text)
		if phononFallbackAllowed(ctx, err) {
			cfg.Provider = "sanotts"
			return TTSSynthesizeInMemoryContext(ctx, cfg, text)
		}
	case "sanotts":
		data, err = ttsSano(cfg, text)
	case "elevenlabs":
		data, err = ttsElevenLabs(cfg, text)
	case "minimax":
		data, err = ttsMiniMax(cfg, text)
	case "mistral":
		data, err = ttsMistral(cfg, text)
	case "xai", "openai", "stepfun", "resemble":
		data, err = ttsCloud(cfg, text)
	case "piper":
		data, err = ttsPiper(cfg, text)
	case "supertonic":
		data, err = ttsSupertonic(cfg, text)
	case "s2s", "speech_lab", "s2s-lab":
		data, _, err = ttsSpeechLab(cfg, text)
	default:
		data, err = ttsGoogleContext(ctx, text, cfg.Language)
	}
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("TTS provider returned empty audio")
	}
	return data, ttsAudioExtension(cfg), nil
}
