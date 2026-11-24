package homedash

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

func f64(v float64) *float64 { return &v }

func jsonSize(v any) (int, error) {
	b, err := json.Marshal(v)
	return len(b), err
}

func TestParseEntityID(t *testing.T) {
	long := strings.Repeat("a", 200)
	for _, tc := range []struct {
		in     string
		ok     bool
		src    SourceID
		native string
	}{
		{"ha:light.kitchen", true, "ha", "light.kitchen"},
		{"docker:local/web", true, "docker", "local/web"},
		{"ha:" + long, true, "ha", long},
		{"ha:" + long + "a", false, "", ""},
		{"ha:", false, "", ""}, {":x", false, "", ""}, {"HA:x", false, "", ""}, {"h:x", false, "", ""},
		{"ha:has space", false, "", ""}, {"ha:tab\tx", false, "", ""}, {"nocolon", false, "", ""},
		{"ha:" + string(rune(0x202e)) + "x", false, "", ""},
	} {
		src, native, err := ParseEntityID(tc.in)
		if (err == nil) != tc.ok || src != tc.src || native != tc.native {
			t.Errorf("%q: %q %q %v", tc.in, src, native, err)
		}
	}
}

func TestValidateArgsPerCapability(t *testing.T) {
	pct := CapabilitySpec{Min: f64(0), Max: f64(100), Step: f64(1)}
	with := func(c Capability, base CapabilitySpec) CapabilitySpec { base.Name = c; return base }
	for _, tc := range []struct {
		name string
		spec CapabilitySpec
		args map[string]any
		code string
	}{
		{"toggle", with(CapToggle, CapabilitySpec{}), map[string]any{}, ""},
		{"toggle extra", with(CapToggle, CapabilitySpec{}), map[string]any{"x": 1.0}, CodeInvalidArgs},
		{"brightness", with(CapBrightness, pct), map[string]any{"value": 40.0}, ""},
		{"brightness range", with(CapBrightness, pct), map[string]any{"value": 150.0}, CodeInvalidArgs},
		{"brightness step", with(CapBrightness, pct), map[string]any{"value": 40.5}, CodeInvalidArgs},
		{"brightness text", with(CapBrightness, pct), map[string]any{"value": "40"}, CodeInvalidArgs},
		{"kelvin", with(CapColorTemp, CapabilitySpec{Min: f64(2200), Max: f64(6500), Step: f64(50)}), map[string]any{"kelvin": 3000.0}, ""},
		{"kelvin step", with(CapColorTemp, CapabilitySpec{Min: f64(2200), Max: f64(6500), Step: f64(50)}), map[string]any{"kelvin": 3025.0}, CodeInvalidArgs},
		{"brightness entity wider max", with(CapBrightness, CapabilitySpec{Min: f64(0), Max: f64(255), Step: f64(1)}), map[string]any{"value": 150.0}, CodeInvalidArgs},
		{"brightness entity wider min", with(CapBrightness, CapabilitySpec{Min: f64(-50), Max: f64(100)}), map[string]any{"value": -10.0}, CodeInvalidArgs},
		{"brightness entity narrower above", with(CapBrightness, CapabilitySpec{Min: f64(10), Max: f64(90)}), map[string]any{"value": 95.0}, CodeInvalidArgs},
		{"brightness entity narrower inside", with(CapBrightness, CapabilitySpec{Min: f64(10), Max: f64(90)}), map[string]any{"value": 50.0}, ""},
		{"kelvin entity narrower", with(CapColorTemp, CapabilitySpec{Min: f64(2200), Max: f64(6500), Step: f64(50)}), map[string]any{"kelvin": 7000.0}, CodeInvalidArgs},
		{"color", with(CapColor, CapabilitySpec{}), map[string]any{"h": 200.0, "s": 50.0}, ""},
		{"color hue", with(CapColor, CapabilitySpec{}), map[string]any{"h": 400.0, "s": 50.0}, CodeInvalidArgs},
		{"color missing", with(CapColor, CapabilitySpec{}), map[string]any{"h": 200.0}, CodeInvalidArgs},
		{"option", with(CapHVACMode, CapabilitySpec{Options: []string{"heat", "off"}}), map[string]any{"option": "heat"}, ""},
		{"option not offered", with(CapHVACMode, CapabilitySpec{Options: []string{"heat", "off"}}), map[string]any{"option": "cool"}, CodeInvalidArgs},
		{"range", with(CapSetTemperatureRange, CapabilitySpec{Min: f64(7), Max: f64(35)}), map[string]any{"low": 18.0, "high": 22.0}, ""},
		{"range inverted", with(CapSetTemperatureRange, CapabilitySpec{Min: f64(7), Max: f64(35)}), map[string]any{"low": 22.0, "high": 18.0}, CodeInvalidArgs},
		{"bool", with(CapOscillate, CapabilitySpec{}), map[string]any{"on": true}, ""},
		{"bool text", with(CapOscillate, CapabilitySpec{}), map[string]any{"on": "yes"}, CodeInvalidArgs},
		{"nan", with(CapSetValue, CapabilitySpec{}), map[string]any{"value": math.NaN()}, CodeInvalidArgs},
		{"read capability", with(CapStream, CapabilitySpec{}), map[string]any{}, CodeUnsupportedCommand},
	} {
		err := ValidateArgs(tc.spec, tc.args)
		var he *Error
		got := ""
		if errors.As(err, &he) {
			got = he.Code
		} else if err != nil {
			got = "untyped"
		}
		if got != tc.code {
			t.Errorf("%s: code %q, want %q (%v)", tc.name, got, tc.code, err)
		}
	}
}

