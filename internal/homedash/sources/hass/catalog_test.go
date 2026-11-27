package hass

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass/hasstest"
	"aurago/internal/homedash/sourcetest"
	"aurago/internal/tools"
)

var fastWS = WSOptions{HandshakeTimeout: time.Second, PingInterval: time.Hour, PongTimeout: time.Minute, AuthRetry: time.Hour,
	RegistryDebounce: 20 * time.Millisecond, Backoff: []time.Duration{20 * time.Millisecond}, Jitter: -1}

var discardLog = slog.New(slog.NewTextHandler(io.Discard, nil))

func demoStates(o hasstest.Options) map[string]HAState {
	out := map[string]HAState{}
	for _, s := range o.States {
		out[s.EntityID] = HAState{EntityID: s.EntityID, State: s.State, Attributes: s.Attributes, LastChanged: s.LastChanged, LastUpdated: s.LastUpdated}
	}
	return out
}

func demoRegistries(o hasstest.Options) Registries {
	var r Registries
	for _, a := range o.Areas {
		r.Areas = append(r.Areas, AreaEntry{AreaID: a.AreaID, Name: a.Name})
	}
	for _, d := range o.Devices {
		r.Devices = append(r.Devices, DeviceEntry{ID: d.ID, AreaID: d.AreaID})
	}
	for _, e := range o.Entities {
		r.Entities = append(r.Entities, EntityEntry{EntityID: e.EntityID, DeviceID: e.DeviceID, AreaID: e.AreaID, HiddenBy: e.HiddenBy, DisabledBy: e.DisabledBy})
	}
	return r
}

func demoCatalog(cfg tools.HAConfig) map[homedash.EntityID]homedash.Entity {
	o := hasstest.DemoOptions()
	out := map[homedash.EntityID]homedash.Entity{}
	for _, e := range buildCatalog(cfg, demoStates(o), indexRegistries(demoRegistries(o))) {
		out[e.ID] = e
	}
	return out
}

func TestHAAreasFromEntityThenDeviceRegistry(t *testing.T) {
	c := demoCatalog(tools.HAConfig{})
	for id, want := range map[homedash.EntityID][2]string{
		"ha:light.kitchen":        {"kitchen", "Kitchen"},         // device area
		"ha:select.heater_mode":   {"kitchen", "Kitchen"},         // entity area wins over its device's living_room
		"ha:light.sofa":           {"living_room", "Living room"}, // device area
		"ha:button.doorbell_test": {"", ""},                       // no registry entry
		"ha:input_number.volume":  {"living_room", "Living room"},
	} {
		if e := c[id]; e.AreaID != want[0] || e.Area != want[1] {
			t.Errorf("%s area = %q/%q, want %q/%q", id, e.AreaID, e.Area, want[0], want[1])
		}
	}
}

func TestHACatalogExcludesHiddenDisabledAndAlarm(t *testing.T) {
	c := demoCatalog(tools.HAConfig{})
	if len(c) != 14 {
		t.Fatalf("catalog has %d entities, want 14 (16 states - hidden guest_mode - alarm panel)", len(c))
	}
	for _, id := range []homedash.EntityID{"ha:input_boolean.guest_mode", "ha:alarm_control_panel.home"} {
		if _, ok := c[id]; ok {
			t.Errorf("%s must be excluded", id)
		}
	}
	reg := Registries{Entities: []EntityEntry{{EntityID: "switch.a", DisabledBy: "user"}}}
	list := buildCatalog(tools.HAConfig{}, map[string]HAState{"switch.a": {EntityID: "switch.a", State: "on"}, "Bad.id": {EntityID: "Bad.id"}}, indexRegistries(reg))
	if len(list) != 0 {
		t.Fatalf("disabled or invalid entities listed: %+v", list)
	}
}

func TestHACatalogCapsAtTwoThousandInEntityIDOrder(t *testing.T) {
	states := map[string]HAState{}
	for i := 0; i < homedash.MaxCatalogEntities+25; i++ {
		id := fmt.Sprintf("sensor.s%04d", i)
		states[id] = HAState{EntityID: id, State: "1"}
	}
	list := buildCatalog(tools.HAConfig{}, states, indexRegistries(Registries{}))
	if len(list) != homedash.MaxCatalogEntities {
		t.Fatalf("catalog has %d entities, want %d", len(list), homedash.MaxCatalogEntities)
	}
	if list[0].ID != "ha:sensor.s0000" || list[len(list)-1].ID != homedash.EntityID(fmt.Sprintf("ha:sensor.s%04d", homedash.MaxCatalogEntities-1)) {
		t.Fatalf("not cut in entity-id order: first %s last %s", list[0].ID, list[len(list)-1].ID)
	}
}

