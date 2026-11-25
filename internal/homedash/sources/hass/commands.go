package hass

import (
	"context"
	"errors"
	"strconv"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/tools"
)

// serviceCall is the Home Assistant service a HomeDash command maps to.
type serviceCall struct {
	Domain, Service string
	Data            map[string]any // service data without entity_id
}

func argNum(args map[string]any, key string) (float64, bool) {
	switch v := args[key].(type) {
	case float64:
		return v, finite(v)
	case int:
		return float64(v), true
	}
	return 0, false
}

func argStr(args map[string]any, key string) (string, bool) {
	s, ok := args[key].(string)
	return s, ok && s != ""
}

// mapCommand maps a validated command to its service (decision D3, design-backend section 6). The
// service comes from the domain table; ok is false when the domain does not offer the capability or
// an argument has the wrong type.
func mapCommand(native string, c homedash.Capability, args map[string]any) (serviceCall, bool) {
	domain, ok := domainOf(native)
	if !ok {
		return serviceCall{}, false
	}
	spec, ok := MapDomain(domain)
	if !ok {
		return serviceCall{}, false
	}
	service := ""
	for _, cs := range spec.Commands {
		if cs.Cap == c {
			service = cs.Service
			break
		}
	}
	if service == "" {
		return serviceCall{}, false
	}
	call := serviceCall{Domain: domain, Service: service, Data: map[string]any{}}
	// num maps a finite number; lo/hi bound it when hi > lo (the hub checked the entity's own range
	// already, this is the adapter's second, fixed-range check).
	rng := func(key, as string, scale, lo, hi float64) bool {
		v, ok := argNum(args, key)
		if ok && hi > lo && (v < lo || v > hi) {
			ok = false
		}
		if ok {
			call.Data[as] = v * scale
		}
		return ok
	}
	num := func(key, as string, scale float64) bool { return rng(key, as, scale, 0, 0) }
	pct := func(key, as string, scale float64) bool { return rng(key, as, scale, 0, 100) }
	str := func(key, as string) bool {
		v, ok := argStr(args, key)
		if ok {
			call.Data[as] = v
		}
		return ok
	}
	boolean := func(key, as string) bool {
		v, ok := args[key].(bool)
		if ok {
			call.Data[as] = v
		}
		return ok
	}
	switch c {
	case homedash.CapToggle, homedash.CapTurnOn, homedash.CapTurnOff, homedash.CapActivate, homedash.CapPress,
		homedash.CapOpen, homedash.CapClose, homedash.CapStop, homedash.CapLock, homedash.CapUnlock,
		homedash.CapPlayPause, homedash.CapNext, homedash.CapPrevious:
		ok = true
	case homedash.CapBrightness:
		ok = pct("value", "brightness_pct", 1)
	case homedash.CapColorTemp:
		ok = num("kelvin", "color_temp_kelvin", 1)
	case homedash.CapColor:
		h, okh := argNum(args, "h")
		s, oks := argNum(args, "s")
		if ok = okh && oks && h >= 0 && h <= 360 && s >= 0 && s <= 100; ok {
			call.Data["hs_color"] = []any{h, s}
		}
	case homedash.CapEffect:
		ok = str("option", "effect")
	case homedash.CapSelectOption:
		ok = str("option", "option")
	case homedash.CapSetValue:
		ok = num("value", "value", 1)
	case homedash.CapSetTemperature:
		ok = num("value", "temperature", 1)
	case homedash.CapSetTemperatureRange:
		ok = num("low", "target_temp_low", 1) && num("high", "target_temp_high", 1)
	case homedash.CapHVACMode:
		ok = str("option", "hvac_mode")
	case homedash.CapPresetMode:
		ok = str("option", "preset_mode")
	case homedash.CapFanMode:
		ok = str("option", "fan_mode")
	case homedash.CapSetPosition:
		ok = pct("value", "position", 1)
	case homedash.CapSetTilt:
		ok = pct("value", "tilt_position", 1)
	case homedash.CapSetPercentage:
		ok = pct("value", "percentage", 1)
	case homedash.CapOscillate:
		ok = boolean("on", "oscillating")
	case homedash.CapVolumeSet:
		ok = pct("value", "volume_level", 0.01)
	case homedash.CapVolumeMute:
		ok = boolean("on", "is_volume_muted")
	case homedash.CapSourceSelect:
		ok = str("option", "source")
	default:
		ok = false
	}
	return call, ok
}

// expectedState predicts the state a W1 command leads to (decision D3). It returns nil for momentary
// commands (activate, press), for other capabilities and for a toggle whose current state is not on
// or off. Attributes carry only what the command sets (lists as []any for BoundAttributes).
func expectedState(domain string, c homedash.Capability, args map[string]any, cur HAState, have bool, now time.Time) *homedash.State {
	on := func(attrs map[string]any) *homedash.State {
		return &homedash.State{Value: true, Text: "on", Attributes: attrs, Available: true, UpdatedAt: now}
	}
	off := func() *homedash.State {
		return &homedash.State{Value: false, Text: "off", Available: true, UpdatedAt: now}
	}
	switch c {
	case homedash.CapToggle:
		if !have {
			return nil
		}
		switch cur.State {
		case "on":
			return off()
		case "off":
			return on(nil)
		}
		return nil
	case homedash.CapTurnOn:
		return on(nil)
	case homedash.CapTurnOff:
		return off()
	case homedash.CapBrightness:
		v, _ := argNum(args, "value")
		if v <= 0 {
			return off()
		}
		return on(map[string]any{"brightness_pct": v})
	case homedash.CapColorTemp:
		v, _ := argNum(args, "kelvin")
		return on(map[string]any{"color_temp_kelvin": v})
	case homedash.CapColor:
		h, _ := argNum(args, "h")
		s, _ := argNum(args, "s")
		return on(map[string]any{"hs_color": []any{h, s}})
	case homedash.CapEffect:
		o, _ := argStr(args, "option")
		return on(map[string]any{"effect": o})
	case homedash.CapSelectOption:
		o, _ := argStr(args, "option")
		return &homedash.State{Value: o, Text: o, Available: true, UpdatedAt: now}
	case homedash.CapSetValue:
		v, _ := argNum(args, "value")
		return &homedash.State{Value: v, Text: strconv.FormatFloat(v, 'f', -1, 64), Available: true, UpdatedAt: now}
	}
	return nil
}

