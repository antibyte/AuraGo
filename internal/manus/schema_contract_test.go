package manus

import (
	"strings"
	"testing"
)

func TestValidateStructuredOutputSchemaRequiresSchemaAtEveryNode(t *testing.T) {
	t.Parallel()

	tests := map[string]map[string]any{
		"property without type ref or anyOf": closedObject(map[string]any{"value": map[string]any{"description": "missing type"}}, "value"),
		"array item without schema":          closedObject(map[string]any{"values": map[string]any{"type": "array", "items": map[string]any{}}}, "values"),
		"empty type array":                   closedObject(map[string]any{"value": map[string]any{"type": []any{}}}, "value"),
		"non-string description":             closedObject(map[string]any{"value": map[string]any{"type": "string", "description": 42}}, "value"),
		"object keywords on string":          closedObject(map[string]any{"value": map[string]any{"type": "string", "properties": map[string]any{}, "required": []any{}, "additionalProperties": false}}, "value"),
		"array keyword on string":            closedObject(map[string]any{"value": map[string]any{"type": "string", "items": map[string]any{"type": "string"}}}, "value"),
		"duplicate required":                 closedObject(map[string]any{"value": map[string]any{"type": "string"}}, "value", "value"),
	}
	for name, schema := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateStructuredOutputSchema(schema); err == nil {
				t.Fatal("error = nil, want validation failure")
			}
		})
	}
}

func TestValidateStructuredOutputSchemaValidatesEnumsAndReferences(t *testing.T) {
	t.Parallel()

	tests := map[string]map[string]any{
		"empty enum":          closedObject(map[string]any{"value": map[string]any{"type": "string", "enum": []any{}}}, "value"),
		"enum type mismatch":  closedObject(map[string]any{"value": map[string]any{"type": "string", "enum": []any{1}}}, "value"),
		"compound enum value": closedObject(map[string]any{"value": map[string]any{"type": "string", "enum": []any{map[string]any{"unsafe": true}}}}, "value"),
		"missing ref target":  closedObject(map[string]any{"value": map[string]any{"$ref": "#/$defs/missing"}}, "value"),
		"remote ref":          closedObject(map[string]any{"value": map[string]any{"$ref": "https://example.com/schema"}}, "value"),
	}
	for name, schema := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateStructuredOutputSchema(schema); err == nil {
				t.Fatal("error = nil, want validation failure")
			}
		})
	}
}

func TestValidateStructuredOutputSchemaAcceptsRecursiveDefsAndSupportedKeywords(t *testing.T) {
	t.Parallel()

	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"description":          "A recursive result",
		"properties": map[string]any{
			"root": map[string]any{"$ref": "#/$defs/node"},
			"kind": map[string]any{"type": "string", "enum": []any{"leaf", "branch"}},
			"metadata": map[string]any{"anyOf": []any{
				map[string]any{"type": "null"},
				map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
			}},
		},
		"required": []any{"root", "kind", "metadata"},
		"$defs": map[string]any{
			"node": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"name":     map[string]any{"type": "string"},
					"children": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/$defs/node"}},
				},
				"required": []any{"name", "children"},
			},
		},
	}
	if err := ValidateStructuredOutputSchema(schema); err != nil {
		t.Fatalf("ValidateStructuredOutputSchema() error = %v", err)
	}
}

func TestValidateStructuredOutputSchemaEnforcesFiveLevels(t *testing.T) {
	t.Parallel()

	level6 := map[string]any{"type": "string"}
	for level := 5; level >= 1; level-- {
		level6 = closedObject(map[string]any{"level": level6}, "level")
	}
	err := ValidateStructuredOutputSchema(level6)
	if err == nil || !strings.Contains(err.Error(), "exceeds 5 levels") {
		t.Fatalf("ValidateStructuredOutputSchema() error = %v", err)
	}
}

func closedObject(properties map[string]any, required ...string) map[string]any {
	requiredValues := make([]any, 0, len(required))
	for _, name := range required {
		requiredValues = append(requiredValues, name)
	}
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": properties, "required": requiredValues,
	}
}
