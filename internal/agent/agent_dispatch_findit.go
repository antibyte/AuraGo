package agent

import (
	"context"
	"encoding/json"

	"aurago/internal/findit"
	"aurago/internal/tools"
)

func dispatchFindIt(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	args, err := json.Marshal(builtinArgsFromToolCall(tc))
	if err != nil {
		return `Tool Output: {"status":"error","code":"invalid","message":"FindIt arguments could not be read."}`
	}
	var req findit.QueryRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return `Tool Output: {"status":"error","code":"invalid","message":"FindIt arguments are invalid."}`
	}
	var cfg = dc.Cfg
	return tools.ExecuteFindIt(ctx, cfg, req)
}
