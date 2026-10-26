package tools

import (
	"context"
	"errors"
	"strings"

	"aurago/internal/phonon"
)

func phononFallbackAllowed(ctx context.Context, err error) bool {
	return err != nil && ctx.Err() == nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

// Resolve before caching so local fallbacks never occupy a Phonon voice's cache entry.
func resolvePhononTTSConfig(cfg TTSConfig) TTSConfig {
	if !strings.EqualFold(strings.TrimSpace(cfg.Provider), "phonon") {
		return cfg
	}
	if cfg.Phonon.Root == "" {
		cfg.Phonon.Root = phonon.Root()
	}
	if cfg.Phonon.VoiceID == "" {
		cfg.Phonon.VoiceID = phonon.DefaultVoice(cfg.Language, cfg.Phonon.Persona)
	}
	if cfg.NoProviderFallback {
		return cfg
	}
	if phonon.Language(cfg.Language) == "" || phonon.Ready(cfg.Phonon.Root) != nil {
		cfg.Provider = "sanotts"
	} else if _, err := phonon.VoicePath(cfg.Phonon.Root, cfg.DataDir, cfg.Phonon.VoiceID); err != nil {
		cfg.Provider = "sanotts"
	}
	return cfg
}
