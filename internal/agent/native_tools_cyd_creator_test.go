package agent

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"aurago/internal/cydcreator"
)

func cydSchemaNames(phase string) []string {
	var names []string
	for _, t := range CYDCreatorPhaseToolSchemas(phase) {
		names = append(names, t.Function.Name)
	}
	return names
}

func TestCYDCreatorPhaseToolSchemasFollowThePhaseTable(t *testing.T) {
	for _, phase := range []string{cydcreator.PhasePlanning, cydcreator.PhaseLayout, cydcreator.PhaseCoding, cydcreator.PhaseRepairing} {
		if got, want := cydSchemaNames(phase), cydcreator.PhaseTools(phase); !slices.Equal(got, want) {
			t.Errorf("%s schemas = %v, want %v", phase, got, want)
		}
	}
	if got := CYDCreatorPhaseToolSchemas(cydcreator.PhaseBuilding); len(got) != 0 {
		t.Fatalf("building has schemas: %v", got)
	}
	for _, tool := range CYDCreatorPhaseToolSchemas(cydcreator.PhaseCoding) {
		params := tool.Function.Parameters.(map[string]interface{})
		if params["additionalProperties"] != false {
			t.Errorf("%s is not closed", tool.Function.Name)
		}
		data, _ := json.Marshal(params)
		if strings.Contains(string(data), `"job_id"`) {
			t.Errorf("%s exposes job_id", tool.Function.Name)
		}
	}
}

func TestCYDCreatorLayoutSchemaMirrorsTheDomain(t *testing.T) {
	var layout map[string]interface{}
	for _, tool := range CYDCreatorPhaseToolSchemas(cydcreator.PhaseLayout) {
		if tool.Function.Name == cydcreator.ToolUpdateLayout {
			layout = tool.Function.Parameters.(map[string]interface{})
		}
	}
	item := layout["properties"].(map[string]interface{})["ops"].(map[string]interface{})["items"].(map[string]interface{})
	props := item["properties"].(map[string]interface{})
	ops := props["op"].(map[string]interface{})["enum"].([]string)
	if len(ops) != 9 || ops[0] != "add_widget" || ops[8] != "set_settings" {
		t.Fatalf("op enum = %v", ops)
	}
	// Free objects become JSON strings for strict providers; dispatch decodes both forms.
	if props["set"].(map[string]interface{})["type"] != "string" {
		t.Fatalf("set schema = %v", props["set"])
	}
	widget := props["widget"].(map[string]interface{})["properties"].(map[string]interface{})
	types := widget["type"].(map[string]interface{})["enum"].([]string)
	if len(types) != len(cydcreator.WidgetCatalog()) || widget["props"].(map[string]interface{})["type"] != "string" {
		t.Fatalf("widget schema = %v", widget)
	}
}

func TestCYDCreatorToolsStayOutOfTheNormalCatalog(t *testing.T) {
	for _, name := range builtinToolNames(allBuiltinToolFeatureFlags()) {
		if strings.HasPrefix(name, "cyd_creator_") {
			t.Fatalf("%s is in the normal tool catalog", name)
		}
	}
}

func TestDecodeCYDLayoutOpsAcceptsStringEncodedObjects(t *testing.T) {
	raw := []any{map[string]any{"op": "add_widget", "screen": "home",
		"widget": map[string]any{"type": "label", "name": "lbl_a", "props": `{"text":"Hi","size":20}`,
			"events": []any{map[string]any{"on": "clicked", "action": `{"type":"logic","hint":"count"}`}}}},
		map[string]any{"op": "update_widget", "id": "lbl_a", "set": `{"props":{"text":"Ho"}}`}}
	ops, err := decodeCYDLayoutOps(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ops[0].Widget.Props["text"] != "Hi" || ops[0].Widget.Events[0].Action.Hint != "count" || ops[1].Set["props"].(map[string]any)["text"] != "Ho" {
		t.Fatalf("ops = %+v", ops)
	}
	asString, _ := json.Marshal([]any{map[string]any{"op": "set_theme", "theme": "night"}})
	if ops, err := decodeCYDLayoutOps(string(asString)); err != nil || ops[0].Theme != "night" {
		t.Fatalf("string ops = %+v, %v", ops, err)
	}
	for _, bad := range []any{nil, []any{}, []any{map[string]any{"op": "set_theme", "colour": "red"}}, "not json"} {
		if _, err := decodeCYDLayoutOps(bad); err == nil {
			t.Errorf("accepted %v", bad)
		}
	}
}
