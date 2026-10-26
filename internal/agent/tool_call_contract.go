package agent

import (
	"context"
	"encoding/json"
	"strings"

	"aurago/internal/security"
)

type toolOutcomeKey struct{}

// setToolOutcome records a handler-owned execution result. This keeps success
// and failure semantics independent from the model-facing prose returned by
// legacy string handlers.
func setToolOutcome(ctx context.Context, status ToolResultStatus) {
	if ctx == nil {
		return
	}
	if outcome, ok := ctx.Value(toolOutcomeKey{}).(*ToolResultStatus); ok && outcome != nil {
		*outcome = status
	}
}

// externalToolOutput receives a locally constructed result envelope, before any
// presentation escaping. Remote payloads must remain nested inside that envelope.
func externalToolOutput(ctx context.Context, raw string) string {
	if status := classifyLegacyToolResult(raw); status != ToolResultUnknown {
		setToolOutcome(ctx, status)
	}
	return "Tool Output: " + security.IsolateExternalData(security.Scrub(raw))
}

// ToolResultStatus describes execution, independently of output presentation.
type ToolResultStatus string

const (
	ToolResultUnknown    ToolResultStatus = "unclassified"
	ToolResultSuccess    ToolResultStatus = "success"
	ToolResultFailed     ToolResultStatus = "failed"
	ToolResultDenied     ToolResultStatus = "denied"
	ToolResultNeedsSetup ToolResultStatus = "needs_setup"
	ToolResultCancelled  ToolResultStatus = "cancelled"
	ToolResultDeferred   ToolResultStatus = "deferred"
)

func (s ToolResultStatus) IsError() bool {
	return s == ToolResultFailed || s == ToolResultDenied || s == ToolResultNeedsSetup || s == ToolResultCancelled
}