func TestHAStateMappingAndAttributes(t *testing.T) {
	c := demoCatalog(tools.HAConfig{})
	k := c["ha:light.kitchen"]
	if k.Kind != homedash.KindLight || k.Name != "Kitchen" || k.IconHint != "bulb" || k.State.Value != true || k.State.Text != "on" ||
		k.State.Attributes["brightness_pct"] != 40.0 || !k.State.UpdatedAt.Equal(hasstest.DemoTime) || k.State.UpdatedAt.Location() != time.UTC {
		t.Fatalf("kitchen = %+v", k)
	}
	for _, key := range []string{"icon", "entity_picture", "supported_features", "friendly_name", "brightness"} {
		if _, ok := k.State.Attributes[key]; ok {
			t.Errorf("attribute %s forwarded", key)
		}
	}
	if got := capNames(k.Capabilities); !slices.Equal(got, []string{"toggle", "turn_on", "turn_off", "brightness", "color_temp", "color", "effect"}) {
		t.Fatalf("kitchen caps = %v", got)
	}
	if ct, _ := findCap(k.Capabilities, homedash.CapColorTemp); *ct.Min != 2202 || *ct.Max != 6535 || *ct.Step != 50 {
		t.Fatalf("color_temp = %+v", ct)
	}
	if s := c["ha:light.sofa"]; s.State.Value != false || !slices.Equal(capNames(s.Capabilities), []string{"toggle", "turn_on", "turn_off"}) {
		t.Fatalf("sofa = %+v", s)
	}
	if s := c["ha:switch.coffee"]; s.IconHint != "outlet" || s.DeviceClass != "outlet" || s.State.Value != false {
		t.Fatalf("coffee = %+v", s)
	}
	if b := c["ha:button.doorbell_test"]; b.State.Value != nil || b.State.Text != "unknown" || !b.State.Available || b.IconHint != "button" {
		t.Fatalf("button = %+v", b)
	}
	if n := c["ha:number.boiler_target"]; n.State.Value != 55.0 || n.Unit != "°C" || n.IconHint != "number" {
		t.Fatalf("number = %+v", n)
	}
	if s := c["ha:select.heater_mode"]; s.State.Value != "comfort" || s.IconHint != "list" {
		t.Fatalf("select = %+v", s)
	}
	if s := c["ha:script.goodnight"]; !s.ReadOnly || s.IconHint != "script" {
		t.Fatalf("script = %+v", s)
	}
	if s := c["ha:sensor.outdoor_temperature"]; s.IconHint != "thermometer" || s.State.Value != 4.5 {
		t.Fatalf("sensor = %+v", s)
	}
	cam := c["ha:camera.door"]
	if cam.Media == nil || !cam.Media.Snapshot || cam.State.Attributes["access_token"] != nil || cam.State.Attributes["entity_picture"] != nil {
		t.Fatalf("camera = %+v", cam)
	}
	if st := mapState("switch", HAState{EntityID: "switch.x", State: "unavailable"}); st.Available || st.Value != nil {
		t.Fatalf("unavailable = %+v", st)
	}
	if st := mapState("switch", HAState{EntityID: "switch.x", State: "unknown"}); !st.Available || st.Value != nil {
		t.Fatalf("unknown = %+v", st)
	}
}

func TestHANeverForwardsSensitiveAttributes(t *testing.T) {
	long := strings.Repeat("x", 600)
	items := make([]any, 100)
	for i := range items {
		items[i] = "v"
	}
	attrs := map[string]any{
		"icon": "mdi:x", "entity_picture": "/api/x?token=1", "access_token": "t", "latitude": 1.0, "longitude": 2.0,
		"gps_accuracy": 3.0, "stream_url": "http://h", "api_token": "t", "wifi_password": "p", "client_secret": "s", "gps_fix": "x",
		"friendly_name": "Thing", "options": items, "mode": long, "device_class": "door",
	}
	e, ok := entityFor(tools.HAConfig{}, HAState{EntityID: "binary_sensor.x", State: "on", Attributes: attrs}, indexRegistries(Registries{}))
	if !ok {
		t.Fatal("entity not mapped")
	}
	for k := range e.State.Attributes {
		for _, bad := range []string{"icon", "picture", "token", "url", "password", "secret", "gps", "latitude", "longitude"} {
			if strings.Contains(k, bad) {
				t.Errorf("attribute %q forwarded", k)
			}
		}
	}
	if s, _ := e.State.Attributes["mode"].(string); len([]rune(s)) > homedash.MaxAttrStringRunes {
		t.Errorf("string attribute not bounded: %d runes", len([]rune(s)))
	}
	if l, _ := e.State.Attributes["options"].([]any); len(l) > homedash.MaxAttrListItems {
		t.Errorf("list attribute not bounded: %d items", len(l))
	}
	if raw, _ := json.Marshal(e.State.Attributes); len(raw) > homedash.MaxAttrBytes {
		t.Errorf("attributes are %d bytes", len(raw))
	}
}

