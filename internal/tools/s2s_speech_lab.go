package tools

import (
	"context"

	"aurago/internal/config"
	"aurago/internal/speechlab"
)

// SpeechLabReady is retained as a compatibility alias for the pre-release WIP.
type SpeechLabReady = speechlab.Ready

// SpeechLabCheckReady delegates to the central Speech Lab client.
func SpeechLabCheckReady(ctx context.Context, cfg config.SpeechLabConfig) (*SpeechLabReady, error) {
	client, err := speechlab.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	ready, err := client.Ready(ctx)
	return &ready, err
}

// SpeechLabTranscribe delegates to the central Speech Lab client.
func SpeechLabTranscribe(ctx context.Context, wav []byte, language string, cfg config.SpeechLabConfig) (string, error) {
	client, err := speechlab.NewClient(cfg)
	if err != nil {
		return "", err
	}
	result, err := client.Transcribe(ctx, wav, language, "")
	return result.Text, err
}

// SpeechLabSynthesize delegates to the central Speech Lab client.
func SpeechLabSynthesize(ctx context.Context, text, language, voice string, cfg config.SpeechLabConfig) ([]byte, string, error) {
	client, err := speechlab.NewClient(cfg)
	if err != nil {
		return nil, "", err
	}
	data, _, _, err := client.Synthesize(ctx, text, language, voice, "", "")
	return data, ".wav", err
}
