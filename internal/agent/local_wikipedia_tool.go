package agent

import (
	"context"

	"aurago/internal/tools"
)

// dispatchLocalWikipedia runs the read-only local_wikipedia tool. Gating
// (enabled, agent access, open edition) and output isolation live in
// tools.ExecuteLocalWikipedia; the query is not logged.
func dispatchLocalWikipedia(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	req := decodeLocalWikipediaArgs(tc)
	if dc.Logger != nil {
		dc.Logger.Info("LLM requested local Wikipedia", "operation", req.Operation)
	}
	return tools.ExecuteLocalWikipedia(ctx, dc.Cfg, req)
}
