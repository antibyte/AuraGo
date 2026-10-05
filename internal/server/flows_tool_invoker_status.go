package server

import (
	"encoding/json"
	"html"
	"regexp"
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
// Remote text must not pick a status: a refusal rule matches only AuraGo's own wording at
// the start of the tool's message, with the integration named as the subject, never a
// message that starts with a transport error (which echoes what a server sent), and never an
// answer the tool itself marked as third-party data. The server's Guardian wraps every tool
// answer in <external_data> (security.SanitizeToolOutput, default "external"); that outer
// layer is presentation and is removed first. A second layer, or a single one when no
// Guardian runs, is the tool's own marking of remote content (agent externalToolOutput).
//
// It never changes the output of a call that succeeded, and it never parses a large
// output: refusals are short, and the flows side parses every output again anyway.

// flowStatusParseLimit is the largest output flowToolOutcome looks into.
const flowStatusParseLimit = 64 << 10

// flowRefusalMessageBytes is how much of a tool's message the refusal rules look at.
const flowRefusalMessageBytes = 500

// flowRefusalRule maps AuraGo's own wording of a refusal to a non-retryable status. pattern
// is matched against the lower-cased, trimmed message and is anchored at its start. The
// first matching rule wins, so the denied rules come first.
type flowRefusalRule struct {
	pattern *regexp.Regexp
	status  agent.ToolResultStatus
}

func flowRule(expr string, status agent.ToolResultStatus) flowRefusalRule {
	return flowRefusalRule{pattern: regexp.MustCompile(`^(?:` + expr + `)`), status: status}
}

// flowRefusalRules are the refusal texts of the tools flows call, with the file and line of
// each real message (flows_tool_invoker_hardening_test.go checks every one of them).
var flowRefusalRules = []flowRefusalRule{
	// "Tool Output: [PERMISSION DENIED] filesystem write operations are disabled in Danger Zone
	// settings …" (internal/agent/dispatch_filesystem.go:128), a protected file (:93),
	// web_scraper switched off (internal/agent/tool_builtin_handlers.go:27). The dispatcher
	// classifies these itself; the rule covers an unclassified status.
	flowRule(`\[permission denied\] `, agent.ToolResultDenied),
	// The Guardian's block (internal/agent/agent_parse.go:52).
	flowRule(`\[tool blocked\] `, agent.ToolResultDenied),
	// "<gate> is disabled by runtime permissions" (internal/tools/permissions.go:244 and
	// :311-419): the write_file refusal, which has no error_code
	// (internal/tools/filesystem.go:522-523), api_request without network access
	// (internal/tools/api_client.go:66-67), and "MQTT publish failed: mqtt publish is disabled
	// by runtime permissions" (internal/agent/dispatch_network.go:186 with permissions.go:364).
	// The whole message must be this sentence.
	flowRule(`(?:mqtt publish failed: )?[a-z][a-z /_-]{0,40} is disabled by runtime permissions\.?$`, agent.ToolResultDenied),
	// The filesystem tool's path refusals, which a write reports without error_code
	// (internal/tools/filesystem.go:296 and :308); pdf_extractor puts "path traversal denied: "
	// in front (internal/tools/tool_input_path.go:26).
	flowRule(`(?:path traversal denied: )?path '.*' is an absolute path outside the project root \(`, agent.ToolResultDenied),
	flowRule(`(?:path traversal denied: )?path '.*' escapes the project root$`, agent.ToolResultDenied),
	// The SSRF block of api_request (internal/tools/api_client.go:82).
	flowRule(`url validation failed: `, agent.ToolResultDenied),
	// Read-only integrations: Home Assistant (internal/agent/agent_dispatch_services.go:1385,
	// internal/tools/homeassistant.go:186), MQTT (internal/agent/dispatch_network.go:174),
	// Discord (internal/agent/dispatch_messaging.go:107, internal/tools/notification.go:96) and
	// Telnyx (internal/agent/dispatch_messaging.go:192, internal/tools/notification.go:104).
	flowRule(`(?:home assistant|mqtt|discord|telnyx) is in read-only mode`, agent.ToolResultDenied),
	// Home Assistant's service lists (internal/agent/home_assistant_policy.go:12 and :23).
	flowRule(`home assistant service \S+ is blocked by home_assistant\.blocked_services$`, agent.ToolResultDenied),
	flowRule(`home assistant service \S+ is not allowed by home_assistant\.allowed_services$`, agent.ToolResultDenied),
	// Integrations that are switched off: Home Assistant
	// (internal/agent/agent_dispatch_services.go:1379), MQTT (internal/agent/dispatch_network.go:171),
	// Discord (internal/agent/dispatch_messaging.go:104), email (internal/agent/dispatch_email.go:180),
	// Brave (internal/agent/tool_builtin_handlers.go:124) and the notification channels
	// (internal/tools/notification.go:163, :221, :304, :319, :360).
	flowRule(`(?:home assistant integration|mqtt|discord|email|brave search integration|ntfy|pushover|telnyx|cyd) is not enabled`, agent.ToolResultNeedsSetup),
	// The planner (internal/agent/agent_dispatch_comm.go:1939, :1942, :1948, :1951 and
	// internal/agent/tool_runtime_availability.go:22).
	flowRule(`planner is disabled\.`, agent.ToolResultNeedsSetup),
	flowRule(`planner database not available\.?$`, agent.ToolResultNeedsSetup),
	// Notification channels without their settings: ntfy (internal/tools/notification.go:172),
	// Pushover (:227) and Telegram (:275, internal/tools/telegram_document.go:147).
	flowRule(`ntfy topic is not configured$`, agent.ToolResultNeedsSetup),
	flowRule(`(?:pushover user_key and app_token|telegram bot_token and telegram_user_id) must be configured$`, agent.ToolResultNeedsSetup),
	// send_notification to "all" with no channel set up (internal/tools/notification.go:147).
	flowRule(`no notification channels are enabled$`, agent.ToolResultNeedsSetup),
}

// flowTransportPrefixes start messages that wrap what a remote server or a transport said:
// api_request (internal/tools/api_client.go:94, :115, :122), Home Assistant
// (internal/tools/homeassistant.go:83, :86, :92, :209, :272), Discord
// (internal/agent/dispatch_messaging.go:123, :146), the web scraper (internal/tools/scraper.go:104,
// :131, :156), the notification channels (internal/tools/notification.go:203, :212, :251, :260)
// and the preferred MCP search (internal/agent/tool_builtin_handlers.go:80). No refusal rule
// is tried on them.
var flowTransportPrefixes = []string{
	"request failed:", "failed to read response:", "failed to create request:",
	"service call failed:", "home assistant api error", "failed to fetch", "failed to parse",
	"discord send failed:", "discord fetch failed:",
	"scrape failed:", "dynamic scrape failed:", "rss scrape failed:",
	"ntfy request failed:", "ntfy returned", "pushover request failed:", "pushover returned",
	"preferred mcp web search failed:",
}

// flowRefusalStatus returns the status of the first rule that matches the start of the
// message, or "" when none does or the message reports a transport error.
func flowRefusalStatus(message string) agent.ToolResultStatus {
	message = strings.TrimSpace(message)
	if len(message) > flowRefusalMessageBytes {
		message = message[:flowRefusalMessageBytes]
	}
	message = strings.ToLower(message)
	for _, prefix := range flowTransportPrefixes {
		if strings.HasPrefix(message, prefix) {
			return ""
		}
	}
	for _, rule := range flowRefusalRules {
		if rule.pattern.MatchString(message) {
			return rule.status
		}
	}
	return ""
}

const (
	flowExternalOpen  = "<external_data>"
	flowExternalClose = "</external_data>"
)

// flowStripToolPrefixes removes the "[Tool Output]" and "Tool Output:" prefixes the
// dispatcher and the handlers put in front of an answer.
func flowStripToolPrefixes(text string) string {
	text = strings.TrimSpace(text)
	for {
		previous := text
		for _, prefix := range []string{"[Tool Output]", "Tool Output:"} {
			text = strings.TrimSpace(strings.TrimPrefix(text, prefix))
		}
		if text == previous {
			return text
		}
	}
}

// flowUnwrapExternal removes one <external_data> layer and its HTML escaping.
func flowUnwrapExternal(text string) (string, bool) {
	if !strings.HasPrefix(text, flowExternalOpen) || !strings.HasSuffix(text, flowExternalClose) {
		return text, false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(text, flowExternalOpen), flowExternalClose)
	return html.UnescapeString(strings.Trim(inner, "\n")), true
}

// flowToolText returns the tool's own text without prefixes and without the Guardian's
// outer <external_data> layer. remote reports that what is left is third-party data the
// tool marked as such: a second layer, or any layer when guardian is false.
func flowToolText(output string, guardian bool) (text string, remote bool) {
	text = flowStripToolPrefixes(output)
	inner, wrapped := flowUnwrapExternal(text)
	if !wrapped {
		return text, false
	}
	if !guardian {
		return text, true
	}
	inner = flowStripToolPrefixes(inner)
	return inner, strings.HasPrefix(inner, flowExternalOpen)
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
// comment at the top of this file). tool is the logical flow tool name; guardian tells
// whether the dispatch ran with a Guardian, which wraps every answer once.
func flowToolOutcome(tool string, res agent.ToolDispatchResult, guardian bool) flows.ToolResponse {
	out := flows.ToolResponse{Output: res.Output, IsError: res.IsError, Status: string(res.Status)}
	if len(res.Output) > flowStatusParseLimit {
		return out
	}
	status := agent.ToolResultStatus(strings.ToLower(strings.TrimSpace(string(res.Status))))
	text, remote := flowToolText(res.Output, guardian)
	if remote {
		return out
	}
	var envelope map[string]any
	if parsed, _ := flows.ParseToolOutput(text); parsed != nil {
		if _, onlyText := parsed["text"]; !(onlyText && len(parsed) == 1) {
			envelope = parsed
		}
	}
	if status == "" || status == agent.ToolResultUnknown {
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
