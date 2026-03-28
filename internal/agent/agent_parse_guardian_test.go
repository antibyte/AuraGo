package agent

import (
	"strings"
	"testing"
)

func TestFormatGuardianBlockedMessageAddsClarificationAndNextStep(t *testing.T) {
	msg := formatGuardianBlockedMessage("execute_shell", "remote code execution via curl pipe sh", 0.85, true, false)
	if !strings.Contains(msg, `"_guardian_justification"`) {
		t.Fatalf("expected clarification guidance, got: %s", msg)
	}
	if !strings.Contains(msg, "curl|sh") {
		t.Fatalf("expected remote execution next-step guidance, got: %s", msg)
	}
}

func TestFormatGuardianBlockedMessageForRejectedClarification(t *testing.T) {
	msg := formatGuardianBlockedMessage("execute_shell", "remote code execution via curl pipe sh", 0.85, true, true)
	if !strings.Contains(msg, "Clarification was reviewed but the action remains blocked") {
		t.Fatalf("expected rejected clarification text, got: %s", msg)
	}
	if !strings.Contains(msg, "built-in tool") && !strings.Contains(msg, "built-in") {
		t.Fatalf("expected safer next-step guidance, got: %s", msg)
	}
}

func TestToolCallParamsMarksProjectRootRelativePaths(t *testing.T) {
	params := toolCallParams(ToolCall{
		Action:    "filesystem",
		Operation: "read_file",
		FilePath:  "../../prompts/tools_manuals/filesystem.md",
	})
	if params["path_scope"] != "project_root_relative" {
		t.Fatalf("path_scope = %q, want project_root_relative", params["path_scope"])
	}
	if params["file_path"] != "project_root/prompts/tools_manuals/filesystem.md" {
		t.Fatalf("file_path = %q", params["file_path"])
	}
}

func TestToolCallParamsLeavesWorkdirPathsUntouched(t *testing.T) {
	params := toolCallParams(ToolCall{
		Action:    "filesystem",
		Operation: "read_file",
		FilePath:  "notes.txt",
	})
	if _, ok := params["path_scope"]; ok {
		t.Fatalf("unexpected path_scope for workdir path: %#v", params)
	}
	if params["file_path"] != "notes.txt" {
		t.Fatalf("file_path = %q, want notes.txt", params["file_path"])
	}
}

func TestParseToolCallCoercesNumericStringFields(t *testing.T) {
	tc := ParseToolCall(`{"action":"fritzbox_network","operation":"get_wlan","wlan_index":"2"}`)
	if !tc.IsTool {
		t.Fatal("expected tool call to be detected")
	}
	if tc.WLANIndex != 2 {
		t.Fatalf("WLANIndex = %d, want 2", tc.WLANIndex)
	}
}
