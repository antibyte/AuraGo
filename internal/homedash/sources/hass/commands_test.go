package hass

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass/hasstest"
	"aurago/internal/homedash/sourcetest"
	"aurago/internal/tools"
)

func TestHAVerbMapping(t *testing.T) {
	for _, tc := range []struct {
		native          string
		c               homedash.Capability
		args            map[string]any
		domain, service string
		data            map[string]any
	}{
		{"switch.coffee", homedash.CapToggle, nil, "switch", "toggle", map[string]any{}},
		{"input_boolean.vacation_mode", homedash.CapTurnOn, nil, "input_boolean", "turn_on", map[string]any{}},
		{"light.kitchen", homedash.CapTurnOff, nil, "light", "turn_off", map[string]any{}},
		{"light.kitchen", homedash.CapBrightness, map[string]any{"value": 40.0}, "light", "turn_on", map[string]any{"brightness_pct": 40.0}},
		{"light.kitchen", homedash.CapColorTemp, map[string]any{"kelvin": 3502.0}, "light", "turn_on", map[string]any{"color_temp_kelvin": 3502.0}},
		{"light.kitchen", homedash.CapColor, map[string]any{"h": 200.0, "s": 80.0}, "light", "turn_on", map[string]any{"hs_color": []any{200.0, 80.0}}},
		{"light.kitchen", homedash.CapEffect, map[string]any{"option": "colorloop"}, "light", "turn_on", map[string]any{"effect": "colorloop"}},
		{"scene.movie", homedash.CapActivate, nil, "scene", "turn_on", map[string]any{}},
		{"script.goodnight", homedash.CapActivate, nil, "script", "turn_on", map[string]any{}},
		{"button.doorbell_test", homedash.CapPress, nil, "button", "press", map[string]any{}},
		{"input_button.bell", homedash.CapPress, nil, "input_button", "press", map[string]any{}},
		{"select.heater_mode", homedash.CapSelectOption, map[string]any{"option": "boost"}, "select", "select_option", map[string]any{"option": "boost"}},
		{"input_select.radio", homedash.CapSelectOption, map[string]any{"option": "Rock"}, "input_select", "select_option", map[string]any{"option": "Rock"}},
		{"number.boiler_target", homedash.CapSetValue, map[string]any{"value": 60.0}, "number", "set_value", map[string]any{"value": 60.0}},
		{"input_number.volume", homedash.CapSetValue, map[string]any{"value": 40.0}, "input_number", "set_value", map[string]any{"value": 40.0}},
		{"climate.hall", homedash.CapSetTemperature, map[string]any{"value": 21.5}, "climate", "set_temperature", map[string]any{"temperature": 21.5}},
		{"cover.garage", homedash.CapSetPosition, map[string]any{"value": 30.0}, "cover", "set_cover_position", map[string]any{"position": 30.0}},
		{"media_player.tv", homedash.CapVolumeSet, map[string]any{"value": 25.0}, "media_player", "volume_set", map[string]any{"volume_level": 0.25}},
	} {
		call, ok := mapCommand(tc.native, tc.c, tc.args)
		if !ok || call.Domain != tc.domain || call.Service != tc.service || !reflect.DeepEqual(call.Data, tc.data) {
			t.Errorf("%s %s = %+v %v, want %s.%s %v", tc.native, tc.c, call, ok, tc.domain, tc.service, tc.data)
		}
	}
	for _, tc := range []struct {
		native string
		c      homedash.Capability
		args   map[string]any
	}{
		{"light.kitchen", homedash.CapPress, nil},
		{"sensor.outdoor_temperature", homedash.CapToggle, nil},
		{"Light.kitchen", homedash.CapToggle, nil},
		{"alarm_control_panel.home", homedash.CapToggle, nil},
		{"light.kitchen", homedash.CapBrightness, map[string]any{"value": "40"}},
		{"select.heater_mode", homedash.CapSelectOption, map[string]any{}},
	} {
		if call, ok := mapCommand(tc.native, tc.c, tc.args); ok {
			t.Errorf("%s %s mapped to %+v", tc.native, tc.c, call)
		}
	}
}

