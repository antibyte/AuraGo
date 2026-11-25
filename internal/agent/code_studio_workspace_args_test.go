package agent

import (
	"fmt"
	"strings"
	"testing"
)

func TestCodeStudioWorkspaceToolArgsFillsToolCallFields(t *testing.T) {
	args := codeStudioWorkspaceToolArgs(ToolCall{Action: "code_studio_workspace", Operation: "write_file", FilePath: "src/a.go", Content: "package a\n", Params: map[string]interface{}{"workspace_id": "ws_7f3k"}})
	if args["operation"] != "write_file" || args["path"] != "src/a.go" || args["content"] != "package a\n" || args["workspace_id"] != "ws_7f3k" {
		t.Fatalf("args = %#v", args)
	}
	kept := codeStudioWorkspaceToolArgs(ToolCall{Operation: "read_file", Path: "ignored", Params: map[string]interface{}{"path": "src/b.go"}})
	if kept["path"] != "src/b.go" {
		t.Fatalf("explicit path overwritten: %#v", kept)
	}
}

func TestCodeStudioWorkspaceSchemaRequiresWorkspaceID(t *testing.T) {
	for _, s := range builtinToolSchemas(ToolFeatureFlags{VirtualDesktopEnabled: true}) {
		if s.Function == nil || s.Function.Name != "code_studio_workspace" {
			continue
		}
		params, _ := s.Function.Parameters.(map[string]interface{})
		props, _ := params["properties"].(map[string]interface{})
		op, _ := props["operation"].(map[string]interface{})
		if got := fmt.Sprint(op["enum"]); got != "[list_files read_file read_file_excerpt search write_file patch_file run_command]" {
			t.Fatalf("operations = %s", got)
		}
		if got := fmt.Sprint(params["required"]); !strings.Contains(got, "workspace_id") || !strings.Contains(got, "operation") {
			t.Fatalf("required = %s", got)
		}
		return
	}
	t.Fatal("code_studio_workspace schema missing with the virtual desktop enabled")
}

func TestCodeStudioWorkspaceToolArgsOperationFollowsThePolicyOperation(t *testing.T) {
	tc := ToolCall{Action: "code_studio_workspace", Operation: "list_files", Params: map[string]interface{}{"operation": "run_command", "command": "id"}}
	if got := codeStudioWorkspaceToolArgs(tc)["operation"]; got != "list_files" {
		t.Fatalf("operation = %v, want the judged list_files", got)
	}
	if !codeStudioWorkspaceOperationMismatch(tc) {
		t.Fatal("a Params operation that differs from the judged one must be flagged")
	}
	same := ToolCall{Action: "code_studio_workspace", Params: map[string]interface{}{"operation": " Read_File "}}
	if codeStudioWorkspaceOperationMismatch(same) || codeStudioWorkspaceToolArgs(same)["operation"] != "read_file" {
		t.Fatalf("params-only operation must stay usable: %#v", codeStudioWorkspaceToolArgs(same))
	}
}
