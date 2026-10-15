package config

import "fmt"

// VideoStudioConfig bounds the optional desktop editor's local processing.
type VideoStudioConfig struct {
	Enabled              bool   `yaml:"enabled" json:"enabled"`
	ReadOnly             bool   `yaml:"readonly" json:"readonly"`
	FFmpegPath           string `yaml:"ffmpeg_path" json:"ffmpeg_path"`
	MaxAssetSizeMB       int    `yaml:"max_asset_size_mb" json:"max_asset_size_mb"`
	MaxProjectSizeMB     int    `yaml:"max_project_size_mb" json:"max_project_size_mb"`
	MaxExportSizeMB      int    `yaml:"max_export_size_mb" json:"max_export_size_mb"`
	RenderTimeoutSeconds int    `yaml:"render_timeout_seconds" json:"render_timeout_seconds"`
}

// NormalizeVideoStudioConfig applies defaults without enabling the editor.
func NormalizeVideoStudioConfig(c *VideoStudioConfig) error {
	if c.MaxProjectSizeMB == 0 {
		c.MaxProjectSizeMB = 16384
	}
	if c.MaxAssetSizeMB == 0 {
		c.MaxAssetSizeMB = min(4096, c.MaxProjectSizeMB)
	}
	if c.MaxExportSizeMB == 0 {
		c.MaxExportSizeMB = min(8192, c.MaxProjectSizeMB)
	}
	if c.RenderTimeoutSeconds == 0 {
		c.RenderTimeoutSeconds = 7200
	}
	if c.MaxAssetSizeMB < 1 || c.MaxAssetSizeMB > 32768 || c.MaxProjectSizeMB < c.MaxAssetSizeMB || c.MaxProjectSizeMB > 65536 || c.RenderTimeoutSeconds < 30 || c.RenderTimeoutSeconds > 14400 || c.MaxExportSizeMB < 1 || c.MaxExportSizeMB > c.MaxProjectSizeMB {
		return fmt.Errorf("video_studio: asset size must be 1–32768 MB, project size at least the asset limit and at most 65536 MB, timeout 30–14400 seconds, export size 1 MB up to the project limit")
	}
	return nil
}
