package homedash

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"unicode/utf8"
)

// Capability names a command or read capability of an entity.
type Capability string

// Command capabilities carry args; read capabilities (stream, snapshot, history, numeric) carry none.
const (
	CapToggle              Capability = "toggle"
	CapTurnOn              Capability = "turn_on"
	CapTurnOff             Capability = "turn_off"
	CapBrightness          Capability = "brightness"
	CapColorTemp           Capability = "color_temp"
	CapColor               Capability = "color"
	CapEffect              Capability = "effect"
	CapSetTemperature      Capability = "set_temperature"
	CapSetTemperatureRange Capability = "set_temperature_range"
	CapHVACMode            Capability = "hvac_mode"
	CapPresetMode          Capability = "preset_mode"
	CapFanMode             Capability = "fan_mode"
	CapOpen                Capability = "open"
	CapClose               Capability = "close"
	CapStop                Capability = "stop"
	CapSetPosition         Capability = "set_position"
	CapSetTilt             Capability = "set_tilt"
	CapLock                Capability = "lock"
	CapUnlock              Capability = "unlock"
	CapSetPercentage       Capability = "set_percentage"
	CapOscillate           Capability = "oscillate"
	CapPlayPause           Capability = "media_play_pause"
	CapNext                Capability = "media_next"
	CapPrevious            Capability = "media_previous"
	CapVolumeSet           Capability = "volume_set"
	CapVolumeMute          Capability = "volume_mute"
	CapSourceSelect        Capability = "source_select"
	CapPress               Capability = "press"
	CapActivate            Capability = "activate"
	CapSetValue            Capability = "set_value"
	CapSelectOption        Capability = "select_option"
	CapSetHumidity         Capability = "set_humidity"
	CapSetMode             Capability = "set_mode"
	CapVacuumStart         Capability = "vacuum_start"
	CapVacuumPause         Capability = "vacuum_pause"
	CapVacuumReturn        Capability = "vacuum_return"
	CapContainerStart      Capability = "container_start"
	CapContainerStop       Capability = "container_stop"
	CapContainerRestart    Capability = "container_restart"
	CapStream              Capability = "stream"
	CapSnapshot            Capability = "snapshot"
	CapHistory             Capability = "history"
	CapNumeric             Capability = "numeric"
)

const (
	ArgNumber = "number"
	ArgString = "string"
	ArgBool   = "bool"
)

// ArgSpec is one argument. EntityRange: the entity's CapabilitySpec Min/Max/Step narrow the fixed range.
type ArgSpec struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	EntityRange bool     `json:"entity_range,omitempty"`
}

// CapabilityInfo describes a capability. Sensitive here means "always confirmed"; adapters mark
// context-dependent cases (scripts, entry covers, AdGuard off) on the entity's CapabilitySpec.
type CapabilityInfo struct {
	Name        Capability `json:"name"`
	Command     bool       `json:"command"`
	Args        []ArgSpec  `json:"args"`
	Sensitive   bool       `json:"sensitive,omitempty"`
	KioskDenied bool       `json:"kiosk_denied,omitempty"`
	Continuous  bool       `json:"continuous,omitempty"` // latest-wins slot
}

func numPtr(v float64) *float64 { return &v }

func capArgs(a ...ArgSpec) []ArgSpec {
	if len(a) == 0 {
		return []ArgSpec{}
	}
	return a
}

var capabilityRegistry = buildCapabilityRegistry()

func buildCapabilityRegistry() []CapabilityInfo {
	v := ArgSpec{Name: "value", Type: ArgNumber, EntityRange: true}
	pct := ArgSpec{Name: "value", Type: ArgNumber, Min: numPtr(0), Max: numPtr(100), EntityRange: true}
	opt := ArgSpec{Name: "option", Type: ArgString}
	on := ArgSpec{Name: "on", Type: ArgBool}
	none := func(c Capability) CapabilityInfo { return CapabilityInfo{Name: c, Command: true, Args: capArgs()} }
	withArgs := func(c Capability, continuous bool, a ...ArgSpec) CapabilityInfo {
		return CapabilityInfo{Name: c, Command: true, Args: capArgs(a...), Continuous: continuous}
	}
	read := func(c Capability) CapabilityInfo { return CapabilityInfo{Name: c, Args: capArgs()} }
	unlock := none(CapUnlock)
	unlock.Sensitive, unlock.KioskDenied = true, true
	cStart := none(CapContainerStart)
	cStart.KioskDenied = true
	cStop := none(CapContainerStop)
	cStop.Sensitive, cStop.KioskDenied = true, true
	cRestart := none(CapContainerRestart)
	cRestart.Sensitive, cRestart.KioskDenied = true, true
	return []CapabilityInfo{
		none(CapToggle), none(CapTurnOn), none(CapTurnOff),
		withArgs(CapBrightness, true, pct),
		withArgs(CapColorTemp, true, ArgSpec{Name: "kelvin", Type: ArgNumber, Min: numPtr(1000), Max: numPtr(12000), EntityRange: true}),
		withArgs(CapColor, true,
			ArgSpec{Name: "h", Type: ArgNumber, Min: numPtr(0), Max: numPtr(360)},
			ArgSpec{Name: "s", Type: ArgNumber, Min: numPtr(0), Max: numPtr(100)}),
		withArgs(CapEffect, false, opt), withArgs(CapHVACMode, false, opt),
		withArgs(CapPresetMode, false, opt), withArgs(CapFanMode, false, opt),
		withArgs(CapSetTemperature, true, v),
		withArgs(CapSetTemperatureRange, false,
			ArgSpec{Name: "low", Type: ArgNumber, EntityRange: true},
			ArgSpec{Name: "high", Type: ArgNumber, EntityRange: true}),
		none(CapOpen), none(CapClose), none(CapStop),
		withArgs(CapSetPosition, true, pct), withArgs(CapSetTilt, true, pct),
		none(CapLock), unlock,
		withArgs(CapSetPercentage, true, pct),
		withArgs(CapOscillate, false, on),
		none(CapPlayPause), none(CapNext), none(CapPrevious),
		withArgs(CapVolumeSet, true, pct),
		withArgs(CapVolumeMute, false, on),
		withArgs(CapSourceSelect, false, opt),
		none(CapPress), none(CapActivate),
		withArgs(CapSetValue, true, v),
		withArgs(CapSelectOption, false, opt),
		withArgs(CapSetHumidity, true, pct),
		withArgs(CapSetMode, false, opt),
		none(CapVacuumStart), none(CapVacuumPause), none(CapVacuumReturn),
		cStart, cStop, cRestart,
		read(CapStream), read(CapSnapshot), read(CapHistory), read(CapNumeric),
	}
}

