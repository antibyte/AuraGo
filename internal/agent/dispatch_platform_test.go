package agent

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestBuildRuntimeTTSConfigIncludesMiniMaxAndPiper(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = "data"
	cfg.TTS.Provider = "minimax"
	cfg.TTS.Language = "de"
	cfg.TTS.CacheRetentionHours = 24
	cfg.TTS.CacheMaxFiles = 10
	cfg.TTS.ElevenLabs.APIKey = "el-key"
	cfg.TTS.ElevenLabs.VoiceID = "el-voice"
	cfg.TTS.ElevenLabs.ModelID = "el-model"
	cfg.TTS.MiniMax.APIKey = "mm-key"
	cfg.TTS.MiniMax.VoiceID = "mm-voice"
	cfg.TTS.MiniMax.ModelID = "mm-model"
	cfg.TTS.MiniMax.Speed = 1.25
	cfg.TTS.Piper.Enabled = true
	cfg.TTS.Piper.ContainerPort = 10200
	cfg.TTS.Piper.Voice = "de_DE-thorsten-high"
	cfg.TTS.Piper.SpeakerID = 3

	ttsCfg := buildRuntimeTTSConfig(cfg, "")
	if ttsCfg.Provider != "minimax" {
		t.Fatalf("Provider = %q, want minimax", ttsCfg.Provider)
	}
	if ttsCfg.Language != "de" {
		t.Fatalf("Language = %q, want de", ttsCfg.Language)
	}
	if ttsCfg.MiniMax.APIKey != "mm-key" || ttsCfg.MiniMax.VoiceID != "mm-voice" || ttsCfg.MiniMax.ModelID != "mm-model" {
		t.Fatalf("MiniMax config not copied: %+v", ttsCfg.MiniMax)
	}
	if ttsCfg.MiniMax.Speed != 1.25 {
		t.Fatalf("MiniMax.Speed = %v, want 1.25", ttsCfg.MiniMax.Speed)
	}
	if ttsCfg.Piper.Port != 10200 || ttsCfg.Piper.Voice != "de_DE-thorsten-high" || ttsCfg.Piper.SpeakerID != 3 {
		t.Fatalf("Piper config not copied: %+v", ttsCfg.Piper)
	}
}

func TestResolveChromecastTargetFallsBackToDiscovery(t *testing.T) {
	orig := discoverChromecastDevices
	discoverChromecastDevices = func(logger *slog.Logger) ([]tools.ChromecastDevice, error) {
		return []tools.ChromecastDevice{
			{
				Name:         "Google-Home-Mini-b39e08d8ca5bd6baa7ed277fd1bb1437",
				FriendlyName: "Arbeitszimmer",
				Addr:         "192.168.6.130",
				Port:         8009,
			},
		}, nil
	}
	defer func() {
		discoverChromecastDevices = orig
	}()

	req := chromecastArgs{DeviceName: "Arbeitszimmer"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := resolveChromecastTarget(&req, nil, logger); err != nil {
		t.Fatalf("resolveChromecastTarget returned error: %v", err)
	}
	if req.DeviceAddr != "192.168.6.130" {
		t.Fatalf("DeviceAddr = %q, want 192.168.6.130", req.DeviceAddr)
	}
	if req.DevicePort != 8009 {
		t.Fatalf("DevicePort = %d, want 8009", req.DevicePort)
	}
}
