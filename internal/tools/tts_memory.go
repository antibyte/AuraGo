package tools

import (
	"context"
	"fmt"
	"strings"
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
	if len([]rune(text)) > 500 {
		text = string([]rune(text)[:500])
	}
	var (
		data []byte
		err  error
	)
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "elevenlabs":
		data, err = ttsElevenLabs(cfg, text)
	case "minimax":
		data, err = ttsMiniMax(cfg, text)
	case "piper":
		data, err = ttsPiper(cfg, text)
	case "supertonic":
		data, err = ttsSupertonic(cfg, text)
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
