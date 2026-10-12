package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Person-detection defaults and limits (contract C5).
const (
	Go2RTCDetectionDefaultModel         = "standard"
	Go2RTCDetectionDefaultBackend       = "auto"
	Go2RTCDetectionDefaultRetentionDays = 14
	Go2RTCDetectionDefaultMaxStorageMB  = 2048
	Go2RTCDetectionDefaultInterval      = 2
	Go2RTCDetectionDefaultMinScore      = 0.5
	Go2RTCDetectionDefaultCooldown      = 300
	Go2RTCDetectionDefaultChannel       = "all"

	Go2RTCDetectionMaxWindows     = 50
	Go2RTCDetectionMaxZones       = 8
	Go2RTCDetectionMaxZoneName    = 48
	Go2RTCDetectionMinZonePoints  = 3
	Go2RTCDetectionMaxZonePoints  = 32
	Go2RTCDetectionMaxInstruction = 500
)

// Go2RTCDetectionChannels are the tools.SendNotification channels a camera may alert on.
var Go2RTCDetectionChannels = []string{"ntfy", "pushover", "telegram", "discord", "push", "telnyx", "cyd", "all"}

var go2RTCDetectionDays = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// Go2RTCDetectionFieldError names the offending field of a detection setting.
type Go2RTCDetectionFieldError struct {
	Field   string
	Message string
}

func (e *Go2RTCDetectionFieldError) Error() string {
	return e.Field + ": " + e.Message
}