func TestBoundAttributesDropsSensitiveKeys(t *testing.T) {
	out := BoundAttributes(map[string]any{
		"access_token":   "abc",
		"entity_picture": "/local/x.png",
		"stream_url":     "x",
		"latitude":       1.5,
		"longitude":      2.5,
		"gps_accuracy":   10.0,
		"api_secret":     "s",
		"password":       "p",
		"brightness":     128.0,
		"friendly_name":  "Lamp",
	})
	for _, k := range []string{"access_token", "entity_picture", "stream_url", "latitude", "longitude", "gps_accuracy", "api_secret", "password"} {
		if _, ok := out[k]; ok {
			t.Errorf("sensitive key %q kept", k)
		}
	}
	for _, k := range []string{"brightness", "friendly_name"} {
		if _, ok := out[k]; !ok {
			t.Errorf("harmless key %q dropped", k)
		}
	}
}

func TestAllowedSetNilDeniesEverything(t *testing.T) {
	var a *AllowedSet
	id := EntityID("light.kitchen")
	if a.Has(id) {
		t.Error("nil AllowedSet.Has must be false")
	}
	if a.Allows(id, CapToggle) {
		t.Error("nil AllowedSet.Allows must be false")
	}
}

func TestSanitizeBoundsTextAndAttributes(t *testing.T) {
	for _, tc := range []struct {
		in   string
		max  int
		want string
	}{
		{"  Kitchen\nLamp\x00 ", 120, "Kitchen Lamp"},
		{"a" + string(rune(0x202e)) + "b", 10, "ab"},
		{"äöüß", 2, "äö"},
		{"\xffok", 10, "ok"},
	} {
		if got := CleanText(tc.in, tc.max); got != tc.want {
			t.Errorf("CleanText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if !ContainsControl("a\x07") || ContainsControl("plain") || !ContainsURL("see https://x") || !ContainsURL("javascript:alert(1)") || ContainsURL("10:30") {
		t.Error("ContainsControl/ContainsURL")
	}
	list := make([]any, 100)
	for i := range list {
		list[i] = float64(i)
	}
	in := map[string]any{"Bad-Key": 1.0, "ok": "x", "nested": map[string]any{"a": 1.0}, "list": list,
		"nan": math.NaN(), "long": strings.Repeat("z", 9000), "n": 3}
	out := BoundAttributes(in)
	if _, ok := out["Bad-Key"]; ok || out["ok"] != "x" || out["n"] != 3.0 {
		t.Fatalf("keys: %v", out)
	}
	if _, ok := out["nested"]; ok {
		t.Fatal("maps must be dropped")
	}
	if _, ok := out["nan"]; ok {
		t.Fatal("NaN kept")
	}
	if len(out["list"].([]any)) != 64 || len([]rune(out["long"].(string))) != 256 {
		t.Fatal("list or string not bounded")
	}
	big := map[string]any{}
	for i := 0; i < 200; i++ {
		big[strings.Repeat("k", 3)+string(rune('a'+i%26))+strings.Repeat("x", i%10)] = strings.Repeat("v", 200)
	}
	if raw, _ := jsonSize(BoundAttributes(big)); raw > MaxAttrBytes {
		t.Fatalf("attributes %d bytes > %d", raw, MaxAttrBytes)
	}
}
