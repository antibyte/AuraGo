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
	if cfg.VideoStudio.HardwareEncoding != "auto" || cfg.VideoStudio.HardwareDevice != "" {
		t.Fatalf("hardware defaults: %+v", cfg.VideoStudio)
	}
	upper := VideoStudioConfig{HardwareEncoding: " VAAPI ", HardwareDevice: "/dev/dri/renderD129"}
	if err := NormalizeVideoStudioConfig(&upper); err != nil || upper.HardwareEncoding != "vaapi" {
		t.Fatalf("hardware mode normalization: %+v (%v)", upper, err)
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
	for _, invalid := range []VideoStudioConfig{
		{MaxAssetSizeMB: -1}, {MaxAssetSizeMB: 32769, MaxProjectSizeMB: 65536}, {MaxProjectSizeMB: -1}, {MaxProjectSizeMB: 65537},
		{RenderTimeoutSeconds: -1}, {MaxExportSizeMB: -1}, {HardwareEncoding: "cuda"}, {HardwareDevice: "relative/node"},
		// FFmpeg reads option syntax inside device strings.
		{HardwareDevice: "/dev/dri/renderD128,driver=iHD"}, {HardwareDevice: "/dev/dri/renderD128:x"},
		{HardwareDevice: "/dev/dri/renderD128@va"}, {HardwareDevice: "/dev/dri/card=0"},
	} {
		if NormalizeVideoStudioConfig(&invalid) == nil {
			t.Errorf("invalid limits accepted: %+v", invalid)
		}
	}
	// An older config keeps its explicit project limit and gains the larger template asset and export
	// defaults through the update merge; both are clamped instead of failing the whole config.
	merged := VideoStudioConfig{MaxProjectSizeMB: 4096, MaxAssetSizeMB: 1024, MaxExportSizeMB: 8192}
	if err := NormalizeVideoStudioConfig(&merged); err != nil || merged.MaxExportSizeMB != 4096 || merged.MaxAssetSizeMB != 1024 {
		t.Fatalf("an export limit above the project limit must clamp to it: %+v (%v)", merged, err)
	}
	handEdited := VideoStudioConfig{MaxProjectSizeMB: 2048, MaxAssetSizeMB: 4096, MaxExportSizeMB: 8192}
	if err := NormalizeVideoStudioConfig(&handEdited); err != nil || handEdited.MaxAssetSizeMB != 2048 || handEdited.MaxExportSizeMB != 2048 {
		t.Fatalf("asset and export limits above the project limit must clamp to it: %+v (%v)", handEdited, err)
	}
	small := VideoStudioConfig{MaxProjectSizeMB: 2048}
	if err := NormalizeVideoStudioConfig(&small); err != nil || small.MaxAssetSizeMB != 2048 {
		t.Fatalf("an explicit small project limit must cap the asset default: %+v (%v)", small, err)
	}
	// The export limit starts at 512 MB: a smaller value from an older config is raised, then clamped to a project
	// limit below 512 MB. An unset export limit still takes its default.
	for _, tc := range []struct {
		name            string
		export, project int
		want            int
	}{
		{"raised to the minimum", 100, 16384, 512},
		{"raised, then clamped to the project", 300, 256, 256},
		{"a tiny project keeps its tiny export", 1, 1, 1},
		{"unset takes the default", 0, 16384, 8192},
		{"unset under a small project", 0, 256, 256},
	} {
		limits := cfg.VideoStudio
		limits.MaxExportSizeMB, limits.MaxProjectSizeMB, limits.MaxAssetSizeMB = tc.export, tc.project, 0
		if err := NormalizeVideoStudioConfig(&limits); err != nil || limits.MaxExportSizeMB != tc.want {
			t.Errorf("%s: export %d MB with a %d MB project = %d MB (%v), want %d MB", tc.name, tc.export, tc.project, limits.MaxExportSizeMB, err, tc.want)
		}
	}
}
