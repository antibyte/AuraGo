package agent

import (
	"context"
	"encoding/json"

	"aurago/internal/desktop"
	"aurago/internal/tools"
)

// dispatchRetroGames runs the retro_games tool and forwards a play's open_app
// event to the desktop exactly like virtual_desktop does.
func dispatchRetroGames(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	req := decodeRetroGamesArgs(tc)
	if dc.Logger != nil {
		dc.Logger.Info("LLM requested retro games", "operation", req.Operation)
	}
	exec := tools.ExecuteRetroGames(ctx, dc.Cfg, req)
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
