package agent

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// virtual_workspace declares text as a boolean ("also return decoded text for read_file"), while ToolCall.Text is the
// text of the TTS tools. A schema-conforming call with "text": false was therefore rejected by the whole-struct decode and
// answered with invalid_arguments, whatever the operation was: a model that fills every optional field with its default
// (step-3.7-flash did, on a plain list) could never reach the tool and the retry feedback named no field.

// The arguments of the first rejected call from the aurago-test log (2026-11-21, operation list).
const loggedVirtualWorkspaceListArgs = `{"_todo": "Open amazon.de in visible browser", "append": false, "cols": 80, "command": "", "content": "", "content_base64": "", "credential_id": "", "cursor": 0, "field_names": [], "grant_id": "", "id": "", "include_closed": false, "input": "", "job_id": "", "limit": 10, "network_profile": "internet_lan", "offset": 0, "operation": "list", "origin": "", "path": "", "pty": false, "purpose": "List available workspaces", "rows": 24, "restart": "", "source": "", "tail": "0", "text": false, "timeout_seconds": 30, "ttl_seconds": 0, "usage_type": "", "volume_id": "", "wait_for_credential_grant": false, "working_dir": "/workspace", "workspace_id": ""}`

func nativeVirtualCall(name, arguments string) ToolCall {
	return NativeToolCallToToolCall(openai.ToolCall{ID: "call-1", Type: "function", Function: openai.FunctionCall{Name: name, Arguments: arguments}}, nil)
}

func TestVirtualWorkspaceCallWithTextFlagIsNotMalformed(t *testing.T) {
	for name, arguments := range map[string]string{
		"logged list call": loggedVirtualWorkspaceListArgs,
		"nulls and None strings": `{"operation":"open","command":null,"content":null,"origin":null,"workspace_id":null,"template":"desktop",` +
			`"text":false,"restart":"None","source":"None","limit":10,"timeout_seconds":30}`,
		"read_file with text": `{"operation":"read_file","workspace_id":"ws-1","path":"notes.txt","text":true}`,
	} {
		tc := nativeVirtualCall("virtual_workspace", arguments)
		if tc.NativeArgsMalformed {
			t.Errorf("%s: flagged malformed: %s", name, tc.NativeArgsError)
		}
		if tc.Action != "virtual_workspace" || tc.Operation == "" {
			t.Errorf("%s: action %q, operation %q", name, tc.Action, tc.Operation)
		}
	}
}

// The tool reads text from the flat Params map: a boolean stays a boolean there and a text stays a text (the tool parses
// "true" itself), only the typed ToolCall.Text field holds the coerced text.
func TestVirtualWorkspaceTextFlagKeepsItsTypeInParams(t *testing.T) {
	for name, c := range map[string]struct {
		json   string
		text   string
		params interface{}
	}{
		"true":        {`{"operation":"read_file","workspace_id":"w","path":"a","text":true}`, "true", true},
		"false":       {`{"operation":"list","text":false}`, "false", false},
		"true text":   {`{"operation":"read_file","workspace_id":"w","path":"a","text":"true"}`, "true", "true"},
		"absent":      {`{"operation":"list"}`, "", nil},
		"null":        {`{"operation":"list","text":null}`, "", nil},
		"plain words": {`{"action":"tts","text":"Hello there"}`, "Hello there", "Hello there"},
	} {
		var tc ToolCall
		if err := json.Unmarshal([]byte(c.json), &tc); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if tc.Text != c.text {
			t.Errorf("%s: typed Text = %q, want %q", name, tc.Text, c.text)
		}
		if got := tc.Params["text"]; !reflect.DeepEqual(got, c.params) {
			t.Errorf("%s: Params[text] = %#v, want %#v", name, got, c.params)
		}
	}
}

// Every property of the virtual_* tool schemas, sent with a value of its declared type, must decode: the schema is what the
// model is told to send, so a type clash with a shared ToolCall field fails a correct call.
func TestVirtualToolSchemaPropertiesDecodeWithTheirDeclaredTypes(t *testing.T) {
	var flags ToolFeatureFlags
	flagValues := reflect.ValueOf(&flags).Elem()
	for i := 0; i < flagValues.NumField(); i++ {
		if flagValues.Field(i).Kind() == reflect.Bool {
			flagValues.Field(i).SetBool(true)
		}
	}
	checked := 0
	for _, def := range BuildNativeToolSchemas("", nil, flags, nil) {
		if def.Function == nil || !strings.HasPrefix(def.Function.Name, "virtual_") {
			continue
		}
		encoded, err := json.Marshal(def.Function.Parameters)
		if err != nil {
			t.Fatalf("%s: %v", def.Function.Name, err)
		}
		var parameters struct {
			Properties map[string]struct {
				Type string        `json:"type"`
				Enum []interface{} `json:"enum"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(encoded, &parameters); err != nil {
			t.Fatalf("%s: %v", def.Function.Name, err)
		}
		for property, declared := range parameters.Properties {
			var sample interface{}
			switch declared.Type {
			case "string":
				sample = "x"
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
			default:
				continue
			}
			arguments, _ := json.Marshal(map[string]interface{}{"operation": "list", property: sample})
			if tc := nativeVirtualCall(def.Function.Name, string(arguments)); tc.NativeArgsMalformed {
				t.Errorf("%s.%s (%s): %s", def.Function.Name, property, declared.Type, tc.NativeArgsError)
			}
			checked++
		}
	}
	if checked < 40 {
		t.Fatalf("only %d virtual tool properties were checked: the schemas were not found", checked)
	}
}
