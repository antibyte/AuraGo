package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

type trustedArtifactContext struct {
	MediaType       string
	StreamID        string
	RegisteredPath  string
	SourceTool      string
	SourceOperation string
	SourcePrompt    string
}

type currentToolRoute struct {
	ToolName      string
	Operation     string
	StreamID      string
	Path          string
	Prompt        string
	Parameters    map[string]interface{}
	ExplicitRetry bool
	Text          string
}

func shouldUseSupervisorToolRoute(runCfg RunConfig) bool {
	if runCfg.IsMission || runCfg.IsMaintenance || runCfg.IsCoAgent {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(runCfg.MessageSource)) {
	case "", "web_chat", "telegram", "discord", "sms", "rocketchat", "agodesk_chat", "virtual_desktop_chat":
		return true
	default:
		return false
	}
}

func (route currentToolRoute) valid() bool {
	return strings.TrimSpace(route.ToolName) != ""
}

func (route currentToolRoute) toolCall() ToolCall {
	params := cloneSafeRetryParameters(route.Parameters)
	if params == nil {
		params = map[string]interface{}{}
	}
	if len(params) > 0 {
		payload := cloneSafeRetryParameters(params)
		payload["action"] = route.ToolName
		raw, err := json.Marshal(payload)
		if err == nil {
			call := ParseToolCall(string(raw))
			call.Action = route.ToolName
			call.Operation = firstNonEmptyToolString(call.Operation, route.Operation)
			call.Params = params
			call.IsTool = true
			call.RawJSON = string(raw)
			return call
		}
	}
	if route.Operation != "" {
		params["operation"] = route.Operation
	}
	if route.StreamID != "" {
		params["stream_id"] = route.StreamID
	}
	if route.Prompt != "" {
		params["prompt"] = route.Prompt
	}
	call := ToolCall{
		Action: route.ToolName, Operation: route.Operation, FilePath: route.Path,
		Prompt: route.Prompt, Params: params, IsTool: true,
	}
	if route.Path != "" {
		params["file_path"] = route.Path
	}
	if raw, err := json.Marshal(map[string]interface{}{"action": route.ToolName, "parameters": params}); err == nil {
		call.RawJSON = string(raw)
	}
	return call
}

func (route currentToolRoute) matches(call ToolCall) bool {
	return route.valid() && strings.EqualFold(strings.TrimSpace(route.ToolName), strings.TrimSpace(call.Action)) &&
		(route.Operation == "" || strings.EqualFold(route.Operation, firstNonEmptyToolString(call.Operation, toolArgString(call.Params, "operation"))))
}

func deriveCurrentToolRoute(messages []openai.ChatCompletionMessage, userMessage string) currentToolRoute {
	artifact, ok := deriveTrustedArtifactContext(messages)
	prompt := strings.TrimSpace(userMessage)
	explicitRetry := isExplicitRetryRequest(prompt)
	if ok && artifactFollowUpIntent(userMessage) && explicitRetry && strings.TrimSpace(artifact.SourcePrompt) != "" {
		prompt = strings.TrimSpace(artifact.SourcePrompt)
	}
	if ok && artifactFollowUpIntent(userMessage) && artifact.SourceTool == "go2rtc" && artifact.StreamID != "" {
		return currentToolRoute{
			ToolName: "go2rtc", Operation: "analyze_snapshot", StreamID: artifact.StreamID, Prompt: prompt, ExplicitRetry: explicitRetry,
			Text: fmt.Sprintf("Call go2rtc exactly once with operation=analyze_snapshot and stream_id=%q. Use the user's question as the vision prompt. Never search internal /files/ URLs with filesystem, shell, Python, or credential lookup. Do not call analyze_image for this camera artifact.", artifact.StreamID),
		}
	}
	if ok && artifactFollowUpIntent(userMessage) && artifact.RegisteredPath != "" {
		return currentToolRoute{
			ToolName: "analyze_image", Path: artifact.RegisteredPath, Prompt: prompt, ExplicitRetry: explicitRetry,
			Text: fmt.Sprintf("Call analyze_image exactly once for the existing general image artifact at %q. Do not search for the path with filesystem, shell, Python, or credential lookup.", artifact.RegisteredPath),
		}
	}
	if explicitRetry {
		return deriveTrustedSafeRetryRoute(messages)
	}
	return currentToolRoute{}
}

