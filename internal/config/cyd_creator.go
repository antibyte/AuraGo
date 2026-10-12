package config

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// DefaultCYDCreatorBuilderImage is the published CYD Creator toolchain image.
const DefaultCYDCreatorBuilderImage = "ghcr.io/antibyte/aurago-cyd-builder:1"

// cydImageMaxLen caps a builder image reference (Docker's name limit).
const cydImageMaxLen = 255

// cydImagePattern accepts [registry-host[:port]/]path[/path...][:tag][@sha256:digest]
// references: lowercase path components separated by a single '.', '_' or '-',
// joined by single slashes, and a Docker tag.
var cydImagePattern = regexp.MustCompile(`^([a-z0-9]+([.-][a-z0-9]+)*(:[0-9]{1,5})?/)?` +
	`[a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*` +
	`(:[A-Za-z0-9_][A-Za-z0-9._-]{0,127})?(@sha256:[0-9a-f]{64})?$`)

func validCYDImage(image string) bool {
	return len(image) <= cydImageMaxLen && cydImagePattern.MatchString(image)
}

// cydPublicHostPattern is the device URL host rule of the CYD Creator config
// blob (internal/cydcreator deviceHostPattern): host names and IPv4 only.
var cydPublicHostPattern = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)

// cydPublicURLProblem mirrors the device URL rule of the CYD Creator config
// blob (internal/cydcreator DeviceConfig.validate and ValidateDeviceURL;
// TestCYDCreatorPublicURLMatchesDeviceRule pins both to the same verdicts;
// config cannot import cydcreator): the firmware joins request URLs as
// strings, checks the literal "https://" prefix and splits host and port by
// hand, so a URL only works on a device when it is printable ASCII without
// credentials, query, fragment or IPv6 literal, the host consists of letters,
// digits, dots and hyphens (no %-escapes) and any port is 1..65535. It
// returns "" for a usable URL.
func cydPublicURLProblem(raw string) string {
	if !strings.HasPrefix(raw, "https://") && !strings.HasPrefix(raw, "http://") {
		return "must start with lowercase http:// or https://"
	}
	for i := 0; i < len(raw); i++ {
		if b := raw[i]; b < 0x21 || b > 0x7E || b == '"' || b == '\\' {
			return "must be printable ASCII without spaces, quotes or backslashes"
		}
	}
	if strings.ContainsAny(raw, "?#") {
		return "must not contain a query or fragment"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Opaque != "" {
		return "must be an absolute URL with a host"
	}
	if u.User != nil {
		return "must not contain credentials"
	}
	if strings.Contains(u.Host, "[") {
		return "must not use an IPv6 literal host"
	}
	if u.Hostname() == "" {
		return "must have a host name"
	}
	// Same host rule as the blob (deviceHostPattern): the firmware hands the
	// raw text to its DNS lookup, so %-escapes and sub-delimiters url.Parse
	// tolerates must not pass.
	if !cydPublicHostPattern.MatchString(u.Hostname()) {
		return "host must be a host name or an IPv4 address (letters, digits, dots and hyphens)"
	}
	if strings.HasSuffix(u.Host, ":") {
		return "port must be 1-65535"
	}
	if port := u.Port(); port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return "port must be 1-65535"
		}
	}
	return ""
}

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

func warnCYDCreatorValue(field string, configured, used any) {
	slog.Warn("[Config] cyd_creator value out of range; using a safe value",
		"field", "cyd_creator."+field, "configured", configured, "used", used)
}

