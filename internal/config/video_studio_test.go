package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVideoStudioConfigRoundTripAndLimits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  ui_language: en\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.VideoStudio.Enabled || cfg.VideoStudio.MaxAssetSizeMB != 1024 || cfg.VideoStudio.MaxProjectSizeMB != 4096 || cfg.VideoStudio.RenderTimeoutSeconds != 3600 {
		t.Fatalf("defaults: %+v", cfg.VideoStudio)
	}
	cfg.VideoStudio.Enabled = true
	cfg.VideoStudio.ReadOnly = true
	cfg.VideoStudio.FFmpegPath = "C:/Media Tools/ffmpeg.exe"
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.VideoStudio != cfg.VideoStudio {
		t.Fatalf("round trip: %+v", loaded.VideoStudio)
	}
	for _, invalid := range []VideoStudioConfig{{MaxAssetSizeMB: -1}, {MaxAssetSizeMB: 8193}, {MaxProjectSizeMB: 512}, {RenderTimeoutSeconds: -1}} {
		if NormalizeVideoStudioConfig(&invalid) == nil {
			t.Errorf("invalid limits accepted: %+v", invalid)
		}
	}
}