func deriveTrustedSafeRetryRoute(messages []openai.ChatCompletionMessage) currentToolRoute {
	for resultIndex := len(messages) - 1; resultIndex >= 0; resultIndex-- {
		result := messages[resultIndex]
		native := result.Role == openai.ChatMessageRoleTool
		textMode := result.Role == openai.ChatMessageRoleUser && strings.HasPrefix(strings.TrimSpace(result.Content), "Tool Output:")
		if !native && !textMode {
			continue
		}
		if !isToolError(result.Content) || strings.Contains(strings.ToLower(result.Content), "[tool blocked]") || strings.Contains(strings.ToLower(result.Content), "guardian") {
			continue
		}
		producer, args, ok := trustedArtifactProducer(messages, resultIndex, result.ToolCallID, native)
		if !ok {
			continue
		}
		args = cloneSafeRetryParameters(args)
		operation := strings.ToLower(strings.TrimSpace(stringValueFromMap(args, "operation", "action_type")))
		if !safeExplicitRetryTool(producer, operation) {
			return currentToolRoute{}
		}
		return currentToolRoute{
			ToolName: producer, Operation: operation, Parameters: args, ExplicitRetry: true,
			Text: fmt.Sprintf("Retry the previously failed %s call exactly once with the same sanitized parameters. Do not add exploratory calls, credential lookup, secret environment access, or _guardian_justification.", producer),
		}
	}
	return currentToolRoute{}
}

func safeExplicitRetryTool(toolName, operation string) bool {
	toolName = strings.ToLower(strings.TrimSpace(toolName))
	operation = strings.ToLower(strings.TrimSpace(operation))
	switch toolName {
	case "analyze_image", "query_memory", "recall_memory", "context_memory", "web_search":
		return true
	case "go2rtc":
		return stringInSet(operation, "status", "list_streams", "stream_status", "snapshot", "analyze_snapshot")
	case "workspace_search":
		return stringInSet(operation, "find", "grep", "glob", "recent", "status", "rescan")
	case "file_search":
		return stringInSet(operation, "", "find", "grep", "glob", "recent", "status")
	case "explore_kg":
		return stringInSet(operation, "", "get_node", "get_neighbors", "subgraph", "explore", "explain_edge")
	default:
		return false
	}
}

func stringInSet(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func cloneSafeRetryParameters(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		lower := strings.ToLower(strings.TrimSpace(key))
		if lower == "_guardian_justification" || strings.Contains(lower, "password") || strings.Contains(lower, "secret") ||
			strings.Contains(lower, "token") || strings.Contains(lower, "credential") || strings.Contains(lower, "authorization") ||
			lower == "headers" || lower == "env" || lower == "environment" || strings.Contains(lower, "api_key") {
			continue
		}
		if safeValue, ok := cloneSafeRetryValue(value); ok {
			out[key] = safeValue
		}
	}
	return out
}

func cloneSafeRetryValue(value interface{}) (interface{}, bool) {
	switch typed := value.(type) {
	case string:
		lower := strings.ToLower(strings.TrimSpace(typed))
		for _, marker := range []string{
			"_guardian_justification", "authorization", "bearer ", "password", "credential",
			"api_key", "api key", "access_token", "refresh_token", "secret", "token=", "token:",
			"sk-", "ghp_", "gho_", "ghu_", "ghs_", "ghr_",
		} {
			if strings.Contains(lower, marker) {
				return nil, false
			}
		}
		safe := sanitizeControlledRetryText(typed, 1200)
		return safe, safe != ""
	case map[string]interface{}:
		safe := cloneSafeRetryParameters(typed)
		return safe, len(safe) > 0
	case map[string]string:
		converted := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			converted[key] = item
		}
		safe := cloneSafeRetryParameters(converted)
		return safe, len(safe) > 0
	case []interface{}:
		safe := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			if safeItem, ok := cloneSafeRetryValue(item); ok {
				safe = append(safe, safeItem)
			}
		}
		return safe, len(safe) > 0
	case []string:
		safe := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			if safeItem, ok := cloneSafeRetryValue(item); ok {
				safe = append(safe, safeItem)
			}
		}
		return safe, len(safe) > 0
	case nil:
		return nil, true
	default:
		// Parsed tool arguments are JSON scalars here. Keep numbers and booleans;
		// any string-bearing containers are handled recursively above.
		return typed, true
	}
}