func detectionFieldError(field, format string, args ...interface{}) error {
	return &Go2RTCDetectionFieldError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// PrefixGo2RTCDetectionField returns err with prefix + "." prepended to its field path.
// Errors that are not field errors are returned unchanged.
func PrefixGo2RTCDetectionField(err error, prefix string) error {
	var fieldErr *Go2RTCDetectionFieldError
	if !errors.As(err, &fieldErr) {
		return err
	}
	return &Go2RTCDetectionFieldError{Field: prefix + "." + fieldErr.Field, Message: fieldErr.Message}
}

// DefaultGo2RTCStreamDetection is the per-camera default set; enabled is kept.
func DefaultGo2RTCStreamDetection(enabled bool) Go2RTCStreamDetectionConfig {
	return Go2RTCStreamDetectionConfig{
		Enabled:         enabled,
		IntervalSeconds: Go2RTCDetectionDefaultInterval,
		MinScore:        Go2RTCDetectionDefaultMinScore,
		CooldownSeconds: Go2RTCDetectionDefaultCooldown,
		Zones:           []Go2RTCDetectionZone{},
		Actions:         Go2RTCDetectionActions{Channel: Go2RTCDetectionDefaultChannel, Desktop: true},
	}
}

// NormalizeGo2RTCDetectionSettings applies the global defaults (empty model/backend,
// zero retention/storage) and canonical spelling. It never shares slices with d.
func NormalizeGo2RTCDetectionSettings(d Go2RTCDetectionConfig) Go2RTCDetectionConfig {
	out := d
	out.Model = strings.ToLower(strings.TrimSpace(d.Model))
	if out.Model == "" {
		out.Model = Go2RTCDetectionDefaultModel
	}
	out.Backend = strings.ToLower(strings.TrimSpace(d.Backend))
	if out.Backend == "" {
		out.Backend = Go2RTCDetectionDefaultBackend
	}
	if out.RetentionDays == 0 {
		out.RetentionDays = Go2RTCDetectionDefaultRetentionDays
	}
	if out.MaxStorageMB == 0 {
		out.MaxStorageMB = Go2RTCDetectionDefaultMaxStorageMB
	}
	out.Schedule = make([]Go2RTCDetectionWindow, 0, len(d.Schedule))
	for _, window := range d.Schedule {
		days := make([]string, 0, len(window.Days))
		seen := make(map[string]bool, len(window.Days))
		for _, day := range window.Days {
			key := strings.ToLower(strings.TrimSpace(day))
			if seen[key] {
				continue
			}
			seen[key] = true
			days = append(days, key)
		}
		out.Schedule = append(out.Schedule, Go2RTCDetectionWindow{Days: days, From: strings.TrimSpace(window.From), To: strings.TrimSpace(window.To)})
	}
	return out
}

// NormalizeGo2RTCStreamDetection applies the per-camera rule: IntervalSeconds == 0 means
// "never configured" and yields the whole default set (Enabled kept). Otherwise names,
// channel and instruction get canonical spelling. It never shares slices with d.
func NormalizeGo2RTCStreamDetection(d Go2RTCStreamDetectionConfig) Go2RTCStreamDetectionConfig {
	if d.IntervalSeconds == 0 {
		return DefaultGo2RTCStreamDetection(d.Enabled)
	}
	out := d
	out.Actions.Channel = strings.ToLower(strings.TrimSpace(d.Actions.Channel))
	out.Actions.AgentInstruction = strings.TrimSpace(d.Actions.AgentInstruction)
	out.Zones = make([]Go2RTCDetectionZone, 0, len(d.Zones))
	for _, zone := range d.Zones {
		points := append([][2]float64{}, zone.Points...)
		out.Zones = append(out.Zones, Go2RTCDetectionZone{Name: strings.TrimSpace(zone.Name), Points: points})
	}
	return out
}

// NormalizeGo2RTCDetection normalizes the global and every per-stream detection setting.
// cfg.Streams is replaced by a new slice; the caller's previous slice is not modified.
func NormalizeGo2RTCDetection(cfg *Go2RTCConfig) {
	if cfg == nil {
		return
	}
	cfg.Detection = NormalizeGo2RTCDetectionSettings(cfg.Detection)
	if cfg.Streams == nil {
		return
	}
	streams := make([]Go2RTCStreamConfig, len(cfg.Streams))
	for i, stream := range cfg.Streams {
		stream.Detection = NormalizeGo2RTCStreamDetection(stream.Detection)
		streams[i] = stream
	}
	cfg.Streams = streams
}

// ParseGo2RTCDetectionClock parses "HH:MM" (00:00-23:59) into minutes since midnight.
func ParseGo2RTCDetectionClock(value string) (int, bool) {
	if len(value) != 5 || value[2] != ':' {
		return 0, false
	}
	digits := value[:2] + value[3:]
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	hours := int(value[0]-'0')*10 + int(value[1]-'0')
	minutes := int(value[3]-'0')*10 + int(value[4]-'0')
	if hours > 23 || minutes > 59 {
		return 0, false
	}
	return hours*60 + minutes, true
}

// ParseGo2RTCDetectionDay maps mon..sun (any case, trimmed) to a weekday.
func ParseGo2RTCDetectionDay(value string) (time.Weekday, bool) {
	day, ok := go2RTCDetectionDays[strings.ToLower(strings.TrimSpace(value))]
	return day, ok
}

func validGo2RTCDetectionChannel(channel string) bool {
	for _, candidate := range Go2RTCDetectionChannels {
		if channel == candidate {
			return true
		}
	}
	return false
}

// ValidateGo2RTCDetectionSettings checks normalized global settings; field paths are
// relative to go2rtc.detection.
func ValidateGo2RTCDetectionSettings(d Go2RTCDetectionConfig) error {
	switch d.Model {
	case "standard", "accurate":
	default:
		return detectionFieldError("model", "must be standard or accurate")
	}
	switch d.Backend {
	case "auto", "gpu", "cpu":
	default:
		return detectionFieldError("backend", "must be auto, gpu or cpu")
	}
	if d.RetentionDays < 1 || d.RetentionDays > 365 {
		return detectionFieldError("retention_days", "must be between 1 and 365")
	}
	if d.MaxStorageMB < 100 || d.MaxStorageMB > 1_000_000 {
		return detectionFieldError("max_storage_mb", "must be between 100 and 1000000")
	}
	if len(d.Schedule) > Go2RTCDetectionMaxWindows {
		return detectionFieldError("schedule", "allows at most %d windows", Go2RTCDetectionMaxWindows)
	}
	for i, window := range d.Schedule {
		field := fmt.Sprintf("schedule[%d]", i)
		if len(window.Days) == 0 {
			return detectionFieldError(field+".days", "needs at least one day")
		}
		for j, day := range window.Days {
			if _, ok := ParseGo2RTCDetectionDay(day); !ok {
				return detectionFieldError(fmt.Sprintf("%s.days[%d]", field, j), "must be one of mon, tue, wed, thu, fri, sat, sun")
			}
		}
		if _, ok := ParseGo2RTCDetectionClock(window.From); !ok {
			return detectionFieldError(field+".from", "must be a time HH:MM")
		}
		if _, ok := ParseGo2RTCDetectionClock(window.To); !ok {
			return detectionFieldError(field+".to", "must be a time HH:MM")
		}
	}
	return nil
}

// ValidateGo2RTCStreamDetection checks a normalized per-camera setting; field paths are
// relative to the detection object.
func ValidateGo2RTCStreamDetection(d Go2RTCStreamDetectionConfig) error {
	if d.IntervalSeconds < 1 || d.IntervalSeconds > 30 {
		return detectionFieldError("interval_seconds", "must be between 1 and 30")
	}
	if !(d.MinScore >= 0.30 && d.MinScore <= 0.95) {
		return detectionFieldError("min_score", "must be between 0.30 and 0.95")
	}
	if d.CooldownSeconds < 0 || d.CooldownSeconds > 86_400 {
		return detectionFieldError("cooldown_seconds", "must be between 0 and 86400")
	}
	if len(d.Zones) > Go2RTCDetectionMaxZones {
		return detectionFieldError("zones", "allows at most %d zones", Go2RTCDetectionMaxZones)
	}
	seen := make(map[string]bool, len(d.Zones))
	for i, zone := range d.Zones {
		field := fmt.Sprintf("zones[%d]", i)
		if n := utf8.RuneCountInString(zone.Name); n < 1 || n > Go2RTCDetectionMaxZoneName {
			return detectionFieldError(field+".name", "must have 1 to %d characters", Go2RTCDetectionMaxZoneName)
		}
		for _, r := range zone.Name {
			if unicode.IsControl(r) {
				return detectionFieldError(field+".name", "contains unsupported characters")
			}
		}
		key := strings.ToLower(zone.Name)
		if seen[key] {
			return detectionFieldError(field+".name", "duplicates zone %q", zone.Name)
		}
		seen[key] = true
		if len(zone.Points) < Go2RTCDetectionMinZonePoints || len(zone.Points) > Go2RTCDetectionMaxZonePoints {
			return detectionFieldError(field+".points", "needs %d to %d points", Go2RTCDetectionMinZonePoints, Go2RTCDetectionMaxZonePoints)
		}
		for j, point := range zone.Points {
			if !(point[0] >= 0 && point[0] <= 1 && point[1] >= 0 && point[1] <= 1) {
				return detectionFieldError(fmt.Sprintf("%s.points[%d]", field, j), "coordinates must be between 0 and 1")
			}
		}
	}
	if !validGo2RTCDetectionChannel(d.Actions.Channel) {
		return detectionFieldError("actions.channel", "must be one of %s", strings.Join(Go2RTCDetectionChannels, ", "))
	}
	if utf8.RuneCountInString(d.Actions.AgentInstruction) > Go2RTCDetectionMaxInstruction {
		return detectionFieldError("actions.agent_instruction", "allows at most %d characters", Go2RTCDetectionMaxInstruction)
	}
	return nil
}

// ValidateGo2RTCDetection checks the normalized global and per-stream settings with
// absolute field paths (go2rtc.detection.…, go2rtc.streams[<id>].detection.…).
func ValidateGo2RTCDetection(cfg Go2RTCConfig) error {
	if err := ValidateGo2RTCDetectionSettings(cfg.Detection); err != nil {
		return PrefixGo2RTCDetectionField(err, "go2rtc.detection")
	}
	for _, stream := range cfg.Streams {
		if err := ValidateGo2RTCStreamDetection(stream.Detection); err != nil {
			return PrefixGo2RTCDetectionField(err, "go2rtc.streams["+stream.ID+"].detection")
		}
	}
	return nil
}
