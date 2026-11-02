package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// StorymakerConfig configures the Storymaker desktop app (explainer videos from a story
// document). Enabled defaults to true in the loader; the app shows capability states for
// export when FFmpeg or a browser is missing.
type StorymakerConfig struct {
	Enabled bool                   `yaml:"enabled" json:"enabled"`
	Render  StorymakerRenderConfig `yaml:"render" json:"render"`
}

// StorymakerRenderConfig bounds the server-side render that turns a story into an MP4.
type StorymakerRenderConfig struct {
	// TimeoutSeconds is the minimum render timeout; the render job raises it for long videos.
	TimeoutSeconds int `yaml:"timeout_seconds" json:"timeout_seconds"`
	// MaxDurationSeconds is the longest video a render accepts.
	MaxDurationSeconds int `yaml:"max_duration_seconds" json:"max_duration_seconds"`
	// BrowserPath is an optional absolute path of the Chrome/Chromium used for rendering.
	BrowserPath string `yaml:"browser_path" json:"browser_path"`
	// Transport selects how frames travel from the render page: "" (automatic), png, webp, raw
	// or screenshot.
	Transport string `yaml:"transport" json:"transport"`
}

const (
	storymakerDefaultTimeoutSeconds     = 3600
	storymakerDefaultMaxDurationSeconds = 900
)

// NormalizeStorymakerConfig applies defaults for unset (zero) values and rejects values outside
// their range: timeout 60 to 14400 seconds, maximum duration 10 to 3600 seconds, a browser path
// that is absolute, and a known frame transport. It does not change Enabled.
func NormalizeStorymakerConfig(c *StorymakerConfig) error {
	r := &c.Render
	if r.TimeoutSeconds == 0 {
		r.TimeoutSeconds = storymakerDefaultTimeoutSeconds
	}
	if r.MaxDurationSeconds == 0 {
		r.MaxDurationSeconds = storymakerDefaultMaxDurationSeconds
	}
	if r.TimeoutSeconds < 60 || r.TimeoutSeconds > 14400 {
		return fmt.Errorf("storymaker: render.timeout_seconds must be 60–14400")
	}
	if r.MaxDurationSeconds < 10 || r.MaxDurationSeconds > 3600 {
		return fmt.Errorf("storymaker: render.max_duration_seconds must be 10–3600")
	}
	r.BrowserPath = strings.TrimSpace(r.BrowserPath)
	if r.BrowserPath != "" {
		// A Linux path ("/usr/bin/chromium") stays valid when the configuration is checked on Windows.
		abs := filepath.IsAbs(r.BrowserPath) || strings.HasPrefix(r.BrowserPath, "/")
		if !abs || len(r.BrowserPath) > 1024 || strings.ContainsAny(r.BrowserPath, "\r\n\x00") {
			return fmt.Errorf("storymaker: render.browser_path must be an absolute path to a Chrome or Chromium executable")
		}
	}
	r.Transport = strings.ToLower(strings.TrimSpace(r.Transport))
	switch r.Transport {
	case "", "png", "webp", "raw", "screenshot":
	default:
		return fmt.Errorf("storymaker: render.transport must be empty, png, webp, raw or screenshot")
	}
	return nil
}
