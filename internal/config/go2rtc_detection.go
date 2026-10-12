package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"gopkg.in/yaml.v3"
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

// go2RTCDetectionChannels are the tools.SendNotification channels a camera may alert on.
var go2RTCDetectionChannels = []string{"ntfy", "pushover", "telegram", "discord", "push", "telnyx", "cyd", "all"}

// Go2RTCDetectionChannels returns a copy of the tools.SendNotification channels a
// camera may alert on, so callers cannot widen what validation accepts.
func Go2RTCDetectionChannels() []string {
	return append([]string(nil), go2RTCDetectionChannels...)
}

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
		out.Schedule = append(out.Schedule, Go2RTCDetectionWindow{Days: normalizeGo2RTCDetectionDays(window.Days), From: strings.TrimSpace(window.From), To: strings.TrimSpace(window.To)})
	}
	return out
}

// normalizeGo2RTCDetectionDays returns the days in canonical spelling. Duplicates are
// dropped only when every day is valid, so a validation error on an invalid day keeps
// the index the user wrote.
func normalizeGo2RTCDetectionDays(in []string) []string {
	days := make([]string, 0, len(in))
	valid := true
	for _, day := range in {
		key := strings.ToLower(strings.TrimSpace(day))
		if _, ok := go2RTCDetectionDays[key]; !ok {
			valid = false
		}
		days = append(days, key)
	}
	if !valid {
		return days
	}
	unique := days[:0]
	seen := make(map[string]bool, len(days))
	for _, day := range days {
		if !seen[day] {
			seen[day] = true
			unique = append(unique, day)
		}
	}
	return unique
}

// go2RTCStreamDetectionUnset reports a block that sets nothing except Enabled.
func go2RTCStreamDetectionUnset(d Go2RTCStreamDetectionConfig) bool {
	return d.IntervalSeconds == 0 && d.MinScore == 0 && d.CooldownSeconds == 0 &&
		len(d.Zones) == 0 && d.Actions == (Go2RTCDetectionActions{})
}

// NormalizeGo2RTCStreamDetection applies the per-camera rule: a block that sets nothing
// except Enabled was never configured and yields the whole default set (Enabled kept).
// Otherwise only values whose zero is never valid are filled (interval, min score,
// channel); cooldown, zones and the action switches stay exactly as written, and names,
// channel and instruction get canonical spelling. It never shares slices with d.
func NormalizeGo2RTCStreamDetection(d Go2RTCStreamDetectionConfig) Go2RTCStreamDetectionConfig {
	if go2RTCStreamDetectionUnset(d) {
		return DefaultGo2RTCStreamDetection(d.Enabled)
	}
	out := d
	if out.IntervalSeconds == 0 {
		out.IntervalSeconds = Go2RTCDetectionDefaultInterval
	}
	if out.MinScore == 0 {
		out.MinScore = Go2RTCDetectionDefaultMinScore
	}
	out.Actions.Channel = strings.ToLower(strings.TrimSpace(d.Actions.Channel))
	if out.Actions.Channel == "" {
		out.Actions.Channel = Go2RTCDetectionDefaultChannel
	}
	out.Actions.AgentInstruction = strings.TrimSpace(d.Actions.AgentInstruction)
	out.Zones = make([]Go2RTCDetectionZone, 0, len(d.Zones))
	for _, zone := range d.Zones {
		points := append([][2]float64{}, zone.Points...)
		out.Zones = append(out.Zones, Go2RTCDetectionZone{Name: strings.TrimSpace(zone.Name), Points: points, badPoint: zone.badPoint})
	}
	return out
}

// UnmarshalYAML decodes points of any length, so a point that is not [x, y] becomes a
// validation error with a field path instead of a YAML decode failure.
func (z *Go2RTCDetectionZone) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		Name   string      `yaml:"name"`
		Points [][]float64 `yaml:"points"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	zone := Go2RTCDetectionZone{Name: raw.Name}
	if raw.Points != nil {
		zone.Points = make([][2]float64, len(raw.Points))
		for i, point := range raw.Points {
			if len(point) != 2 {
				if zone.badPoint == 0 {
					zone.badPoint = i + 1
				}
				continue
			}
			zone.Points[i] = [2]float64{point[0], point[1]}
		}
	}
	*z = zone
	return nil
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
	for _, candidate := range go2RTCDetectionChannels {
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
	names := make([]string, 0, len(d.Zones))
	for i, zone := range d.Zones {
		field := fmt.Sprintf("zones[%d]", i)
		if !utf8.ValidString(zone.Name) {
			return detectionFieldError(field+".name", "must be valid UTF-8")
		}
		if n := utf8.RuneCountInString(zone.Name); n < 1 || n > Go2RTCDetectionMaxZoneName {
			return detectionFieldError(field+".name", "must have 1 to %d characters", Go2RTCDetectionMaxZoneName)
		}
		for _, r := range zone.Name {
			if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
				return detectionFieldError(field+".name", "contains unsupported characters")
			}
		}
		// Names compare in NFC with simple case folding (strings.EqualFold), so
		// "ΟΔΟΣ"/"οδος" and composed/decomposed "Café" collide while "Straße" and
		// "STRASSE" stay distinct. At most 8 zones keep the pairwise check cheap.
		name := norm.NFC.String(zone.Name)
		for j, earlier := range names {
			if strings.EqualFold(earlier, name) {
				return detectionFieldError(field+".name", "duplicates zone %q", d.Zones[j].Name)
			}
		}
		names = append(names, name)
		if len(zone.Points) < Go2RTCDetectionMinZonePoints || len(zone.Points) > Go2RTCDetectionMaxZonePoints {
			return detectionFieldError(field+".points", "needs %d to %d points", Go2RTCDetectionMinZonePoints, Go2RTCDetectionMaxZonePoints)
		}
		for j, point := range zone.Points {
			if j == zone.badPoint-1 {
				return detectionFieldError(fmt.Sprintf("%s.points[%d]", field, j), "must be [x, y]")
			}
			if !(point[0] >= 0 && point[0] <= 1 && point[1] >= 0 && point[1] <= 1) {
				return detectionFieldError(fmt.Sprintf("%s.points[%d]", field, j), "coordinates must be between 0 and 1")
			}
		}
	}
	if !validGo2RTCDetectionChannel(d.Actions.Channel) {
		return detectionFieldError("actions.channel", "must be one of %s", strings.Join(go2RTCDetectionChannels, ", "))
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
	for i, stream := range cfg.Streams {
		if err := ValidateGo2RTCStreamDetection(stream.Detection); err != nil {
			id := stream.ID
			if strings.TrimSpace(id) == "" {
				id = fmt.Sprintf("#%d", i)
			}
			return PrefixGo2RTCDetectionField(err, "go2rtc.streams["+id+"].detection")
		}
	}
	return nil
}
