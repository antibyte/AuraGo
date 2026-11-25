package agent

// codeStudioWorkspaceToolArgs copies the native arguments and fills the
// fields a text-mode call may carry on the ToolCall instead of in Params.
// Explicit parameters win.
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
	fill("operation", tc.Operation)
	fill("path", firstNonEmptyToolString(tc.Path, tc.FilePath))
	fill("content", tc.Content)
	fill("command", tc.Command)
	fill("query", tc.Query)
	return args
}
