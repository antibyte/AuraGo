package manus

import (
	"fmt"
	"math"
	"reflect"
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
	if err := validateSchemaNode(schema, schema, 1, "$", true); err != nil {
		return err
	}
	return nil
}

func validateSchemaNode(root, node map[string]any, depth int, path string, requireClosedObject bool) error {
	if depth > maxStructuredSchemaDepth {
		return fmt.Errorf("Manus structured output schema exceeds %d levels at %s", maxStructuredSchemaDepth, path)
	}
	for key := range node {
		if _, ok := supportedSchemaKeywords[key]; !ok {
			return fmt.Errorf("unsupported Manus schema keyword %q at %s", key, path)
		}
	}
	_, hasType := node["type"]
	_, hasRef := node["$ref"]
	_, hasAnyOf := node["anyOf"]
	if !hasType && !hasRef && !hasAnyOf {
		return fmt.Errorf("Manus schema node at %s requires type, $ref, or anyOf", path)
	}
	if description, ok := node["description"]; ok {
		if _, ok := description.(string); !ok {
			return fmt.Errorf("Manus schema description at %s must be a string", path)
		}
	}
	if rawRef, ok := node["$ref"]; ok {
		ref, ok := rawRef.(string)
		if !ok || !strings.HasPrefix(ref, "#/$defs/") {
			return fmt.Errorf("Manus schema reference %q at %s must target #/$defs", ref, path)
		}
		if _, ok := resolveSchemaRef(root, ref); !ok {
			return fmt.Errorf("Manus schema reference %q at %s does not exist", ref, path)
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
			if err := validateSchemaNode(root, child, depth+1, fmt.Sprintf("%s.anyOf[%d]", path, i), false); err != nil {
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
			if err := validateSchemaNode(root, child, depth+1, path+".$defs."+name, false); err != nil {
				return err
			}
		}
	}

	isObject := hasSchemaType(node["type"], "object") || requireClosedObject
	if !isObject && hasAnyKeyword(node, "properties", "required", "additionalProperties") {
		return fmt.Errorf("Manus object keywords at %s require object type", path)
	}
	if isObject {
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
			if err := validateSchemaNode(root, child, depth+1, path+"."+name, false); err != nil {
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
	isArray := hasSchemaType(node["type"], "array")
	if !isArray && hasAnyKeyword(node, "items") {
		return fmt.Errorf("Manus array keyword at %s requires array type", path)
	}
	if isArray {
		child, ok := node["items"].(map[string]any)
		if !ok {
			return fmt.Errorf("Manus array schema at %s requires an items schema", path)
		}
		if err := validateSchemaNode(root, child, depth+1, path+"[]", false); err != nil {
			return err
		}
	}
	if enum, ok := node["enum"]; ok {
		if !hasType {
			return fmt.Errorf("Manus schema enum at %s requires an explicit type", path)
		}
		if err := validateSchemaEnum(enum, node["type"], path); err != nil {
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
		if len(typed) == 0 {
			return fmt.Errorf("Manus schema type array at %s must not be empty", path)
		}
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

func resolveSchemaRef(root map[string]any, ref string) (map[string]any, bool) {
	if !strings.HasPrefix(ref, "#/") {
		return nil, false
	}
	var current any = root
	for _, token := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[token]
		if !ok {
			return nil, false
		}
	}
	result, ok := current.(map[string]any)
	return result, ok
}

func validateSchemaEnum(value, schemaTypes any, path string) error {
	values, ok := value.([]any)
	if !ok || len(values) == 0 {
		return fmt.Errorf("Manus schema enum at %s must be a non-empty array", path)
	}
	seen := make([]any, 0, len(values))
	for _, candidate := range values {
		if !enumMatchesSchemaType(candidate, schemaTypes) {
			return fmt.Errorf("Manus schema enum value at %s does not match its type", path)
		}
		for _, previous := range seen {
			if reflect.DeepEqual(previous, candidate) {
				return fmt.Errorf("Manus schema enum at %s contains duplicate values", path)
			}
		}
		seen = append(seen, candidate)
	}
	return nil
}

func enumMatchesSchemaType(value, schemaTypes any) bool {
	for _, schemaType := range schemaTypeNames(schemaTypes) {
		switch schemaType {
		case "null":
			if value == nil {
				return true
			}
		case "string":
			_, ok := value.(string)
			if ok {
				return true
			}
		case "boolean":
			_, ok := value.(bool)
			if ok {
				return true
			}
		case "number":
			if isSchemaNumber(value, false) {
				return true
			}
		case "integer":
			if isSchemaNumber(value, true) {
				return true
			}
		}
	}
	return false
}

func schemaTypeNames(value any) []string {
	if text, ok := value.(string); ok {
		return []string{text}
	}
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func isSchemaNumber(value any, integerOnly bool) bool {
	switch number := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float32:
		return !integerOnly || math.Trunc(float64(number)) == float64(number)
	case float64:
		return !integerOnly || math.Trunc(number) == number
	default:
		return false
	}
}

func hasAnyKeyword(node map[string]any, keywords ...string) bool {
	for _, keyword := range keywords {
		if _, ok := node[keyword]; ok {
			return true
		}
	}
	return false
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
		if _, duplicate := result[text]; duplicate {
			return nil, fmt.Errorf("duplicate value")
		}
		result[text] = struct{}{}
	}
	return result, nil
}
