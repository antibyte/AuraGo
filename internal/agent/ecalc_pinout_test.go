package agent

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
)

// The pinout operation of the ecalc tool as the agent sees it: pin and variant are no ToolCall fields (a pin number
// arrives as a number as often as a text), the dispatcher runs the operation without the desktop, and discovery finds
// the tool for connector words.

func TestECalcPinoutParametersAreNoToolCallFields(t *testing.T) {
	typ := reflect.TypeOf(ToolCall{})
	for i := 0; i < typ.NumField(); i++ {
		name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if name == "pin" || name == "variant" {
			t.Errorf("ToolCall has the field %s (json %q): ecalc would have to share its type", typ.Field(i).Name, name)
		}
	}
	if _, ok := toolCallStringFields["pin"]; ok {
		t.Error("pin must not be a coerced string field")
	}
}

func TestECalcNativeCallDecodesPinoutArguments(t *testing.T) {
	req := decodeECalcArgs(ecalcNativeCall(t, `{"operation":"pinout","id":"rj45","variant":"t568b","pin":"TX+","filter":"data","offset":2,"limit":5}`))
	if req.Operation != "pinout" || req.ID != "rj45" || req.Variant != "t568b" || req.Pin != "TX+" || req.Filter != "data" || req.Offset != 2 || req.Limit != 5 {
		t.Fatalf("request = %+v", req)
	}
	// A pin number arrives as a number (or a numeric text) and is read as typed; a call without them leaves both empty.
	for name, arguments := range map[string]string{
		"number":      `{"operation":"pinout","id":"usb_a","pin":4}`,
		"numeric":     `{"operation":"pinout","id":"usb_a","pin":"4"}`,
		"padded text": `{"operation":"pinout","id":"usb_a","pin":" 4 "}`,
	} {
		if got := decodeECalcArgs(ecalcNativeCall(t, arguments)); got.Pin != "4" || got.Variant != "" {
			t.Errorf("%s: pin %q, variant %q", name, got.Pin, got.Variant)
		}
	}
	if got := decodeECalcArgs(ecalcNativeCall(t, `{"operation":"pinout","id":"usb_a","variant":2}`)); got.Variant != "2" {
		t.Errorf("a number as variant = %q", got.Variant)
	}
	if got := decodeECalcArgs(ecalcNativeCall(t, `{"operation":"pinout","id":"usb_a"}`)); got.Pin != "" || got.Variant != "" || got.Filter != "" {
		t.Errorf("a plain pinout call = %+v", got)
	}
	// A text-mode call (a JSON object in the reply) reaches the same request.
	req = decodeECalcArgs(ParseToolCall(`{"action":"ecalc","operation":"pinout","id":"raspberry_pi_40","pin":"GND","limit":3}`))
	if req.Operation != "pinout" || req.ID != "raspberry_pi_40" || req.Pin != "GND" || req.Limit != 3 {
		t.Fatalf("text-mode request = %+v", req)
	}
}

func TestECalcDispatchRunsPinoutCallsWithoutTheDesktop(t *testing.T) {
	broker := &ecalcJSONBroker{}
	dc := &DispatchContext{Cfg: &config.Config{}, Logger: testLogger, Broker: broker}
	// The shipped catalog may have no pinouts yet: an unknown id is the tool's own error, not a failure of the dispatch.
	out, ok := dispatchPlatform(context.Background(), ecalcNativeCall(t, `{"operation":"pinout","id":"no_such_pinout","pin":4}`), dc)
	if !ok || !strings.HasPrefix(out, "Tool Output: ") || !strings.Contains(out, `"status":"error"`) || !strings.Contains(out, `"code":"unknown_id"`) || strings.Contains(out, "invalid_operation") {
		t.Fatalf("dispatch = %v %s", ok, out)
	}
	out, _ = dispatchPlatform(context.Background(), ecalcNativeCall(t, `{"operation":"pinout","id":"awg"}`), dc)
	if !strings.Contains(out, `"code":"not_a_pinout"`) {
		t.Fatalf("pinout on a table: %s", out)
	}
	if len(broker.json) != 0 {
		t.Fatalf("pinout must not broadcast: %v", broker.json)
	}
	if a := DescribeAction(ecalcNativeCall(t, `{"operation":"pinout","id":"usb_a"}`), nil); a.Mutating || a.Operation != "pinout" {
		t.Fatalf("action = %+v", a)
	}
}

func TestECalcSearchTermsFindPinouts(t *testing.T) {
	catalog := BuildToolCatalog(BuildNativeToolSchemas("", nil, ToolFeatureFlags{}, nil), nil, "")
	for _, query := range []string{"connector pinout", "USB connector pins", "Steckerbelegung", "Pinbelegung Stecker"} {
		matches := catalog.Search(query)
		found := false
		for _, entry := range matches[:min(5, len(matches))] {
			found = found || entry.Name == "ecalc"
		}
		if !found {
			t.Errorf("%q did not find ecalc in the top five", query)
		}
	}
}
