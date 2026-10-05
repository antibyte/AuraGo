package server

import (
	"encoding/json"
	"strings"

	"aurago/internal/agent"
	"aurago/internal/flows"
)

// How a flow sees the outcome of a tool call.
//
// The flows side (callTool in internal/flows/catalog_tools.go) classifies a response by its
// Status first: "denied" and "policy_denied" give FLOW_TOOL_DENIED and "needs_setup" gives
// FLOW_NODE_UNAVAILABLE, and the engine retries neither. Anything else that failed becomes
// the retried FLOW_TOOL_ERROR. agent.DispatchToolCallResult already classifies the raw tool
// result (classifyLegacyToolResult: JSON envelopes, "[PERMISSION DENIED]", "[TOOL BLOCKED]",
// "ERROR …" and friends). flowToolOutcome adds what a flow needs on top:
//   - the same plain-text shapes again when the status came back empty or unclassified
//     (generic nodes do not check for success themselves, so a refusal in plain text must
//     not pass as output);
//   - refusals that the tools report as an ordinary error turned into denied or
//     needs_setup (flowRefusalRules), so a retry does not pay for the same refusal again;
//   - a notification answer whose channels all failed for such a reason, although the
//     top-level status says success.
//
// It never changes the output of a call that succeeded, and it never parses a large
// output: refusals are short, and the flows side parses every output again anyway.

// flowStatusParseLimit is the largest output flowToolOutcome looks into.
const flowStatusParseLimit = 64 << 10

// flowRefusalMessageBytes is how much of a tool's message the refusal rules look at. The
// refusal texts start the message or follow a short subject.
const flowRefusalMessageBytes = 500

// flowRefusalRule maps the text of a tool's own refusal to a non-retryable status. text is
// lower case; prefix rules must start the message, the others may sit anywhere in its first
// flowRefusalMessageBytes bytes. The first matching rule wins, so the denied rules come first.
type flowRefusalRule struct {
	text   string
	prefix bool
	status agent.ToolResultStatus
}

// flowRefusalRules are the refusal texts of the tools flows call, with the file and line of
// each real message (flows_tool_invoker_hardening_test.go checks every one of them).
var flowRefusalRules = []flowRefusalRule{
	// "Tool Output: [PERMISSION DENIED] filesystem write operations are disabled in Danger Zone
	// settings …" (internal/agent/dispatch_filesystem.go:128), a protected file (:93),
	// web_scraper switched off (internal/agent/tool_builtin_handlers.go:27). The dispatcher
	// classifies these itself; the rule covers an unclassified status.
	{"[permission denied]", true, agent.ToolResultDenied},
	// The Guardian's block (internal/agent/agent_parse.go:52).
	{"[tool blocked]", true, agent.ToolResultDenied},
	// "filesystem write is disabled by runtime permissions": the write_file refusal, which has
	// no error_code (internal/tools/filesystem.go:522-523 via internal/tools/permissions.go:244);
	// "mqtt publish is disabled by runtime permissions" (internal/tools/permissions.go:364).
	{"is disabled by runtime permissions", false, agent.ToolResultDenied},
	// The filesystem tool's path refusals, which a write reports without error_code
	// (internal/tools/filesystem.go:296 and :308).
	{"is an absolute path outside the project root", false, agent.ToolResultDenied},
	{"escapes the project root", false, agent.ToolResultDenied},
	// The SSRF block of api_request (internal/tools/api_client.go:82).
	{"url validation failed:", true, agent.ToolResultDenied},
	// Read-only integrations: Home Assistant (internal/agent/agent_dispatch_services.go:1385,
	// internal/tools/homeassistant.go:186), MQTT (internal/agent/dispatch_network.go:174),
	// Discord (internal/agent/dispatch_messaging.go:109, internal/tools/notification.go:96) and
	// Telnyx as a notification channel (internal/tools/notification.go:104).
	{"is in read-only mode", false, agent.ToolResultDenied},
	// Home Assistant's service lists (internal/agent/home_assistant_policy.go:12 and :23).
	{"is blocked by home_assistant.blocked_services", false, agent.ToolResultDenied},
	{"is not allowed by home_assistant.allowed_services", false, agent.ToolResultDenied},
	// Integrations that are switched off: Home Assistant
	// (internal/agent/agent_dispatch_services.go:1379), MQTT (internal/agent/dispatch_network.go:171),
	// Discord (internal/agent/dispatch_messaging.go:104), email (internal/agent/dispatch_email.go:180),
	// and the notification channels (internal/tools/notification.go:163, :221, :304, :319, :360).
	{"is not enabled", false, agent.ToolResultNeedsSetup},
	// The planner (internal/agent/agent_dispatch_comm.go:1939, :1942, :1948, :1951 and
	// internal/agent/tool_runtime_availability.go:22).
	{"planner is disabled", true, agent.ToolResultNeedsSetup},
	{"planner database not available", true, agent.ToolResultNeedsSetup},
	// Notification channels without their settings: ntfy (internal/tools/notification.go:172),
	// Pushover (:227) and Telegram (:275, internal/tools/telegram_document.go:147).
	{"is not configured", false, agent.ToolResultNeedsSetup},
	{"must be configured", false, agent.ToolResultNeedsSetup},
	// send_notification to "all" with no channel set up (internal/tools/notification.go:147).
	{"no notification channels are enabled", true, agent.ToolResultNeedsSetup},
}

// flowRefusalStatus returns the status of the first rule whose text the message holds, or ""
// when no rule matches.
func flowRefusalStatus(message string) agent.ToolResultStatus {
	message = strings.TrimSpace(message)
	if len(message) > flowRefusalMessageBytes {
		message = message[:flowRefusalMessageBytes]
	}
	message = strings.ToLower(message)
	for _, rule := range flowRefusalRules {
		if rule.prefix && strings.HasPrefix(message, rule.text) || !rule.prefix && strings.Contains(message, rule.text) {
			return rule.status
		}
	}
	return ""
}

