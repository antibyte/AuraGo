package manus

import "testing"

func TestValidateStructuredOutputSchemaAcceptsSupportedObject(t *testing.T) {
	t.Parallel()

	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"answer": map[string]any{"type": "string", "description": "The answer"},
			"score":  map[string]any{"type": []any{"integer", "null"}},
		},
		"required": []any{"answer", "score"},
	}
	if err := ValidateStructuredOutputSchema(schema); err != nil {
		t.Fatalf("ValidateStructuredOutputSchema() error = %v", err)
	}
}

func TestValidateStructuredOutputSchemaRejectsUnsupportedOrIncompleteObjects(t *testing.T) {
	t.Parallel()

	tests := map[string]map[string]any{
		"non-object root": {
			"type": "array",
		},
		"additional properties": {
			"type": "object", "additionalProperties": true, "properties": map[string]any{}, "required": []any{},
		},
		"missing required property": {
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{"name": map[string]any{"type": "string"}}, "required": []any{},
		},
		"unsupported keyword": {
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{"name": map[string]any{"type": "string", "minLength": 1}}, "required": []any{"name"},
		},
	}
	for name, schema := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateStructuredOutputSchema(schema); err == nil {
				t.Fatal("error = nil, want validation failure")
			}
		})
	}
}
