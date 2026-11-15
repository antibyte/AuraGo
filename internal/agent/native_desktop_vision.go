package agent

import (
	"context"
	"fmt"
	"strings"

	"aurago/internal/config"
	"aurago/internal/tools"
	"aurago/internal/virtualcomputers"
)

func nativeDesktopVisionGate(tc ToolCall, dc *DispatchContext) string {
	op := firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"))
	switch op {
	case "screenshot", "click_xy", "scroll", "drag":
	default:
		return ""
	}
	if dc == nil || dc.Cfg == nil {
		return tools.ErrorJSONf("Vision is unavailable; use semantic controls or hand control to the user")
	}
	if dc.BudgetTracker != nil && dc.BudgetTracker.IsBlocked("vision") {
		return tools.ErrorJSONf("Vision budget is exhausted; use semantic controls or hand control to the user")
	}
	cfg := dc.Cfg
	mcp := cfg.MCP.PreferredCapabilities.Vision
	available := cfg.Vision.Mirage.Enabled || (cfg.MCP.Enabled && cfg.Agent.AllowMCP && mcp.Server != "" && mcp.Tool != "")
	available = available || (!tools.VisionRequiresPublicImageURL(cfg) && (cfg.Vision.APIKey != "" || cfg.LLM.APIKey != "") && (cfg.Vision.BaseURL != "" || cfg.LLM.BaseURL != ""))
	if !available {
		return tools.ErrorJSONf("No Vision route for private screenshots; use semantic controls or hand control to the user")
	}
	return ""
}

func prepareNativeDesktopVision(ctx context.Context, dc *DispatchContext, input string) (string, *config.Config, func(), error) {
	if dc.IsCoAgent || isA2ARun(dc) {
		return "", dc.Cfg, func() {}, fmt.Errorf("native desktop images are restricted to the main agent")
	}
	m := virtualcomputers.DefaultWorkspaceManager()
	if m == nil {
		return "", dc.Cfg, func() {}, fmt.Errorf("workspace manager unavailable")
	}
	path, err := m.ResolveDesktopImage(ctx, virtualcomputers.WorkspaceIdentity{SessionID: dc.SessionID, MissionID: dc.MissionID, Actor: "agent"}, strings.TrimSpace(input))
	if err != nil {
		return "", dc.Cfg, func() {}, err
	}
	return prepareManagedVisionInput(path, dc.Cfg)
}
