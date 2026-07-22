package agent

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

type trustedArtifactContext struct {
	MediaType       string
	StreamID        string
	LocalPath       string
	WebPath         string
	MediaID         int64
	SourceTool      string
	SourceOperation string
	SourcePrompt    string
}

type currentToolRouteContext struct {
	RunConfig    RunConfig
	EnabledTools map[string]bool
}

func (ctx currentToolRouteContext) toolEnabled(name string) bool {
	return ctx.EnabledTools[strings.ToLower(strings.TrimSpace(name))]
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

func deriveCurrentToolRoute(messages []openai.ChatCompletionMessage, userMessage string, ctx currentToolRouteContext) currentToolRoute {
	turnStart, turnEnd := previousHumanTurnBounds(messages)
	prompt := strings.TrimSpace(userMessage)
	explicitRetry := isExplicitRetryRequest(prompt)
	if explicitRetry {
		if route := deriveTrustedSafeRetryRoute(messages, turnStart, turnEnd, ctx); route.valid() {
			return route
		}
	}
	artifact, ok := deriveTrustedArtifactContext(messages, turnStart, turnEnd)
	if ok && artifactFollowUpIntent(userMessage) && isPureRetryRequest(userMessage) && strings.TrimSpace(artifact.SourcePrompt) != "" {
		prompt = strings.TrimSpace(artifact.SourcePrompt)
	}
	if ok && artifactFollowUpIntent(userMessage) && artifact.SourceTool == "go2rtc" && artifact.StreamID != "" && ctx.toolEnabled("go2rtc") {
		return currentToolRoute{
			ToolName: "go2rtc", Operation: "analyze_snapshot", StreamID: artifact.StreamID, Prompt: prompt, ExplicitRetry: explicitRetry,
			Text: fmt.Sprintf("Call go2rtc exactly once with operation=analyze_snapshot and stream_id=%q. Use the user's question as the vision prompt. Never search internal /files/ URLs with filesystem, shell, Python, or credential lookup. Do not call analyze_image for this camera artifact.", artifact.StreamID),
		}
	}
	if ok && artifactFollowUpIntent(userMessage) && ctx.toolEnabled("analyze_image") {
		localPath, resolved := resolveRegisteredImageArtifact(ctx.RunConfig, artifact)
		if !resolved {
			return currentToolRoute{}
		}
		return currentToolRoute{
			ToolName: "analyze_image", Path: localPath, Prompt: prompt, ExplicitRetry: explicitRetry,
			Text: fmt.Sprintf("Call analyze_image exactly once for the existing registered general image artifact at %q. Do not search for the path with filesystem, shell, Python, or credential lookup.", localPath),
		}
	}
	return currentToolRoute{}
}

func deriveTrustedSafeRetryRoute(messages []openai.ChatCompletionMessage, turnStart, turnEnd int, ctx currentToolRouteContext) currentToolRoute {
	for resultIndex := turnEnd - 1; resultIndex >= turnStart; resultIndex-- {
		result := messages[resultIndex]
		native := result.Role == openai.ChatMessageRoleTool
		textMode := result.Role == openai.ChatMessageRoleUser && strings.HasPrefix(strings.TrimSpace(result.Content), "Tool Output:")
		if !native && !textMode {
			continue
		}
		if !isToolError(result.Content) {
			continue
		}
		lowerResult := strings.ToLower(result.Content)
		if strings.Contains(lowerResult, "[tool blocked]") || strings.Contains(lowerResult, "guardian") {
			return currentToolRoute{}
		}
		producer, args, ok := trustedArtifactProducer(messages, turnStart, resultIndex, result.ToolCallID, native)
		if !ok {
			return currentToolRoute{}
		}
		args = cloneSafeRetryParameters(args)
		operation := strings.ToLower(strings.TrimSpace(stringValueFromMap(args, "operation", "action_type")))
		if !safeExplicitRetryTool(producer, operation) || !ctx.toolEnabled(producer) {
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

func previousHumanTurnBounds(messages []openai.ChatCompletionMessage) (int, int) {
	turnEnd := len(messages)
	for index := len(messages) - 1; index >= 0; index-- {
		if isHumanUserMessage(messages[index]) {
			turnEnd = index
			break
		}
	}
	turnStart := 0
	for index := turnEnd - 1; index >= 0; index-- {
		if isHumanUserMessage(messages[index]) {
			turnStart = index + 1
			break
		}
	}
	return turnStart, turnEnd
}

func isHumanUserMessage(message openai.ChatCompletionMessage) bool {
	return message.Role == openai.ChatMessageRoleUser && !strings.HasPrefix(strings.TrimSpace(message.Content), "Tool Output:")
}

func deriveTrustedArtifactContext(messages []openai.ChatCompletionMessage, turnStart, turnEnd int) (trustedArtifactContext, bool) {
	for resultIndex := turnEnd - 1; resultIndex >= turnStart; resultIndex-- {
		result := messages[resultIndex]
		isToolResult := result.Role == openai.ChatMessageRoleTool
		isTextToolResult := result.Role == openai.ChatMessageRoleUser && strings.HasPrefix(strings.TrimSpace(result.Content), "Tool Output:")
		if !isToolResult && !isTextToolResult {
			continue
		}
		producer, args, ok := trustedArtifactProducer(messages, turnStart, resultIndex, result.ToolCallID, isToolResult)
		if !ok {
			continue
		}
		producer = strings.ToLower(strings.TrimSpace(producer))
		payload := toolResultJSON(result.Content)
		successful := payload != nil && successfulToolResultPayload(payload)
		if producer == "analyze_image" && !isToolError(result.Content) {
			successful = true
		}
		if !successful {
			continue
		}
		artifact := trustedArtifactContext{
			SourceTool:      producer,
			SourceOperation: stringValueFromMap(args, "operation"),
			SourcePrompt:    stringValueFromMap(args, "prompt"),
		}
		if producer == "analyze_image" {
			artifact.MediaType = "image"
			artifact.LocalPath = stringValueFromMap(args, "file_path", "path")
			assignArtifactPath(&artifact, stringValueFromMap(args, "image_path", "image_url"))
		}
		if nested := mapValueFromMap(payload, "artifact"); nested != nil {
			if source := strings.TrimSpace(stringValueFromMap(nested, "source_tool")); source != "" && !strings.EqualFold(source, producer) {
				continue
			}
			artifact.MediaType = stringValueFromMap(nested, "media_type", "type")
			artifact.StreamID = stringValueFromMap(nested, "stream_id")
			artifact.LocalPath = stringValueFromMap(nested, "local_path", "file_path")
			artifact.WebPath = stringValueFromMap(nested, "web_path")
			artifact.MediaID = int64ValueFromMap(nested, "media_id")
			assignArtifactPath(&artifact, stringValueFromMap(nested, "registered_path", "path"))
		}
		if source := strings.TrimSpace(stringValueFromMap(payload, "source_tool")); source != "" && !strings.EqualFold(source, producer) {
			continue
		}
		if artifact.StreamID == "" {
			artifact.StreamID = firstNonEmptyToolString(stringValueFromMap(payload, "stream_id"), stringValueFromMap(args, "stream_id"))
		}
		if artifact.LocalPath == "" {
			artifact.LocalPath = stringValueFromMap(payload, "local_path", "file_path")
		}
		if artifact.WebPath == "" {
			artifact.WebPath = stringValueFromMap(payload, "web_path")
		}
		if artifact.MediaID == 0 {
			artifact.MediaID = int64ValueFromMap(payload, "media_id")
		}
		assignArtifactPath(&artifact, stringValueFromMap(payload, "image_path", "registered_path"))
		if snapshot := mapValueFromMap(payload, "snapshot"); snapshot != nil {
			if artifact.WebPath == "" {
				artifact.WebPath = stringValueFromMap(snapshot, "web_path")
			}
			if artifact.MediaID == 0 {
				artifact.MediaID = int64ValueFromMap(snapshot, "media_id")
			}
			if artifact.StreamID == "" {
				artifact.StreamID = stringValueFromMap(snapshot, "stream_id")
			}
		}
		if artifact.MediaType == "" && (artifact.LocalPath != "" || artifact.WebPath != "" || artifact.MediaID > 0) {
			artifact.MediaType = "image"
		}
		if strings.EqualFold(artifact.MediaType, "image") && (artifact.LocalPath != "" || artifact.WebPath != "" || artifact.MediaID > 0 || artifact.StreamID != "") {
			return artifact, true
		}
	}
	return trustedArtifactContext{}, false
}

func trustedArtifactProducer(messages []openai.ChatCompletionMessage, turnStart, resultIndex int, callID string, native bool) (string, map[string]interface{}, bool) {
	if native && strings.TrimSpace(callID) == "" {
		return "", nil, false
	}
	for index := resultIndex - 1; index >= turnStart; index-- {
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

func successfulToolResultPayload(payload map[string]interface{}) bool {
	status := strings.ToLower(strings.TrimSpace(stringValueFromMap(payload, "status")))
	return stringInSet(status, "ok", "success", "succeeded", "completed")
}

func assignArtifactPath(artifact *trustedArtifactContext, value string) {
	if artifact == nil {
		return
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(value, "/files/") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		if artifact.WebPath == "" {
			artifact.WebPath = value
		}
		return
	}
	if artifact.LocalPath == "" {
		artifact.LocalPath = value
	}
}

func int64ValueFromMap(values map[string]interface{}, key string) int64 {
	if values == nil {
		return 0
	}
	value, ok := values[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return int64(typed)
		}
	case int64:
		if typed > 0 {
			return typed
		}
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil && parsed > 0 {
			return parsed
		}
	case float64:
		if typed > 0 && typed <= math.MaxInt64 && math.Trunc(typed) == typed {
			return int64(typed)
		}
	}
	return 0
}

func resolveRegisteredImageArtifact(runCfg RunConfig, artifact trustedArtifactContext) (string, bool) {
	if runCfg.Config == nil || runCfg.MediaRegistryDB == nil {
		return "", false
	}

	var (
		item *tools.MediaItem
		err  error
	)
	switch {
	case artifact.MediaID > 0:
		item, err = tools.GetMedia(runCfg.MediaRegistryDB, artifact.MediaID)
	case strings.TrimSpace(artifact.LocalPath) != "":
		item, err = tools.GetMediaByFilePath(runCfg.MediaRegistryDB, strings.TrimSpace(artifact.LocalPath))
		if err != nil {
			localPath, resolveErr := tools.ResolveRegisteredMediaFilePath(artifact.LocalPath, runCfg.Config)
			if resolveErr != nil {
				return "", false
			}
			item, err = tools.GetMediaByFilePath(runCfg.MediaRegistryDB, localPath)
		}
	case strings.HasPrefix(strings.TrimSpace(artifact.WebPath), "/files/"):
		item, err = tools.GetMediaByWebPath(runCfg.MediaRegistryDB, strings.TrimSpace(artifact.WebPath))
	default:
		return "", false
	}
	if err != nil || item == nil || !strings.EqualFold(strings.TrimSpace(item.MediaType), "image") {
		return "", false
	}
	if artifact.MediaID > 0 && item.ID != artifact.MediaID {
		return "", false
	}
	if webPath := strings.TrimSpace(artifact.WebPath); webPath != "" && webPath != strings.TrimSpace(item.WebPath) {
		return "", false
	}

	registeredPath, err := tools.ResolveRegisteredMediaFilePath(item.FilePath, runCfg.Config)
	if err != nil {
		return "", false
	}
	if localPath := strings.TrimSpace(artifact.LocalPath); localPath != "" {
		resolvedLocal, resolveErr := tools.ResolveRegisteredMediaFilePath(localPath, runCfg.Config)
		if resolveErr != nil || !sameCanonicalPath(resolvedLocal, registeredPath) {
			return "", false
		}
	}
	info, err := os.Stat(registeredPath)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	file, err := os.Open(registeredPath)
	if err != nil {
		return "", false
	}
	defer file.Close()
	probe := make([]byte, 512)
	n, err := file.Read(probe)
	if err != nil && n == 0 {
		return "", false
	}
	switch strings.ToLower(strings.TrimSpace(http.DetectContentType(probe[:n]))) {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp":
		return registeredPath, true
	default:
		return "", false
	}
}

func sameCanonicalPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func artifactFollowUpIntent(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return false
	}
	for _, phrase := range []string{
		"ignoriere das bild", "bild ignorieren", "ignoriere das foto", "foto ignorieren", "keine bildanalyse", "ohne bildanalyse",
		"ignore the image", "ignore the photo", "do not analyze the image", "don't analyze the image", "skip image analysis",
	} {
		if containsNormalizedRoutePhrase(lower, phrase) {
			return false
		}
	}
	if isExplicitRetryRequest(lower) {
		return true
	}
	for _, phrase := range []string{"wie viele", "how many"} {
		if containsNormalizedRoutePhrase(lower, phrase) {
			return true
		}
	}
	for _, token := range normalizedRouteTokens(lower) {
		if stringInSet(token,
			"count", "counts", "zähl", "zähle", "zählen", "pkw", "pkws", "auto", "autos",
			"fahrzeug", "fahrzeuge", "vehicle", "vehicles", "kamera", "kameras", "camera", "cameras",
			"snapshot", "snapshots", "bild", "bilder", "image", "images", "foto", "fotos", "photo", "photos",
			"siehst", "sehen", "erkennst", "erkennen", "analysiere", "analysieren", "analyze", "analyse") {
			return true
		}
	}
	return false
}

func normalizedRouteTokens(message string) []string {
	return strings.FieldsFunc(strings.ToLower(message), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func containsNormalizedRoutePhrase(message, phrase string) bool {
	messageTokens := normalizedRouteTokens(message)
	phraseTokens := normalizedRouteTokens(phrase)
	if len(phraseTokens) == 0 || len(phraseTokens) > len(messageTokens) {
		return false
	}
	for start := 0; start+len(phraseTokens) <= len(messageTokens); start++ {
		matched := true
		for offset := range phraseTokens {
			if messageTokens[start+offset] != phraseTokens[offset] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func isPureRetryRequest(message string) bool {
	if !isExplicitRetryRequest(message) {
		return false
	}
	for _, token := range normalizedRouteTokens(message) {
		if !stringInSet(token,
			"bitte", "please", "es", "it", "das", "that", "jetzt", "now", "noch", "einmal", "again",
			"erneut", "retry", "try", "wieder", "wiederhole", "wiederholen", "versuch", "versuche", "probier", "nochmal") {
			return false
		}
	}
	return true
}

func isExplicitRetryRequest(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return false
	}
	for _, marker := range []string{
		"versuche es erneut", "versuch es erneut", "probier es erneut", "noch einmal", "nochmal",
		"retry", "try again", "wiederhole",
	} {
		if containsNormalizedRoutePhrase(lower, marker) {
			return true
		}
	}
	return false
}
