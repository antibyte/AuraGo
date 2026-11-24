package hass

import (
	"slices"
	"strings"
	"testing"

	"aurago/internal/homedash"
	"aurago/internal/tools"
)

func capNames(caps []homedash.CapabilitySpec) []string {
	out := []string{}
	for _, c := range caps {
		out = append(out, string(c.Name))
	}
	return out
}

func findCap(caps []homedash.CapabilitySpec, n homedash.Capability) (homedash.CapabilitySpec, bool) {
	for _, c := range caps {
		if c.Name == n {
			return c, true
		}
	}
	return homedash.CapabilitySpec{}, false
}

func TestHADomainMappingTable(t *testing.T) {
	cfg := tools.HAConfig{}
	for _, tc := range []struct {
		id, state string
		attrs     map[string]any
		kind      homedash.Kind
		caps      []string
	}{
		{"switch.a", "on", nil, homedash.KindSwitch, []string{"toggle", "turn_on", "turn_off", "history"}},
		{"input_boolean.a", "off", nil, homedash.KindSwitch, []string{"toggle", "turn_on", "turn_off", "history"}},
		{"light.a", "on", map[string]any{"supported_color_modes": []any{"color_temp", "hs"}, "effect_list": []any{"rainbow"}},
			homedash.KindLight, []string{"toggle", "turn_on", "turn_off", "brightness", "color_temp", "color", "effect"}},
		{"light.b", "on", map[string]any{"supported_color_modes": []any{"onoff"}}, homedash.KindLight, []string{"toggle", "turn_on", "turn_off"}},
		{"sensor.t", "21.5", nil, homedash.KindSensor, []string{"numeric", "history"}},
		{"sensor.s", "ok", nil, homedash.KindSensor, []string{}},
		{"climate.c", "heat", map[string]any{"temperature": 21.0, "hvac_modes": []any{"heat", "off"}}, homedash.KindClimate, []string{"set_temperature", "hvac_mode", "history"}},
		{"cover.c", "open", map[string]any{"current_position": 40.0}, homedash.KindCover, []string{"open", "close", "stop", "set_position"}},
		{"lock.l", "locked", nil, homedash.KindLock, []string{"lock", "unlock"}},
		{"scene.s", "x", nil, homedash.KindScene, []string{"activate"}},
		{"script.s", "off", nil, homedash.KindScript, []string{"activate"}},
		{"humidifier.h", "on", nil, homedash.KindHumidifier, []string{"toggle", "turn_on", "turn_off"}},
		{"vacuum.v", "docked", nil, homedash.KindVacuum, []string{}},
		{"camera.c", "idle", nil, homedash.KindCamera, []string{"snapshot"}},
		{"person.p", "home", nil, homedash.KindPresence, []string{}},
		{"automation.a", "on", nil, homedash.KindGeneric, []string{}},
		{"timer.t", "idle", nil, homedash.KindGeneric, []string{}},
	} {
		domain, _, _ := strings.Cut(tc.id, ".")
		spec, ok := MapDomain(domain)
		got := capNames(Capabilities(cfg, tc.id, tc.state, tc.attrs))
		if !ok || spec.Kind != tc.kind || !slices.Equal(got, tc.caps) {
			t.Errorf("%s: kind %s caps %v, want %s %v", tc.id, spec.Kind, got, tc.kind, tc.caps)
		}
	}
	if _, ok := MapDomain("alarm_control_panel"); ok {
		t.Error("alarm_control_panel must be excluded")
	}
	for id, ok := range map[string]bool{"light.kitchen": true, "_x.y_1": true, "Light.x": false, "light": false, "light.a-b": false} {
		if NativeIDValid(id) != ok {
			t.Errorf("NativeIDValid(%q)", id)
		}
	}
}

func TestHACapabilitiesHonourServicePolicy(t *testing.T) {
	garage := map[string]any{"current_position": 0.0, "device_class": "garage"}
	open, _ := findCap(Capabilities(tools.HAConfig{}, "cover.g", "closed", garage), homedash.CapOpen)
	if open.Allowed || open.BlockedBy != "home_assistant.allowed_services" || !open.Sensitive {
		t.Fatalf("default policy cover: %+v", open)
	}
	allowed, _ := findCap(Capabilities(tools.HAConfig{AllowedServices: []string{"cover.open_cover"}}, "cover.g", "closed", garage), homedash.CapOpen)
	if !allowed.Allowed {
		t.Fatalf("allowlisted cover.open_cover: %+v", allowed)
	}
	unlock, _ := findCap(Capabilities(tools.HAConfig{}, "lock.l", "locked", nil), homedash.CapUnlock)
	if unlock.Allowed || !unlock.Sensitive {
		t.Fatalf("unlock: %+v", unlock)
	}
	act, _ := findCap(Capabilities(tools.HAConfig{}, "script.s", "off", nil), homedash.CapActivate)
	if act.Allowed || !act.Sensitive {
		t.Fatalf("script: %+v", act)
	}
	ro, _ := findCap(Capabilities(tools.HAConfig{ReadOnly: true}, "switch.a", "on", nil), homedash.CapToggle)
	if ro.Allowed || ro.BlockedBy != "home_assistant.readonly" {
		t.Fatalf("readonly: %+v", ro)
	}
	bl, _ := findCap(Capabilities(tools.HAConfig{BlockedServices: []string{"switch.toggle"}}, "switch.a", "on", nil), homedash.CapToggle)
	if bl.Allowed || bl.BlockedBy != "home_assistant.blocked_services" {
		t.Fatalf("blocked: %+v", bl)
	}
}

func TestHAAttributeAllowlistDropsEntityPictureAndCoordinates(t *testing.T) {
	name, out := FilterAttributes(map[string]any{
		"friendly_name": "Kitchen\nLamp", "entity_picture": "/api/camera_proxy/x?token=abc", "latitude": 1.0, "longitude": 2.0,
		"gps_accuracy": 5.0, "access_token": "abc", "stream_url": "rtsp://x", "brightness": 255.0, "unit_of_measurement": "°C",
		"random_key": "x", "hvac_modes": []any{"heat", "off"},
	})
	if name != "Kitchen Lamp" {
		t.Fatalf("name %q", name)
	}
	for _, k := range []string{"entity_picture", "latitude", "longitude", "gps_accuracy", "access_token", "stream_url", "random_key", "brightness", "friendly_name"} {
		if _, ok := out[k]; ok {
			t.Errorf("%s kept", k)
		}
	}
	if out["brightness_pct"] != 100.0 || out["unit_of_measurement"] != "°C" || len(out["hvac_modes"].([]any)) != 2 {
		t.Fatalf("kept attributes: %v", out)
	}
}
