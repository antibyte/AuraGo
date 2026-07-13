package manus

import (
	"fmt"
	"sort"
	"strings"
)

const maxStructuredSchemaDepth = 5

var supportedSchemaKeywords = map[string]struct{}{
	"$defs": {}, "$ref": {}, "additionalProperties": {}, "anyOf": {}, "description": {},
	"enum": {}, "items": {}, "properties": {}, "required": {}, "type": {},
}

var supportedSchemaTypes = map[string]struct{}{
	"string": {}, "number": {}, "integer": {}, "boolean": {}, "null": {}, "object": {}, "array": {},
}

// ValidateStructuredOutputSchema enforces the JSON Schema subset supported by Manus v2.
func ValidateStructuredOutputSchema(schema map[string]any) error {
	if schema == nil {
		return nil
	}
	if schemaType(schema["type"]) != "object" {
		return fmt.Errorf("Manus structured output schema root must be an object")
	}
	if err := validateSchemaNode(schema, 1, "$", true); err != nil {
		return err
	}
	return nil
}

func validateSchemaNode(node map[string]any, depth int, path string, requireClosedObject bool) error {
	if depth > maxStructuredSchemaDepth {
		return fmt.Errorf("Manus structured output schema exceeds %d levels at %s", maxStructuredSchemaDepth, path)
	}
	for key := range node {
		if _, ok := supportedSchemaKeywords[key]; !ok {
			return fmt.Errorf("unsupported Manus schema keyword %q at %s", key, path)
		}
	}
	if ref, ok := node["$ref"].(string); ok {
		if !strings.HasPrefix(ref, "#/$defs/") {
			return fmt.Errorf("Manus schema reference %q at %s must target #/$defs", ref, path)
		}
	}
	if value, ok := node["type"]; ok {
		if err := validateSchemaTypes(value, path); err != nil {
			return err
		}
	}
	if variants, ok := node["anyOf"]; ok {
		items, ok := variants.([]any)
		if !ok || len(items) == 0 {
			return fmt.Errorf("Manus schema anyOf at %s must be a non-empty array", path)
		}
		for i, item := range items {
			child, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("Manus schema anyOf item %d at %s must be an object", i, path)
			}
			if err := validateSchemaNode(child, depth+1, fmt.Sprintf("%s.anyOf[%d]", path, i), false); err != nil {
				return err
			}
		}
	}
	if defs, ok := node["$defs"]; ok {
		defMap, ok := defs.(map[string]any)
		if !ok {
			return fmt.Errorf("Manus schema $defs at %s must be an object", path)
		}
		for name, raw := range defMap {
			child, ok := raw.(map[string]any)
			if !ok {
				return fmt.Errorf("Manus schema definition %q must be an object", name)
			}
			if err := validateSchemaNode(child, depth+1, path+".$defs."+name, false); err != nil {
				return err
			}
		}
	}

	if hasSchemaType(node["type"], "object") || requireClosedObject {
		closed, ok := node["additionalProperties"].(bool)
		if !ok || closed {
			return fmt.Errorf("Manus object schema at %s requires additionalProperties=false", path)
		}
		properties, ok := node["properties"].(map[string]any)
		if !ok {
			return fmt.Errorf("Manus object schema at %s requires properties", path)
		}
		required, err := stringSet(node["required"])
		if err != nil {
			return fmt.Errorf("Manus object schema at %s requires a string-array required field", path)
		}
		missing := make([]string, 0)
		for name, raw := range properties {
			if _, ok := required[name]; !ok {
				missing = append(missing, name)
			}
			child, ok := raw.(map[string]any)
			if !ok {
				return fmt.Errorf("Manus property %s.%s must be a schema object", path, name)
			}
			if err := validateSchemaNode(child, depth+1, path+"."+name, false); err != nil {
				return err
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return fmt.Errorf("Manus object schema at %s must require properties: %s", path, strings.Join(missing, ", "))
		}
		for name := range required {
			if _, ok := properties[name]; !ok {
				return fmt.Errorf("Manus object schema at %s requires unknown property %q", path, name)
			}
		}
	}
	if hasSchemaType(node["type"], "array") {
		child, ok := node["items"].(map[string]any)
		if !ok {
			return fmt.Errorf("Manus array schema at %s requires an items schema", path)
		}
		if err := validateSchemaNode(child, depth+1, path+"[]", false); err != nil {
			return err
		}
	}
	return nil
}

func validateSchemaTypes(value any, path string) error {
	types := make([]string, 0, 2)
	switch typed := value.(type) {
	case string:
		types = append(types, typed)
	case []any:
		for _, raw := range typed {
			value, ok := raw.(string)
			if !ok {
				return fmt.Errorf("Manus schema type array at %s must contain strings", path)
			}
			types = append(types, value)
		}
	default:
		return fmt.Errorf("Manus schema type at %s must be a string or string array", path)
	}
	for _, value := range types {
		if _, ok := supportedSchemaTypes[value]; !ok {
			return fmt.Errorf("unsupported Manus schema type %q at %s", value, path)
		}
	}
	return nil
}

func schemaType(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func hasSchemaType(value any, want string) bool {
	if value == want {
		return true
	}
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func stringSet(value any) (map[string]struct{}, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("not an array")
	}
	result := make(map[string]struct{}, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("not a string")
		}
		result[text] = struct{}{}
	}
	return result, nil
}
