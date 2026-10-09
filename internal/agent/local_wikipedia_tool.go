package agent

import (
	"context"

	"aurago/internal/tools"
)

// dispatchLocalWikipedia runs the read-only local_wikipedia tool. Gating
// (enabled, agent access, open edition) and output isolation live in
// tools.ExecuteLocalWikipedia; the query is not logged. The answer is sized
// to the inline budget so the output vault never archives it.
func dispatchLocalWikipedia(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	req := decodeLocalWikipediaArgs(tc)
	req.OutputBudget = toolResultInlineBudget(dc.Cfg)
	if dc.Logger != nil {
		dc.Logger.Info("LLM requested local Wikipedia", "operation", req.Operation)
	}
	return tools.ExecuteLocalWikipedia(ctx, dc.Cfg, req)
}
