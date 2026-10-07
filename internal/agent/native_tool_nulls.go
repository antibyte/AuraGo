package agent

import (
	"encoding/json"
	"reflect"
	"strings"
)

func schemaHasType(schema map[string]interface{}, want string) bool {
	switch types := schema["type"].(type) {
	case string:
		return types == want
	case []string:
		for _, typ := range types {
			if typ == want {
				return true
			}
		}
	case []interface{}:
		for _, typ := range types {
			if typ == want {
				return true
			}
		}
	}
	return false
}

func schemaRequiredNames(schema map[string]interface{}) map[string]bool {
	names := make(map[string]bool)
	switch required := schema["required"].(type) {
	case []string:
		for _, name := range required {
			names[name] = true
		}
	case []interface{}:
		for _, value := range required {
			if name, ok := value.(string); ok {
				names[name] = true
			}
		}
	}
	return names
}

// schemaAllowsNull checks the constraints that can reject a null value. Other
// constraints (properties, bounds, patterns, items) apply only to non-null types.
func schemaAllowsNull(schema map[string]interface{}) bool {
	if _, typed := schema["type"]; typed && !schemaHasType(schema, "null") {
		return false
	}
	if value, exists := schema["const"]; exists && value != nil {
		return false
	}
	if values, exists := schema["enum"]; exists {
		found := false
		if values, ok := values.([]interface{}); ok {
			for _, value := range values {
				found = found || value == nil
			}
		}
		if !found {
			return false
		}
	}
	for _, key := range []string{"anyOf", "oneOf", "allOf"} {
		if branches, ok := schema[key].([]interface{}); ok {
			matches := 0
			for _, branch := range branches {
				if child, ok := branch.(map[string]interface{}); ok && schemaAllowsNull(child) {
					matches++
				}
			}
			if key == "anyOf" && matches == 0 || key == "oneOf" && matches != 1 || key == "allOf" && matches != len(branches) {
				return false
			}
		}
	}
	if child, ok := schema["not"].(map[string]interface{}); ok && schemaAllowsNull(child) {
		return false
	}
	return true
}

func makeSchemaNullable(schema map[string]interface{}) {
	if schemaAllowsNull(schema) {
		return
	}
	_, hasConst := schema["const"]
	_, hasNot := schema["not"]
	if hasSchemaCombinator(schema) || hasConst || hasNot {
		original := make(map[string]interface{}, len(schema))
		for key, value := range schema {
			original[key] = value
			delete(schema, key)
		}
		schema["anyOf"] = []interface{}{original, map[string]interface{}{"type": "null"}}
		return
	}
	if !schemaHasType(schema, "null") {
		switch types := schema["type"].(type) {
		case string:
			schema["type"] = []string{types, "null"}
		case []string:
			schema["type"] = append(types, "null")
		case []interface{}:
			schema["type"] = append(types, "null")
		}
	}
	if values, ok := schema["enum"].([]string); ok {
		converted := make([]interface{}, len(values))
		for i, value := range values {
			converted[i] = value
		}
		schema["enum"] = converted
	}
	if values, ok := schema["enum"].([]interface{}); ok {
		for _, value := range values {
			if value == nil {
				return
			}
		}
		schema["enum"] = append(values, nil)
	}
}

// stripStrictNulls touches only declared structured arguments. JSON strings,
// open payload maps and originally nullable values retain their meaning.
// The caller owns value; catalog schemas are never modified.
func stripStrictNulls(value interface{}, schema map[string]interface{}) bool {
	changed := false
	switch value := value.(type) {
	case map[string]interface{}:
		props, _ := schema["properties"].(map[string]interface{})
		required := schemaRequiredNames(schema)
		for name, raw := range props {
			child, ok := raw.(map[string]interface{})
			arg, present := value[name]
			if !ok || !present {
				continue
			}
			if arg == nil && !required[name] && !schemaAllowsNull(child) {
				delete(value, name)
				changed = true
			} else if stripStrictNulls(arg, child) {
				changed = true
			}
		}
	case []interface{}:
		if items, ok := schema["items"].(map[string]interface{}); ok {
			for _, item := range value {
				if stripStrictNulls(item, items) {
					changed = true
				}
			}
		}
	}
	return changed
}

func originalToolArgumentSchema(name string, dc *DispatchContext) map[string]interface{} {
	if dc != nil {
		if catalog := GetToolCatalogState(dc.discoveryKey()); catalog != nil {
			if entry, ok := catalog.Get(name); ok && entry.Schema.Function != nil {
				schema, _ := entry.Schema.Function.Parameters.(map[string]interface{})
				return schema
			}
		}
	}
	// Standalone native callers have no run catalog. This is schema lookup only;
	// it neither exposes tools nor grants permission to execute them.
	for _, tool := range builtinToolSchemasCached(allBuiltinToolFeatureFlags()) {
		if tool.Function != nil && tool.Function.Name == name {
			schema, _ := tool.Function.Parameters.(map[string]interface{})
			return schema
		}
	}
	return nil
}

func normalizeDirectToolCall(tc ToolCall, dc *DispatchContext) ToolCall {
	if tc.nativeCall != nil {
		native := *tc.nativeCall
		var args map[string]interface{}
		if json.Unmarshal([]byte(native.Function.Arguments), &args) != nil || !stripStrictNulls(args, originalToolArgumentSchema(native.Function.Name, dc)) {
			return tc
		}
		raw, err := json.Marshal(args)
		if err != nil {
			return tc
		}
		native.Function.Arguments = string(raw)
		decoded := NativeToolCallToToolCall(native, nil)
		decoded.TransportAction, decoded.RawJSON = tc.TransportAction, tc.RawJSON
		decoded.IsTool, decoded.Todo = tc.IsTool, tc.Todo
		return decoded
	}
	if tc.Params == nil {
		return tc
	}
	args := cloneJSONSchemaValue(tc.Params).(map[string]interface{})
	if !stripStrictNulls(args, originalToolArgumentSchema(tc.Action, dc)) {
		return tc
	}
	// Keep programmatically supplied fields absent from Params, but decode every
	// supplied field afresh so typed maps cannot retain removed nested keys.
	decoded := tc
	fields := reflect.ValueOf(&decoded).Elem()
	for i := 0; i < fields.NumField(); i++ {
		name := strings.Split(fields.Type().Field(i).Tag.Get("json"), ",")[0]
		if _, present := tc.Params[name]; present && name != "action" && fields.Field(i).CanSet() {
			fields.Field(i).SetZero()
		}
	}
	decoded.Params = nil
	raw, err := json.Marshal(args)
	if err != nil || json.Unmarshal([]byte(normalizeTagsInJSON(string(raw))), &decoded) != nil {
		return tc
	}
	decoded.Action, decoded.Params = tc.Action, args
	return decoded
}