// flowToolText removes the "[Tool Output]" and "Tool Output:" prefixes the dispatcher and the
// handlers put in front of an answer. external reports that the rest is wrapped as untrusted
// third-party data (security.IsolateExternalData).
func flowToolText(output string) (text string, external bool) {
	text = strings.TrimSpace(output)
	for {
		previous := text
		for _, prefix := range []string{"[Tool Output]", "Tool Output:"} {
			text = strings.TrimSpace(strings.TrimPrefix(text, prefix))
		}
		if text == previous {
			break
		}
	}
	return text, strings.HasPrefix(text, "<external_data>")
}

// flowShapeStatus classifies an answer whose status came back empty or unclassified, like
// the dispatcher classifies raw results: a JSON envelope by its status, code or success
// field, plain text by the markers AuraGo's tools start their failures with. It returns ""
// when the answer says nothing about its outcome.
func flowShapeStatus(text string, envelope map[string]any) agent.ToolResultStatus {
	if envelope != nil {
		for _, key := range []string{"code", "status"} {
			value, _ := envelope[key].(string)
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "policy_denied", "tool_scope_denied", "permission_denied", "denied", "blocked":
				return agent.ToolResultDenied
			case "connect_required", "needs_setup", "not_configured", "disconnected":
				return agent.ToolResultNeedsSetup
			case "cancelled", "canceled":
				return agent.ToolResultCancelled
			case "error", "failed", "failure":
				return agent.ToolResultFailed
			}
		}
		if ok, isBool := envelope["success"].(bool); isBool && !ok {
			return agent.ToolResultFailed
		}
		return ""
	}
	lower := strings.ToLower(text)
	for _, prefix := range []string{"[permission denied]", "[tool blocked]"} {
		if strings.HasPrefix(lower, prefix) {
			return agent.ToolResultDenied
		}
	}
	for _, prefix := range []string{"[error]", "[execution error]", "error:", "error ", "timeout:"} {
		if strings.HasPrefix(lower, prefix) {
			return agent.ToolResultFailed
		}
	}
	return ""
}

// flowToolMessage is the tool's own description of a failure: the message or error field of
// a JSON envelope, else the text itself.
func flowToolMessage(text string, envelope map[string]any) string {
	if envelope == nil {
		return text
	}
	for _, key := range []string{"message", "error"} {
		if s, ok := envelope[key].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// flowNotifyRefusal handles an answer of send_notification or send_telegram whose top-level
// status is success but whose channels all failed (tools.SendNotification reports a failed
// channel only in its results). When every channel failed for a reason a refusal rule knows,
// the call is reported with that status (denied if any channel was refused, else
// needs_setup), and the rewritten answer carries the channels' texts as its message. A
// channel that failed for any other reason keeps the answer as it is: the node then fails
// with the retried FLOW_NOTIFY_FAILED.
func flowNotifyRefusal(envelope map[string]any) (agent.ToolResultStatus, string) {
	results, ok := envelope["results"].([]any)
	if !ok || len(results) == 0 {
		return "", ""
	}
	status := agent.ToolResultNeedsSetup
	details := make([]string, 0, len(results))
	for _, item := range results {
		entry, ok := item.(map[string]any)
		if !ok {
			return "", ""
		}
		if s, _ := entry["status"].(string); s == "sent" {
			return "", ""
		}
		detail, _ := entry["detail"].(string)
		refused := flowRefusalStatus(detail)
		if refused == "" {
			return "", ""
		}
		if refused == agent.ToolResultDenied {
			status = agent.ToolResultDenied
		}
		details = append(details, detail)
	}
	rewritten := make(map[string]any, len(envelope)+1)
	for k, v := range envelope {
		rewritten[k] = v
	}
	rewritten["status"] = "error"
	rewritten["message"] = flowBoundRunes(strings.Join(details, "; "), flowLogRunes)
	data, err := json.Marshal(rewritten)
	if err != nil {
		return "", ""
	}
	return status, string(data)
}

// flowToolOutcome turns a dispatcher result into the response a flow node gets (see the
// comment at the top of this file). tool is the logical flow tool name.
func flowToolOutcome(tool string, res agent.ToolDispatchResult) flows.ToolResponse {
	out := flows.ToolResponse{Output: res.Output, IsError: res.IsError, Status: string(res.Status)}
	if len(res.Output) > flowStatusParseLimit {
		return out
	}
	status := agent.ToolResultStatus(strings.ToLower(strings.TrimSpace(string(res.Status))))
	text, external := flowToolText(res.Output)
	var envelope map[string]any
	if parsed, _ := flows.ParseToolOutput(res.Output); parsed != nil {
		if _, onlyText := parsed["text"]; !(onlyText && len(parsed) == 1) {
			envelope = parsed
		}
	}
	if (status == "" || status == agent.ToolResultUnknown) && !external {
		if shape := flowShapeStatus(text, envelope); shape != "" {
			status = shape
		}
	}
	switch status {
	case agent.ToolResultFailed:
		if refused := flowRefusalStatus(flowToolMessage(text, envelope)); refused != "" {
			status = refused
		}
	case agent.ToolResultSuccess:
		if tool == "send_notification" || tool == "send_telegram" {
			if refused, rewritten := flowNotifyRefusal(envelope); refused != "" {
				status, out.Output = refused, rewritten
			}
		}
	}
	if status != "" {
		out.Status = string(status)
	}
	out.IsError = out.IsError || status.IsError()
	return out
}
