package server

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"unicode/utf8"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/tools"
)

// flowToolInvoker runs one tool for a flow node through the agent dispatcher. It applies
// the same runtime permissions, guardian and output handling as agent tool calls. The LLM
// Guardian is off because tool nodes are authored by the user, not by a model.
//
// What flows get on top of an agent tool call:
//   - The configuration is the server's snapshot (ConfigSnapshot, as for the catalog env)
//     with the summary modes of web_scraper, ddg_search, wikipedia_search and
//     pdf_extractor off and the preferred MCP web search cleared (flowDispatchConfig): a
//     summary is a model call outside the flow budget, and an MCP search answers in a shape
//     the web.search node cannot read.
//   - The dispatch is marked with agent.MessageSourceFlow. The agent then never grants a flow
//     the local Ollama exception of api_request, so every flow request passes the SSRF check.
//   - document_creator always gets block_remote_content, and Home Assistant services of the
//     script, shell_command, python_script and hassio domains are refused unless
//     home_assistant.allowed_services lists them (flowHAServiceRefusal).
//   - The outcome is classified for the flow engine (flowToolOutcome): known refusals become
//     denied or needs_setup, which the engine does not retry. A call whose context ended
//     without success returns the context's error, so the engine sees a cancel or a timeout
//     instead of a tool error.
type flowToolInvoker struct {
	s        *Server
	names    func(cfg *config.Config) map[string]bool
	dispatch func(ctx context.Context, tc *agent.ToolCall, dc *agent.DispatchContext) agent.ToolDispatchResult

	// cfgMu guards the flow configuration derived from the last snapshot.
	cfgMu   sync.Mutex
	cfgSrc  *config.Config
	cfgFlow *config.Config
}

func newFlowToolInvoker(s *Server, env *flowCatalogEnv) *flowToolInvoker {
	return &flowToolInvoker{s: s, names: env.toolNames,
		dispatch: func(ctx context.Context, tc *agent.ToolCall, dc *agent.DispatchContext) agent.ToolDispatchResult {
			return agent.DispatchToolCallResult(ctx, tc, dc, "")
		}}
}

// flowLogRunes bounds every user-supplied value the invoker logs or echoes.
const flowLogRunes = 200

// flowBoundRunes cuts s to at most maxRunes runes, marking a cut with an ellipsis.
func flowBoundRunes(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	n := 0
	for i := range s {
		if n == maxRunes {
			return s[:i] + "…"
		}
		n++
	}
	return s
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

// flowDispatchConfig returns the configuration flow tool calls run with: a shallow copy of
// cfg whose summary modes are off and whose preferred MCP web search is cleared. Only value
// fields of the copy are written (four bools and one struct value). Maps, slices and
// pointers are shared with cfg and, like the snapshot itself, never written by the
// dispatcher. AuthorizationSnapshots is copied with them, so agent.DispatchToolCallResult
// still intersects the copy's gates with the server's current snapshot; the summary and
// MCP fields are no gates and keep the copy's values. The copy is cached per snapshot.
func (i *flowToolInvoker) flowDispatchConfig(cfg *config.Config) *config.Config {
	i.cfgMu.Lock()
	defer i.cfgMu.Unlock()
	if i.cfgSrc == cfg && i.cfgFlow != nil {
		return i.cfgFlow
	}
	flowCfg := *cfg
	flowCfg.Tools.WebScraper.SummaryMode = false
	flowCfg.Tools.Wikipedia.SummaryMode = false
	flowCfg.Tools.DDGSearch.SummaryMode = false
	flowCfg.Tools.PDFExtractor.SummaryMode = false
	flowCfg.MCP.PreferredCapabilities.WebSearch = config.MCPPreferredToolSelection{}
	i.cfgSrc, i.cfgFlow = cfg, &flowCfg
	return &flowCfg
}

// flowHAScriptDomains are Home Assistant service domains that run scripts or change the
// system. A flow may call their services only when home_assistant.allowed_services lists
// the service itself.
var flowHAScriptDomains = map[string]bool{"shell_command": true, "python_script": true, "script": true, "hassio": true}

// flowHAServiceRefusal returns why a home_assistant call of a flow is refused, or "".
func flowHAServiceRefusal(cfg *config.Config, args map[string]any) string {
	text := func(key string) string {
		s, _ := args[key].(string)
		return strings.ToLower(strings.TrimSpace(s))
	}
	if op := text("operation"); op != "call_service" && op != "service" {
		return ""
	}
	domain, service := text("domain"), text("service")
	if d, s, ok := strings.Cut(service, "."); ok && (domain == "" || domain == d) {
		domain, service = d, s
	}
	if !flowHAScriptDomains[domain] {
		return ""
	}
	full := domain + "." + service
	for _, allowed := range cfg.HomeAssistant.AllowedServices {
		if strings.ToLower(strings.TrimSpace(allowed)) == full {
			return ""
		}
	}
	return "Home Assistant service " + flowBoundRunes(full, flowLogRunes) + " is refused for flows: services of the " +
		domain + " domain run only when home_assistant.allowed_services lists them"
}

// InvokeTool implements flows.ToolInvoker. Every call is checked against the node's allow
// list and the configured agent tools before it reaches the dispatcher.
func (i *flowToolInvoker) InvokeTool(ctx context.Context, req flows.ToolRequest) (flows.ToolResponse, error) {
	cfg := i.s.ConfigSnapshot()
	if cfg == nil {
		return flows.ToolResponse{}, flows.NewNodeError("FLOW_TOOLS_UNAVAILABLE", "the configuration is not ready")
	}
	allowed := false
	for _, name := range req.AllowedTools {
		allowed = allowed || name == req.Tool
	}
	if !allowed || req.Tool == flows.GotenbergTool {
		return flowToolRefusal(agent.ToolResultDenied, "the tool "+flowBoundRunes(req.Tool, flowLogRunes)+" is not allowed for this node"), nil
	}
	if a := flowToolAvailability(cfg, i.names(cfg), req.Tool); a.State != flows.AvailableState {
		return flowToolRefusal(agent.ToolResultNeedsSetup, "the tool "+flowBoundRunes(req.Tool, flowLogRunes)+" is not set up or is disabled"), nil
	}
	if err := ctx.Err(); err != nil {
		return flows.ToolResponse{}, err
	}
	action, skill := flowToolRoute(req.Tool)
	payload := make(map[string]any, len(req.Args))
	for k, v := range req.Args {
		payload[k] = v
	}
	switch req.Tool {
	case "home_assistant":
		if msg := flowHAServiceRefusal(cfg, payload); msg != "" {
			return flowToolRefusal(agent.ToolResultDenied, msg), nil
		}
	case "document_creator":
		// A JSON bool: the tool reads the flag with toolArgBool, which ignores text.
		payload["block_remote_content"] = true
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
	res := i.dispatch(ctx, &tc, i.dispatchContext(i.flowDispatchConfig(cfg), req.FlowID, action))
	resp := flowToolOutcome(req.Tool, res)
	if err := ctx.Err(); err != nil && resp.Status != string(agent.ToolResultSuccess) {
		return flows.ToolResponse{}, err
	}
	return resp, nil
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
		PreparationService: s.PreparationService, WorkspaceSearch: s.WorkspaceSearch, MessageSource: agent.MessageSourceFlow,
		ToolScopeRestricted: true, AllowedTools: map[string]struct{}{action: {}},
	}
}
