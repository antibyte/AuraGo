package config

import "fmt"

// VideoStudioConfig bounds the optional desktop editor's local processing.
type VideoStudioConfig struct {
	Enabled              bool   `yaml:"enabled" json:"enabled"`
	ReadOnly             bool   `yaml:"readonly" json:"readonly"`
	FFmpegPath           string `yaml:"ffmpeg_path" json:"ffmpeg_path"`
	MaxAssetSizeMB       int    `yaml:"max_asset_size_mb" json:"max_asset_size_mb"`
	MaxProjectSizeMB     int    `yaml:"max_project_size_mb" json:"max_project_size_mb"`
	RenderTimeoutSeconds int    `yaml:"render_timeout_seconds" json:"render_timeout_seconds"`
}

// NormalizeVideoStudioConfig applies defaults without enabling the editor.
func NormalizeVideoStudioConfig(c *VideoStudioConfig) error {
	if c.MaxAssetSizeMB == 0 {
		c.MaxAssetSizeMB = 1024
	}
	if c.MaxProjectSizeMB == 0 {
		c.MaxProjectSizeMB = 4096
	}
	if c.RenderTimeoutSeconds == 0 {
		c.RenderTimeoutSeconds = 3600
	}
	if c.MaxAssetSizeMB < 1 || c.MaxAssetSizeMB > 8192 || c.MaxProjectSizeMB < c.MaxAssetSizeMB || c.MaxProjectSizeMB > 65536 || c.RenderTimeoutSeconds < 30 || c.RenderTimeoutSeconds > 14400 {
		return fmt.Errorf("video_studio: asset size must be 1–8192 MB, project size at least the asset limit and at most 65536 MB, and timeout 30–14400 seconds")
	}
	return nil
}
