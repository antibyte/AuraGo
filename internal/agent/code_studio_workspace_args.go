package agent

import "strings"

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
