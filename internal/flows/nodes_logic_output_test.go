package flows

import "testing"

func TestSetOutputFieldsAreConservative(t *testing.T) {
	def := setNodeDef()
	n := &Node{Params: map[string]any{"fields": []any{
		map[string]any{"name": "greeting", "type": "text"},
		map[string]any{"name": "count", "type": "number"},
		map[string]any{"name": "value", "type": "auto"},
	}}}
	fields := def.FieldsOf(n)
	if len(fields) != 3 || fields[0].Name != "greeting" || fields[0].Type != "text" || !fields[0].Primary || fields[1].Type != "number" || fields[2].Type != "any" {
		t.Fatalf("declared fields = %+v", fields)
	}
	for _, keep := range []any{true, "{{trigger.data.flag}}"} {
		n.Params["keep_input"] = keep
		if got := def.FieldsOf(n); got != nil {
			t.Fatalf("keep_input %v declared partial fields: %+v", keep, got)
		}
	}
	delete(n.Params, "keep_input")
	n.Params["fields"] = []any{map[string]any{"name": "{{trigger.data.key}}"}}
	if got := def.FieldsOf(n); got != nil {
		t.Fatalf("templated field name declared: %+v", got)
	}
}
