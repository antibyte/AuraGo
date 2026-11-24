// Package hass maps Home Assistant domains, capabilities and attributes onto the HomeDash entity model.
package hass

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"aurago/internal/homedash"
	"aurago/internal/tools"
)

// CommandSpec binds a capability to the Home Assistant service that executes it.
type CommandSpec struct {
	Cap     homedash.Capability
	Service string // Home Assistant service in the entity's domain
}

// DomainSpec describes how one Home Assistant domain maps to a HomeDash kind.
type DomainSpec struct {
	Domain   string
	Kind     homedash.Kind
	Commands []CommandSpec
	Read     []homedash.Capability
}

var nativeIDRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*\.[a-z0-9_]+$`)

// NativeIDValid checks a Home Assistant entity id (the native part of "ha:<id>").
func NativeIDValid(id string) bool {
	return len(id) <= homedash.MaxNativeIDBytes && nativeIDRe.MatchString(id)
}

func cmds(groups ...[]CommandSpec) []CommandSpec {
	var out []CommandSpec
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

func cs(c homedash.Capability, service string) []CommandSpec { return []CommandSpec{{c, service}} }

var (
	onOff   = []CommandSpec{{homedash.CapToggle, "toggle"}, {homedash.CapTurnOn, "turn_on"}, {homedash.CapTurnOff, "turn_off"}}
	history = []homedash.Capability{homedash.CapHistory}
)

var domainTable = map[string]DomainSpec{
	"switch":        {Kind: homedash.KindSwitch, Commands: onOff, Read: history},
	"input_boolean": {Kind: homedash.KindSwitch, Commands: onOff, Read: history},
	"light": {Kind: homedash.KindLight, Commands: cmds(onOff, cs(homedash.CapBrightness, "turn_on"), cs(homedash.CapColorTemp, "turn_on"),
		cs(homedash.CapColor, "turn_on"), cs(homedash.CapEffect, "turn_on"))},
	"sensor":        {Kind: homedash.KindSensor},
	"binary_sensor": {Kind: homedash.KindBinarySensor, Read: history},
	"climate": {Kind: homedash.KindClimate, Read: history, Commands: cmds(cs(homedash.CapSetTemperature, "set_temperature"),
		cs(homedash.CapSetTemperatureRange, "set_temperature"), cs(homedash.CapHVACMode, "set_hvac_mode"),
		cs(homedash.CapPresetMode, "set_preset_mode"), cs(homedash.CapFanMode, "set_fan_mode"))},
	"cover": {Kind: homedash.KindCover, Commands: cmds(cs(homedash.CapOpen, "open_cover"), cs(homedash.CapClose, "close_cover"),
		cs(homedash.CapStop, "stop_cover"), cs(homedash.CapSetPosition, "set_cover_position"), cs(homedash.CapSetTilt, "set_cover_tilt_position"))},
	"fan": {Kind: homedash.KindFan, Commands: cmds(onOff, cs(homedash.CapSetPercentage, "set_percentage"),
		cs(homedash.CapPresetMode, "set_preset_mode"), cs(homedash.CapOscillate, "oscillate"))},
	"lock": {Kind: homedash.KindLock, Commands: cmds(cs(homedash.CapLock, "lock"), cs(homedash.CapUnlock, "unlock"))},
	"media_player": {Kind: homedash.KindMediaPlayer, Commands: cmds(cs(homedash.CapPlayPause, "media_play_pause"),
		cs(homedash.CapNext, "media_next_track"), cs(homedash.CapPrevious, "media_previous_track"), cs(homedash.CapVolumeSet, "volume_set"),
		cs(homedash.CapVolumeMute, "volume_mute"), cs(homedash.CapSourceSelect, "select_source"), cs(homedash.CapToggle, "toggle"))},
	"scene":          {Kind: homedash.KindScene, Commands: cs(homedash.CapActivate, "turn_on")},
	"script":         {Kind: homedash.KindScript, Commands: cs(homedash.CapActivate, "turn_on")},
	"button":         {Kind: homedash.KindButton, Commands: cs(homedash.CapPress, "press")},
	"input_button":   {Kind: homedash.KindButton, Commands: cs(homedash.CapPress, "press")},
	"number":         {Kind: homedash.KindNumber, Commands: cs(homedash.CapSetValue, "set_value"), Read: history},
	"input_number":   {Kind: homedash.KindNumber, Commands: cs(homedash.CapSetValue, "set_value"), Read: history},
	"select":         {Kind: homedash.KindSelect, Commands: cs(homedash.CapSelectOption, "select_option")},
	"input_select":   {Kind: homedash.KindSelect, Commands: cs(homedash.CapSelectOption, "select_option")},
	"humidifier":     {Kind: homedash.KindHumidifier, Commands: onOff}, // modes deferred (main design section 10)
	"vacuum":         {Kind: homedash.KindVacuum},                      // commands deferred (main design section 10)
	"camera":         {Kind: homedash.KindCamera, Read: []homedash.Capability{homedash.CapSnapshot}},
	"person":         {Kind: homedash.KindPresence},
	"device_tracker": {Kind: homedash.KindPresence},
	"sun":            {Kind: homedash.KindSun},
	"weather":        {Kind: homedash.KindWeather},
	"update":         {Kind: homedash.KindUpdate},
}

// MapDomain maps a Home Assistant domain. alarm_control_panel is excluded (ok=false); every other
// unknown domain (automation, timer, counter ...) is a read-only generic entity.
func MapDomain(domain string) (DomainSpec, bool) {
	if domain == "alarm_control_panel" {
		return DomainSpec{}, false
	}
	s, ok := domainTable[domain]
	if !ok {
		s = DomainSpec{Kind: homedash.KindGeneric}
	}
	s.Domain = domain
	return s, true
}

// Capabilities lists one entity's capabilities. Command capabilities carry Allowed/BlockedBy from the
// shared Home Assistant service policy; attributes decide optional features, ranges and options.
func Capabilities(cfg tools.HAConfig, entityID, state string, attrs map[string]any) []homedash.CapabilitySpec {
	domain, _, _ := strings.Cut(entityID, ".")
	spec, ok := MapDomain(domain)
	if !ok {
		return nil
	}
	out := []homedash.CapabilitySpec{}
	for _, c := range spec.Commands {
		cap, keep := shapeCapability(spec.Kind, c.Cap, attrs)
		if !keep {
			continue
		}
		cap.Allowed = tools.HAServiceAllowed(cfg, domain, c.Service)
		if !cap.Allowed {
			cap.BlockedBy = blockedBy(cfg, domain+"."+c.Service)
		}
		out = append(out, cap)
	}
	for _, r := range spec.Read {
		out = append(out, homedash.CapabilitySpec{Name: r, Allowed: true})
	}
	if spec.Kind == homedash.KindSensor {
		if f, err := strconv.ParseFloat(state, 64); err == nil && finite(f) {
			out = append(out, homedash.CapabilitySpec{Name: homedash.CapNumeric, Allowed: true}, homedash.CapabilitySpec{Name: homedash.CapHistory, Allowed: true})
		}
	}
	return out
}

func blockedBy(cfg tools.HAConfig, full string) string {
	if cfg.ReadOnly {
		return "home_assistant.readonly"
	}
	for _, b := range cfg.BlockedServices {
		if strings.EqualFold(strings.TrimSpace(b), full) {
			return "home_assistant.blocked_services"
		}
	}
	return "home_assistant.allowed_services"
}

func fp(v float64) *float64 { return &v }

func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

func num(attrs map[string]any, key string, def float64) *float64 {
	if f, ok := attrs[key].(float64); ok && finite(f) {
		return &f
	}
	return &def
}

func stringList(v any) []string {
	raw, _ := v.([]any)
	out := []string{}
	for _, e := range raw {
		if s, ok := e.(string); ok && len(out) < 64 {
			if s = homedash.CleanText(s, 64); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func has(attrs map[string]any, key string) bool { _, ok := attrs[key]; return ok }

func entryCover(attrs map[string]any) bool {
	dc, _ := attrs["device_class"].(string)
	return dc == "garage" || dc == "gate" || dc == "door"
}

func shapeCapability(kind homedash.Kind, c homedash.Capability, attrs map[string]any) (homedash.CapabilitySpec, bool) {
	s := homedash.CapabilitySpec{Name: c}
	modes := stringList(attrs["supported_color_modes"])
	anyMode := func(list ...string) bool {
		return slices.ContainsFunc(modes, func(m string) bool { return slices.Contains(list, m) })
	}
	options := func(key string) bool { s.Options = stringList(attrs[key]); return len(s.Options) > 0 }
	switch c {
	case homedash.CapBrightness:
		s.Min, s.Max, s.Step = fp(0), fp(100), fp(1)
		return s, anyMode("brightness", "color_temp", "hs", "xy", "rgb", "rgbw", "rgbww", "white")
	case homedash.CapColorTemp:
		s.Min, s.Max, s.Step = num(attrs, "min_color_temp_kelvin", 2000), num(attrs, "max_color_temp_kelvin", 6500), fp(50)
		return s, anyMode("color_temp")
	case homedash.CapColor:
		return s, anyMode("hs", "xy", "rgb", "rgbw", "rgbww")
	case homedash.CapEffect:
		return s, options("effect_list")
	case homedash.CapSetTemperature, homedash.CapSetTemperatureRange:
		s.Min, s.Max, s.Step = num(attrs, "min_temp", 7), num(attrs, "max_temp", 35), num(attrs, "target_temp_step", 0.5)
		if c == homedash.CapSetTemperature {
			return s, has(attrs, "temperature") || !has(attrs, "target_temp_low")
		}
		return s, has(attrs, "target_temp_low")
	case homedash.CapHVACMode:
		return s, options("hvac_modes")
	case homedash.CapPresetMode:
		return s, options("preset_modes")
	case homedash.CapFanMode:
		return s, options("fan_modes")
	case homedash.CapOpen, homedash.CapClose:
		s.Sensitive = entryCover(attrs)
	case homedash.CapSetPosition:
		s.Min, s.Max, s.Step, s.Sensitive = fp(0), fp(100), fp(1), entryCover(attrs)
		return s, has(attrs, "current_position")
	case homedash.CapSetTilt:
		s.Min, s.Max, s.Step = fp(0), fp(100), fp(1)
		return s, has(attrs, "current_tilt_position")
	case homedash.CapSetPercentage:
		s.Min, s.Max, s.Step = fp(0), fp(100), num(attrs, "percentage_step", 1)
		return s, has(attrs, "percentage")
	case homedash.CapOscillate:
		return s, has(attrs, "oscillating")
	case homedash.CapVolumeSet:
		s.Min, s.Max, s.Step = fp(0), fp(100), fp(1) // percent; the adapter sends volume_level/100
		return s, has(attrs, "volume_level")
	case homedash.CapVolumeMute:
		return s, has(attrs, "is_volume_muted")
	case homedash.CapSourceSelect:
		return s, options("source_list")
	case homedash.CapSetValue:
		s.Min, s.Max, s.Step = num(attrs, "min", 0), num(attrs, "max", 100), num(attrs, "step", 1)
	case homedash.CapSelectOption:
		return s, options("options")
	case homedash.CapUnlock:
		s.Sensitive = true
	case homedash.CapActivate:
		s.Sensitive = kind == homedash.KindScript
	}
	return s, true
}
