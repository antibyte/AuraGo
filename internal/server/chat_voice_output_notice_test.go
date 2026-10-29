package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestTTSNoticePhononFallbackThenAudio(t *testing.T) {
	original := chatVoiceOutputSynthesize
	t.Cleanup(func() { chatVoiceOutputSynthesize = original })
	chatVoiceOutputSynthesize = func(cfg tools.TTSConfig, _ string) (string, error) {
		if cfg.Provider != "phonon" {
			t.Fatalf("provider = %q", cfg.Provider)
		}
		return "ja.wav", nil
	}

	base := &chatVoiceOutputCaptureBroker{}
	broker := newChatVoiceOutputTrackingBroker(base)
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.TTS.Provider = "phonon"
	cfg.TTS.Language = "ja"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	maybeEmitChatVoiceOutputFallback(context.Background(), cfg, logger, agent.RunConfig{VoiceOutputActive: true, MessageSource: "web_chat"}, broker, "Hallo")

	if len(base.events) != 2 || base.events[0].event != "tts_notice" || base.events[1].event != "audio" {
		t.Fatalf("events = %+v", base.events)
	}
	var notice struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(base.events[0].message), &notice); err != nil {
		t.Fatal(err)
	}
	if notice.Code != "phonon_fallback_sanotts" {
		t.Fatalf("code = %q", notice.Code)
	}
}

func TestTTSNoticeFailedLocalSynthesis(t *testing.T) {
	original := chatVoiceOutputSynthesize
	t.Cleanup(func() { chatVoiceOutputSynthesize = original })
	chatVoiceOutputSynthesize = func(tools.TTSConfig, string) (string, error) {
		return "", errors.New("synthesis failed")
	}
	base := &chatVoiceOutputCaptureBroker{}
	broker := newChatVoiceOutputTrackingBroker(base)
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.TTS.Provider = "supertonic"
	cfg.TTS.Supertonic.URL = "http://127.0.0.1:7788"
	maybeEmitChatVoiceOutputFallback(context.Background(), cfg, nil, agent.RunConfig{VoiceOutputActive: true, MessageSource: "web_chat"}, broker, "Hallo")

	if len(base.events) != 1 || base.events[0].event != "tts_notice" {
		t.Fatalf("events = %+v", base.events)
	}
	var notice struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(base.events[0].message), &notice); err != nil {
		t.Fatal(err)
	}
	if notice.Code != "tts_failed" {
		t.Fatalf("code = %q", notice.Code)
	}
}

func TestTTSNoticeDeadlineReportsWhenTheRequestContinues(t *testing.T) {
	original := chatVoiceOutputSynthesize
	t.Cleanup(func() { chatVoiceOutputSynthesize = original })
	chatVoiceOutputSynthesize = func(tools.TTSConfig, string) (string, error) {
		return "", context.DeadlineExceeded
	}
	base := &chatVoiceOutputCaptureBroker{}
	broker := newChatVoiceOutputTrackingBroker(base)
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.TTS.Provider = "supertonic"
	cfg.TTS.Supertonic.URL = "http://127.0.0.1:7788"
	maybeEmitChatVoiceOutputFallback(context.Background(), cfg, nil, agent.RunConfig{VoiceOutputActive: true, MessageSource: "web_chat"}, broker, "Hallo")
	if len(base.events) != 1 || base.events[0].event != "tts_notice" {
		t.Fatalf("events = %+v", base.events)
	}
	var notice struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(base.events[0].message), &notice); err != nil {
		t.Fatal(err)
	}
	if notice.Code != "tts_failed" {
		t.Fatalf("code = %q", notice.Code)
	}
}

func TestTTSNoticeCancellationIsSilent(t *testing.T) {
	original := chatVoiceOutputSynthesize
	t.Cleanup(func() { chatVoiceOutputSynthesize = original })
	chatVoiceOutputSynthesize = func(tools.TTSConfig, string) (string, error) {
		return "", context.Canceled
	}
	base := &chatVoiceOutputCaptureBroker{}
	broker := newChatVoiceOutputTrackingBroker(base)
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.TTS.Provider = "supertonic"
	cfg.TTS.Supertonic.URL = "http://127.0.0.1:7788"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	maybeEmitChatVoiceOutputFallback(ctx, cfg, nil, agent.RunConfig{VoiceOutputActive: true, MessageSource: "web_chat"}, broker, "Hallo")
	if len(base.events) != 0 {
		t.Fatalf("events = %+v", base.events)
	}
}