// optionKeys names the attribute that lists the options of an option capability.
var optionKeys = map[homedash.Capability]string{
	homedash.CapEffect: "effect_list", homedash.CapSelectOption: "options", homedash.CapHVACMode: "hvac_modes",
	homedash.CapPresetMode: "preset_modes", homedash.CapFanMode: "fan_modes", homedash.CapSourceSelect: "source_list",
}

// optionAllowed re-checks an option argument against the entity's current option list. It is true for
// capabilities without options.
func optionAllowed(c homedash.Capability, args, attrs map[string]any) bool {
	key, ok := optionKeys[c]
	if !ok {
		return true
	}
	want, _ := argStr(args, "option")
	list, _ := attrs[key].([]any)
	for _, e := range list {
		if s, ok := e.(string); ok && s == want {
			return true
		}
	}
	return false
}

func errSourceUnavailable() *homedash.Error {
	return homedash.SourceUnavailable(SourceID, "Home Assistant is not reachable")
}

func errCommandFailed() *homedash.Error {
	return &homedash.Error{Code: homedash.CodeCommandFailed, Message: "Home Assistant did not accept the command"}
}

// Command maps the command to its service, predicts the expected state from the state before the
// call, calls the service through tools.HACallServiceStates (read-only mode, service policy and
// target refusal are re-checked there) and feeds the changed states of the answer into the sink at
// once. Clients get fixed codes only; Home Assistant's text goes to the log, scrubbed.
func (a *Adapter) Command(ctx context.Context, cmd homedash.Command) (homedash.CommandResult, error) {
	native := cmd.Native
	if native == "" {
		if src, n, err := homedash.ParseEntityID(string(cmd.Entity)); err == nil && src == SourceID {
			native = n
		}
	}
	// The target is the native id only: it must be a plain Home Assistant entity id.
	call, ok := mapCommand(native, cmd.Name, cmd.Args)
	if !NativeIDValid(native) || !ok {
		return homedash.CommandResult{}, &homedash.Error{Code: homedash.CodeUnsupportedCommand, Message: "Home Assistant cannot run this command for this entity"}
	}
	a.mu.Lock()
	cur, have := a.states[native]
	known := a.complete
	a.mu.Unlock()
	if known && !have {
		return homedash.CommandResult{}, &homedash.Error{Code: homedash.CodeEntityNotFound, Message: "unknown entity"}
	}
	if have && !optionAllowed(cmd.Name, cmd.Args, cur.Attributes) {
		return homedash.CommandResult{}, &homedash.Error{Code: homedash.CodeInvalidArgs, Message: "the option is not offered by the entity"}
	}
	expected := expectedState(call.Domain, cmd.Name, cmd.Args, cur, have, a.o.Now().UTC())
	changed, err := tools.HACallServiceStates(ctx, a.ha, call.Domain, call.Service, native, call.Data)
	if err != nil {
		var se *tools.HAServiceError
		if !errors.As(err, &se) || se.Kind != tools.HAServiceBadAnswer {
			return homedash.CommandResult{}, a.commandError(ctx, call, err)
		}
		a.log.Debug("homedash: Home Assistant answered a service call with an unexpected body",
			"service", call.Domain+"."+call.Service, "status", se.Status)
	}
	a.mu.Lock()
	for _, st := range changed {
		a.applyStateLocked(HAState{EntityID: st.EntityID, State: st.State, Attributes: st.Attributes, LastChanged: st.LastChanged, LastUpdated: st.LastUpdated})
	}
	a.mu.Unlock()
	return homedash.CommandResult{Expected: expected}, nil
}

// commandError maps a failed call to a fixed HomeDash error. The caller's own cancellation is
// returned as the context error so the hub reports it as such (504/abandoned dedupe entry).
func (a *Adapter) commandError(ctx context.Context, call serviceCall, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var se *tools.HAServiceError
	if !errors.As(err, &se) {
		a.log.Warn("homedash: Home Assistant service call failed", "service", call.Domain+"."+call.Service, "error", err)
		return errCommandFailed()
	}
	a.log.Warn("homedash: Home Assistant service call failed", "service", call.Domain+"."+call.Service,
		"kind", string(se.Kind), "status", se.Status, "detail", se.LogDetail)
	switch se.Kind {
	case tools.HAServicePolicy:
		return &homedash.Error{Code: homedash.CodeReadOnly, Message: "commands are switched off", BlockedBy: blockedBy(a.ha, call.Domain+"."+call.Service)}
	case tools.HAServiceUnreachable, tools.HAServiceAuth:
		return errSourceUnavailable()
	case tools.HAServiceUpstream:
		if se.Status == 502 || se.Status == 503 || se.Status == 504 {
			return errSourceUnavailable()
		}
	}
	return errCommandFailed()
}
