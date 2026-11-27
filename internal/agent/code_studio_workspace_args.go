package agent

import (
	"encoding/json"
	"strings"

	"aurago/internal/security"
)

// codeStudioWorkspaceToolArgs copies the native arguments and fills the
// fields a text-mode call may carry on the ToolCall instead of in Params.
// Explicit parameters win, except for the operation: it is always the one the
// role policy and the action policy judged (toolCallSnapshotOperation), so a
// Params["operation"] cannot select a different one at execution.
func codeStudioWorkspaceToolArgs(tc ToolCall) map[string]interface{} {
	args := make(map[string]interface{}, len(tc.Params)+5)
	for key, value := range tc.Params {
		args[key] = value
	}
	fill := func(key, value string) {
		if _, ok := args[key]; !ok && value != "" {
			args[key] = value
		}
	}
	if op := toolCallSnapshotOperation(tc); op != "" {
		args["operation"] = op
	}
	fill("path", firstNonEmptyToolString(tc.Path, tc.FilePath))
	fill("content", tc.Content)
	fill("command", tc.Command)
	fill("query", tc.Query)
	return args
}

// codeStudioWorkspaceOperationMismatch reports a call whose Params name another
// operation than the one the policies judged. Execution refuses it.
func codeStudioWorkspaceOperationMismatch(tc ToolCall) bool {
	declared := toolCallSnapshotOperation(tc)
	raw := toolArgString(tc.Params, "operation")
	return raw != "" && strings.ToLower(strings.TrimSpace(raw)) != declared
}

// isolateCodeStudioWorkspaceOutput fences the result of a remote workspace call
// as external data: file contents, search hits and command output are written by
// whoever controls the remote repository. An error envelope keeps its status
// readable so the call is still classified as failed, but its other fields are
// fenced too: they can carry remote stderr (grep naming files the repository
// owner chose) or partial command output.
func isolateCodeStudioWorkspaceOutput(output string) string {
	var envelope map[string]interface{}
	if json.Unmarshal([]byte(strings.TrimSpace(output)), &envelope) != nil {
		return security.IsolateExternalData(output)
	}
	if status, _ := envelope["status"].(string); !strings.EqualFold(status, "error") {
		return security.IsolateExternalData(output)
	}
	status := envelope["status"]
	delete(envelope, "status")
	for key, value := range envelope {
		text, ok := value.(string)
		if !ok {
			raw, err := json.Marshal(value)
			if err != nil {
				continue
			}
			text = string(raw)
		}
		envelope[key] = security.IsolateExternalData(text)
	}
	statusJSON, _ := json.Marshal(status)
	rest, err := json.Marshal(envelope)
	if err != nil {
		return security.IsolateExternalData(output)
	}
	// status first, as every tool envelope reads.
	if len(envelope) == 0 {
		return `{"status":` + string(statusJSON) + `}`
	}
	return `{"status":` + string(statusJSON) + `,` + string(rest[1:])
}

// codeStudioWorkspaceRefusal admits code_studio_workspace only for the desktop
// chat of a Code Studio window whose SSH workspace the server resolved, and only
// for that workspace. Missions, flows, channels, A2A and co-agents never reach a
// remote host through it.
func codeStudioWorkspaceRefusal(tc ToolCall, dc *DispatchContext) string {
	refuse := func(message string) string {
		return `Tool Output: {"status":"policy_denied","message":"` + message + `"}`
	}
	if dc == nil || dc.IsCoAgent || dc.MissionID != "" || isA2ARun(dc) ||
		strings.ToLower(strings.TrimSpace(dc.MessageSource)) != "virtual_desktop_chat" {
		return refuse("code_studio_workspace is only available in the desktop chat of a Code Studio SSH workspace.")
	}
	allowed := strings.TrimSpace(dc.CodeStudioWorkspaceID)
	if allowed == "" {
		return refuse("This chat is not bound to a Code Studio SSH workspace.")
	}
	if requested := strings.TrimSpace(toolArgString(codeStudioWorkspaceToolArgs(tc), "workspace_id")); requested != allowed {
		return refuse("workspace_id must be the Code Studio workspace of this chat.")
	}
	return ""
}
