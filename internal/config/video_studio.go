package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// VideoStudioConfig bounds the optional desktop editor's local processing.
type VideoStudioConfig struct {
	Enabled              bool   `yaml:"enabled" json:"enabled"`
	ReadOnly             bool   `yaml:"readonly" json:"readonly"`
	FFmpegPath           string `yaml:"ffmpeg_path" json:"ffmpeg_path"`
	MaxAssetSizeMB       int    `yaml:"max_asset_size_mb" json:"max_asset_size_mb"`
	MaxProjectSizeMB     int    `yaml:"max_project_size_mb" json:"max_project_size_mb"`
	MaxExportSizeMB      int    `yaml:"max_export_size_mb" json:"max_export_size_mb"`
	RenderTimeoutSeconds int    `yaml:"render_timeout_seconds" json:"render_timeout_seconds"`
	HardwareEncoding     string `yaml:"hardware_encoding" json:"hardware_encoding"`
	HardwareDevice       string `yaml:"hardware_device" json:"hardware_device"`
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
	c.HardwareEncoding = strings.ToLower(strings.TrimSpace(c.HardwareEncoding))
	if c.HardwareEncoding == "" {
		c.HardwareEncoding = "auto"
	}
	switch c.HardwareEncoding {
	case "auto", "off", "vaapi", "qsv", "nvenc", "amf":
	default:
		return fmt.Errorf("video_studio: hardware_encoding must be auto, off, vaapi, qsv, nvenc or amf")
	}
	c.HardwareDevice = strings.TrimSpace(c.HardwareDevice)
	// Linux device paths ("/dev/dri/renderD128") stay valid when the configuration is checked on Windows.
	// FFmpeg parses ",key=value", "name:device" and "@parent" syntax inside device strings, so these
	// characters would smuggle device options into the single argv element.
	if c.HardwareDevice != "" && (!filepath.IsAbs(c.HardwareDevice) && !strings.HasPrefix(c.HardwareDevice, "/") || len(c.HardwareDevice) > 255 || strings.ContainsAny(c.HardwareDevice, "\r\n\x00,:@=")) {
		return fmt.Errorf("video_studio: hardware_device must be an absolute device path without , : @ or =")
	}
	if c.MaxAssetSizeMB < 1 || c.MaxAssetSizeMB > 32768 || c.MaxProjectSizeMB < 1 || c.MaxProjectSizeMB > 65536 || c.RenderTimeoutSeconds < 30 || c.RenderTimeoutSeconds > 14400 || c.MaxExportSizeMB < 1 {
		return fmt.Errorf("video_studio: asset size must be 1–32768 MB, project size 1–65536 MB, timeout 30–14400 seconds and export size at least 1 MB")
	}
	// The update merge keeps an older config's explicit project limit and adds the template's asset
	// and export defaults, which can exceed it. Neither an asset nor an export can outgrow the
	// project limit anyway, so both are clamped instead of failing the whole configuration.
	c.MaxAssetSizeMB = min(c.MaxAssetSizeMB, c.MaxProjectSizeMB)
	c.MaxExportSizeMB = min(c.MaxExportSizeMB, c.MaxProjectSizeMB)
	return nil
}
