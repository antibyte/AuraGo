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
	Agent     StorymakerAgentConfig     `yaml:"agent" json:"agent"`
	Render    StorymakerRenderConfig    `yaml:"render" json:"render"`
	Narration StorymakerNarrationConfig `yaml:"narration" json:"narration"`
}

// StorymakerAgentConfig configures the agent that writes a story from a topic. Enabled defaults
// to true in the loader.
type StorymakerAgentConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Provider is a provider id for the job; empty means the main route.
	Provider string `yaml:"provider" json:"provider"`
	// Model overrides the provider's model; empty means the provider's model.
	Model string `yaml:"model" json:"model"`
	// JobTimeoutSeconds bounds one agent job.
	JobTimeoutSeconds int `yaml:"job_timeout_seconds" json:"job_timeout_seconds"`
	// MaxLengthSeconds is the longest film the agent is asked to write.
	MaxLengthSeconds int `yaml:"max_length_seconds" json:"max_length_seconds"`
	// MaxDailyJobs limits agent starts per local day.
	MaxDailyJobs int `yaml:"max_daily_jobs" json:"max_daily_jobs"`
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

const (
	storymakerDefaultAgentJobTimeoutSeconds = 1800
	storymakerDefaultAgentMaxLengthSeconds  = 180
	storymakerDefaultAgentMaxDailyJobs      = 10
)

// NormalizeStorymakerConfig applies defaults for unset (zero) values and rejects values outside
// their range: timeout 60 to 14400 seconds, maximum duration 10 to 3600 seconds, a browser path
// that is absolute, a known frame transport, 1000 to 100000000 daily narration characters and
// 100 to 10000000 narration characters per job, and the agent limits of normalizeStorymakerAgent.
// It does not change Enabled.
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
	return normalizeStorymakerAgent(&c.Agent)
}

// normalizeStorymakerAgent applies the agent defaults for unset (zero) values and rejects values
// outside their range: job timeout 120 to 7200 seconds, maximum length 15 to 3600 seconds, 1 to
// 1000 jobs per day. It does not change Enabled.
func normalizeStorymakerAgent(a *StorymakerAgentConfig) error {
	a.Provider = strings.TrimSpace(a.Provider)
	a.Model = strings.TrimSpace(a.Model)
	if a.JobTimeoutSeconds == 0 {
		a.JobTimeoutSeconds = storymakerDefaultAgentJobTimeoutSeconds
	}
	if a.MaxLengthSeconds == 0 {
		a.MaxLengthSeconds = storymakerDefaultAgentMaxLengthSeconds
	}
	if a.MaxDailyJobs == 0 {
		a.MaxDailyJobs = storymakerDefaultAgentMaxDailyJobs
	}
	if a.JobTimeoutSeconds < 120 || a.JobTimeoutSeconds > 7200 {
		return fmt.Errorf("storymaker: agent.job_timeout_seconds must be 120–7200")
	}
	if a.MaxLengthSeconds < 15 || a.MaxLengthSeconds > 3600 {
		return fmt.Errorf("storymaker: agent.max_length_seconds must be 15–3600")
	}
	if a.MaxDailyJobs < 1 || a.MaxDailyJobs > 1000 {
		return fmt.Errorf("storymaker: agent.max_daily_jobs must be 1–1000")
	}
	return nil
}