// NormalizeCYDCreatorConfig fills empty values with the defaults and clamps
// out-of-range numbers at load time, so a hand-edited value never stops the
// start; the config save path rejects such values first
// (ValidateCYDCreatorConfig). Replaced or clamped values are logged as
// warnings (an invalid public_url without its value, it may hold credentials).
func NormalizeCYDCreatorConfig(c *CYDCreatorConfig) {
	if c == nil {
		return
	}
	d := DefaultCYDCreatorConfig()
	clamp := func(field string, v *int, lo, hi, def int) {
		got := clampInt(*v, lo, hi, def)
		if *v != 0 && got != *v {
			warnCYDCreatorValue(field, *v, got)
		}
		*v = got
	}
	c.BuilderImage = strings.TrimSpace(c.BuilderImage)
	if c.BuilderImage == "" || !validCYDImage(c.BuilderImage) {
		if c.BuilderImage != "" {
			slog.Warn("[Config] cyd_creator.builder_image is not a valid image reference; using the default",
				"used", d.BuilderImage)
		}
		c.BuilderImage = d.BuilderImage
	}
	clamp("build_timeout_seconds", &c.BuildTimeoutSeconds, 60, 1800, d.BuildTimeoutSeconds)
	clamp("cold_build_timeout_seconds", &c.ColdBuildTimeoutSeconds, 60, 3600, d.ColdBuildTimeoutSeconds)
	if c.ColdBuildTimeoutSeconds < c.BuildTimeoutSeconds {
		warnCYDCreatorValue("cold_build_timeout_seconds", c.ColdBuildTimeoutSeconds, c.BuildTimeoutSeconds)
		c.ColdBuildTimeoutSeconds = c.BuildTimeoutSeconds
	}
	clamp("memory_mb", &c.MemoryMB, 512, 16384, d.MemoryMB)
	switch {
	case c.CPUCores == 0:
		c.CPUCores = d.CPUCores
	case math.IsNaN(c.CPUCores) || math.IsInf(c.CPUCores, 0):
		warnCYDCreatorValue("cpu_cores", c.CPUCores, d.CPUCores)
		c.CPUCores = d.CPUCores
	case c.CPUCores < 0.5:
		warnCYDCreatorValue("cpu_cores", c.CPUCores, 0.5)
		c.CPUCores = 0.5
	case c.CPUCores > 16:
		warnCYDCreatorValue("cpu_cores", c.CPUCores, 16)
		c.CPUCores = 16
	}
	clamp("idle_stop_minutes", &c.IdleStopMinutes, 1, 1440, d.IdleStopMinutes)
	if c.MaxRepairAttempts < 0 {
		warnCYDCreatorValue("max_repair_attempts", c.MaxRepairAttempts, 0)
		c.MaxRepairAttempts = 0
	}
	if c.MaxRepairAttempts > 10 {
		warnCYDCreatorValue("max_repair_attempts", c.MaxRepairAttempts, 10)
		c.MaxRepairAttempts = 10
	}
	clamp("job_timeout_minutes", &c.JobTimeoutMinutes, 5, 120, d.JobTimeoutMinutes)
	c.Model = strings.TrimSpace(c.Model)
	clamp("keep_builds_per_project", &c.KeepBuildsPerProject, 1, 50, d.KeepBuildsPerProject)
	clamp("ota.heartbeat_seconds", &c.OTA.HeartbeatSeconds, 30, 600, d.OTA.HeartbeatSeconds)
	c.OTA.PublicURL = strings.TrimRight(strings.TrimSpace(c.OTA.PublicURL), "/")
	if c.OTA.PublicURL != "" {
		if problem := cydPublicURLProblem(c.OTA.PublicURL); problem != "" {
			slog.Warn("[Config] cyd_creator.ota.public_url is ignored; devices use the URL derived from the server config",
				"reason", "public_url "+problem)
			c.OTA.PublicURL = ""
		}
	}
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
	if image := strings.TrimSpace(c.BuilderImage); image != "" && !validCYDImage(image) {
		problems = append(problems, "cyd_creator.builder_image must be an image reference like ghcr.io/owner/name:tag")
	}
	check("build_timeout_seconds", c.BuildTimeoutSeconds, 60, 1800)
	check("cold_build_timeout_seconds", c.ColdBuildTimeoutSeconds, 60, 3600)
	if c.BuildTimeoutSeconds != 0 && c.ColdBuildTimeoutSeconds != 0 && c.ColdBuildTimeoutSeconds < c.BuildTimeoutSeconds {
		problems = append(problems, "cyd_creator.cold_build_timeout_seconds must be at least build_timeout_seconds")
	}
	check("memory_mb", c.MemoryMB, 512, 16384)
	if math.IsNaN(c.CPUCores) || math.IsInf(c.CPUCores, 0) || (c.CPUCores != 0 && (c.CPUCores < 0.5 || c.CPUCores > 16)) {
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
		if problem := cydPublicURLProblem(raw); problem != "" {
			problems = append(problems, "cyd_creator.ota.public_url "+problem)
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
