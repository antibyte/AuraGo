package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// ActionDescriptor is a credential-free description of one resolved action.
// The arguments hash binds approvals to the exact server-held call.
type ActionDescriptor struct {
	targets       []string
	targetUnknown bool
	egressSecret  bool   // A registered secret value sits in the arguments of an egress call.
	scheduling    bool   // High-risk only because it stores or starts agent work (schedulingActionHighRisk).
	Tool          string `json:"tool"`
	Operation     string `json:"operation"`
	Target        string `json:"target"`
	RunType       string `json:"run_type"`
	ScopeID       string `json:"scope_id,omitempty"`
	ArgumentsHash string `json:"arguments_hash"`
	HighRisk      bool   `json:"high_risk"`
	Mutating      bool   `json:"mutating"`
}

func DescribeAction(tc ToolCall, dc *DispatchContext) ActionDescriptor {
	raw, _ := json.Marshal(tc)
	digest := sha256.Sum256(raw)
	var args map[string]interface{}
	_ = json.Unmarshal(raw, &args)
	for key, value := range tc.Params {
		args[key] = value
	}
	value := func(keys ...string) string {
		for _, key := range keys {
			if s, ok := args[key].(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
		return ""
	}
	op := strings.ToLower(strings.TrimSpace(tc.Operation))
	if op == "" {
		op = strings.ToLower(value("operation", "sub_operation", "action_type"))
	}
	if op == "" {
		op = "execute"
	}
	op = canonicalPolicyOperation(tc.Action, op)
	a := ActionDescriptor{Tool: tc.Action, Operation: op, ArgumentsHash: hex.EncodeToString(digest[:]), RunType: "interactive"}
	if dc != nil {
		switch {
		case dc.IsMaintenance:
			a.RunType = "maintenance"
		case dc.IsCoAgent:
			a.RunType = "coagent"
		case dc.MessageSource == MessageSourceFlow:
			a.RunType, a.ScopeID = "flow", dc.FlowID
		case dc.MissionID != "":
			a.RunType, a.ScopeID = "mission", dc.MissionID
		case dc.MessageSource == "background" || isAutonomousMessageSource(dc.MessageSource):
			a.RunType = "background"
		}
	}
	a.Mutating = !readActionOperation(a.Operation)
	switch tc.Action {
	case "qr_code":
		a.Mutating = a.Operation != "presets" && a.Operation != "scan"
	case "discover_tools", "read_tool_manual", "query_inventory", "query_memory", "context_memory", "context_manager", "web_capture", "brave_search", "web_search", "web_scraper":
		a.Mutating = false
	case "execute_shell", "execute_python", "run_tool", "execute_skill", "run_agent_skill_script":
		a.Operation = "execute" // These tools have no caller-selectable operation.
		a.Mutating = true
		a.HighRisk = dc != nil && dc.Cfg != nil && (dc.Cfg.Agent.AllowUnsafeHostExecution || tc.Action == "execute_shell" && (dc.Cfg.Agent.AllowUnsandboxedShell || dc.Cfg.Agent.LegacyUnsandboxedShell))
		switch tc.TransportAction {
		case "git_push", "git_pull", "git_commit", "git_rollback":
			a.HighRisk = true
		}
	case "execute_sudo", "execute_remote_shell", "package_manager", "manage_updates":
		a.HighRisk = true
		if tc.Action == "execute_sudo" || tc.Action == "execute_remote_shell" {
			a.Operation, a.Mutating = "execute", true
		}
	case "send_email", "send_telegram", "send_discord", "send_rocketchat", "send_push", "send_push_notification", "send_notification", "send_sms", "send_voice", "send_document", "send_image", "send_audio", "send_video":
		a.Operation, a.Mutating, a.HighRisk = "execute", true, true
	case "agentmail", "agentmail_messages", "agentmail_drafts":
		switch a.Operation {
		case "send_message", "reply_message", "reply_all_message", "forward_message", "send_draft":
			a.Mutating, a.HighRisk = true, true
		}
	case "docker":
		if a.Operation == "exec" {
			a.Mutating, a.HighRisk = true, true
		}
	case "ansible":
		switch canonicalPolicyOperation("ansible", decodeAnsibleArgs(tc).Operation) {
		case "adhoc", "playbook":
			a.Mutating, a.HighRisk = true, true
		}
	case "api_request":
		request := decodeAPIRequestArgs(tc)
		method := strings.ToUpper(request.Method)
		if method == "" {
			method = "GET"
		}
		a.Operation = strings.ToLower(method)
		a.Mutating = method != "GET" && method != "HEAD" && method != "OPTIONS"
		a.HighRisk = a.Mutating || request.Body != "" || value("data") != ""
		for _, key := range []string{"body", "data"} {
			if object, ok := args[key].(map[string]interface{}); ok && len(object) > 0 {
				a.HighRisk = true
			}
		}
	case "virustotal", "virustotal_scan":
		a.Operation = strings.ToLower(value("mode"))
		if a.Operation != "upload" {
			a.Operation = "hash"
		}
		a.Mutating = a.Operation == "upload"
		a.HighRisk = a.Mutating
	case "form_automation", "browser_automation":
		a.HighRisk = a.Mutating && a.Operation != "screenshot" && a.Operation != "extract" && a.Operation != "navigate"
	}
	if a.Mutating {
		for _, risk := range []string{"delete", "remove", "destroy", "wipe", "format", "kill", "terminate", "shutdown", "power_off", "reboot", "restart", "send", "upload", "publish", "deploy", "execute_command", "run_command"} {
			if a.Operation == risk || strings.HasPrefix(a.Operation, risk+"_") || strings.HasSuffix(a.Operation, "_"+risk) || strings.Contains(a.Operation, "_"+risk+"_") {
				a.HighRisk = true
			}
		}
	}
	applyExtendedActionRisk(&a, tc, dc, args)
	a.targets, a.targetUnknown = canonicalActionTargets(tc, dc)
	a.Target = actionTargetSummary(a.targets)
	return a
}

func readActionOperation(op string) bool {
	switch op {
	case "get", "list", "status", "read", "query", "search", "inspect", "info", "capabilities", "logs", "history", "preview", "validate", "test", "test_connection", "discover", "browse", "download", "hash":
		return true
	}
	return strings.HasPrefix(op, "get_") || strings.HasPrefix(op, "list_") || strings.HasPrefix(op, "read_") || strings.HasPrefix(op, "query_") || strings.HasPrefix(op, "search_")
}

func actionRuleScopeMatches(rule config.ActionPolicyRule, a ActionDescriptor) bool {
	match := func(rule, actual string) bool {
		return rule == "*" || strings.EqualFold(strings.TrimSpace(rule), actual)
	}
	return match(rule.Tool, a.Tool) && match(rule.Operation, a.Operation) &&
		(rule.RunType == "" || rule.RunType == a.RunType) && (rule.ScopeID == "" || rule.ScopeID == a.ScopeID) &&
		(rule.ArgumentsHash == "" || rule.ArgumentsHash == a.ArgumentsHash)
}

func actionDenyRuleMatches(rule config.ActionPolicyRule, a ActionDescriptor) bool {
	if !actionRuleScopeMatches(rule, a) {
		return false
	}
	if rule.Target == "*" || strings.TrimSpace(rule.Target) == a.Target {
		return true
	}
	for _, denied := range actionTargetComponents(rule.Target) {
		for _, target := range a.targets {
			if denied == target {
				return true
			}
			if a.Tool == "docker" && strings.HasPrefix(denied, "container_id:") && strings.HasPrefix(target, "container_id:") &&
				(!immutableDockerContainerID(strings.TrimPrefix(denied, "container_id:")) || !immutableDockerContainerID(strings.TrimPrefix(target, "container_id:"))) {
				return true // Names and ID prefixes may refer to the same container.
			}
		}
	}
	// An unknown effective identity must not evade a concrete deny through a
	// broad allow. A decoder or an exact arguments hash is needed to disambiguate.
	return a.targetUnknown
}

func actionAllowRulesCover(rules []config.ActionPolicyRule, a ActionDescriptor) bool {
	covered := make(map[string]bool, len(a.targets))
	for _, rule := range rules {
		if !actionRuleScopeMatches(rule, a) {
			continue
		}
		target := strings.TrimSpace(rule.Target)
		if a.targetUnknown && rule.ArgumentsHash == "" && target != "*" && target != "tool:"+a.Tool && target != "host:local" {
			continue
		}
		if target == "*" || target == a.Target {
			return true
		}
		for _, permitted := range actionTargetComponents(target) {
			for _, actual := range a.targets {
				if permitted == actual {
					covered[actual] = true
				}
			}
		}
	}
	return len(a.targets) > 0 && len(covered) == len(a.targets)
}

func effectiveActionPolicy(cfg *config.Config) config.ActionPolicyConfig {
	policy := cfg.Agent.ActionPolicy
	if cfg.AuthorizationSnapshots != nil {
		_, current := cfg.AuthorizationSnapshots()
		if current != nil {
			policy = current.Agent.ActionPolicy
			// A policy enabled at run start is not silently disabled for that run.
			policy.Enabled = policy.Enabled || cfg.Agent.ActionPolicy.Enabled
		}
	}
	return policy
}

func actionPolicyRefusal(code, message string, status ToolResultStatus) *ToolDispatchResult {
	encoded, _ := json.Marshal(map[string]interface{}{"status": string(status), "code": code, "message": message})
	return &ToolDispatchResult{Output: "Tool Output: " + string(encoded), Status: status, IsError: status.IsError()}
}

// unavailableToolRefusal answers a call to a tool outside the run's effective
// catalog. A known native capability reports why it is off (code tool_disabled,
// hidden_reason) and gives no discovery advice, because discovery cannot enable
// it. Any other name is unknown (code unknown_tool) and points at discovery and
// the closest enabled tools.
func unavailableToolRefusal(name string, native bool, dc *DispatchContext) *ToolDispatchResult {
	shown := echoedToolName(name)
	body := map[string]interface{}{"status": string(ToolResultDenied)}
	if native {
		reason := "config_disabled"
		if catalog := GetToolCatalogState(dc.discoveryKey()); catalog != nil {
			if entry, ok := canonicalCatalogEntry(catalog, name); ok && !entry.Enabled && entry.HiddenReason != "" {
				reason = entry.HiddenReason
			}
		}
		body["code"] = "tool_disabled"
		if runPolicyHiddenReason(reason) {
			// Run policy, not configuration: enabling the tool changes nothing.
			body["message"] = `The tool "` + shown + `" is not available in this run.`
		} else {
			body["hidden_reason"] = reason
			body["message"] = `The tool "` + shown + `" is disabled (` + reason + `) and cannot run until an administrator enables it.`
		}
	} else {
		guidance, suggestions := unknownToolGuidance(dc, name)
		body["code"] = "unknown_tool"
		body["message"] = strings.TrimSpace(`"` + shown + `" is not a tool you can call directly in this run. ` + guidance)
		if len(suggestions) > 0 {
			body["suggestions"] = suggestions
		}
	}
	encoded, _ := json.Marshal(body)
	return &ToolDispatchResult{Output: "Tool Output: " + string(encoded), Status: ToolResultDenied, IsError: true}
}

func checkDispatchActionPolicy(ctx context.Context, tc ToolCall, dc *DispatchContext) *ToolDispatchResult {
	if err := ctx.Err(); err != nil {
		return actionPolicyRefusal("cancelled", "The action was cancelled before execution.", ToolResultCancelled)
	}
	if dc == nil {
		return actionPolicyRefusal("policy_unavailable", "Execution policy is unavailable.", ToolResultDenied)
	}
	if !dispatchToolAllowed(dc, tc.Action) || tc.TransportAction != "" && !dispatchToolAllowed(dc, tc.TransportAction) {
		denied := tc.Action
		if dispatchToolAllowed(dc, denied) {
			denied = tc.TransportAction
		}
		return scopeDeniedRefusal(dc, denied)
	}
	if reason := roleToolRestriction(tc, dc); reason != "" {
		return &ToolDispatchResult{Output: reason, Status: ToolResultDenied, IsError: true}
	}
	if dc.Cfg == nil {
		return actionPolicyRefusal("policy_unavailable", "Execution policy is unavailable.", ToolResultDenied)
	}
	a := DescribeAction(tc, dc)
	if a.Mutating && strings.Contains(tc.Action, "config") {
		payload, _ := json.Marshal(tc)
		if strings.Contains(strings.ToLower(string(payload)), "action_policy") || strings.Contains(strings.ToLower(string(payload)), "injection_guard") {
			return actionPolicyRefusal("operator_configuration_required", "Action policy rules can only be changed through the administrator configuration UI/API.", ToolResultDenied)
		}
	}
	policy := effectiveActionPolicy(dc.Cfg)
	enabled := false
	for _, schema := range dispatchCatalogSchemas(dc) {
		if schema.Function != nil && canonicalActionName(schema.Function.Name) == tc.Action {
			enabled = true
			break
		}
	}
	private := dc.ExecutionHooks != nil && dc.ToolScopeRestricted && dispatchToolAllowed(dc, tc.Action) && (strings.HasPrefix(tc.Action, "game_maker_") || strings.HasPrefix(tc.Action, "cyd_creator_"))
	missingCatalog := !enabled && !private
	if missingCatalog {
		// Private prepared-profile hooks retain their existing policy-off route;
		// native capabilities must remain gated even without the extra policy.
		native := false
		for name := range allBuiltinToolNameSet() {
			native = native || canonicalActionName(name) == tc.Action
		}
		if policy.Enabled || native {
			if toolRuntimeUnavailable(dc, tc.Action) == "" || !configuredNativeToolEnabled(dc, tc.Action) {
				return unavailableToolRefusal(tc.Action, native, dc)
			}
		}
	}
	if a.Mutating && catalogReadOnly(dc.Cfg, &ToolCatalogEntry{Name: tc.Action}) {
		return actionPolicyRefusal("readonly", "The integration is read-only; approval cannot override this restriction.", ToolResultDenied)
	}
	if missingCatalog || dc.ExecutionHooks == nil || dc.ExecutionHooks.HandleTool == nil {
		if reason := toolRuntimeUnavailable(dc, tc.Action); reason != "" {
			return actionPolicyRefusal("runtime_dependency_unavailable", reason, ToolResultNeedsSetup)
		}
	}
	if policy.Enabled {
		for _, rule := range policy.Deny {
			if actionDenyRuleMatches(rule, a) {
				return actionPolicyRefusal("action_policy_denied", "An administrator action rule denies this operation and target.", ToolResultDenied)
			}
		}
	}
	// A registered secret never leaves through an egress call, with policy off, an
	// allow rule or an approval alike.
	if a.egressSecret {
		return egressSecretRefusal(dc, a)
	}
	if privateURLExfiltration(tc, dc) {
		return actionPolicyRefusal("possible_exfiltration", "The URL contains private conversation, file or email content after an external read. Use web_search for research; an exact URL must be supplied by the authenticated user.", ToolResultDenied)
	}
	// A copied isolated span waits even when policy is off and even when an allow rule matches.
	if stop, refusal := gateCopiedIsolatedSpan(ctx, tc, dc, a); stop {
		return refusal
	}
	if !policy.Enabled {
		return nil
	}
	if actionAllowRulesCover(policy.Allow, a) {
		return nil
	}
	// treg already requires endpoint/method/path/action-class grants in its leaf.
	// Those exact first-party grants remain authoritative and are rechecked there.
	if tc.Action == "treg_call" {
		return nil
	}
	// An unattended fetch or probe is not high-risk, but when its URL or host carries
	// text copied from earlier isolated output it needs a rule that names it.
	fetchCopy := !a.HighRisk && a.RunType != "interactive" && copiedFetchSpan(tc, dc)
	if !a.HighRisk && !fetchCopy {
		return nil
	}
	if grant, ok := ctx.Value(approvedActionKey{}).(approvedActionGrant); ok && grant.SessionID == dc.SessionID && grant.ArgumentsHash == a.ArgumentsHash {
		if grant.expired() {
			return actionPolicyRefusal("approval_expired", "The approval expired before execution began. No action was executed.", ToolResultDenied)
		}
		return nil
	}
	if preview, _ := ctx.Value(actionPreviewKey{}).(bool); preview {
		return actionPolicyRefusal("action_rule_required", "A matching administrator rule or one-shot approval is required.", ToolResultDenied)
	}
	if fetchCopy {
		return actionPolicyRefusal("action_rule_required", "This unattended request carries text copied from earlier isolated tool output in its URL or host. It needs an administrator rule for its tool, operation and target. It was blocked without waiting for approval.", ToolResultDenied)
	}
	if a.RunType != "interactive" {
		// Scheduling is the one class a clean unattended run may do on its own:
		// the operator wrote the mission or cron prompt, and a session that read
		// third-party content within the taint window, or whose parent session did
		// (a co-agent), is the case to refuse.
		if a.scheduling && !taintedForQueuedRun(dc) {
			return nil
		}
		return actionPolicyRefusal("action_rule_required", "This high-risk unattended action needs an administrator rule for its tool, operation and target. It was blocked without waiting for approval.", ToolResultDenied)
	}
	return requestActionApproval(tc, dc, a)
}

func safeActionTarget(target string) string {
	return security.RedactSensitiveInfo(security.Scrub(target))
}

func withBackgroundCodeAction(ctx context.Context, tc ToolCall, dc *DispatchContext) context.Context {
	switch tc.Action {
	case "execute_shell", "execute_python", "run_tool":
	default:
		return ctx
	}
	call, err := json.Marshal(tc)
	if err != nil {
		return ctx
	}
	names := func(set map[string]struct{}) []string {
		result := make([]string, 0, len(set))
		for name := range set {
			result = append(result, name)
		}
		sort.Strings(result)
		return result
	}
	return tools.WithBackgroundCodeAction(ctx, tools.BackgroundCodeAction{
		Call: call, TransportAction: tc.TransportAction,
		SessionID: dc.SessionID, MissionID: dc.MissionID, FlowID: dc.FlowID,
		MessageSource: dc.MessageSource, IsMaintenance: dc.IsMaintenance,
		IsCoAgent: dc.IsCoAgent, ParentIsMaintenance: dc.ParentIsMaintenance, CoAgentSpecialist: dc.CoAgentSpecialist,
		AllowedTools: names(dc.AllowedTools), ToolScopeRestricted: dc.ToolScopeRestricted,
		AllowedAgentSkills: names(dc.AllowedAgentSkills), SkillScopeRestricted: dc.SkillScopeRestricted,
	})
}

// ValidateBackgroundCodeAction rechecks a frozen queued call against current
// configuration without inheriting or creating a one-shot approval.
func ValidateBackgroundCodeAction(ctx context.Context, cfg *config.Config, logger *slog.Logger, action tools.BackgroundCodeAction) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if cfg == nil || action.SessionID == "" || len(action.Call) == 0 || len(action.Call) > 1<<20 {
		return fmt.Errorf("%w: background action identity is unavailable", tools.ErrBackgroundTaskBlocked)
	}
	if cfg.AuthorizationSnapshots != nil {
		_, current := cfg.AuthorizationSnapshots()
		if current == nil {
			return fmt.Errorf("%w: current policy is unavailable", tools.ErrBackgroundTaskBlocked)
		}
		cfg = current
	}
	var tc ToolCall
	if err := json.Unmarshal(action.Call, &tc); err != nil {
		return fmt.Errorf("%w: invalid background action", tools.ErrBackgroundTaskBlocked)
	}
	switch tc.Action {
	case "execute_shell", "execute_python", "run_tool":
	default:
		return fmt.Errorf("%w: invalid background execution tool", tools.ErrBackgroundTaskBlocked)
	}
	if tc.Action == "execute_shell" && !cfg.Agent.AllowShell || tc.Action != "execute_shell" && !cfg.Agent.AllowPython {
		return fmt.Errorf("%w: background code capability is disabled", tools.ErrBackgroundTaskBlocked)
	}
	tc.TransportAction = action.TransportAction
	source := action.MessageSource
	if action.MissionID == "" && action.FlowID == "" && !action.IsMaintenance && !action.IsCoAgent {
		source = "background"
	}
	dc := &DispatchContext{
		Cfg: cfg, Logger: logger, SessionID: action.SessionID, MissionID: action.MissionID, FlowID: action.FlowID,
		MessageSource: source, IsMaintenance: action.IsMaintenance, IsCoAgent: action.IsCoAgent,
		ParentIsMaintenance: action.ParentIsMaintenance, CoAgentSpecialist: action.CoAgentSpecialist,
		AllowedTools: normalizedAllowedToolSet(action.AllowedTools), ToolScopeRestricted: action.ToolScopeRestricted,
		AllowedAgentSkills: normalizedAllowedToolSet(action.AllowedAgentSkills), SkillScopeRestricted: action.SkillScopeRestricted,
	}
	tc = prepareToolCall(tc, dc)
	if tc.PreparationError != "" {
		return fmt.Errorf("%w: background arguments are invalid", tools.ErrBackgroundTaskBlocked)
	}
	ctx = context.WithValue(ctx, approvedActionKey{}, approvedActionGrant{})
	ctx = context.WithValue(ctx, actionPreviewKey{}, true)
	if refusal := checkDispatchActionPolicy(ctx, tc, dc); refusal != nil {
		action := DescribeAction(tc, dc)
		return fmt.Errorf("%w: %s/%s (%s): %s", tools.ErrBackgroundTaskBlocked, action.Tool, action.Operation, safeActionTarget(action.Target), refusal.Output)
	}
	return nil
}