func TestHAExpectedState(t *testing.T) {
	now := time.Date(2026, 11, 25, 10, 0, 1, 0, time.UTC)
	on, off := HAState{State: "on"}, HAState{State: "off"}
	type want struct {
		value any
		text  string
		attrs map[string]any
	}
	for _, tc := range []struct {
		name   string
		domain string
		c      homedash.Capability
		args   map[string]any
		cur    HAState
		have   bool
		want   *want
	}{
		{"toggle on", "switch", homedash.CapToggle, nil, on, true, &want{false, "off", nil}},
		{"toggle off", "light", homedash.CapToggle, nil, off, true, &want{true, "on", nil}},
		{"toggle unknown", "switch", homedash.CapToggle, nil, HAState{State: "unavailable"}, true, nil},
		{"toggle without state", "switch", homedash.CapToggle, nil, HAState{}, false, nil},
		{"turn_on", "input_boolean", homedash.CapTurnOn, nil, off, true, &want{true, "on", nil}},
		{"turn_off", "light", homedash.CapTurnOff, nil, on, true, &want{false, "off", nil}},
		{"brightness", "light", homedash.CapBrightness, map[string]any{"value": 40.0}, off, true, &want{true, "on", map[string]any{"brightness_pct": 40.0}}},
		{"brightness 0", "light", homedash.CapBrightness, map[string]any{"value": 0.0}, on, true, &want{false, "off", nil}},
		{"color_temp", "light", homedash.CapColorTemp, map[string]any{"kelvin": 3502.0}, on, true, &want{true, "on", map[string]any{"color_temp_kelvin": 3502.0}}},
		{"color", "light", homedash.CapColor, map[string]any{"h": 200.0, "s": 80.0}, on, true, &want{true, "on", map[string]any{"hs_color": []any{200.0, 80.0}}}},
		{"effect", "light", homedash.CapEffect, map[string]any{"option": "colorloop"}, on, true, &want{true, "on", map[string]any{"effect": "colorloop"}}},
		{"select", "select", homedash.CapSelectOption, map[string]any{"option": "boost"}, HAState{State: "eco"}, true, &want{"boost", "boost", nil}},
		{"set_value", "number", homedash.CapSetValue, map[string]any{"value": 60.0}, HAState{State: "55.0"}, true, &want{60.0, "60", nil}},
		{"activate", "scene", homedash.CapActivate, nil, HAState{}, true, nil},
		{"press", "button", homedash.CapPress, nil, HAState{}, true, nil},
		{"non-W1", "climate", homedash.CapSetTemperature, map[string]any{"value": 21.0}, HAState{State: "heat"}, true, nil},
	} {
		got := expectedState(tc.domain, tc.c, tc.args, tc.cur, tc.have, now)
		if tc.want == nil {
			if got != nil {
				t.Errorf("%s: expected %+v, want none", tc.name, got)
			}
			continue
		}
		if got == nil || got.Value != tc.want.value || got.Text != tc.want.text || !reflect.DeepEqual(got.Attributes, tc.want.attrs) ||
			!got.Available || !got.UpdatedAt.Equal(now) {
			t.Errorf("%s: expected %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestHACommandFeedsChangedStates(t *testing.T) {
	o := hasstest.DemoOptions()
	o.NoEvents = true // only the service answer can carry the new state
	ha := hasstest.New(t, o)
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"light.kitchen"})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusLive, "") }, "live")
	res, err := a.Command(context.Background(), homedash.Command{Entity: "ha:light.kitchen", Native: "light.kitchen", Name: homedash.CapBrightness, Args: map[string]any{"value": 60.0}})
	if err != nil || res.Expected == nil || res.Expected.Value != true || res.Expected.Attributes["brightness_pct"] != 60.0 {
		t.Fatalf("command = %+v %v", res, err)
	}
	calls := ha.Calls()
	if len(calls) != 1 || calls[0].Domain != "light" || calls[0].Service != "turn_on" || calls[0].Data["entity_id"] != "light.kitchen" || calls[0].Data["brightness_pct"] != 60.0 {
		t.Fatalf("calls = %+v", calls)
	}
	if st, ok := sink.last("light.kitchen"); !ok || st.Attributes["brightness_pct"] != 60.0 || st.Value != true {
		t.Fatalf("fed state = %+v %v", st, ok)
	}
}

