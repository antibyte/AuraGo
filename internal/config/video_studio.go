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

	// Plan configures plan mode: the agent that plans a clip as a storyboard.
	Plan VideoStudioPlanConfig `yaml:"plan" json:"plan"`
}

// VideoStudioPlanConfig configures plan mode: an agent plans a clip from an idea as a storyboard that the user reviews
// and edits (docs/superpowers/specs/2026-11-23-video-studio-plan-mode-design.md). Enabled defaults to true in the
// loader.
type VideoStudioPlanConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	// AgentProvider is a provider id for planning jobs; empty means the main route.
	AgentProvider string `yaml:"agent_provider" json:"agent_provider"`
	// AgentModel overrides the provider's model; empty means the provider's model.
	AgentModel string `yaml:"agent_model" json:"agent_model"`
	// MaxDailyJobs limits planning job starts (new plans and scene revisions) per local day.
	MaxDailyJobs int `yaml:"max_daily_jobs" json:"max_daily_jobs"`
	// JobTimeoutSeconds bounds one planning job.
	JobTimeoutSeconds int `yaml:"job_timeout_seconds" json:"job_timeout_seconds"`
}

// NormalizeVideoStudioConfig applies defaults without enabling the editor. The plan settings go through
// normalizeVideoStudioPlan, which never fails.
func NormalizeVideoStudioConfig(c *VideoStudioConfig) error {
	normalizeVideoStudioPlan(&c.Plan)
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
	// The export limit starts at 512 MB; smaller values from older configs are raised, not rejected. A project
	// limit below 512 MB still caps the export limit through the clamp below.
	if c.MaxExportSizeMB > 0 && c.MaxExportSizeMB < 512 {
		c.MaxExportSizeMB = 512
	}
	if c.MaxAssetSizeMB < 1 || c.MaxAssetSizeMB > 32768 || c.MaxProjectSizeMB < 1 || c.MaxProjectSizeMB > 65536 || c.RenderTimeoutSeconds < 30 || c.RenderTimeoutSeconds > 14400 || c.MaxExportSizeMB < min(512, c.MaxProjectSizeMB) {
		return fmt.Errorf("video_studio: asset size must be 1–32768 MB, project size 1–65536 MB, timeout 30–14400 seconds and export size at least min(512, project limit) MB")
	}
	// The update merge keeps an older config's explicit project limit and adds the template's asset
	// and export defaults, which can exceed it. Neither an asset nor an export can outgrow the
	// project limit anyway, so both are clamped instead of failing the whole configuration.
	c.MaxAssetSizeMB = min(c.MaxAssetSizeMB, c.MaxProjectSizeMB)
	c.MaxExportSizeMB = min(c.MaxExportSizeMB, c.MaxProjectSizeMB)
	return nil
}

const (
	videoStudioPlanDefaultMaxDailyJobs      = 20
	videoStudioPlanDefaultJobTimeoutSeconds = 900
)

// normalizeVideoStudioPlan trims the agent route and applies the defaults for unset (zero) values. Limits outside
// their range are clamped, not rejected: 1 to 1000 jobs per day, 120 to 3600 seconds per job. It does not change
// Enabled.
func normalizeVideoStudioPlan(p *VideoStudioPlanConfig) {
	p.AgentProvider = strings.TrimSpace(p.AgentProvider)
	p.AgentModel = strings.TrimSpace(p.AgentModel)
	if p.MaxDailyJobs == 0 {
		p.MaxDailyJobs = videoStudioPlanDefaultMaxDailyJobs
	}
	p.MaxDailyJobs = min(max(p.MaxDailyJobs, 1), 1000)
	if p.JobTimeoutSeconds == 0 {
		p.JobTimeoutSeconds = videoStudioPlanDefaultJobTimeoutSeconds
	}
	p.JobTimeoutSeconds = min(max(p.JobTimeoutSeconds, 120), 3600)
}
