package agent

import "aurago/internal/tools"

func decodeHuggingFaceArgs(tc ToolCall) tools.HuggingFaceRequest {
	rows := tools.HuggingFaceRequest{
		Operation:      firstNonEmptyToolString(tc.Operation, tc.ActionType, toolArgString(tc.Params, "operation")),
		Query:          firstNonEmptyToolString(tc.Query, tc.Content, toolArgString(tc.Params, "query", "content")),
		Limit:          firstNonEmptyInt(toolArgInt(tc.Params, 0, "limit")),
		RepoType:       toolArgString(tc.Params, "repo_type", "type"),
		RepoID:         firstNonEmptyToolString(tc.Name, tc.ID, toolArgString(tc.Params, "repo_id", "repo")),
		Name:           firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Revision:       toolArgString(tc.Params, "revision", "branch"),
		Path:           firstNonEmptyToolString(tc.Path, tc.FilePath, toolArgString(tc.Params, "path", "file_path")),
		Destination:    firstNonEmptyToolString(tc.Destination, tc.Dest, toolArgString(tc.Params, "destination", "dest")),
		Dataset:        toolArgString(tc.Params, "dataset"),
		Config:         toolArgString(tc.Params, "config"),
		Split:          toolArgString(tc.Params, "split"),
		Offset:         toolArgInt(tc.Params, 0, "offset"),
		Length:         toolArgInt(tc.Params, 0, "length", "rows"),
		Where:          toolArgString(tc.Params, "where", "filter"),
		PaperID:        firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "paper_id")),
		JobID:          firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "job_id")),
		Hardware:       toolArgString(tc.Params, "hardware"),
		TimeoutMinutes: toolArgInt(tc.Params, 0, "timeout_minutes", "timeout"),
		Script:         toolArgString(tc.Params, "script"),
		Image:          toolArgString(tc.Params, "image"),
		Command:        toolArgString(tc.Params, "command"),
		Title:          firstNonEmptyToolString(tc.Description, toolArgString(tc.Params, "title")),
		Body:           firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "body")),
		Number:         toolArgInt(tc.Params, 0, "number", "discussion_number"),
		LocalPath:      toolArgString(tc.Params, "local_path"),
		Message:        toolArgString(tc.Params, "message", "commit_message"),
	}
	if private, ok := toolArgBool(tc.Params, "private"); ok {
		rows.Private = private
	}
	if scheduled, ok := toolArgBool(tc.Params, "scheduled"); ok {
		rows.Scheduled = scheduled
	}
	rows.Schedule = toolArgString(tc.Params, "schedule", "cron")
	if raw := toolArgJSONInterfaceMap(tc.Params, "env"); raw != nil {
		rows.Env = make(map[string]string, len(raw))
		for key, value := range raw {
			if text, ok := value.(string); ok {
				rows.Env[key] = text
			}
		}
	}
	rows.Args = toolArgJSONInterfaceMap(tc.Params, "args")
	return rows
}