func TestHACommandsHonourReadonlyAndServicePolicy(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	cmd := func(native string, c homedash.Capability) homedash.Command {
		return homedash.Command{Entity: homedash.MakeEntityID(SourceID, native), Native: native, Name: c, Args: map[string]any{}, Confirmed: true}
	}
	for _, tc := range []struct {
		cfg       tools.HAConfig
		native    string
		c         homedash.Capability
		blockedBy string
	}{
		{tools.HAConfig{ReadOnly: true}, "switch.coffee", homedash.CapToggle, "home_assistant.readonly"},
		{tools.HAConfig{BlockedServices: []string{"switch.toggle"}}, "switch.coffee", homedash.CapToggle, "home_assistant.blocked_services"},
		{tools.HAConfig{}, "script.goodnight", homedash.CapActivate, "home_assistant.allowed_services"},
		{tools.HAConfig{AllowedServices: []string{"light.turn_on"}}, "switch.coffee", homedash.CapToggle, "home_assistant.allowed_services"},
	} {
		a := newTestAdapter(t, ha, tc.cfg)
		_, err := a.Command(context.Background(), cmd(tc.native, tc.c))
		var he *homedash.Error
		if !errors.As(err, &he) || he.Code != homedash.CodeReadOnly || he.BlockedBy != tc.blockedBy {
			t.Errorf("%s %s with %+v = %v", tc.native, tc.c, tc.cfg, err)
		}
	}
	if n := len(ha.Calls()); n != 0 {
		t.Fatalf("%d calls reached Home Assistant", n)
	}
	a := newTestAdapter(t, ha, tools.HAConfig{AllowedServices: []string{"script.turn_on"}})
	if _, err := a.Command(context.Background(), cmd("script.goodnight", homedash.CapActivate)); err != nil {
		t.Fatalf("allowlisted script = %v", err)
	}
	if c := ha.Calls(); len(c) != 1 || c[0].Domain != "script" || c[0].Service != "turn_on" {
		t.Fatalf("calls = %+v", c)
	}
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
func (l *lockedBuffer) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

func TestHACommandErrorsNeverCarryUpstreamText(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	logs := &lockedBuffer{}
	a := New(tools.HAConfig{URL: ha.URL, AccessToken: ha.Token}, 5, FactoryOptions{Logger: slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})), WS: fastWS})
	cmd := homedash.Command{Entity: "ha:switch.coffee", Native: "switch.coffee", Name: homedash.CapToggle, Args: map[string]any{}}
	for _, tc := range []struct {
		status int
		code   string
	}{{400, homedash.CodeCommandFailed}, {404, homedash.CodeCommandFailed}, {500, homedash.CodeCommandFailed},
		{502, homedash.CodeSourceUnavailable}, {503, homedash.CodeSourceUnavailable}, {504, homedash.CodeSourceUnavailable}} {
		ha.FailServices(tc.status, hasstest.UpstreamSecret+" "+ha.Token)
		_, err := a.Command(context.Background(), cmd)
		var he *homedash.Error
		if !errors.As(err, &he) || he.Code != tc.code || strings.Contains(err.Error(), hasstest.UpstreamSecret) {
			t.Fatalf("HTTP %d = %v", tc.status, err)
		}
		if tc.code == homedash.CodeSourceUnavailable && he.Source != string(SourceID) {
			t.Fatalf("HTTP %d: source = %q", tc.status, he.Source)
		}
	}
	ha.FailServices(0, "")
	if out := logs.String(); !strings.Contains(out, hasstest.UpstreamSecret) || strings.Contains(out, ha.Token) {
		t.Fatalf("log must keep the scrubbed excerpt and never the token:\n%s", out)
	}
	bad := New(tools.HAConfig{URL: ha.URL, AccessToken: "wrong-token"}, 5, FactoryOptions{Logger: discardLog, WS: fastWS})
	var he *homedash.Error
	if _, err := bad.Command(context.Background(), cmd); !errors.As(err, &he) || he.Code != homedash.CodeSourceUnavailable {
		t.Fatalf("wrong token = %v", err)
	}
	ha.Close()
	if _, err := a.Command(context.Background(), cmd); !errors.As(err, &he) || he.Code != homedash.CodeSourceUnavailable || he.Source != "ha" {
		t.Fatalf("Home Assistant gone = %v", err)
	}
	if _, err := a.Command(context.Background(), homedash.Command{Entity: "ha:light.kitchen", Native: "light.kitchen", Name: homedash.CapPress}); !errors.As(err, &he) || he.Code != homedash.CodeUnsupportedCommand {
		t.Fatalf("unmapped = %v", err)
	}
}