func TestHAIconHintsAreOnTheFixedList(t *testing.T) {
	domains := []string{"alarm_control_panel", "automation", "timer"}
	for d := range domainTable {
		domains = append(domains, d)
	}
	for _, d := range domains {
		for _, dc := range []string{"", "outlet", "temperature", "humidity", "moisture", "door"} {
			for _, numeric := range []bool{false, true} {
				if h := iconHint(d, dc, numeric); h != "" && !homedash.KnownIcon(h) {
					t.Errorf("iconHint(%s, %s, %v) = %q is not on the contract list", d, dc, numeric, h)
				}
			}
		}
	}
}

// contractExamples are the W1 examples of contract section 2, copied literally.
var contractExamples = map[string]string{
	"light.kitchen": `{"id":"ha:light.kitchen","source":"ha","kind":"light","name":"Kitchen","area":"Kitchen","area_id":"kitchen","icon_hint":"bulb",
 "capabilities":[{"name":"toggle","allowed":true},{"name":"turn_on","allowed":true},{"name":"turn_off","allowed":true},
  {"name":"brightness","allowed":true,"min":0,"max":100,"step":1},
  {"name":"color_temp","allowed":true,"min":2202,"max":6535,"step":50},
  {"name":"color","allowed":true},
  {"name":"effect","allowed":true,"options":["none","colorloop"]}],
 "readonly":false,
 "state":{"value":true,"text":"on","attributes":{"brightness_pct":40,"color_mode":"color_temp","color_temp_kelvin":3003,"effect":"none","effect_list":["none","colorloop"],"hs_color":[27.2,56.1],"max_color_temp_kelvin":6535,"min_color_temp_kelvin":2202,"supported_color_modes":["color_temp","hs"]},"available":true,"updated_at":"2026-11-25T09:00:00Z"}}`,
	"light.sofa": `{"id":"ha:light.sofa","source":"ha","kind":"light","name":"Sofa lamp","area":"Living room","area_id":"living_room","icon_hint":"bulb",
 "capabilities":[{"name":"toggle","allowed":true},{"name":"turn_on","allowed":true},{"name":"turn_off","allowed":true}],
 "readonly":false,
 "state":{"value":false,"text":"off","attributes":{"color_mode":null,"supported_color_modes":["onoff"]},"available":true,"updated_at":"2026-11-25T09:00:00Z"}}`,
	"switch.coffee": `{"id":"ha:switch.coffee","source":"ha","kind":"switch","name":"Coffee machine","area":"Kitchen","area_id":"kitchen","device_class":"outlet","icon_hint":"outlet",
 "capabilities":[{"name":"toggle","allowed":true},{"name":"turn_on","allowed":true},{"name":"turn_off","allowed":true},{"name":"history","allowed":true}],
 "readonly":false,
 "state":{"value":false,"text":"off","attributes":{"device_class":"outlet"},"available":true,"updated_at":"2026-11-25T09:00:00Z"}}`,
	"input_boolean.vacation_mode": `{"id":"ha:input_boolean.vacation_mode","source":"ha","kind":"switch","name":"Vacation mode","icon_hint":"power",
 "capabilities":[{"name":"toggle","allowed":true},{"name":"turn_on","allowed":true},{"name":"turn_off","allowed":true},{"name":"history","allowed":true}],
 "readonly":false,
 "state":{"value":false,"text":"off","available":true,"updated_at":"2026-11-25T09:00:00Z"}}`,
	"scene.movie": `{"id":"ha:scene.movie","source":"ha","kind":"scene","name":"Movie night","area":"Living room","area_id":"living_room","icon_hint":"scene",
 "capabilities":[{"name":"activate","allowed":true}],
 "readonly":false,
 "state":{"value":"2026-11-24T20:15:00.000000+00:00","text":"2026-11-24T20:15:00.000000+00:00","available":true,"updated_at":"2026-11-25T09:00:00Z"}}`,
	"script.goodnight": `{"id":"ha:script.goodnight","source":"ha","kind":"script","name":"Good night","icon_hint":"script",
 "capabilities":[{"name":"activate","allowed":false,"blocked_by":"home_assistant.allowed_services","sensitive":true}],
 "readonly":true,"readonly_reason":"home_assistant.allowed_services",
 "state":{"value":false,"text":"off","available":true,"updated_at":"2026-11-25T09:00:00Z"}}`,
	"button.doorbell_test": `{"id":"ha:button.doorbell_test","source":"ha","kind":"button","name":"Doorbell test","icon_hint":"button",
 "capabilities":[{"name":"press","allowed":true}],
 "readonly":false,
 "state":{"value":null,"text":"unknown","available":true,"updated_at":"2026-11-25T09:00:00Z"}}`,
	"select.heater_mode": `{"id":"ha:select.heater_mode","source":"ha","kind":"select","name":"Heater mode","area":"Kitchen","area_id":"kitchen","icon_hint":"list",
 "capabilities":[{"name":"select_option","allowed":true,"options":["eco","comfort","boost"]}],
 "readonly":false,
 "state":{"value":"comfort","text":"comfort","attributes":{"options":["eco","comfort","boost"]},"available":true,"updated_at":"2026-11-25T09:00:00Z"}}`,
	"input_select.radio": `{"id":"ha:input_select.radio","source":"ha","kind":"select","name":"Radio","icon_hint":"list",
 "capabilities":[{"name":"select_option","allowed":true,"options":["Jazz","Rock","News","Classic","Pop","Lounge"]}],
 "readonly":false,
 "state":{"value":"Jazz","text":"Jazz","attributes":{"options":["Jazz","Rock","News","Classic","Pop","Lounge"]},"available":true,"updated_at":"2026-11-25T09:00:00Z"}}`,
	"number.boiler_target": `{"id":"ha:number.boiler_target","source":"ha","kind":"number","name":"Boiler target","unit":"°C","icon_hint":"number",
 "capabilities":[{"name":"set_value","allowed":true,"min":30,"max":70,"step":1},{"name":"history","allowed":true}],
 "readonly":false,
 "state":{"value":55,"text":"55.0","attributes":{"max":70,"min":30,"mode":"box","step":1,"unit_of_measurement":"°C"},"available":true,"updated_at":"2026-11-25T09:00:00Z"}}`,
	"input_number.volume": `{"id":"ha:input_number.volume","source":"ha","kind":"number","name":"Volume","area":"Living room","area_id":"living_room","icon_hint":"number",
 "capabilities":[{"name":"set_value","allowed":true,"min":0,"max":100,"step":5},{"name":"history","allowed":true}],
 "readonly":false,
 "state":{"value":35,"text":"35.0","attributes":{"max":100,"min":0,"mode":"slider","step":5},"available":true,"updated_at":"2026-11-25T09:00:00Z"}}`,
}

func asJSONMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("bad JSON %s: %v", raw, err)
	}
	return m
}

func TestHACatalogMatchesContractExamples(t *testing.T) {
	c := demoCatalog(tools.HAConfig{})
	for native, example := range contractExamples {
		e, ok := c[homedash.MakeEntityID(SourceID, native)]
		if !ok {
			t.Errorf("%s missing from the catalog", native)
			continue
		}
		got, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(asJSONMap(t, got), asJSONMap(t, []byte(example))) {
			t.Errorf("%s differs from contract section 2\n got: %s\nwant: %s", native, got, example)
		}
	}
}

func TestHAScriptBecomesAllowedAndSensitiveWithAllowedService(t *testing.T) {
	c := demoCatalog(tools.HAConfig{AllowedServices: []string{"script.turn_on"}})
	s := c["ha:script.goodnight"]
	if len(s.Capabilities) != 1 || !s.Capabilities[0].Allowed || !s.Capabilities[0].Sensitive || s.Capabilities[0].BlockedBy != "" ||
		s.ReadOnly || s.ReadOnlyReason != "" {
		t.Fatalf("script = %+v", s)
	}
}

func TestHAReadOnlyReasonIsTheFirstBlocker(t *testing.T) {
	c := demoCatalog(tools.HAConfig{ReadOnly: true})
	if s := c["ha:switch.coffee"]; !s.ReadOnly || s.ReadOnlyReason != "home_assistant.readonly" {
		t.Fatalf("coffee = %+v", s)
	}
}

func TestHAWebsocketFetchOnce(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	c, err := NewWSClient(ha.URL, ha.Token, nopHandler{}, fastWS)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	states, reg, err := c.Fetch(ctx)
	if err != nil || len(states) != 16 || len(reg.Areas) != 3 || len(reg.Devices) != 2 || len(reg.Entities) != 9 {
		t.Fatalf("fetch = %d states, %+v, %v", len(states), reg, err)
	}
	sourcetest.WaitFor(t, time.Second, func() bool { return ha.OpenWebsockets() == 0 }, "the one-shot session closes")
	bad, _ := NewWSClient(ha.URL, "wrong", nopHandler{}, fastWS)
	if _, _, err := bad.Fetch(ctx); err == nil || !errors.Is(err, errAuthInvalid) {
		t.Fatalf("wrong token = %v", err)
	}
	sourcetest.WaitFor(t, time.Second, func() bool { return ha.OpenWebsockets() == 0 }, "the rejected session closes")
}

func TestHAWebsocketFetchHonoursTheDeadline(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	c, _ := NewWSClient(ha.URL, ha.Token, nopHandler{}, fastWS)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := c.Fetch(ctx); err == nil {
		t.Fatal("cancelled context fetched")
	}
}
