package config

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// DefaultCYDCreatorBuilderImage is the published CYD Creator toolchain image.
const DefaultCYDCreatorBuilderImage = "ghcr.io/antibyte/aurago-cyd-builder:1"

// cydImagePattern accepts registry/repository[:tag][@sha256:digest] references.
var cydImagePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*(:[0-9]+)?[a-z0-9._/-]*(:[A-Za-z0-9._-]{1,128})?(@sha256:[0-9a-f]{64})?$`)

// DefaultCYDCreatorConfig returns the spec §15 defaults (disabled).
func DefaultCYDCreatorConfig() CYDCreatorConfig {
	return CYDCreatorConfig{
		BuilderImage:            DefaultCYDCreatorBuilderImage,
		BuildTimeoutSeconds:     300,
		ColdBuildTimeoutSeconds: 600,
		MemoryMB:                2048,
		CPUCores:                2,
		IdleStopMinutes:         20,
		MaxRepairAttempts:       3,
		JobTimeoutMinutes:       20,
		KeepBuildsPerProject:    5,
		OTA:                     CYDCreatorOTAConfig{Enabled: true, HeartbeatSeconds: 60},
	}
}

func clampInt(v, lo, hi, def int) int {
	switch {
	case v == 0:
		return def
	case v < lo:
		return lo
	case v > hi:
		return hi
	}
	return v
}

// NormalizeCYDCreatorConfig fills empty values with the defaults and clamps
// out-of-range numbers at load time, so a hand-edited value never stops the
// start; the config save path rejects such values first
// (ValidateCYDCreatorConfig).
func NormalizeCYDCreatorConfig(c *CYDCreatorConfig) {
	if c == nil {
		return
	}
	d := DefaultCYDCreatorConfig()
	c.BuilderImage = strings.TrimSpace(c.BuilderImage)
	if c.BuilderImage == "" || !cydImagePattern.MatchString(c.BuilderImage) {
		c.BuilderImage = d.BuilderImage
	}
	c.BuildTimeoutSeconds = clampInt(c.BuildTimeoutSeconds, 60, 1800, d.BuildTimeoutSeconds)
	c.ColdBuildTimeoutSeconds = clampInt(c.ColdBuildTimeoutSeconds, 60, 3600, d.ColdBuildTimeoutSeconds)
	if c.ColdBuildTimeoutSeconds < c.BuildTimeoutSeconds {
		c.ColdBuildTimeoutSeconds = c.BuildTimeoutSeconds
	}
	c.MemoryMB = clampInt(c.MemoryMB, 512, 16384, d.MemoryMB)
	switch {
	case c.CPUCores == 0:
		c.CPUCores = d.CPUCores
	case c.CPUCores < 0.5:
		c.CPUCores = 0.5
	case c.CPUCores > 16:
		c.CPUCores = 16
	}
	c.IdleStopMinutes = clampInt(c.IdleStopMinutes, 1, 1440, d.IdleStopMinutes)
	if c.MaxRepairAttempts < 0 {
		c.MaxRepairAttempts = 0
	}
	if c.MaxRepairAttempts > 10 {
		c.MaxRepairAttempts = 10
	}
	c.JobTimeoutMinutes = clampInt(c.JobTimeoutMinutes, 5, 120, d.JobTimeoutMinutes)
	c.Model = strings.TrimSpace(c.Model)
	c.KeepBuildsPerProject = clampInt(c.KeepBuildsPerProject, 1, 50, d.KeepBuildsPerProject)
	c.OTA.HeartbeatSeconds = clampInt(c.OTA.HeartbeatSeconds, 30, 600, d.OTA.HeartbeatSeconds)
	c.OTA.PublicURL = strings.TrimRight(strings.TrimSpace(c.OTA.PublicURL), "/")
}

// ValidateCYDCreatorConfig rejects values a config save must not store.
// Zero means "default" and is accepted.
func ValidateCYDCreatorConfig(c CYDCreatorConfig) error {
	var problems []string
	check := func(name string, v, lo, hi int) {
		if v != 0 && (v < lo || v > hi) {
			problems = append(problems, fmt.Sprintf("cyd_creator.%s must be %d-%d", name, lo, hi))
		}
	}
	if image := strings.TrimSpace(c.BuilderImage); image != "" && !cydImagePattern.MatchString(image) {
		problems = append(problems, "cyd_creator.builder_image must be an image reference like ghcr.io/owner/name:tag")
	}
	check("build_timeout_seconds", c.BuildTimeoutSeconds, 60, 1800)
	check("cold_build_timeout_seconds", c.ColdBuildTimeoutSeconds, 60, 3600)
	if c.BuildTimeoutSeconds != 0 && c.ColdBuildTimeoutSeconds != 0 && c.ColdBuildTimeoutSeconds < c.BuildTimeoutSeconds {
		problems = append(problems, "cyd_creator.cold_build_timeout_seconds must be at least build_timeout_seconds")
	}
	check("memory_mb", c.MemoryMB, 512, 16384)
	if c.CPUCores != 0 && (c.CPUCores < 0.5 || c.CPUCores > 16) {
		problems = append(problems, "cyd_creator.cpu_cores must be 0.5-16")
	}
	check("idle_stop_minutes", c.IdleStopMinutes, 1, 1440)
	if c.MaxRepairAttempts < 0 || c.MaxRepairAttempts > 10 {
		problems = append(problems, "cyd_creator.max_repair_attempts must be 0-10")
	}
	check("job_timeout_minutes", c.JobTimeoutMinutes, 5, 120)
	check("keep_builds_per_project", c.KeepBuildsPerProject, 1, 50)
	check("ota.heartbeat_seconds", c.OTA.HeartbeatSeconds, 30, 600)
	if raw := strings.TrimSpace(c.OTA.PublicURL); raw != "" {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			problems = append(problems, "cyd_creator.ota.public_url must be an http(s) URL without credentials, query or fragment")
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
