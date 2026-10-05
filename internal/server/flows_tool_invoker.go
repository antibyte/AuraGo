package server

import (
	"context"
	"encoding/json"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/tools"
)

// flowToolInvoker runs one tool for a flow node through the agent dispatcher. It applies
// the same runtime permissions, guardian and output handling as agent tool calls. The LLM
// Guardian is off because tool nodes are authored by the user, not by a model.
type flowToolInvoker struct {
	s        *Server
	names    func(cfg *config.Config) map[string]bool
	dispatch func(ctx context.Context, tc *agent.ToolCall, dc *agent.DispatchContext) agent.ToolDispatchResult
}

func newFlowToolInvoker(s *Server, env *flowCatalogEnv) *flowToolInvoker {
	return &flowToolInvoker{s: s, names: env.toolNames,
		dispatch: func(ctx context.Context, tc *agent.ToolCall, dc *agent.DispatchContext) agent.ToolDispatchResult {
			return agent.DispatchToolCallResult(ctx, tc, dc, "")
		}}
}

// flowToolRoute maps a logical flow tool name to the dispatcher action and, for skills,
// the skill name.
func flowToolRoute(tool string) (action, skill string) {
	if tool == flows.PDFExtractorTool {
		return "execute_skill", flows.PDFExtractorTool
	}
	return tool, ""
}

func flowToolRefusal(status agent.ToolResultStatus, message string) flows.ToolResponse {
	data, _ := json.Marshal(map[string]string{"status": "error", "message": message})
	return flows.ToolResponse{Output: string(data), IsError: true, Status: string(status)}
}

// InvokeTool implements flows.ToolInvoker. Every call is checked against the node's allow
// list and the configured agent tools before it reaches the dispatcher.
func (i *flowToolInvoker) InvokeTool(ctx context.Context, req flows.ToolRequest) (flows.ToolResponse, error) {
	i.s.CfgMu.RLock()
	cfg := i.s.Cfg
	i.s.CfgMu.RUnlock()
	if cfg == nil {
		return flows.ToolResponse{}, flows.NewNodeError("FLOW_TOOLS_UNAVAILABLE", "the configuration is not ready")
	}
	allowed := false
	for _, name := range req.AllowedTools {
		allowed = allowed || name == req.Tool
	}
	if !allowed || req.Tool == flows.GotenbergTool {
		return flowToolRefusal(agent.ToolResultDenied, "the tool "+req.Tool+" is not allowed for this node"), nil
	}
	if a := flowToolAvailability(cfg, i.names(cfg), req.Tool); a.State != flows.AvailableState {
		return flowToolRefusal(agent.ToolResultNeedsSetup, "the tool "+req.Tool+" is not set up or is disabled"), nil
	}
	action, skill := flowToolRoute(req.Tool)
	payload := make(map[string]any, len(req.Args))
	for k, v := range req.Args {
		payload[k] = v
	}
	encoded := any(payload)
	if skill != "" {
		encoded = map[string]any{"skill": skill, "skill_args": payload}
	}
	raw, err := json.Marshal(encoded)
	if err != nil {
		return flows.ToolResponse{}, flows.NewNodeError("FLOW_PARAM_INVALID", "the tool arguments cannot be encoded: %v", err)
	}
	var tc agent.ToolCall
	if err := json.Unmarshal(raw, &tc); err != nil {
		return flows.ToolResponse{}, flows.NewNodeError("FLOW_PARAM_INVALID", "the tool arguments are invalid: %v", err)
	}
	tc.Action, tc.IsTool = action, true
	if skill == "" {
		tc.Params = payload
	}
	res := i.dispatch(ctx, &tc, i.dispatchContext(cfg, req.FlowID, action))
	return flows.ToolResponse{Output: res.Output, IsError: res.IsError, Status: string(res.Status)}, nil
}

// dispatchContext mirrors buildLooperRuntime with a scope of exactly one tool.
func (i *flowToolInvoker) dispatchContext(cfg *config.Config, flowID, action string) *agent.DispatchContext {
	s := i.s
	return &agent.DispatchContext{
		Cfg: cfg, Logger: s.Logger, LLMClient: s.LLMClient, Vault: s.Vault, Registry: s.Registry,
		Manifest: tools.NewManifest(cfg.Directories.ToolsDir), CronManager: s.CronManager, MissionManagerV2: s.MissionManagerV2,
		LongTermMem: s.LongTermMem, ShortTermMem: s.ShortTermMem, KG: s.KG,
		InventoryDB: s.InventoryDB, InvasionDB: s.InvasionDB, CheatsheetDB: s.CheatsheetDB, ImageGalleryDB: s.ImageGalleryDB,
		MediaRegistryDB: s.MediaRegistryDB, HomepageRegistryDB: s.HomepageRegistryDB, ContactsDB: s.ContactsDB,
		PlannerDB: s.PlannerDB, SQLConnectionsDB: s.SQLConnectionsDB, SQLConnectionPool: s.SQLConnectionPool,
		RemoteHub: s.RemoteHub, HistoryMgr: s.HistoryManager, IsMaintenance: tools.IsBusy(),
		Guardian: s.Guardian, SessionID: "flow-" + flowID, Broker: looperDesktopBroker{sse: s.SSE},
		CoAgentRegistry: s.CoAgentRegistry, BudgetTracker: s.BudgetTracker, DaemonSupervisor: s.DaemonSupervisor,
		PreparationService: s.PreparationService, WorkspaceSearch: s.WorkspaceSearch, MessageSource: "flow",
		ToolScopeRestricted: true, AllowedTools: map[string]struct{}{action: {}},
	}
}