// sampleCommandArgs gives valid arguments (and the service data they map to) for every command
// capability. A capability of the domain table without a row here fails TestHAEveryDomainTableCapabilityIsMapped,
// so a new capability cannot ship without a mapping and a test.
var sampleCommandArgs = map[homedash.Capability]struct {
	args map[string]any
	data map[string]any
}{
	homedash.CapToggle:              {nil, map[string]any{}},
	homedash.CapTurnOn:              {nil, map[string]any{}},
	homedash.CapTurnOff:             {nil, map[string]any{}},
	homedash.CapBrightness:          {map[string]any{"value": 40.0}, map[string]any{"brightness_pct": 40.0}},
	homedash.CapColorTemp:           {map[string]any{"kelvin": 3000.0}, map[string]any{"color_temp_kelvin": 3000.0}},
	homedash.CapColor:               {map[string]any{"h": 10.0, "s": 20.0}, map[string]any{"hs_color": []any{10.0, 20.0}}},
	homedash.CapEffect:              {map[string]any{"option": "x"}, map[string]any{"effect": "x"}},
	homedash.CapSetTemperature:      {map[string]any{"value": 21.5}, map[string]any{"temperature": 21.5}},
	homedash.CapSetTemperatureRange: {map[string]any{"low": 18.0, "high": 24.0}, map[string]any{"target_temp_low": 18.0, "target_temp_high": 24.0}},
	homedash.CapHVACMode:            {map[string]any{"option": "heat"}, map[string]any{"hvac_mode": "heat"}},
	homedash.CapPresetMode:          {map[string]any{"option": "eco"}, map[string]any{"preset_mode": "eco"}},
	homedash.CapFanMode:             {map[string]any{"option": "auto"}, map[string]any{"fan_mode": "auto"}},
	homedash.CapOpen:                {nil, map[string]any{}},
	homedash.CapClose:               {nil, map[string]any{}},
	homedash.CapStop:                {nil, map[string]any{}},
	homedash.CapSetPosition:         {map[string]any{"value": 30.0}, map[string]any{"position": 30.0}},
	homedash.CapSetTilt:             {map[string]any{"value": 20.0}, map[string]any{"tilt_position": 20.0}},
	homedash.CapLock:                {nil, map[string]any{}},
	homedash.CapUnlock:              {nil, map[string]any{}},
	homedash.CapSetPercentage:       {map[string]any{"value": 50.0}, map[string]any{"percentage": 50.0}},
	homedash.CapOscillate:           {map[string]any{"on": true}, map[string]any{"oscillating": true}},
	homedash.CapPlayPause:           {nil, map[string]any{}},
	homedash.CapNext:                {nil, map[string]any{}},
	homedash.CapPrevious:            {nil, map[string]any{}},
	homedash.CapVolumeSet:           {map[string]any{"value": 25.0}, map[string]any{"volume_level": 0.25}},
	homedash.CapVolumeMute:          {map[string]any{"on": true}, map[string]any{"is_volume_muted": true}},
	homedash.CapSourceSelect:        {map[string]any{"option": "HDMI"}, map[string]any{"source": "HDMI"}},
	homedash.CapPress:               {nil, map[string]any{}},
	homedash.CapActivate:            {nil, map[string]any{}},
	homedash.CapSetValue:            {map[string]any{"value": 60.0}, map[string]any{"value": 60.0}},
	homedash.CapSelectOption:        {map[string]any{"option": "a"}, map[string]any{"option": "a"}},
}

// TestHAEveryDomainTableCapabilityIsMapped walks domainTable itself: every command capability of
// every domain must map to exactly the service the table names (Task 7, lead answer Q7).
func TestHAEveryDomainTableCapabilityIsMapped(t *testing.T) {
	rows := 0
	for domain, spec := range domainTable {
		for _, c := range spec.Commands {
			rows++
			sample, ok := sampleCommandArgs[c.Cap]
			if !ok {
				t.Errorf("%s %s: capability has no sample row in this test (add a mapping row)", domain, c.Cap)
				continue
			}
			native := domain + ".sample"
			call, ok := mapCommand(native, c.Cap, sample.args)
			if !ok || call.Domain != domain || call.Service != c.Service || !reflect.DeepEqual(call.Data, sample.data) {
				t.Errorf("%s %s = %+v %v, want %s.%s %v", native, c.Cap, call, ok, domain, c.Service, sample.data)
			}
		}
	}
	if rows < 40 {
		t.Fatalf("only %d domain table rows walked", rows)
	}
}

