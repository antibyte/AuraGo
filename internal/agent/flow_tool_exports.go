package agent

import "aurago/internal/config"

// Thin exports for the EasyDrag flow tool invoker (internal/server/flows_tool_invoker*.go),
// so flows classify answers and resolve served paths exactly as the agent does.

// ClassifyToolResult classifies a tool's text answer as the dispatcher classifies raw
// results (classifyLegacyToolResult): JSON envelopes by code, status, success or exit_code,
// pending and deferred answers, and the plain-text failure markers.
func ClassifyToolResult(output string) ToolResultStatus {
	return classifyLegacyToolResult(output)
}

// ResolveServedFilePath maps a path the server serves (/files/documents/<name> and the other
// /files/ roots) to its local file, as send_document does: the query is dropped, and
// traversal or a backslash is an error. matched is false for any other path.
func ResolveServedFilePath(rawPath string, cfg *config.Config) (localPath, webPath string, matched bool, err error) {
	return resolveServedFilePath(rawPath, cfg)
}
