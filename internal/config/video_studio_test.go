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
	if cfg.VideoStudio.Enabled || cfg.VideoStudio.MaxAssetSizeMB != 4096 || cfg.VideoStudio.MaxProjectSizeMB != 16384 || cfg.VideoStudio.RenderTimeoutSeconds != 7200 {
		t.Fatalf("defaults: %+v", cfg.VideoStudio)
	}
	if cfg.VideoStudio.MaxExportSizeMB != 8192 {
		t.Fatalf("export default: %+v", cfg.VideoStudio)
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
	for _, invalid := range []VideoStudioConfig{{MaxAssetSizeMB: -1}, {MaxAssetSizeMB: 32769, MaxProjectSizeMB: 65536}, {MaxProjectSizeMB: 512, MaxAssetSizeMB: 1024}, {RenderTimeoutSeconds: -1}, {MaxProjectSizeMB: 4096, MaxAssetSizeMB: 1024, MaxExportSizeMB: 8192}} {
		if NormalizeVideoStudioConfig(&invalid) == nil {
			t.Errorf("invalid limits accepted: %+v", invalid)
		}
	}
	small := VideoStudioConfig{MaxProjectSizeMB: 2048}
	if err := NormalizeVideoStudioConfig(&small); err != nil || small.MaxAssetSizeMB != 2048 {
		t.Fatalf("an explicit small project limit must cap the asset default: %+v (%v)", small, err)
	}
}