// CapabilityRegistry returns a copy of the capability registry.
func CapabilityRegistry() []CapabilityInfo {
	out := make([]CapabilityInfo, len(capabilityRegistry))
	for i, c := range capabilityRegistry {
		c.Args = slices.Clone(c.Args)
		out[i] = c
	}
	return out
}

// LookupCapability finds a capability in the registry.
func LookupCapability(c Capability) (CapabilityInfo, bool) {
	for _, info := range capabilityRegistry {
		if info.Name == c {
			info.Args = slices.Clone(info.Args)
			return info, true
		}
	}
	return CapabilityInfo{}, false
}

func (i CapabilityInfo) hasArg(name string) bool {
	for _, a := range i.Args {
		if a.Name == name {
			return true
		}
	}
	return false
}

func invalidArgs(format string, a ...any) *Error {
	return &Error{Code: CodeInvalidArgs, Message: fmt.Sprintf(format, a...)}
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, !math.IsNaN(t) && !math.IsInf(t, 0)
	case int:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
	}
	return 0, false
}

// ValidateArgs checks a command's arguments against the registry and the entity's spec
// (min/max/step/options). It rejects unknown and missing arguments.
func ValidateArgs(spec CapabilitySpec, args map[string]any) error {
	info, ok := LookupCapability(spec.Name)
	if !ok || !info.Command {
		return newError(CodeUnsupportedCommand, "not a command capability")
	}
	for k := range args {
		if !info.hasArg(k) {
			return invalidArgs("unknown argument %q", k)
		}
	}
	nums := map[string]float64{}
	for _, a := range info.Args {
		v, present := args[a.Name]
		if !present {
			return invalidArgs("missing argument %q", a.Name)
		}
		switch a.Type {
		case ArgNumber:
			f, ok := toFloat(v)
			if !ok {
				return invalidArgs("%s must be a number", a.Name)
			}
			lo, hi, step := a.Min, a.Max, (*float64)(nil)
			var anchor *float64
			if a.EntityRange {
				// The entity's range narrows the fixed range, never widens it.
				if spec.Min != nil && (lo == nil || *spec.Min > *lo) {
					lo = spec.Min
				}
				if spec.Max != nil && (hi == nil || *spec.Max < *hi) {
					hi = spec.Max
				}
				step = spec.Step
				anchor = spec.Min
			}
			if (lo != nil && f < *lo) || (hi != nil && f > *hi) {
				return invalidArgs("%s is out of range", a.Name)
			}
			if step != nil && *step > 0 {
				base := 0.0
				if anchor != nil {
					base = *anchor
				} else if lo != nil {
					base = *lo
				}
				q := (f - base) / *step
				if math.Abs(q-math.Round(q)) > 1e-6 {
					return invalidArgs("%s is not a multiple of %g", a.Name, *step)
				}
			}
			nums[a.Name] = f
		case ArgString:
			s, ok := v.(string)
			if !ok || s == "" || utf8.RuneCountInString(s) > 64 || ContainsControl(s) {
				return invalidArgs("%s must be a short text", a.Name)
			}
			if len(spec.Options) > 0 && !slices.Contains(spec.Options, s) {
				return invalidArgs("%s is not an offered option", a.Name)
			}
		case ArgBool:
			if _, ok := v.(bool); !ok {
				return invalidArgs("%s must be true or false", a.Name)
			}
		}
	}
	if spec.Name == CapSetTemperatureRange && nums["low"] >= nums["high"] {
		return invalidArgs("low must be below high")
	}
	return nil
}