func deriveTrustedArtifactContext(messages []openai.ChatCompletionMessage) (trustedArtifactContext, bool) {
	for resultIndex := len(messages) - 1; resultIndex >= 0; resultIndex-- {
		result := messages[resultIndex]
		isToolResult := result.Role == openai.ChatMessageRoleTool
		isTextToolResult := result.Role == openai.ChatMessageRoleUser && strings.HasPrefix(strings.TrimSpace(result.Content), "Tool Output:")
		if !isToolResult && !isTextToolResult {
			continue
		}
		producer, args, ok := trustedArtifactProducer(messages, resultIndex, result.ToolCallID, isToolResult)
		if !ok {
			continue
		}
		payload := toolResultJSON(result.Content)
		if payload == nil {
			continue
		}
		artifact := trustedArtifactContext{
			SourceTool:      producer,
			SourceOperation: stringValueFromMap(args, "operation"),
			SourcePrompt:    stringValueFromMap(args, "prompt"),
		}
		if nested := mapValueFromMap(payload, "artifact"); nested != nil {
			artifact.MediaType = stringValueFromMap(nested, "media_type", "type")
			artifact.StreamID = stringValueFromMap(nested, "stream_id")
			artifact.RegisteredPath = stringValueFromMap(nested, "registered_path", "web_path", "path")
			if source := stringValueFromMap(nested, "source_tool"); source != "" {
				artifact.SourceTool = source
			}
		}
		if artifact.StreamID == "" {
			artifact.StreamID = firstNonEmptyToolString(stringValueFromMap(payload, "stream_id"), stringValueFromMap(args, "stream_id"))
		}
		if artifact.RegisteredPath == "" {
			artifact.RegisteredPath = stringValueFromMap(payload, "image_path", "web_path", "registered_path")
		}
		if artifact.RegisteredPath == "" {
			if snapshot := mapValueFromMap(payload, "snapshot"); snapshot != nil {
				artifact.RegisteredPath = stringValueFromMap(snapshot, "web_path")
				if artifact.StreamID == "" {
					artifact.StreamID = stringValueFromMap(snapshot, "stream_id")
				}
			}
		}
		if artifact.MediaType == "" && artifact.RegisteredPath != "" {
			artifact.MediaType = "image"
		}
		if artifact.MediaType == "image" && (artifact.RegisteredPath != "" || artifact.StreamID != "") {
			return artifact, true
		}
	}
	return trustedArtifactContext{}, false
}

func trustedArtifactProducer(messages []openai.ChatCompletionMessage, resultIndex int, callID string, native bool) (string, map[string]interface{}, bool) {
	for index := resultIndex - 1; index >= 0; index-- {
		message := messages[index]
		if message.Role != openai.ChatMessageRoleAssistant {
			if native {
				continue
			}
			break
		}
		if native {
			for _, call := range message.ToolCalls {
				if call.ID != callID {
					continue
				}
				args := map[string]interface{}{}
				_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
				return strings.TrimSpace(call.Function.Name), args, call.Function.Name != ""
			}
			continue
		}
		var parsed map[string]interface{}
		if json.Unmarshal([]byte(strings.TrimSpace(message.Content)), &parsed) != nil {
			return "", nil, false
		}
		producer := stringValueFromMap(parsed, "action", "tool_name", "tool")
		args := mapValueFromMap(parsed, "parameters", "params", "arguments")
		if args == nil {
			args = parsed
		}
		return producer, args, producer != ""
	}
	return "", nil, false
}

func toolResultJSON(content string) map[string]interface{} {
	start := strings.Index(content, "{")
	if start < 0 {
		return nil
	}
	var payload map[string]interface{}
	if json.Unmarshal([]byte(strings.TrimSpace(content[start:])), &payload) != nil {
		return nil
	}
	return payload
}

func artifactFollowUpIntent(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return false
	}
	if isExplicitRetryRequest(lower) {
		return true
	}
	for _, marker := range []string{
		"wie viele", "how many", "count", "zähl", "pkw", "auto", "fahrzeug", "vehicle",
		"kamera", "camera", "snapshot", "bild", "image", "siehst", "erkennst", "analy",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func isExplicitRetryRequest(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return false
	}
	for _, marker := range []string{"versuche es erneut", "noch einmal", "nochmal", "retry", "try again", "wiederhole"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
