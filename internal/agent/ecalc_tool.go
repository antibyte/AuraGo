package agent

import (
	"context"
	"encoding/json"

	"aurago/internal/desktop"
	"aurago/internal/tools"
)

// dispatchECalc runs the ecalc tool and forwards the open_app event of an open to the desktop exactly like
// virtual_desktop and retro_games do. The answer is sized to the inline budget so the output vault never archives
// it. The gate of open (desktop, agent control) lives in tools.ExecuteECalc; the A2A restriction of open is the role
// policy's (a2aDesktopWrite).
func dispatchECalc(_ context.Context, tc ToolCall, dc *DispatchContext) string {
	req := decodeECalcArgs(tc)
	req.OutputBudget = toolResultInlineBudget(dc)
	if dc.Logger != nil {
		dc.Logger.Info("LLM requested ECalc", "operation", req.Operation)
	}
	exec := tools.ExecuteECalc(dc.Cfg, req)
	if exec.Event != nil && dc.Broker != nil {
		payload, err := json.Marshal(struct {
			Type    string         `json:"type"`
			Payload *desktop.Event `json:"payload"`
		}{"virtual_desktop_event", exec.Event})
		if err == nil {
			dc.Broker.SendJSON(string(payload))
		}
	}
	return "Tool Output: " + exec.Output
}
