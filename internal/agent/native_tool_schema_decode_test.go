package agent

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Every property of every native tool schema, sent with a value of its declared type, must decode: the schema is what the
// model is told to send, so a type clash with a shared ToolCall field (one JSON key, one typed Go field, two tools that
// disagree about the type) fails a correct call with invalid_arguments, whatever the operation is.
//
// Strings are sent as the first enum value, otherwise as the text of a JSON object (so the JSON-in-string fields such as
// headers, params or env are not false positives); arrays as an empty array; objects as {"k":"v"}.
func TestNativeToolSchemaPropertiesDecodeWithTheirDeclaredTypes(t *testing.T) {
	var flags ToolFeatureFlags
	flagValues := reflect.ValueOf(&flags).Elem()
	for i := 0; i < flagValues.NumField(); i++ {
		if flagValues.Field(i).Kind() == reflect.Bool {
			flagValues.Field(i).SetBool(true)
		}
	}
	checked := 0
	var offenders []string
	// BuildNativeToolSchemas drops the mega schemas that a focused set replaces (remote_control, homepage, agentmail,
	// invasion_control, virtual_computer_ui, virtual_desktop); the unfiltered builtin list still has them.
	schemas := append(BuildNativeToolSchemas("", nil, flags, nil), builtinToolSchemas(flags)...)
	for _, def := range schemas {
		if def.Function == nil {
			continue
		}
		encoded, err := json.Marshal(def.Function.Parameters)
		if err != nil {
			t.Fatalf("%s: %v", def.Function.Name, err)
		}
		var parameters struct {
			Properties map[string]struct {
				Type json.RawMessage `json:"type"`
				Enum []interface{}   `json:"enum"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(encoded, &parameters); err != nil {
			t.Fatalf("%s: %v", def.Function.Name, err)
		}
		for property, declared := range parameters.Properties {
			var declaredType string
			if json.Unmarshal(declared.Type, &declaredType) != nil {
				continue // no type, or a type list: nothing to send
			}
			var sample interface{}
			switch declaredType {
			case "string":
				sample = `{"k":"v"}`
				if len(declared.Enum) > 0 {
					sample = declared.Enum[0]
				}
			case "boolean":
				sample = true
			case "integer":
				sample = 1
			case "number":
				sample = 1.5
			case "array":
				sample = []interface{}{}
			case "object":
				sample = map[string]interface{}{"k": "v"}
			default:
				continue
			}
			arguments, err := json.Marshal(map[string]interface{}{"operation": "list", property: sample})
			if err != nil {
				t.Fatalf("%s.%s: %v", def.Function.Name, property, err)
			}
			if tc := nativeProbeCall(def.Function.Name, string(arguments)); tc.NativeArgsMalformed {
				offenders = append(offenders, fmt.Sprintf("%s.%s (%s): %s", def.Function.Name, property, declaredType, tc.NativeArgsError))
			}
			checked++
		}
	}
	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Errorf("%d native tool properties are rejected when sent with their declared type:\n%s", len(offenders), strings.Join(offenders, "\n"))
	}
	if checked <= 400 {
		t.Fatalf("only %d native tool properties were checked: the schemas were not found", checked)
	}
	t.Logf("checked %d native tool properties", checked)
}
