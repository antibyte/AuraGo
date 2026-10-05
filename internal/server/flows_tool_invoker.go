package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
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
//   - Arguments: mqtt_publish gets qos as an int and retain as a bool; a typed ToolCall field
//     an argument does not fit is left empty, as on the agent's own invoke path, and the
//     tool reads the argument from Params. The invoker never logs arguments; the dispatcher's
//     own log lines go through flowLogHandler (bounded values, no URL query, no headers or
//     bodies). The secrets a call reads (bot tokens, the ntfy topic, passwords, keys) are
//     registered with the output scrubber before it runs and redacted from its output.
//   - send_telegram with a file: when the text went out as a message of its own and the
//     document then failed ("text_sent": true), the call is denied, which the engine does
//     not retry, and a later attempt of the same node in the same run is refused without a
//     dispatch. The engine retries a node whose attempt timed out whatever the code says,
//     so the memory is what keeps the text from being sent twice.
//   - Documents: a file.read or doc.pdf_read of a documents-folder path (what doc.pdf_create
//     returns) reads a scratch copy in the workspace, made through the attachment jail
//     (bridgeDocument, flows_tool_invoker_documents.go).
//
// send_email: the SMTP delivery (deliverSMTP) takes no context, so the dispatch, and with it
// InvokeTool, returns only when the SMTP session has ended: at most about 7 minutes for the
// session of the largest mail, 10 minutes for the final reply and 10 seconds for QUIT. The
// node's per-attempt timeout cannot cut that short, and the engine starts a retry only after
// the attempt returned (executeNode runs the attempts one after another on one goroutine),
// so two attempts of one email node never overlap. When the run itself is cancelled or
// times out, the engine abandons the node 30 seconds later and does not retry it; the mail
// may still go out in the background. A failure reported after the server took the mail (the
// answer was lost) is retried and can send the mail twice, as emailDef in
// internal/flows/catalog_actions_notify.go says; a QUIT failure after acceptance counts as
// sent.
type flowToolInvoker struct {
	s        *Server
	names    func(cfg *config.Config) map[string]bool
	dispatch func(ctx context.Context, tc *agent.ToolCall, dc *agent.DispatchContext) agent.ToolDispatchResult

	// sentMu guards textSent: the run and node of each send_telegram whose text went out
	// while its document failed (see InvokeTool).
	sentMu   sync.Mutex
	textSent map[string]time.Time
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
// MCP fields are no gates and keep the copy's values. The copy is made for every call and
// not cached: some handlers write into the live snapshot in place (the Ansible token, MCP
// secrets, the indexing folders), and a cached copy would keep the old values.
func flowDispatchConfig(cfg *config.Config) *config.Config {
	flowCfg := *cfg
	flowCfg.Tools.WebScraper.SummaryMode = false
	flowCfg.Tools.Wikipedia.SummaryMode = false
	flowCfg.Tools.DDGSearch.SummaryMode = false
	flowCfg.Tools.PDFExtractor.SummaryMode = false
	flowCfg.MCP.PreferredCapabilities.WebSearch = config.MCPPreferredToolSelection{}
	return &flowCfg
}

// flowHAScriptDomains are Home Assistant service domains that run scripts or change the
// system. A flow may call their services only when home_assistant.allowed_services lists
// the service itself.
var flowHAScriptDomains = map[string]bool{"shell_command": true, "python_script": true, "script": true, "hassio": true}

// flowFirstArg mirrors the agent decoders' firstNonEmptyToolString(typed,
// toolArgString(params, key)): the typed ToolCall field (which JSON fills from a key of any
// case, "Domain" included), else the exact key in Params.
func flowFirstArg(typed string, params map[string]any, key string) string {
	if typed != "" {
		return typed
	}
	s, _ := params[key].(string)
	return s
}

// flowHAServiceRefusal returns why a home_assistant call of a flow is refused, or "". It
// reads operation, domain and service from the built tool call the way
// decodeHomeAssistantArgs does, so a key in another case ("Domain", "OPERATION") cannot
// slip past it.
func flowHAServiceRefusal(cfg *config.Config, tc agent.ToolCall) string {
	text := func(typed, key string) string {
		return strings.ToLower(strings.TrimSpace(flowFirstArg(typed, tc.Params, key)))
	}
	if op := text(tc.Operation, "operation"); op != "call_service" && op != "service" {
		return ""
	}
	domain, service := text(tc.Domain, "domain"), text(tc.Service, "service")
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
	sentKey := ""
	if req.Tool == "send_telegram" {
		sentKey = flowTextSentKey(req)
		if i.textAlreadySent(sentKey) {
			return flowToolRefusal(agent.ToolResultDenied, "the text of this Telegram message went out in an earlier attempt of the node "+
				"and its document failed; the node is not tried again, so the text is not sent twice"), nil
		}
	}
	action, skill := flowToolRoute(req.Tool)
	payload := make(map[string]any, len(req.Args))
	for k, v := range req.Args {
		payload[k] = v
	}
	switch req.Tool {
	case "document_creator":
		// A JSON bool: the tool reads the flag with toolArgBool, which ignores text.
		payload["block_remote_content"] = true
	case "mqtt_publish":
		flowNormalizeMQTTArgs(payload)
	}
	cleanup, refusal := bridgeDocument(cfg, req, payload)
	defer cleanup()
	if refusal != nil {
		return *refusal, nil
	}
	tc, err := flowToolCall(action, skill, payload)
	if err != nil {
		return flows.ToolResponse{}, err
	}
	if req.Tool == "home_assistant" {
		if msg := flowHAServiceRefusal(cfg, tc); msg != "" {
			return flowToolRefusal(agent.ToolResultDenied, msg), nil
		}
	}
	secrets, release := flowRegisterSecrets(cfg, req.Tool)
	defer release()
	res := i.dispatch(ctx, &tc, i.dispatchContext(flowDispatchConfig(cfg), req.FlowID, action))
	res.Output = flowRedactSecrets(res.Output, secrets)
	textSent := sentKey != "" && flowTelegramTextSent(res.Output)
	if textSent {
		i.rememberTextSent(sentKey, time.Now())
	}
	resp := flowToolOutcome(req.Tool, res, i.s.Guardian != nil)
	if err := ctx.Err(); err != nil && resp.Status != string(agent.ToolResultSuccess) {
		return flows.ToolResponse{}, err
	}
	if textSent {
		resp.Status, resp.IsError = string(agent.ToolResultDenied), true
	}
	return resp, nil
}

// flowMaxTypeErrors bounds how many arguments flowToolCall leaves out of the typed decode.
const flowMaxTypeErrors = 16

// flowToolCall builds the dispatcher's tool call. Typed ToolCall fields are filled from the
// arguments where they fit; Params keeps all arguments with their Go types (the invoker's
// copy) and stays authoritative, and the tools read them from there. ToolCall's decoder
// keeps nothing when one argument does not fit its typed field (send_notification's
// priority "normal" against ToolCall.Priority, an int), which is what the agent's own
// invoke path (toolCallFromInvokeArgs) ends up with; here such an argument is left out of
// the typed decode only, and the decode is repeated, so every other typed field is filled.
func flowToolCall(action, skill string, payload map[string]any) (agent.ToolCall, error) {
	typed := make(map[string]any, len(payload))
	for k, v := range payload {
		typed[k] = v
	}
	var tc agent.ToolCall
	for attempt := 0; ; attempt++ {
		encoded := any(typed)
		if skill != "" {
			encoded = map[string]any{"skill": skill, "skill_args": typed}
		}
		raw, err := json.Marshal(encoded)
		if err != nil {
			return agent.ToolCall{}, flows.NewNodeError("FLOW_PARAM_INVALID", "the tool arguments cannot be encoded: %v", err)
		}
		tc = agent.ToolCall{}
		err = json.Unmarshal(raw, &tc)
		if err == nil {
			break
		}
		var typeErr *json.UnmarshalTypeError
		if !errors.As(err, &typeErr) {
			return agent.ToolCall{}, flows.NewNodeError("FLOW_PARAM_INVALID", "the tool arguments are invalid: %v", err)
		}
		field, _, _ := strings.Cut(typeErr.Field, ".")
		removed := false
		for k := range typed {
			if strings.EqualFold(k, field) {
				delete(typed, k)
				removed = true
			}
		}
		if !removed || attempt == flowMaxTypeErrors {
			tc = agent.ToolCall{}
			break
		}
	}
	tc.Action, tc.IsTool = action, true
	if skill != "" {
		tc.Skill, tc.SkillArgs, tc.Params = skill, payload, map[string]any{"skill": skill, "skill_args": payload}
	} else {
		tc.Params = payload
		if tc.Operation == "" {
			tc.Operation, _ = payload["operation"].(string)
		}
	}
	return tc, nil
}

// dispatchContext mirrors buildLooperRuntime with a scope of exactly one tool.
func (i *flowToolInvoker) dispatchContext(cfg *config.Config, flowID, action string) *agent.DispatchContext {
	s := i.s
	return &agent.DispatchContext{
		Cfg: cfg, Logger: flowDispatchLogger(s.Logger), LLMClient: s.LLMClient, Vault: s.Vault, Registry: s.Registry,
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