// prepareToolCall resolves transport wrappers before policy, task rules, hooks,
// and effect tracking. Protocol identity is retained; resolution never executes.
func prepareToolCall(tc ToolCall, dc *DispatchContext) ToolCall {
	if tc.PreparationError != "" {
		return tc
	}
	if tc.NativeArgsMalformed {
		tc.PreparationError = `Tool Output: {"status":"error","code":"invalid_arguments","message":"Supply a complete JSON object matching the tool schema."}`
		return tc
	}
	tc = normalizeDirectToolCall(tc, dc)
	tc.Operation = toolCallSnapshotOperation(tc)
	if tc.Action != "invoke_tool" {
		tc = canonicalizeLeafCall(tc, dc)
		markDirectHiddenNativeCall(tc, dc)
		return tc
	}
	fail := func(message string) ToolCall {
		tc.PreparationError = message
		return tc
	}
	if !dispatchToolAllowed(dc, "invoke_tool") {
		return fail(toolScopeDeniedFor(dc, "invoke_tool"))
	}
	name := stringValueFromMap(tc.Params, "tool_name", "name", "tool")
	if strings.HasPrefix(name, "git_") && canonicalActionName(name) == "execute_shell" {
		return fail(`Tool Output: {"status":"policy_denied","code":"tool_not_found","message":"Git compatibility aliases are not catalog tools. Invoke execute_shell with an exact command instead."}`)
	}
	entry, ok := preparationCatalogEntry(name, dc)
	if !ok || name == "" {
		fields := map[string]interface{}{}
		if _, suggestions := unknownToolGuidance(dc, name); len(suggestions) > 0 {
			fields["suggestions"] = suggestions
		}
		return fail(invokeToolRefusal("error", "tool_not_found", "Use discover_tools to select an unambiguous tool name.", fields))
	}
	if entry.Name == "invoke_tool" {
		return fail(invokeToolRefusal("policy_denied", "recursive_invoke", "invoke_tool cannot invoke itself; call the target tool or name another tool_name.", nil))
	}
	// Scope is checked before availability, so a tool outside the run's scope
	// answers exactly like a direct call to it and never discloses its state.
	if !catalogEntryAllowed(dc, entry) {
		return fail(toolScopeDeniedFor(dc, entry.Name))
	}
	if entry.Status == ToolStatusNeedsSetup {
		return fail(toolSetupRequiredOutput(entry.HiddenReason))
	}
	if !entry.Enabled || entry.Status == ToolStatusDisabled {
		shown := echoedToolName(entry.Name)
		if runPolicyHiddenReason(entry.HiddenReason) {
			return fail(invokeToolRefusal("policy_denied", "tool_disabled", `Tool "`+shown+`" is not available in this run.`, nil))
		}
		fields := map[string]interface{}{}
		if entry.HiddenReason != "" {
			fields["hidden_reason"] = entry.HiddenReason
		}
		return fail(invokeToolRefusal("policy_denied", "tool_disabled", `Tool "`+shown+`" is disabled and cannot run until it is enabled.`, fields))
	}
	args := mapValueFromMap(tc.Params, "arguments", "params", "skill_args")
	if args == nil {
		args = flattenedInvokeArgs(tc.Params)
		logInvokeToolArgumentSource(dc.Logger, name, "flattened", tc.Params, args)
	}
	args = cloneJSONSchemaValue(args).(map[string]interface{})
	stripStrictNulls(args, originalToolArgumentSchema(entry.Name, dc))
	if entry.Kind == ToolKindNative && entry.Status == ToolStatusHidden {
		MarkDiscoverRequestedTool(dc.discoveryKey(), entry.Name)
	}
	action := entry.Routing.NativeAction
	if action == "" {
		action = entry.Name
	}
	routed := toolCallFromInvokeArgs(action, args)
	switch entry.Kind {
	case ToolKindPackage:
		routed = toolCallFromInvokeArgs("activate_agent_skill", map[string]interface{}{"name": entry.Routing.SkillName})
	case ToolKindMCP:
		if entry.Routing.MCPTool != "" {
			routed = toolCallFromInvokeArgs("mcp_call", map[string]interface{}{"operation": "call_tool", "server": entry.Routing.MCPServer, "tool_name": entry.Routing.MCPTool, "args": args})
		}
	case ToolKindSkill:
		routed = ToolCall{Action: "execute_skill", Skill: entry.Routing.SkillName, SkillArgs: args, Params: args}
	case ToolKindCustom:
		routed = toolCallFromInvokeArgs("run_tool", normalizeCustomToolShortcutArgs(entry.Routing.CustomName, args))
	}
	routed.TransportAction = "invoke_tool"
	routed.NativeCallID, routed.RawJSON = tc.NativeCallID, tc.RawJSON
	routed.NativeArgsMalformed, routed.NativeArgsError, routed.NativeArgsRaw = tc.NativeArgsMalformed, tc.NativeArgsError, tc.NativeArgsRaw
	routed.IsTool, routed.Todo = tc.IsTool, tc.Todo
	routed.Operation = toolCallSnapshotOperation(routed)
	return canonicalizeLeafCall(routed, dc)
}

// runPolicyHiddenReason reports whether a disabled catalog entry is off because
// of this run's policy (role, scope or a live authorization change) rather than
// because of the configuration. An administrator enabling the tool would not
// change a run policy, so the model is neither told to wait for that nor shown
// the reason.
func runPolicyHiddenReason(reason string) bool {
	switch reason {
	case "role_restricted", "authorization_changed", "out_of_scope":
		return true
	}
	return false
}

// invokeToolRefusal encodes an invoke_tool preparation failure. fields adds keys
// beyond status, code and message.
func invokeToolRefusal(status, code, message string, fields map[string]interface{}) string {
	body := map[string]interface{}{"status": status, "code": code, "message": message}
	for key, value := range fields {
		body[key] = value
	}
	encoded, _ := json.Marshal(body)
	return "Tool Output: " + string(encoded)
}

// markDirectHiddenNativeCall requests schema re-injection for a hidden native
// tool that the model called by its own name, exactly as an invoke_tool call or
// a discover_tools lookup does. It changes presentation only; the call itself
// still runs through the same policy checks. Only a run that re-injects schemas
// (dc.discoveryReinjects) marks, and only for an enabled, in-scope entry of the
// run catalog under that exact name.
func markDirectHiddenNativeCall(tc ToolCall, dc *DispatchContext) {
	if dc == nil || !dc.discoveryReinjects || tc.PreparationError != "" || tc.TransportAction != "" {
		return
	}
	catalog := GetToolCatalogState(dc.discoveryKey())
	if catalog == nil {
		return
	}
	entry, ok := catalog.Get(tc.Action)
	if !ok || entry.Name != tc.Action || entry.Kind != ToolKindNative || !entry.Enabled || entry.Status != ToolStatusHidden ||
		entry.Schema.Function == nil || !catalogEntryAllowed(dc, entry) {
		return
	}
	MarkDiscoverRequestedTool(dc.discoveryKey(), entry.Name)
}

