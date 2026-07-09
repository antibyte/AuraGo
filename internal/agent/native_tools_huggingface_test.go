package agent

import (
	"reflect"
	"testing"
)

func TestHuggingFaceNativeToolIsFeatureGated(t *testing.T) {
	if containsName(toolNames(builtinToolSchemas(ToolFeatureFlags{})), "huggingface") {
		t.Fatal("huggingface tool must be hidden when the integration is disabled")
	}
	if !containsName(toolNames(builtinToolSchemas(ToolFeatureFlags{HuggingFaceEnabled: true})), "huggingface") {
		t.Fatal("huggingface tool must be exposed when the integration is enabled")
	}
}

func TestHuggingFaceNativeToolExposesCorrectJobOperations(t *testing.T) {
	props := nativeToolProperties(t, builtinToolSchemas(ToolFeatureFlags{HuggingFaceEnabled: true}), "huggingface")
	operation := props["operation"].(map[string]interface{})
	enum := operation["enum"].([]string)
	for _, want := range []string{"job_logs", "job_run_python", "job_run_uv_script", "job_run_container"} {
		if !containsName(enum, want) {
			t.Fatalf("missing Hugging Face operation %q: %v", want, enum)
		}
	}
	if containsName(enum, "job_run_script") {
		t.Fatalf("deprecated job_run_script must not be exposed: %v", enum)
	}
	if props["command"].(map[string]interface{})["type"] != "array" {
		t.Fatalf("command schema = %#v", props["command"])
	}
	if props["arguments"].(map[string]interface{})["type"] != "array" {
		t.Fatalf("arguments schema = %#v", props["arguments"])
	}
}

func TestDecodeHuggingFaceArrayArgumentsPreservesValues(t *testing.T) {
	request := decodeHuggingFaceArgs(ToolCall{Params: map[string]interface{}{
		"operation": "job_run_container",
		"command":   []interface{}{"python", "-c", "print(1)", ""},
		"arguments": []interface{}{"--name", "", "value with spaces"},
	}})
	wantCommand := []string{"python", "-c", "print(1)", ""}
	wantArguments := []string{"--name", "", "value with spaces"}
	if !reflect.DeepEqual(request.Command, wantCommand) || !reflect.DeepEqual(request.Arguments, wantArguments) {
		t.Fatalf("decoded Hugging Face arrays = %#v, %#v; want %#v, %#v", request.Command, request.Arguments, wantCommand, wantArguments)
	}
}
