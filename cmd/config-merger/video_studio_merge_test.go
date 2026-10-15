package main

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

// The video_studio section as the template shipped it before Video Studio Pro
// (3be4f82ff). Installs updated from it keep the explicit 4096 MB project
// limit, and the merge adds the new template's 8192 MB export limit on top.
// config.Load must still accept the merged file: a failure here makes the
// Docker entrypoint restore factory defaults and native installs refuse to
// start.
const preProVideoStudioSection = `video_studio:
    enabled: false
    readonly: false
    ffmpeg_path: ""
    max_asset_size_mb: 1024
    max_project_size_mb: 4096
    render_timeout_seconds: 3600
`

func TestRunUpgradesPreProVideoStudioConfigThatStillLoads(t *testing.T) {
	templatePath := filepath.Join("..", "..", "config_template.yaml")
	for _, tc := range []struct {
		name       string
		section    string
		wantExport int
		wantLimits config.VideoStudioConfig
	}{
		{
			name:       "previous template defaults",
			section:    preProVideoStudioSection,
			wantExport: 4096,
			wantLimits: config.VideoStudioConfig{MaxAssetSizeMB: 1024, MaxProjectSizeMB: 4096, RenderTimeoutSeconds: 3600},
		},
		{
			name:       "raised project limit",
			section:    "video_studio:\n    enabled: true\n    max_asset_size_mb: 2048\n    max_project_size_mb: 20000\n",
			wantExport: 8192,
			wantLimits: config.VideoStudioConfig{Enabled: true, MaxAssetSizeMB: 2048, MaxProjectSizeMB: 20000, RenderTimeoutSeconds: 7200},
		},
		{
			// The merge also adds the template's 4096 MB asset limit above it.
			name:       "hand-written project limit below the new defaults",
			section:    "video_studio:\n    max_project_size_mb: 2048\n",
			wantExport: 2048,
			wantLimits: config.VideoStudioConfig{MaxAssetSizeMB: 2048, MaxProjectSizeMB: 2048, RenderTimeoutSeconds: 7200},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sourcePath := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(sourcePath, []byte("server:\n    port: 8088\n"+tc.section), 0o600); err != nil {
				t.Fatal(err)
			}

			run(sourcePath, templatePath, sourcePath)

			outData, err := readNormalized(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			outMap, err := parseYAMLMap(outData)
			if err != nil {
				t.Fatalf("merger output is not valid YAML: %v", err)
			}
			if got := mergedValue(outMap, "video_studio.max_export_size_mb"); got != 8192 {
				t.Fatalf("merged video_studio.max_export_size_mb = %#v, want the template's 8192", got)
			}
			cfg, err := config.Load(sourcePath)
			if err != nil {
				t.Fatalf("Load(merged): %v", err)
			}
			vs := cfg.VideoStudio
			if vs.MaxExportSizeMB != tc.wantExport {
				t.Fatalf("export limit = %d, want %d", vs.MaxExportSizeMB, tc.wantExport)
			}
			got := config.VideoStudioConfig{Enabled: vs.Enabled, MaxAssetSizeMB: vs.MaxAssetSizeMB, MaxProjectSizeMB: vs.MaxProjectSizeMB, RenderTimeoutSeconds: vs.RenderTimeoutSeconds}
			if got != tc.wantLimits {
				t.Fatalf("user limits after the upgrade = %+v, want %+v", got, tc.wantLimits)
			}
			if vs.HardwareEncoding != "auto" || vs.HardwareDevice != "" {
				t.Fatalf("hardware settings after the upgrade = %q/%q, want auto and empty", vs.HardwareEncoding, vs.HardwareDevice)
			}
		})
	}
}
