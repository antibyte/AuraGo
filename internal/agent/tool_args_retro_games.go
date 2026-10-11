package agent

import "aurago/internal/tools"

// decodeRetroGamesArgs reads a retro_games call from native params first and
// falls back to the generic ToolCall fields of text-mode calls.
func decodeRetroGamesArgs(tc ToolCall) tools.RetroGamesRequest {
	return tools.RetroGamesRequest{
		Operation:  firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Query:      firstNonEmptyToolString(toolArgString(tc.Params, "query"), tc.Query),
		Platform:   toolArgString(tc.Params, "platform"),
		Decade:     toolArgInt(tc.Params, 0, "decade"),
		Limit:      firstNonEmptyInt(toolArgInt(tc.Params, 0, "limit"), tc.Limit),
		Identifier: toolArgString(tc.Params, "identifier", "id", "game"),
	}
}
