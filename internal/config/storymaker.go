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
	Enabled   bool                      `yaml:"enabled" json:"enabled"`
	Render    StorymakerRenderConfig    `yaml:"render" json:"render"`
	Narration StorymakerNarrationConfig `yaml:"narration" json:"narration"`
}

// StorymakerNarrationConfig caps the characters narrate jobs send to cloud TTS providers. Local
// providers are exempt.
type StorymakerNarrationConfig struct {
	// MaxDailyChars is the limit per local day.
	MaxDailyChars int `yaml:"max_daily_chars" json:"max_daily_chars"`
	// MaxJobChars is the limit per narrate job.
	MaxJobChars int `yaml:"max_job_chars" json:"max_job_chars"`
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
	storymakerDefaultMaxDailyChars      = 100000
	storymakerDefaultMaxJobChars        = 20000
)

// NormalizeStorymakerConfig applies defaults for unset (zero) values and rejects values outside
// their range: timeout 60 to 14400 seconds, maximum duration 10 to 3600 seconds, a browser path
// that is absolute, a known frame transport, 1000 to 100000000 daily narration characters and
// 100 to 10000000 narration characters per job. It does not change Enabled.
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
	n := &c.Narration
	if n.MaxDailyChars == 0 {
		n.MaxDailyChars = storymakerDefaultMaxDailyChars
	}
	if n.MaxJobChars == 0 {
		n.MaxJobChars = storymakerDefaultMaxJobChars
	}
	if n.MaxDailyChars < 1000 || n.MaxDailyChars > 100000000 {
		return fmt.Errorf("storymaker: narration.max_daily_chars must be 1000–100000000")
	}
	if n.MaxJobChars < 100 || n.MaxJobChars > 10000000 {
		return fmt.Errorf("storymaker: narration.max_job_chars must be 100–10000000")
	}
	return nil
}