type legacyOutcomeEnvelope struct {
	Status   string `json:"status"`
	Code     string `json:"code"`
	Success  *bool  `json:"success"`
	ExitCode *int   `json:"exit_code"`
}

// firstOutcomeEnvelope decodes the outcome keys of a JSON result envelope using
// their FIRST occurrence. Handler templates write the outcome first, while
// encoding/json keeps the last duplicate (matching keys case-insensitively), so
// a value interpolated into a hand-built template could otherwise append
// `"status":"ok"` and turn a failure into a success.
func firstOutcomeEnvelope(value string) (legacyOutcomeEnvelope, bool) {
	var envelope legacyOutcomeEnvelope
	if json.Unmarshal([]byte(value), &envelope) != nil {
		return envelope, false
	}
	decoder := json.NewDecoder(strings.NewReader(value))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return envelope, true
	}
	seen := make(map[string]bool, 4)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return envelope, true
		}
		key, _ := token.(string)
		key = strings.ToLower(key)
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return envelope, true
		}
		if seen[key] {
			continue
		}
		switch key {
		case "status":
			var first string
			if json.Unmarshal(raw, &first) == nil {
				envelope.Status = first
			}
		case "code":
			var first string
			if json.Unmarshal(raw, &first) == nil {
				envelope.Code = first
			}
		case "success":
			var first *bool
			if json.Unmarshal(raw, &first) == nil {
				envelope.Success = first
			}
		case "exit_code":
			var first *int
			if json.Unmarshal(raw, &first) == nil {
				envelope.ExitCode = first
			}
		default:
			continue
		}
		seen[key] = true
	}
	return envelope, true
}

// classifyLegacyToolResult is the single adapter for string-returning handlers.
// Unknown prose is retained, but is not evidence for success-based learning.
func classifyLegacyToolResult(output string) ToolResultStatus {
	value := strings.TrimSpace(output)
	for {
		previous := value
		for _, prefix := range []string{"[Tool Output]", "Tool Output:", "Agent Skill script result:"} {
			value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
		}
		if previous == value {
			break
		}
	}
	if envelope, ok := firstOutcomeEnvelope(value); ok {
		for _, status := range []string{envelope.Code, envelope.Status} {
			switch strings.ToLower(strings.TrimSpace(status)) {
			case "policy_denied", "tool_scope_denied", "permission_denied", "denied", "blocked":
				return ToolResultDenied
			case "connect_required", "needs_setup", "not_configured", "disconnected":
				return ToolResultNeedsSetup
			case "cancelled", "canceled":
				return ToolResultCancelled
			case "pending", "deferred", "queued", "not_executed":
				return ToolResultDeferred
			case "error", "failed", "failure":
				return ToolResultFailed
			}
		}
		if envelope.Success != nil && !*envelope.Success || envelope.ExitCode != nil && *envelope.ExitCode != 0 {
			return ToolResultFailed
		}
		if envelope.Success != nil && *envelope.Success || envelope.ExitCode != nil && *envelope.ExitCode == 0 {
			return ToolResultSuccess
		}
		switch strings.ToLower(envelope.Status) {
		case "ok", "success", "completed":
			return ToolResultSuccess
		}
	}
	lower := strings.ToLower(value)
	for _, prefix := range []string{"[permission denied]", "[tool blocked]"} {
		if strings.HasPrefix(lower, prefix) {
			return ToolResultDenied
		}
	}
	for _, prefix := range []string{"[error]", "[execution error]", "error:", "error ", "timeout:"} {
		if strings.HasPrefix(lower, prefix) {
			return ToolResultFailed
		}
	}
	return ToolResultUnknown
}