func TestHAMapCommandRejectsBadArguments(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	for _, tc := range []struct {
		native string
		c      homedash.Capability
		args   map[string]any
	}{
		{"light.kitchen", homedash.CapBrightness, map[string]any{"value": nan}},
		{"light.kitchen", homedash.CapBrightness, map[string]any{"value": inf}},
		{"light.kitchen", homedash.CapBrightness, map[string]any{"value": 150.0}},
		{"light.kitchen", homedash.CapColor, map[string]any{"h": 400.0, "s": 10.0}},
		{"light.kitchen", homedash.CapColor, map[string]any{"h": 10.0, "s": nan}},
		{"light.kitchen", homedash.CapEffect, map[string]any{"option": 3.0}},
		{"number.boiler_target", homedash.CapSetValue, map[string]any{"value": inf}},
		{"climate.hall", homedash.CapSetTemperatureRange, map[string]any{"low": 18.0}},
		{"fan.bedroom", homedash.CapOscillate, map[string]any{"on": "yes"}},
		{"media_player.tv", homedash.CapVolumeSet, map[string]any{"value": -1.0}},
		{"cover.garage", homedash.CapSetPosition, map[string]any{"value": 101.0}},
	} {
		if call, ok := mapCommand(tc.native, tc.c, tc.args); ok {
			t.Errorf("%s %s %v mapped to %+v", tc.native, tc.c, tc.args, call)
		}
	}
}

// The service answer carries Home Assistant's raw attributes. They must pass the same attribute
// filter as websocket events before the sink sees them (security requirement D12).
func TestHACommandFilteredChangedStateAttributes(t *testing.T) {
	o := hasstest.DemoOptions()
	o.NoEvents = true
	o.States[0].Attributes["access_token"] = "leak-token-1"
	ha := hasstest.New(t, o)
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"light.kitchen"})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusLive, "") }, "live")
	if _, err := a.Command(context.Background(), homedash.Command{Entity: "ha:light.kitchen", Native: "light.kitchen", Name: homedash.CapBrightness, Args: map[string]any{"value": 60.0}}); err != nil {
		t.Fatal(err)
	}
	st, ok := sink.last("light.kitchen")
	if !ok || st.Attributes["brightness_pct"] != 60.0 {
		t.Fatalf("fed state = %+v %v", st, ok)
	}
	for _, k := range []string{"entity_picture", "access_token", "icon"} {
		if _, bad := st.Attributes[k]; bad {
			t.Errorf("attribute %q reached the sink", k)
		}
	}
}

func TestHACommandChecksTargetAndOptions(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"light.kitchen"})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusLive, "") }, "live")
	var he *homedash.Error
	_, err := a.Command(context.Background(), homedash.Command{Entity: "ha:light.ghost", Native: "light.ghost", Name: homedash.CapToggle, Args: map[string]any{}})
	if !errors.As(err, &he) || he.Code != homedash.CodeEntityNotFound {
		t.Fatalf("entity outside the catalog = %v", err)
	}
	_, err = a.Command(context.Background(), homedash.Command{Entity: "ha:light.kitchen", Native: "light.kitchen", Name: homedash.CapEffect, Args: map[string]any{"option": "evil"}})
	if !errors.As(err, &he) || he.Code != homedash.CodeInvalidArgs {
		t.Fatalf("option outside the entity list = %v", err)
	}
	_, err = a.Command(context.Background(), homedash.Command{Entity: "ha:light.kitchen", Native: "light.kitchen\nlight.sofa", Name: homedash.CapToggle, Args: map[string]any{}})
	if !errors.As(err, &he) || he.Code != homedash.CodeUnsupportedCommand {
		t.Fatalf("malformed native = %v", err)
	}
	if n := len(ha.Calls()); n != 0 {
		t.Fatalf("%d calls reached Home Assistant", n)
	}
}

// Amendment X1: a Home Assistant that does not answer ends at the caller's deadline. The adapter
// returns the context error, not a *homedash.Error; the hub turns that into 502 command_failed.
func TestHACommandDeadlineReturnsContextError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body) // reading the body lets the server notice the closed connection
		<-r.Context().Done()               // blocks until the request ends
	}))
	defer srv.Close()
	a := New(tools.HAConfig{URL: srv.URL, AccessToken: "tok"}, 5, FactoryOptions{Logger: discardLog, WS: fastWS})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := a.Command(ctx, homedash.Command{Entity: "ha:switch.coffee", Native: "switch.coffee", Name: homedash.CapToggle, Args: map[string]any{}})
	var he *homedash.Error
	if !errors.Is(err, context.DeadlineExceeded) || errors.As(err, &he) {
		t.Fatalf("err = %v (%T)", err, err)
	}
}
