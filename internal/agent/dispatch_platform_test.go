package agent

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
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

func TestIsTTSConfiguredRequiresUsableBackend(t *testing.T) {
	if isTTSConfigured(&config.Config{}) {
		t.Fatal("empty TTS config should not be considered configured")
	}

	googleCfg := &config.Config{}
	googleCfg.TTS.Provider = "google"
	if !isTTSConfigured(googleCfg) {
		t.Fatal("google provider should be considered configured")
	}

	missingKeyCfg := &config.Config{}
	missingKeyCfg.TTS.Provider = "minimax"
	if isTTSConfigured(missingKeyCfg) {
		t.Fatal("minimax without API key should not be considered configured")
	}

	minimaxCfg := &config.Config{}
	minimaxCfg.TTS.Provider = "minimax"
	minimaxCfg.TTS.MiniMax.APIKey = "mm-key"
	if !isTTSConfigured(minimaxCfg) {
		t.Fatal("minimax with API key should be considered configured")
	}

	piperCfg := &config.Config{}
	piperCfg.TTS.Piper.Enabled = true
	if !isTTSConfigured(piperCfg) {
		t.Fatal("enabled Piper should be considered configured")
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

func TestPrepareChromecastLocalMediaURLCopiesWorkspaceFileToTTSDir(t *testing.T) {
	root := t.TempDir()
	workdir := filepath.Join(root, "agent_workspace", "workdir")
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("MkdirAll workdir: %v", err)
	}
	src := filepath.Join(workdir, "ueberall_zuhause.mp3")
	if err := os.WriteFile(src, []byte("fake mp3"), 0o644); err != nil {
		t.Fatalf("WriteFile src: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workdir
	cfg.Directories.DataDir = dataDir
	cfg.Server.Host = "192.168.6.238"
	cfg.Chromecast.TTSPort = 8090

	req := chromecastArgs{LocalPath: "workdir/ueberall_zuhause.mp3"}
	if err := prepareChromecastLocalMediaURL(cfg, &req); err != nil {
		t.Fatalf("prepareChromecastLocalMediaURL: %v", err)
	}
	if req.URL != "http://192.168.6.238:8090/tts/ueberall_zuhause.mp3" {
		t.Fatalf("URL = %q", req.URL)
	}
	if req.ContentType != "audio/mpeg" {
		t.Fatalf("ContentType = %q, want audio/mpeg", req.ContentType)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "tts", "ueberall_zuhause.mp3")); err != nil {
		t.Fatalf("expected published file in tts dir: %v", err)
	}
}
