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
	requested := cfg.Provider
	cfg = resolvePhononTTSConfig(cfg)
	notePhononFallback(cfg.Result, requested, cfg.Provider)
	if err := ensureSpeechLabSnapshot(&cfg); err != nil {
		return nil, "", err
	}
	data, err := synthesizeProvider(cfg, text)
	if err != nil && phononRuntimeFallback(cfg, err) {
		notePhononFallback(cfg.Result, "phonon", "sanotts")
		cfg.Provider = "sanotts"
		return TTSSynthesizeInMemoryContext(ctx, cfg, text)
	}
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("TTS provider returned empty audio")
	}
	return data, ttsAudioExtension(cfg), nil
}
