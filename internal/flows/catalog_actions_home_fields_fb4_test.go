package flows

import (
	"reflect"
	"testing"
)

func TestFB4HomeAssistantFieldsFollowTheOperation(t *testing.T) {
	def := lookupDef(t, homeRegistry(t), TypeHomeAssistant)
	names := func(op any) []string {
		params := map[string]any{"entity": "light.flur"}
		if op != nil {
			params["operation"] = op
		}
		var out []string
		for _, f := range def.FieldsOf(&Node{Params: params}) {
			out = append(out, f.Name)
		}
		return out
	}
	service := []string{"ok", "service", "affected_entities"}
	for op, want := range map[any][]string{
		"get_state":           {"state", "attributes", "entity_id", "last_changed"},
		"call_service":        service,
		" Call_Service ":      service,
		"{{trigger.data.op}}": {"state", "attributes", "entity_id", "last_changed", "ok", "service", "affected_entities"},
	} {
		if got := names(op); !reflect.DeepEqual(got, want) {
			t.Errorf("operation %v: fields %v, want %v", op, got, want)
		}
	}
	if got := names(nil); !reflect.DeepEqual(got, service) {
		t.Errorf("no operation (default call_service): fields %v", got)
	}
	if dynamicFields[TypeHomeAssistant] != "operation" {
		t.Fatal("the editor must know that the operation drives the fields")
	}
}
