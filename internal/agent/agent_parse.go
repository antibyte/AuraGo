package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/tools"
	"aurago/internal/webhooks"

	"github.com/sashabaranov/go-openai"
)

var (
	localIPCache sync.Map
	localIPDial  = func(network, address string) (net.Conn, error) {
		return net.Dial(network, address)
	}
)

func guardianBlockNextStep(reason string) string {
	lower := strings.ToLower(strings.TrimSpace(reason))
	switch {
	case strings.Contains(lower, "remote code execution"), strings.Contains(lower, "curl pipe sh"), strings.Contains(lower, "wget pipe sh"), strings.Contains(lower, "pipe sh"):
		return "Do not retry with curl|sh, wget|sh, or similar remote-install patterns. Use a built-in tool/manual-driven workflow instead, or ask the user for an alternative approach."
	case strings.Contains(lower, "credentials"), strings.Contains(lower, "token"), strings.Contains(lower, "secret"):
		return "Do not guess or hardcode credentials. Use the vault-backed workflow or ask the user to provide/store the required secret safely."
	default:
		return "Do not repeat the blocked action blindly. Re-check the tool manual, choose a safer built-in workflow, or ask the user for an alternative approach."
	}
}

func formatGuardianBlockedMessage(action, reason string, risk float64, allowClarification bool, clarificationRejected bool) string {
	base := fmt.Sprintf("[TOOL BLOCKED] Security check failed for %s: %s (risk: %.0f%%).", action, reason, risk*100)
	nextStep := guardianBlockNextStep(reason)
	if clarificationRejected {
		return base + " Clarification was reviewed but the action remains blocked. " + nextStep
	}
	if allowClarification {
		return base + ` You may retry this tool call once by adding a "_guardian_justification" field explaining why this action is necessary and safe.` + " " + nextStep
	}
	return base + " " + nextStep
}

// DispatchToolCall executes the appropriate tool based on the parsed ToolCall.
// It automatically handles LLM Guardian pre-check, Redaction, Guardian sanitization,
// and ensures the output is correctly prefixed with "[Tool Output]\n" unless it's a known error marker.
// If the tool is blocked by Guardian, tc.GuardianBlocked and tc.GuardianBlockReason are set.
func DispatchToolCall(ctx context.Context, tc *ToolCall, dc *DispatchContext, userContext string) string {
	cfg := dc.Cfg
	logger := dc.Logger
	guardian := dc.Guardian
	llmGuardian := dc.LLMGuardian

	// LLM Guardian: pre-execution security check
	if llmGuardian != nil {
		var regexLevel security.ThreatLevel
		if guardian != nil {
			scanText := toolCallScanText(*tc)
			regexLevel = guardian.ScanForInjection(scanText).Level
		}
		// Build guardian context: prefer the triggering user message over tc.Content
		guardianCtx := userContext
		if guardianCtx == "" {
			guardianCtx = tc.Content
		}
		if len(guardianCtx) > 300 {
			guardianCtx = guardianCtx[:300]
		}
		check := security.GuardianCheck{
			Operation:  tc.Action,
			Parameters: toolCallParams(*tc),
			Context:    guardianCtx,
			RegexLevel: regexLevel,
		}
		if llmGuardian.ShouldCheck(check) {
			result := llmGuardian.EvaluateWithFailSafe(ctx, check)
			if result.Decision == security.DecisionBlock {
				// Clarification: if agent provided a justification AND clarification is enabled, re-evaluate once
				if tc.GuardianJustification != "" && cfg.LLMGuardian.AllowClarification {
					check.Justification = tc.GuardianJustification
					clarResult := llmGuardian.EvaluateClarification(ctx, check)
					if clarResult.Decision != security.DecisionBlock {
						logger.Info("[LLM Guardian] Clarification accepted, proceeding",
							"tool", tc.Action, "decision", clarResult.Decision, "reason", clarResult.Reason)
						if clarResult.Decision == security.DecisionQuarantine {
							logger.Warn("[LLM Guardian] Clarification resulted in quarantine (proceeding with caution)",
								"tool", tc.Action, "reason", clarResult.Reason, "risk", clarResult.RiskScore)
						}
						goto proceed
					}
					// Clarification rejected — final block (no more retries)
					logger.Warn("[LLM Guardian] Clarification rejected, final block",
						"tool", tc.Action, "reason", clarResult.Reason, "risk", clarResult.RiskScore)
					tc.GuardianBlocked = true
					tc.GuardianBlockReason = clarResult.Reason
					return formatGuardianBlockedMessage(tc.Action, clarResult.Reason, clarResult.RiskScore, cfg.LLMGuardian.AllowClarification, true)
				}

				logger.Warn("[LLM Guardian] Blocked tool call",
					"tool", tc.Action, "reason", result.Reason, "risk", result.RiskScore)
				tc.GuardianBlocked = true
				tc.GuardianBlockReason = result.Reason
				return formatGuardianBlockedMessage(tc.Action, result.Reason, result.RiskScore, cfg.LLMGuardian.AllowClarification, false)
			}
			if result.Decision == security.DecisionQuarantine {
				logger.Warn("[LLM Guardian] Quarantined tool call (proceeding with caution)",
					"tool", tc.Action, "reason", result.Reason, "risk", result.RiskScore)
			}
		}
	}
proceed:

	startTime := time.Now()
	rawResult := dispatchInner(ctx, *tc, dc)
	dc.ExecutionTimeMs = time.Since(startTime).Milliseconds()

	// Apply scrubbing and redaction to tool output.
	// Scrub() removes registered runtime secrets (vault keys, API tokens, etc.).
	// RedactSensitiveInfo() catches regex-identified patterns (key=value pairs, etc.).
	sanitized := security.StripThinkingTags(security.RedactSensitiveInfo(security.Scrub(rawResult)))

	// Guardian: Sanitize tool output (isolation + role-marker stripping)
	if guardian != nil {
		sanitized = guardian.SanitizeToolOutput(tc.Action, sanitized)
	}

	// Make sure errors from execute_python are preserved for context
	if tc.Action == "execute_python" {
		if strings.Contains(sanitized, "[EXECUTION ERROR]") || strings.Contains(sanitized, "TIMEOUT") {
			// handled outside in isErrorState flags if necessary, but we preserve the string here
		}
	}

	// Prefix to clearly identify it as tool output
	if !strings.HasPrefix(sanitized, "[TOOL ") && !strings.HasPrefix(sanitized, "[Tool ") {
		sanitized = "[Tool Output]\n" + sanitized
	}

	return sanitized
}

// getLocalIP returns a LAN-reachable IP address for the TTS audio server.
func getLocalIP(cfg *config.Config) string {
	host := cfg.Server.Host
	if host == "" || host == "127.0.0.1" || host == "0.0.0.0" {
		if cached, ok := localIPCache.Load(host); ok {
			return cached.(string)
		}

		conn, err := localIPDial("udp", "8.8.8.8:80")
		if err == nil {
			defer conn.Close()
			resolved := conn.LocalAddr().(*net.UDPAddr).IP.String()
			localIPCache.Store(host, resolved)
			return resolved
		}
		localIPCache.Store(host, "127.0.0.1")
		return "127.0.0.1"
	}
	return host
}

// runMemoryOrchestrator handles the Priority-Based Forgetting System across both RAG and Knowledge Graph.
func runMemoryOrchestrator(req memoryOrchestratorArgs, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph) string {
	thresholdLow := req.ThresholdLow
	if thresholdLow == 0 {
		thresholdLow = 1
	}
	thresholdMedium := req.ThresholdMedium
	if thresholdMedium == 0 {
		thresholdMedium = 3
	}

	metas, err := shortTermMem.GetAllMemoryMeta(50000, 0)
	if err != nil {
		logger.Error("Failed to fetch memory tracking metadata", "error", err)
		return fmt.Sprintf(`{"status": "error", "message": "Failed to fetch metadata: %v"}`, err)
	}

	highCount, mediumCount, lowCount := 0, 0, 0
	var lowDocs []string
	var mediumDocs []string

	for _, meta := range metas {
		if meta.Protected || meta.KeepForever {
			highCount++
			continue
		}

		lastA, err := time.Parse(time.RFC3339, strings.Replace(meta.LastAccessed, " ", "T", 1)+"Z")
		_ = lastA
		_ = err

		priority := adjustedMemoryPriority(meta, time.Now())

		if priority < thresholdLow {
			lowCount++
			lowDocs = append(lowDocs, meta.DocID)
		} else if priority < thresholdMedium {
			mediumCount++
			mediumDocs = append(mediumDocs, meta.DocID)
		} else {
			highCount++
		}
	}

	graphRemoved := 0
	if !req.Preview {
		// 1. Process VectorDB Low Priority
		for _, docID := range lowDocs {
			_ = longTermMem.DeleteDocument(docID)
			_ = shortTermMem.DeleteMemoryMeta(docID)
		}

		// 2. Process VectorDB Medium Priority (Compression)
		compressionClient, compressionModel := resolveHelperBackedLLM(cfg, client, cfg.LLM.Model)
		for _, docID := range mediumDocs {
			content, err := longTermMem.GetByID(docID)
			if err != nil || len(content) < 300 {
				continue
			}

			compressionTimeout := time.Duration(cfg.CircuitBreaker.LLMTimeoutSeconds) * time.Second
			if compressionTimeout <= 0 {
				compressionTimeout = 10 * time.Minute
			}
			compressionCtx, cancelCompression := context.WithTimeout(context.Background(), compressionTimeout)
			resp, err := llm.ExecuteWithRetry(
				compressionCtx,
				compressionClient,
				openai.ChatCompletionRequest{
					Model: compressionModel,
					Messages: []openai.ChatCompletionMessage{
						{Role: openai.ChatMessageRoleSystem, Content: "You are an AI compressing old memories. Summarize the following RAG memory into a dense, concise bullet-point list containing only core facts. Lose the verbose narrative immediately."},
						{Role: openai.ChatMessageRoleUser, Content: content},
					},
					MaxTokens: 500,
				},
				logger,
				nil,
			)
			cancelCompression()
			if err == nil && len(resp.Choices) > 0 {
				compressed := resp.Choices[0].Message.Content

				parts := strings.SplitN(content, "\n\n", 2)
				concept := "Compressed Memory"
				if len(parts) == 2 {
					concept = parts[0]
				}

				newIDs, err2 := longTermMem.StoreDocument(concept, compressed)
				if err2 == nil {
					_ = longTermMem.DeleteDocument(docID)
					_ = shortTermMem.DeleteMemoryMeta(docID)
					for _, newID := range newIDs {
						_ = shortTermMem.UpsertMemoryMeta(newID)
					}
				}
			}
		}

		// 3. Process Graph Low Priority
		graphRemoved, _ = kg.OptimizeGraph(thresholdLow)
	}

	return fmt.Sprintf(
		`{"status": "success", "preview": %v, "memory_rag": {"high_kept": %d, "medium_compressed": %d, "low_archived": %d}, "graph_nodes_archived": %d}`,
		req.Preview, highCount, mediumCount, lowCount, graphRemoved,
	)
}

// parseWorkflowPlan extracts tool names from a <workflow_plan>["t1","t2"]</workflow_plan> tag.
// Returns the parsed tool list and the content with the tag removed.
// If no tag is found, returns nil and the original content unchanged.
func parseWorkflowPlan(content string) ([]string, string) {
	const openTag = "<workflow_plan>"
	const closeTag = "</workflow_plan>"

	startIdx := strings.Index(content, openTag)
	if startIdx < 0 {
		return nil, content
	}
	endIdx := strings.Index(content[startIdx:], closeTag)
	if endIdx < 0 {
		return nil, content
	}
	endIdx += startIdx // absolute position

	inner := strings.TrimSpace(content[startIdx+len(openTag) : endIdx])
	if inner == "" {
		return nil, content
	}

	// Parse the JSON array of tool names
	var tools []string
	if err := json.Unmarshal([]byte(inner), &tools); err != nil {
		// Fallback: try comma-separated without JSON
		inner = strings.Trim(inner, "[]")
		for _, t := range strings.Split(inner, ",") {
			t = strings.Trim(strings.TrimSpace(t), "\"'")
			if t != "" {
				tools = append(tools, t)
			}
		}
	}

	if len(tools) == 0 {
		return nil, content
	}

	// Cap at 5 to prevent abuse
	if len(tools) > 5 {
		tools = tools[:5]
	}

	// Strip the tag from the content
	stripped := content[:startIdx] + content[endIdx+len(closeTag):]
	return tools, stripped
}

// extractExtraToolCalls scans content for additional valid JSON tool calls beyond the first
// one already parsed (identified by firstRawJSON). Used to handle LLM responses that contain
// multiple sequential tool calls in one message (e.g. two manage_memory adds).
func extractExtraToolCalls(content, firstRawJSON string) []ToolCall {
	var results []ToolCall
	// Skip past the already-extracted JSON blob so we don't re-parse it
	remaining := content
	if firstRawJSON != "" {
		idx := strings.Index(remaining, firstRawJSON)
		if idx >= 0 {
			remaining = remaining[idx+len(firstRawJSON):]
		}
	}
	// Extract all remaining valid JSON tool calls
	for {
		start := strings.Index(remaining, "{")
		if start == -1 {
			break
		}
		bStr := remaining[start:]
		found := false
		for j := strings.LastIndex(bStr, "}"); j > 0; {
			candidate := bStr[:j+1]
			normalized := normalizeTagsInJSON(candidate)
			var tmp ToolCall
			if json.Unmarshal([]byte(normalized), &tmp) == nil && tmp.Action != "" {
				tmp.IsTool = true
				tmp.RawJSON = candidate
				results = append(results, tmp)
				remaining = bStr[j+1:]
				found = true
				break
			}
			j = strings.LastIndex(bStr[:j], "}")
			if j < 0 {
				break
			}
		}
		if !found {
			break
		}
	}
	return results
}

var numericStringToolFields = map[string]bool{
	"wlan_index":   true,
	"tam_index":    true,
	"msg_index":    true,
	"phonebook_id": true,
	"limit":        true,
	"max_results":  true,
	"tail":         true,
}

// normalizeTagsInJSON pre-processes a JSON string before unmarshaling into ToolCall.
// It fixes a few recurring type mismatches that would otherwise cause the whole
// tool call to be discarded:
//   - "tags" sent as a JSON array        → converted to comma-separated string
//   - "body" sent as an object/array     → re-serialized as a JSON string
//   - selected numeric fields as strings → converted to JSON numbers
func normalizeTagsInJSON(s string) string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return s
	}
	changed := false

	// tags: array → comma-separated string
	if tagsRaw, ok := raw["tags"]; ok && len(tagsRaw) > 0 && tagsRaw[0] == '[' {
		var arr []string
		if err := json.Unmarshal(tagsRaw, &arr); err == nil {
			if joined, err := json.Marshal(strings.Join(arr, ",")); err == nil {
				raw["tags"] = json.RawMessage(joined)
				changed = true
			}
		}
	}

	// body: object or array → JSON string
	// ToolCall.Body is string; if the LLM sends an object literal, json.Unmarshal
	// would fail on the whole struct and IsTool would never be set.
	if bodyRaw, ok := raw["body"]; ok && len(bodyRaw) > 0 && bodyRaw[0] != '"' {
		if encoded, err := json.Marshal(string(bodyRaw)); err == nil {
			raw["body"] = json.RawMessage(encoded)
			changed = true
		}
	}

	for field := range numericStringToolFields {
		fieldRaw, ok := raw[field]
		if !ok || len(fieldRaw) < 2 || fieldRaw[0] != '"' {
			continue
		}
		var asString string
		if err := json.Unmarshal(fieldRaw, &asString); err != nil {
			continue
		}
		if v, err := strconv.Atoi(strings.TrimSpace(asString)); err == nil {
			if encoded, err := json.Marshal(v); err == nil {
				raw[field] = json.RawMessage(encoded)
				changed = true
			}
		}
	}

	if !changed {
		return s
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return s
	}
	return string(b)
}

// parseBracketToolCallBlock parses the body of a [TOOL_CALL]...[/TOOL_CALL] block.
// It handles the {tool => "name", args => {--key "value" ...}} format some models emit.
func parseBracketToolCallBlock(block string) (ToolCall, bool) {
	var tc ToolCall
	lower := strings.ToLower(block)

	toolIdx := strings.Index(lower, "tool")
	if toolIdx == -1 {
		return tc, false
	}
	// Find the arrow (=> or ->) that immediately follows the tool name.
	// First try -> (MiniMax variant), then fall back to => (standard).
	// Only accept an arrow if it appears within 40 chars of "tool" and
	// the character immediately before the arrow is whitespace or punctuation.
	searchAfterTool := toolIdx + 4
	searchLimit := searchAfterTool + 40
	if searchLimit > len(block) {
		searchLimit = len(block)
	}
	searchSlice := lower[searchAfterTool:searchLimit]

	arrowStr := "->"
	arrowIdx := strings.Index(searchSlice, arrowStr)
	arrowAbs := -1
	if arrowIdx != -1 {
		// Verify it's immediately after "tool" (only whitespace/separator between)
		beforeArrow := strings.TrimLeft(searchSlice[:arrowIdx], " \t->")
		if len(beforeArrow) == 0 {
			arrowAbs = searchAfterTool + arrowIdx
		}
	}
	if arrowAbs == -1 {
		// Fall back to standard => arrow
		arrowStr = "=>"
		arrowIdx = strings.Index(searchSlice, arrowStr)
		if arrowIdx != -1 {
			beforeArrow := strings.TrimLeft(searchSlice[:arrowIdx], " \t->")
			if len(beforeArrow) == 0 {
				arrowAbs = searchAfterTool + arrowIdx
			}
		}
	}
	if arrowAbs == -1 {
		return tc, false
	}
	afterArrow := strings.TrimSpace(block[arrowAbs+len(arrowStr):])
	if len(afterArrow) == 0 || (afterArrow[0] != '"' && afterArrow[0] != '\'') {
		return tc, false
	}
	quoteChar := afterArrow[0]
	closeQ := strings.IndexByte(afterArrow[1:], quoteChar)
	if closeQ == -1 {
		return tc, false
	}
	tc.Action = strings.TrimSpace(afterArrow[1 : 1+closeQ])
	if tc.Action == "" {
		return tc, false
	}

	fields := map[string]interface{}{
		"action": tc.Action,
	}
	rest := block
	for {
		dashIdx := strings.Index(rest, "--")
		if dashIdx == -1 {
			break
		}
		rest = rest[dashIdx+2:]
		keyEnd := strings.IndexAny(rest, " \t\r\n")
		if keyEnd == -1 {
			break
		}
		key := rest[:keyEnd]
		rest = strings.TrimLeft(rest[keyEnd:], " \t\r\n")
		if len(rest) == 0 {
			break
		}
		var val string
		if rest[0] == '"' || rest[0] == '\'' {
			qc := rest[0]
			i := 1
			for i < len(rest) {
				if rest[i] == qc && (i == 0 || rest[i-1] != '\\') {
					break
				}
				i++
			}
			val = rest[1:i]
			if i < len(rest) {
				rest = rest[i+1:]
			} else {
				rest = ""
			}
		} else {
			valEnd := strings.IndexAny(rest, " \t\r\n")
			if valEnd == -1 {
				val = rest
				rest = ""
			} else {
				val = rest[:valEnd]
				rest = rest[valEnd:]
			}
		}
		if key != "" && val != "" {
			fields[key] = val
		}
	}

	jsonBytes, err := json.Marshal(fields)
	if err != nil {
		return tc, false
	}
	var out ToolCall
	if err := json.Unmarshal(jsonBytes, &out); err != nil {
		return tc, false
	}
	out.IsTool = true
	out.RawJSON = string(jsonBytes)
	return out, true
}

func mergeToolCallParameterObject(tc *ToolCall, raw interface{}) {
	if tc == nil || raw == nil {
		return
	}

	var params map[string]interface{}
	switch v := raw.(type) {
	case map[string]interface{}:
		params = v
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return
		}
		_ = json.Unmarshal([]byte(trimmed), &params)
	case json.RawMessage:
		if len(v) == 0 {
			return
		}
		_ = json.Unmarshal(v, &params)
	}
	if len(params) == 0 {
		return
	}

	if tc.Params == nil {
		tc.Params = make(map[string]interface{}, len(params))
	}
	for key, value := range params {
		if _, exists := tc.Params[key]; !exists {
			tc.Params[key] = value
		}
	}
}

func ParseToolCall(content string) ToolCall {
	var tc ToolCall
	lowerContent := strings.ToLower(content)

	// Handle [TOOL_CALL]...[/TOOL_CALL] bracket format (custom model format).
	// Example: [TOOL_CALL]{tool => "generate_image", args => {--prompt "..." --size "1024x1792"}}[/TOOL_CALL]
	// Some models (e.g. MiniMax) use -> instead of => and ) instead of ] in the closing tag.
	if blockStart := strings.Index(lowerContent, "[tool_call]"); blockStart != -1 {
		searchArea := lowerContent[blockStart:]
		blockEnd := strings.Index(searchArea, "[/tool_call]")
		// Also check for malformed closing tag with ) instead of ]
		if blockEnd == -1 {
			blockEnd = strings.Index(searchArea, "[/tool_call)")
		}
		if blockEnd != -1 {
			block := content[blockStart+11 : blockStart+blockEnd]
			if parsed, ok := parseBracketToolCallBlock(block); ok {
				parsed.XMLFallbackDetected = true
				return parsed
			}
			// Fallback: model sent standard JSON inside [TOOL_CALL]...[/TOOL_CALL]
			// (e.g. [TOOL_CALL]{"tool": "docker_exec", "container": "caddy"}[/TOOL_CALL])
			trimmedBlock := strings.TrimSpace(block)
			if strings.HasPrefix(trimmedBlock, "{") {
				normalized := normalizeTagsInJSON(trimmedBlock)
				var tmp ToolCall
				if json.Unmarshal([]byte(normalized), &tmp) == nil {
					// Promote "tool" key to Action if Action is empty
					if tmp.Action == "" && tmp.Tool != "" {
						tmp.Action = tmp.Tool
					}
					if tmp.Action == "" && tmp.ToolCallAction != "" {
						tmp.Action = tmp.ToolCallAction
					}
					if tmp.Action == "" && tmp.Name != "" {
						tmp.Action = tmp.Name
					}
					if tmp.Action != "" {
						tmp.IsTool = true
						tmp.XMLFallbackDetected = true
						tmp.RawJSON = trimmedBlock
						return tmp
					}
				}
			}
		}
	}

	// <action>toolname</action> bare tag format — name only, no parameters.
	// Also handles the hybrid format <action>toolname","key":"value"...} where some models
	// replace {"action":"toolname" with <action>toolname (dropping the { and "action":" prefix).
	if idx := strings.Index(lowerContent, "<action>"); idx != -1 {
		rest := content[idx+8:]
		lowerRest := lowerContent[idx+8:]
		if endIdx := strings.Index(lowerRest, "</action>"); endIdx != -1 {
			// Format: <action>name</action> — bare name, no args
			name := strings.TrimSpace(rest[:endIdx])
			if name != "" && !strings.ContainsAny(name, " \t\n<>\"'{}[]") {
				tc.Action = name
				tc.IsTool = true
				tc.XMLFallbackDetected = true
				return tc
			}
		} else if strings.Contains(lowerRest, `"`) && strings.Contains(lowerRest, "}") {
			// Hybrid format: <action>toolname","key":"value"...}
			// Reconstruct as proper JSON by prepending {"action":"
			// Find the last closing brace to bound the JSON object.
			candidate := `{"action":"` + rest
			if closeIdx := strings.LastIndex(candidate, "}"); closeIdx != -1 {
				jsonStr := candidate[:closeIdx+1]
				normalized := normalizeTagsInJSON(jsonStr)
				var tmp ToolCall
				if json.Unmarshal([]byte(normalized), &tmp) == nil {
					if tmp.Action == "" && tmp.ToolCallAction != "" {
						tmp.Action = tmp.ToolCallAction
					}
					if tmp.Action == "" && tmp.Name != "" {
						tmp.Action = tmp.Name
					}
					if tmp.Action != "" {
						tmp.IsTool = true
						tmp.XMLFallbackDetected = true
						tmp.RawJSON = jsonStr
						return tmp
					}
				}
			}
		}
	}

	// Stepfun / OpenRouter <tool_call> fallback
	// Format 1: <function=name> ... </function>
	// Format 2: <tool_calls><invoke name="..."> ... </invoke></tool_calls>
	// Format 3a: minimax:tool_call\n<invoke name="..."> ... </invoke> (MiniMax XML format)
	// Format 3b: <minimax:tool_call>{"action":"name",...} or minimax:tool_call\n{"tool_call":"name",...}
	//            (GLM/Zhipu models that append a JSON object directly after the marker)
	if start := strings.Index(lowerContent, "minimax:tool_call"); start != -1 {
		if invStart := strings.Index(lowerContent[start:], "<invoke name="); invStart != -1 {
			// Format 3a: <invoke name="..."> XML body
			invStart += start
			invNameStart := invStart + 13
			invEndChar := strings.Index(lowerContent[invNameStart:], ">")
			if invEndChar != -1 {
				rawName := content[invNameStart : invNameStart+invEndChar]
				// Some models emit <invoke name="execute_shell","command":"..."> —
				// extract only the first quoted or unquoted token as the action name.
				if idx := strings.IndexAny(rawName, "\","); idx > 0 {
					rawName = rawName[:idx]
				}
				tc.Action = strings.Trim(strings.TrimSpace(rawName), "\"'")
				if tc.Action != "" {
					tc.IsTool = true
					tc.XMLFallbackDetected = true
					bodyStart := invNameStart + invEndChar + 1
					bodyEnd := strings.Index(lowerContent[bodyStart:], "</invoke>")
					if bodyEnd != -1 {
						parseXMLParams(&tc, content[bodyStart:bodyStart+bodyEnd])
					}
					return tc
				}
			}
		}
		// Format 3b: JSON object immediately follows the tag (GLM/Zhipu style)
		afterPrefix := content[start+len("minimax:tool_call"):]
		if braceIdx := strings.Index(afterPrefix, "{"); braceIdx != -1 {
			jsonSection := afterPrefix[braceIdx:]
			for j := strings.LastIndex(jsonSection, "}"); j >= 0; j = strings.LastIndex(jsonSection[:j], "}") {
				candidate := jsonSection[:j+1]
				normalized := normalizeTagsInJSON(candidate)
				var tmp ToolCall
				if json.Unmarshal([]byte(normalized), &tmp) == nil {
					// Promote proprietary key variants to Action
					if tmp.Action == "" && tmp.ToolCallAction != "" {
						tmp.Action = tmp.ToolCallAction
					}
					if tmp.Action == "" && tmp.Name != "" {
						tmp.Action = tmp.Name
					}
					if tmp.Action == "" && tmp.Tool != "" {
						tmp.Action = tmp.Tool
					}
					if tmp.Action != "" {
						tmp.IsTool = true
						tmp.XMLFallbackDetected = true
						tmp.RawJSON = candidate
						return tmp
					}
				}
				if j == 0 {
					break
				}
			}
		}
	}
	if start := strings.Index(lowerContent, "<tool_calls>"); start != -1 {
		tc.IsTool = true
		// Extract first invoke
		if invStart := strings.Index(lowerContent[start:], "<invoke name="); invStart != -1 {
			invStart += start
			invNameStart := invStart + 13
			invEndChar := strings.Index(lowerContent[invNameStart:], ">")
			if invEndChar != -1 {
				tc.Action = strings.Trim(strings.TrimSpace(content[invNameStart:invNameStart+invEndChar]), "\"'")

				// Extract params
				bodyStart := invNameStart + invEndChar + 1
				bodyEnd := strings.Index(lowerContent[bodyStart:], "</invoke>")
				if bodyEnd != -1 {
					paramSearch := content[bodyStart : bodyStart+bodyEnd]
					parseXMLParams(&tc, paramSearch)
				}
			}
		}
		return tc
	}

	if start := strings.Index(lowerContent, "<function="); start != -1 {
		end := strings.Index(lowerContent[start:], ">")
		if end != -1 {
			actionName := content[start+10 : start+end]
			actionName = strings.Trim(strings.TrimSpace(actionName), "\"'")
			tc.IsTool = true
			tc.Action = actionName

			// Extract any JSON arguments inside <function=...>{...}</function> if present
			funcBodyStart := start + end + 1
			funcBodyEnd := strings.Index(lowerContent[funcBodyStart:], "</function>")
			if funcBodyEnd != -1 {
				jsonBody := content[funcBodyStart : funcBodyStart+funcBodyEnd]
				parseXMLParams(&tc, jsonBody)
			}

			// AGGRESSIVE RECOVERY for LLMs placing the python block OUTSIDE the JSON
			if (tc.Action == "execute_python" || tc.Action == "save_tool") && tc.Code == "" {
				if blockStart := strings.Index(content, "```python"); blockStart != -1 {
					if blockEnd := strings.Index(content[blockStart+9:], "```"); blockEnd != -1 {
						tc.Code = strings.TrimSpace(content[blockStart+9 : blockStart+9+blockEnd])
					}
				}
			}
			return tc
		}
	}

	// Also allow pure operation-only JSON through (e.g. native function call arguments leaked as plain text
	// by models that emit {"operation":"list_devices"} instead of using the structured tool_calls API).
	if (strings.Contains(lowerContent, "\"action\"") || strings.Contains(lowerContent, "'action'") || strings.Contains(lowerContent, "\"tool\"") || strings.Contains(lowerContent, "\"tool_call\"") || strings.Contains(lowerContent, "\"command\"") || strings.Contains(lowerContent, "\"operation\"") || (strings.Contains(lowerContent, "\"name\"") && strings.Contains(lowerContent, "\"arguments\""))) && (strings.Contains(lowerContent, "{") || strings.Contains(lowerContent, "```")) {
		extractedFromFence := false

		// Try all common fence variants: ```json, ``` json, ```JSON, plain ```
		fenceVariants := []string{"```json\n", "```json\r\n", "```json ", "```json", "``` json", "```JSON"}
		for _, fv := range fenceVariants {
			if start := strings.Index(content, fv); start != -1 {
				after := content[start+len(fv):]
				// Trim any leading whitespace/newline after the fence marker
				after = strings.TrimLeft(after, " \t\r\n")
				// Find closing ```
				if end := strings.Index(after, "```"); end != -1 {
					candidate := strings.TrimSpace(after[:end])
					if strings.HasPrefix(candidate, "{") {
						normalized := normalizeTagsInJSON(candidate)
						var tmp ToolCall
						if json.Unmarshal([]byte(normalized), &tmp) == nil && toolCallHasRecoverableFields(tmp) {
							tc = tmp
							extractedFromFence = true
							tc.RawJSON = candidate
							break
						} else if wrapped, ok := parseWrappedToolCallJSON(normalized); ok {
							tc = wrapped
							extractedFromFence = true
							tc.RawJSON = candidate
							break
						}
					}
				}
			}
		}

		if !extractedFromFence {
			// No fence or fence extraction failed — try raw brace extraction from content.
			// Try all '{' positions as potential JSON starts.
			for i := 0; i < len(content); i++ {
				if content[i] == '{' {
					bStr := content[i:]
					// Search from the end for the furthest '}' that yields a valid ToolCall
					for j := strings.LastIndex(bStr, "}"); j != -1; j = strings.LastIndex(bStr[:j], "}") {
						candidate := bStr[:j+1]
						normalized := normalizeTagsInJSON(candidate)
						var tmp ToolCall
						if json.Unmarshal([]byte(normalized), &tmp) == nil && toolCallHasRecoverableFields(tmp) {
							tc = tmp
							extractedFromFence = true
							tc.RawJSON = candidate
							break
						} else if wrapped, ok := parseWrappedToolCallJSON(normalized); ok {
							tc = wrapped
							extractedFromFence = true
							tc.RawJSON = candidate
							break
						}
					}
				}
				if extractedFromFence {
					break
				}
			}
		}
		if tc.Action != "" || tc.ToolCallAction != "" || tc.Operation != "" || tc.Name != "" || tc.Tool != "" || tc.Command != "" {
			tc.IsTool = true

			// AGGRESSIVE RECOVERY: Handle wrappers like {"action": "execute_tool", "tool": "name", "args": {...}}
			if (tc.Action == "execute_tool" || tc.Action == "run_tool" || tc.Action == "execute_tool_call") && tc.Tool != "" {
				tc.Action = tc.Tool
			}

			// Fallback: LLM used "tool" key instead of "action"
			if tc.Action == "" && tc.Tool != "" {
				tc.Action = tc.Tool
			}

			// Fallback: MiniMax / models that use "tool_call" key instead of "action"
			if tc.Action == "" && tc.ToolCallAction != "" {
				tc.Action = tc.ToolCallAction
			}

			// Fallback: LLM sent only "command" — treat as execute_shell
			if tc.Action == "" && tc.Command != "" {
				tc.Action = "execute_shell"
			}

			// Fallback: MeshCentral LLM hallucinated operation as action or omitted action entirely.
			// This covers the case where the model emits bare native-function-call arguments as plain
			// text (e.g. {"operation":"list_devices"} without an "action" wrapper).
			if tc.Action == "" && tc.Operation != "" {
				switch strings.ToLower(tc.Operation) {
				case "list_groups", "list_devices", "nodes", "meshes", "wake", "power_action", "run_command":
					// These operation values are unique to the meshcentral tool.
					tc.Action = "meshcentral"
				}
				// Generic heuristic: JSON body or struct fields hint at meshcentral.
				if tc.Action == "" {
					if strings.Contains(tc.RawJSON, "\"meshcentral\"") || tc.MeshID != "" || tc.NodeID != "" || tc.PowerAction != 0 {
						tc.Action = "meshcentral"
					}
				}
			}

			// Fallback: OpenAI native function_call format {"name": "tool", "arguments": {...}}
			if tc.Action == "" && tc.Name != "" {
				tc.Action = tc.Name
			}

			// If LLM uses 'arguments' (hallucination)
			if tc.Arguments != nil {
				if tc.Params == nil {
					tc.Params = make(map[string]interface{})
				}
				switch v := tc.Arguments.(type) {
				case map[string]interface{}:
					for k, val := range v {
						tc.Params[k] = val
					}
				case string:
					// Robust recovery: the LLM sometimes JSON-encodes the arguments into a string
					var argMap map[string]interface{}
					if err := json.Unmarshal([]byte(v), &argMap); err == nil {
						for k, val := range argMap {
							tc.Params[k] = val
						}
					}
				}
			}

			mergeToolCallParameterObject(&tc, tc.Parameters)

			// Recovery for map-based 'args' which fails to unmarshal into tc.Args ([]string)
			if argsMap, ok := tc.Args.(map[string]interface{}); ok {
				if tc.Params == nil {
					tc.Params = make(map[string]interface{})
				}
				for k, v := range argsMap {
					tc.Params[k] = v
				}
			}

			// Flatten action_input (LangChain-style nested params) into Params
			if tc.ActionInput != nil {
				if tc.Params == nil {
					tc.Params = make(map[string]interface{})
				}
				for k, v := range tc.ActionInput {
					tc.Params[k] = v
				}
			}

			// Final parameter promotion: Ensure specific fields are populated from Params if missing
			if tc.Params != nil {
				promoteString := func(target *string, keys ...string) {
					if *target != "" {
						return
					}
					for _, k := range keys {
						if v, ok := tc.Params[k].(string); ok && v != "" {
							*target = v
							return
						}
					}
				}
				promoteInt := func(target *int, keys ...string) {
					if *target != 0 {
						return
					}
					for _, k := range keys {
						if v, ok := tc.Params[k].(float64); ok && v != 0 {
							*target = int(v)
							return
						}
					}
				}

				promoteString(&tc.Hostname, "hostname", "host", "server_id")
				promoteString(&tc.IPAddress, "ip_address", "ip", "address")
				promoteString(&tc.Username, "username", "user")
				promoteString(&tc.Password, "password", "pass")
				promoteString(&tc.Tags, "tags", "tag")
				promoteString(&tc.PrivateKeyPath, "private_key_path", "key_path", "private_key")
				promoteString(&tc.ServerID, "server_id", "serverId", "id", "hostname", "host")
				promoteString(&tc.Command, "command", "cmd")
				promoteString(&tc.Tag, "tag", "tags")
				promoteString(&tc.LocalPath, "local_path", "localPath", "source")
				promoteString(&tc.RemotePath, "remote_path", "remotePath", "destination", "dest")
				promoteString(&tc.Direction, "direction")
				promoteString(&tc.Operation, "operation", "op")
				promoteString(&tc.FilePath, "file_path", "path", "filepath", "filename", "file")
				if tc.FilePath != "" && tc.Path == "" {
					tc.Path = tc.FilePath
				}
				promoteString(&tc.Destination, "destination", "dest", "target")
				if tc.Destination != "" && tc.Dest == "" {
					tc.Dest = tc.Destination
				}
				promoteString(&tc.Content, "content", "query")
				promoteString(&tc.Query, "query", "content")
				promoteString(&tc.Name, "name")
				promoteString(&tc.Description, "description")
				promoteString(&tc.Code, "code", "script")
				promoteString(&tc.Package, "package", "package_name")
				promoteString(&tc.ToolName, "tool_name", "toolName")
				promoteString(&tc.Label, "label")
				promoteString(&tc.TaskPrompt, "task_prompt")
				promoteString(&tc.Prompt, "prompt")
				// Notes / Vision / STT fields
				promoteString(&tc.Title, "title")
				promoteString(&tc.Category, "category")
				promoteString(&tc.DueDate, "due_date", "dueDate")
				// Home Assistant fields
				promoteString(&tc.EntityID, "entity_id", "entityId", "entity")
				promoteString(&tc.Domain, "domain")
				promoteString(&tc.Service, "service")
				// Docker fields
				promoteString(&tc.ContainerID, "container_id", "containerId", "container")
				promoteString(&tc.Image, "image")
				promoteString(&tc.Restart, "restart", "restart_policy")
				// Co-Agent fields
				promoteString(&tc.CoAgentID, "co_agent_id", "coAgentId", "coagent_id", "agent_id", "agentId")
				promoteString(&tc.Task, "task")
				// Invasion Control fields
				promoteString(&tc.NestID, "nest_id", "nestId")
				promoteString(&tc.NestName, "nest_name", "nestName")
				promoteString(&tc.EggID, "egg_id", "eggId")
				promoteString(&tc.EggName, "egg_name", "eggName")
				// context_hints is []string — promote manually
				if len(tc.ContextHints) == 0 {
					for _, k := range []string{"context_hints", "contextHints", "hints"} {
						if arr, ok := tc.Params[k].([]interface{}); ok && len(arr) > 0 {
							for _, v := range arr {
								if s, ok := v.(string); ok {
									tc.ContextHints = append(tc.ContextHints, s)
								}
							}
							break
						}
					}
				}

				promoteInt(&tc.Port, "port")
				promoteInt(&tc.PID, "pid")
				promoteInt(&tc.Priority, "priority")
				promoteInt(&tc.Done, "done")
				// Media Registry / Homepage Registry fields
				promoteString(&tc.MediaType, "media_type", "mediaType", "type")
				promoteString(&tc.TagMode, "tag_mode", "tagMode", "mode")
				promoteString(&tc.Reason, "reason")
				promoteString(&tc.Problem, "problem")
				promoteString(&tc.Status, "status")
				promoteString(&tc.Notes, "notes", "note")
				// NoteID is int64 — promote manually
				if tc.NoteID == 0 {
					for _, k := range []string{"note_id", "noteId", "id"} {
						if v, ok := tc.Params[k].(float64); ok && v != 0 {
							tc.NoteID = int64(v)
							break
						}
					}
				}
			}

			// AGGRESSIVE RECOVERY for LLMs placing the python block OUTSIDE the JSON
			if (tc.Action == "execute_python" || tc.Action == "save_tool") && tc.Code == "" {
				if blockStart := strings.Index(content, "```python"); blockStart != -1 {
					if blockEnd := strings.Index(content[blockStart+9:], "```"); blockEnd != -1 {
						tc.Code = strings.TrimSpace(content[blockStart+9 : blockStart+9+blockEnd])
					}
				}
			}
		}
		return tc
	}

	// ── Native-function bare-args fallback ───────────────────────────────────
	// Some providers (e.g. InceptionLabs Mercury) emit raw function arguments in
	// message content instead of proper ToolCalls, without an "action" field.
	// Infer the action from the unique field combination present in the JSON.
	if strings.Contains(lowerContent, "{") {
		for i := 0; i < len(content); i++ {
			if content[i] == '{' {
				bStr := content[i:]
				for j := strings.LastIndex(bStr, "}"); j != -1; j = strings.LastIndex(bStr[:j], "}") {
					candidate := bStr[:j+1]
					var tmp ToolCall
					if json.Unmarshal([]byte(candidate), &tmp) == nil && tmp.Action == "" {
						switch {
						case tmp.Path != "" && (tmp.Title != "" || tmp.Caption != "") && isSupportedVideoFormat(strings.TrimPrefix(filepath.Ext(tmp.Path), ".")):
							// send_video: {"path": "...mp4", "title": "..."}
							tmp.Action = "send_video"
						case tmp.Path != "" && tmp.Caption != "":
							// send_image: {"path": "...", "caption": "..."}
							tmp.Action = "send_image"
						case tmp.Skill != "" && tmp.SkillArgs != nil:
							// execute_skill: {"skill": "...", "skill_args": {...}}
							tmp.Action = "execute_skill"
						case tmp.Content != "" && tmp.Fact == "" && tmp.Command == "":
							// query_memory / store_memory: ambiguous without action, skip
						default:
							continue
						}
						if tmp.Action != "" {
							tmp.IsTool = true
							tmp.RawJSON = candidate
							return tmp
						}
					}
				}
				break
			}
		}
	}

	if command, ok := parseBareDiagnosticShellCommand(content); ok {
		return ToolCall{
			IsTool:  true,
			Action:  "execute_shell",
			Command: command,
		}
	}

	if strings.HasPrefix(lowerContent, "import ") ||
		strings.HasPrefix(lowerContent, "def ") ||
		strings.HasPrefix(lowerContent, "print(") ||
		strings.HasPrefix(lowerContent, "# ") ||
		strings.Contains(lowerContent, "```python") {
		return ToolCall{RawCodeDetected: true}
	}

	return ToolCall{}
}

func parseBareDiagnosticShellCommand(content string) (string, bool) {
	cmd := strings.TrimSpace(content)
	if cmd == "" || strings.ContainsAny(cmd, "\r\n") {
		return "", false
	}
	if strings.HasPrefix(cmd, "```") || strings.HasPrefix(cmd, "{") || strings.HasPrefix(cmd, "[") {
		return "", false
	}

	lower := strings.ToLower(cmd)
	blocked := []string{
		";", "&&", " sudo ", " su ", " rm ", " del ", " rmdir ",
		" chmod ", " chown ", " shutdown", " reboot", " curl ", " wget ",
		"docker rm", "docker stop", "docker restart", "docker kill", "docker run",
		"docker exec", "docker compose down", "docker compose rm",
		"powershell -enc", "powershell -encodedcommand",
	}
	padded := " " + lower + " "
	for _, blockedPattern := range blocked {
		if strings.Contains(padded, blockedPattern) {
			return "", false
		}
	}

	allowedPrefixes := []string{
		"docker stats", "docker ps", "docker system df", "docker compose ps",
		"free", "ps", "top", "df", "du", "tasklist", "get-process",
		"get-ciminstance", "get-counter", "wmic",
	}
	for _, prefix := range allowedPrefixes {
		if lower == prefix || strings.HasPrefix(lower, prefix+" ") || strings.HasPrefix(lower, prefix+"|") || strings.HasPrefix(lower, prefix+"\t") {
			return cmd, true
		}
	}

	return "", false
}

func toolCallHasRecoverableFields(tc ToolCall) bool {
	return tc.Action != "" ||
		tc.ToolCallAction != "" ||
		tc.Tool != "" ||
		tc.Command != "" ||
		tc.Name != "" ||
		tc.Operation != ""
}

func parseWrappedToolCallJSON(raw string) (ToolCall, bool) {
	type wrapperCall struct {
		Name      string      `json:"name"`
		Operation string      `json:"operation"`
		Path      string      `json:"path"`
		Content   string      `json:"content"`
		Target    string      `json:"target"`
		Arguments interface{} `json:"arguments"`
		Function  struct {
			Name      string      `json:"name"`
			Arguments interface{} `json:"arguments"`
		} `json:"function"`
	}
	type wrapper struct {
		ToolCalls []wrapperCall `json:"tool_calls"`
	}

	var parsed wrapper
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil || len(parsed.ToolCalls) == 0 {
		return ToolCall{}, false
	}

	first := parsed.ToolCalls[0]
	tc := ToolCall{
		Action:    strings.TrimSpace(first.Name),
		Operation: strings.TrimSpace(first.Operation),
		Path:      strings.TrimSpace(first.Path),
		Content:   first.Content,
		Target:    strings.TrimSpace(first.Target),
		Arguments: first.Arguments,
	}
	if tc.Action == "" {
		tc.Action = strings.TrimSpace(first.Function.Name)
	}
	if first.Function.Arguments != nil {
		tc.Arguments = first.Function.Arguments
	}
	if tc.Action == "" && !toolCallHasRecoverableFields(tc) {
		return ToolCall{}, false
	}
	tc.IsTool = true
	return tc, true
}

func parseXMLParams(tc *ToolCall, body string) {
	hasXMLParams := false
	lowerBody := strings.ToLower(body)
	paramSearch := lowerBody

	for {
		// Support <parameter=name> and <parameter name="...">
		pStart := strings.Index(paramSearch, "<parameter")
		if pStart == -1 {
			break
		}
		pAttrEnd := strings.Index(paramSearch[pStart:], ">")
		if pAttrEnd == -1 {
			break
		}
		pAttrEnd += pStart

		attrStr := body[pStart : pAttrEnd+1]
		paramName := ""
		if strings.Contains(attrStr, "=") {
			// <parameter=name>
			eqIdx := strings.Index(attrStr, "=")
			paramName = strings.Trim(strings.TrimSpace(attrStr[eqIdx+1:len(attrStr)-1]), "\"' ")
		} else if strings.Contains(attrStr, "name=") {
			// <parameter name="name">
			nameIdx := strings.Index(attrStr, "name=")
			paramName = strings.Trim(strings.TrimSpace(attrStr[nameIdx+5:len(attrStr)-1]), "\"' ")
		}

		vStart := pAttrEnd + 1
		vEndOffset := strings.Index(paramSearch[vStart:], "</parameter>")
		if vEndOffset == -1 {
			break
		}

		paramVal := strings.TrimSpace(body[vStart : vStart+vEndOffset])
		hasXMLParams = true

		switch paramName {
		case "code":
			tc.Code = paramVal
		case "name":
			tc.Name = strings.Trim(paramVal, "\"'")
		case "tool_name":
			tc.ToolName = strings.Trim(paramVal, "\"'")
		case "package":
			tc.Package = strings.Trim(paramVal, "\"'")
		case "key":
			tc.Key = strings.Trim(paramVal, "\"'")
		case "value":
			tc.Value = strings.Trim(paramVal, "\"'")
		case "skill":
			tc.Skill = strings.Trim(paramVal, "\"'")
		case "skill_args", "params":
			_ = json.Unmarshal([]byte(paramVal), &tc.Params)
			tc.SkillArgs = tc.Params
		case "operation":
			tc.Operation = strings.Trim(paramVal, "\"'")
		case "file_path", "path":
			tc.FilePath = strings.Trim(paramVal, "\"'")
			tc.Path = tc.FilePath
		case "destination", "dest":
			tc.Destination = strings.Trim(paramVal, "\"'")
			tc.Dest = tc.Destination
		case "content":
			tc.Content = paramVal
		case "query":
			tc.Query = paramVal
		case "task_prompt", "plan", "description":
			tc.TaskPrompt = paramVal
		case "prompt":
			tc.Prompt = paramVal
		case "title":
			tc.Title = strings.Trim(paramVal, "\"'")
		case "category":
			tc.Category = strings.Trim(paramVal, "\"'")
		case "priority":
			if v, err := strconv.Atoi(strings.TrimSpace(paramVal)); err == nil {
				tc.Priority = v
			}
		case "due_date":
			tc.DueDate = strings.Trim(paramVal, "\"'")
		case "note_id":
			if v, err := strconv.ParseInt(strings.TrimSpace(paramVal), 10, 64); err == nil {
				tc.NoteID = v
			}
		case "done":
			if v, err := strconv.Atoi(strings.TrimSpace(paramVal)); err == nil {
				tc.Done = v
			}
		case "args":
			_ = json.Unmarshal([]byte(paramVal), &tc.Args)
		case "new_str":
			// MiniMax and other models use "new_str" instead of the canonical "new" key.
			// Map to tc.Params["new"] so decodeFileEditorArgs can retrieve it.
			if tc.Params == nil {
				tc.Params = make(map[string]interface{})
			}
			tc.Params["new"] = paramVal
		default:
			// Store all unrecognized params in tc.Params so tool decode functions
			// (e.g. decodeFileEditorArgs) can retrieve them via toolArgString.
			if paramName != "" {
				if tc.Params == nil {
					tc.Params = make(map[string]interface{})
				}
				if _, alreadySet := tc.Params[paramName]; !alreadySet {
					tc.Params[paramName] = paramVal
				}
			}
		}

		// advance
		advance := vStart + vEndOffset + 12
		if advance >= len(paramSearch) {
			break
		}
		paramSearch = paramSearch[advance:]
		body = body[advance:]
	}

	// 2. If no XML parameters were found, fallback to parsing as JSON
	if !hasXMLParams {
		jsonBody := strings.TrimSpace(body)
		// Strip markdown markdown block strings
		if strings.HasPrefix(jsonBody, "```json") {
			jsonBody = strings.TrimPrefix(jsonBody, "```json")
		} else if strings.HasPrefix(jsonBody, "```") {
			jsonBody = strings.TrimPrefix(jsonBody, "```")
		}
		jsonBody = strings.TrimSuffix(jsonBody, "```")
		jsonBody = strings.TrimSpace(jsonBody)

		if strings.HasPrefix(jsonBody, "{") {
			_ = json.Unmarshal([]byte(jsonBody), tc)
		}
	}
}

func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return truncateUTF8Prefix(s, n) + "..."
}

func truncateUTF8Prefix(s string, maxBytes int) string {
	if maxBytes <= 0 || s == "" {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}

	cut := maxBytes
	for cut > 0 && cut < len(s) && !utf8.RuneStart(s[cut]) {
		cut--
	}
	for cut > 0 && !utf8.ValidString(s[:cut]) {
		cut--
		for cut > 0 && cut < len(s) && !utf8.RuneStart(s[cut]) {
			cut--
		}
	}
	return s[:cut]
}

func truncateUTF8ToLimit(s string, limit int, suffix string) string {
	if limit <= 0 {
		return ""
	}
	if len(s) <= limit {
		return s
	}
	if suffix == "" {
		return truncateUTF8Prefix(s, limit)
	}
	if len(suffix) >= limit {
		return truncateUTF8Prefix(suffix, limit)
	}
	return truncateUTF8Prefix(s, limit-len(suffix)) + suffix
}

// isFollowUpQuestion returns true when a follow_up task_prompt looks like a
// question directed at the user rather than a self-contained task for the agent.
// Using follow_up to ask for user input causes infinite recursion because each
// invocation re-triggers the same unanswerable question.
func isFollowUpQuestion(prompt string) bool {
	// Ends with a question mark → almost certainly a user-facing question
	if strings.HasSuffix(prompt, "?") {
		return true
	}

	// Common German/English phrases that introduce a request for user input
	lower := strings.ToLower(prompt)
	questionPhrases := []string{
		"bitte gib",
		"bitte teile",
		"bitte sag",
		"bitte nenne",
		"bitte geben sie",
		"please provide",
		"please tell me",
		"please give",
		"please specify",
		"please enter",
		"köntest du",
		"könntest du",
		"kannst du mir",
		"what is the",
		"what path",
		"which path",
		"what interval",
	}
	for _, phrase := range questionPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// GetActiveProcessStatus returns a comma-separated string of PIDs for the manifest sysprompt.
func GetActiveProcessStatus(registry *tools.ProcessRegistry) string {
	list := registry.List()
	if len(list) == 0 {
		return "None"
	}
	var names []string
	for _, p := range list {
		alive, _ := p["alive"].(bool)
		if alive {
			pid, _ := p["pid"].(int)
			names = append(names, fmt.Sprintf("PID:%d", pid))
		}
	}
	if len(names) == 0 {
		return "None"
	}
	return strings.Join(names, ", ")
}

// runGitCommand helper runs a git command with enforced environment and safe.directory config.
func runGitCommand(dir string, args ...string) ([]byte, error) {
	// Add safe.directory to bypass ownership warnings when running as root in user dirs
	fullArgs := append([]string{"-c", "safe.directory=" + dir}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Dir = dir

	// Ensure HOME is set, otherwise git may fail with exit status 128
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/root" // Default for root-run services
	}
	cmd.Env = append(os.Environ(), "HOME="+home)

	return cmd.CombinedOutput()
}

// handleWebhookToolCall processes manage_webhooks tool calls.
func handleWebhookToolCall(req manageWebhooksArgs, mgr *webhooks.Manager, logger *slog.Logger) string {
	switch req.Operation {
	case "list":
		list := mgr.List()
		type summary struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Slug    string `json:"slug"`
			Enabled bool   `json:"enabled"`
			URL     string `json:"url"`
		}
		out := make([]summary, len(list))
		for i, w := range list {
			out[i] = summary{ID: w.ID, Name: w.Name, Slug: w.Slug, Enabled: w.Enabled, URL: "/webhook/" + w.Slug}
		}
		data, _ := json.Marshal(map[string]any{"status": "ok", "webhooks": out})
		return "Tool Output: " + string(data)

	case "get":
		if req.ID == "" {
			return `Tool Output: {"status":"error","message":"id is required"}`
		}
		w, err := mgr.Get(req.ID)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err)
		}
		data, _ := json.Marshal(map[string]any{"status": "ok", "webhook": w})
		return "Tool Output: " + string(data)

	case "create":
		w := webhooks.Webhook{
			Name:    req.Name,
			Slug:    req.Slug,
			Enabled: true,
		}
		if req.TokenID != "" {
			w.TokenID = req.TokenID
		}
		created, err := mgr.Create(w)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err)
		}
		logger.Info("Webhook created via tool", "id", created.ID, "slug", created.Slug)
		data, _ := json.Marshal(map[string]any{"status": "ok", "webhook_id": created.ID, "slug": created.Slug, "url": "/webhook/" + created.Slug})
		return "Tool Output: " + string(data)

	case "update":
		if req.ID == "" {
			return `Tool Output: {"status":"error","message":"id is required"}`
		}
		patch := webhooks.Webhook{Name: req.Name, Slug: req.Slug, Enabled: req.Enabled}
		if req.TokenID != "" {
			patch.TokenID = req.TokenID
		}
		updated, err := mgr.Update(req.ID, patch)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err)
		}
		logger.Info("Webhook updated via tool", "id", updated.ID)
		data, _ := json.Marshal(map[string]any{"status": "ok", "webhook_id": updated.ID})
		return "Tool Output: " + string(data)

	case "delete":
		if req.ID == "" {
			return `Tool Output: {"status":"error","message":"id is required"}`
		}
		err := mgr.Delete(req.ID)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err)
		}
		logger.Info("Webhook deleted via tool", "id", req.ID)
		return `Tool Output: {"status":"ok","message":"webhook deleted"}`

	case "logs":
		whLog := mgr.GetLog()
		n := 20
		var entries []webhooks.LogEntry
		if req.ID != "" {
			entries = whLog.ForWebhook(req.ID, n)
		} else {
			entries = whLog.Recent(n)
		}
		data, _ := json.Marshal(map[string]any{"status": "ok", "entries": entries})
		return "Tool Output: " + string(data)

	default:
		return `Tool Output: {"status":"error","message":"Unknown operation. Use: list, get, create, update, delete, logs"}`
	}
}

// decodeBase64 decodes a standard or URL-safe base64 string.
func decodeBase64(s string) ([]byte, error) {
	// Try standard encoding first, then URL-safe
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		data, err = base64.URLEncoding.DecodeString(s)
	}
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(s)
	}
	return data, err
}

// calculateEffectiveMaxCalls berechnet das effektive Circuit Breaker Limit
// basierend auf Personality Traits, Homepage-MaxCalls und explizitem Override.
// homepageActiveInChain wird true sobald das Homepage-Tool in der aktuellen Aktionskette
// aufgerufen wurde – ab dann gilt das erhöhte Limit für die gesamte Kette.
func calculateEffectiveMaxCalls(cfg *config.Config, tc ToolCall, homepageActiveInChain bool, personalityEnabled bool, shortTermMem *memory.SQLiteMemory, logger *slog.Logger) int {
	effectiveMaxCalls := cfg.CircuitBreaker.MaxToolCalls

	// 1. Personality Engine V2: Thoroughness Trait
	if personalityEnabled && cfg.Personality.EngineV2 && shortTermMem != nil {
		if traits, err := shortTermMem.GetTraits(); err == nil {
			if thoroughness, ok := traits[memory.TraitThoroughness]; ok && thoroughness > 0.8 {
				effectiveMaxCalls = int(float64(effectiveMaxCalls) * 1.5)
				logger.Debug("[Behavioral Tool Calling] Increased MaxToolCalls due to high Thoroughness", "new_max", effectiveMaxCalls)
			}
		}
	}

	// 1b. Structured emotion state: when the agent is in a tense recovery state,
	// slightly reduce exploratory retries so it focuses on one concrete correction.
	if personalityEnabled && shortTermMem != nil {
		if latestEmotion, err := shortTermMem.GetLatestEmotion(); err == nil && latestEmotion != nil {
			if latestEmotion.Confidence >= 0.45 && latestEmotion.Valence <= -0.25 && latestEmotion.Arousal >= 0.65 {
				if effectiveMaxCalls > 1 {
					effectiveMaxCalls--
					logger.Debug("[Behavioral Tool Calling] Reduced MaxToolCalls due to tense recovery state",
						"new_max", effectiveMaxCalls,
						"valence", latestEmotion.Valence,
						"arousal", latestEmotion.Arousal,
						"confidence", latestEmotion.Confidence)
				}
			}
		}
	}

	// 2. Homepage Tool: absolutes Limit für komplexe Web-Workflows
	// Gilt sobald Homepage in der aktuellen Kette aktiv ist ODER der aktuelle Call homepage ist.
	isHomepage := tc.Action == "homepage" || tc.Action == "homepage_tool" || tc.Tool == "homepage"
	if (isHomepage || homepageActiveInChain) && cfg.Homepage.Enabled && cfg.Homepage.CircuitBreakerMaxCalls > 0 {
		if cfg.Homepage.CircuitBreakerMaxCalls > effectiveMaxCalls {
			logger.Debug("[Circuit Breaker] Homepage max calls applied", "base_limit", effectiveMaxCalls, "homepage_limit", cfg.Homepage.CircuitBreakerMaxCalls)
			effectiveMaxCalls = cfg.Homepage.CircuitBreakerMaxCalls
		}
	}

	// 3. Expliziter Override im ToolCall (höchste Priorität)
	// Nur anwenden wenn tc bekannt ist
	if tc.Tool != "" && tc.CircuitBreakerOverride > 0 {
		// Max 3x Standard-Limit für Sicherheit
		maxAllowed := cfg.CircuitBreaker.MaxToolCalls * 3
		if tc.CircuitBreakerOverride > maxAllowed {
			logger.Warn("[Circuit Breaker] Override exceeds maximum allowed, capping", "requested", tc.CircuitBreakerOverride, "max_allowed", maxAllowed)
			effectiveMaxCalls = maxAllowed
		} else {
			logger.Debug("[Circuit Breaker] Explicit override applied", "override", tc.CircuitBreakerOverride)
			effectiveMaxCalls = tc.CircuitBreakerOverride
		}
	}

	return effectiveMaxCalls
}

// calculateEffectivePromptTokenBudget starts from the adaptive base prompt
// budget and temporarily raises it further for homepage action chains when
// explicitly allowed in config. The homepage uplift scales relative to the
// higher homepage circuit breaker limit.
func calculateEffectivePromptTokenBudget(cfg *config.Config, tc ToolCall, homepageActiveInChain bool, logger *slog.Logger) int {
	if cfg == nil {
		return 0
	}

	baseBudget := config.CalculateAdaptiveSystemPromptTokenBudget(cfg)
	if baseBudget <= 0 {
		return 0
	}

	if !cfg.Homepage.Enabled || !cfg.Homepage.AllowTemporaryTokenBudgetOverflow {
		return baseBudget
	}

	isHomepage := tc.Action == "homepage" || tc.Action == "homepage_tool" || tc.Tool == "homepage"
	if !isHomepage && !homepageActiveInChain {
		return baseBudget
	}

	baseCalls := cfg.CircuitBreaker.MaxToolCalls
	if baseCalls <= 0 {
		baseCalls = 1
	}

	homepageCalls := cfg.Homepage.CircuitBreakerMaxCalls
	if homepageCalls <= baseCalls {
		return baseBudget
	}

	scaledBudget := int(math.Round(float64(baseBudget) * (float64(homepageCalls) / float64(baseCalls))))
	if scaledBudget <= baseBudget {
		return baseBudget
	}

	contextWindow := cfg.Agent.ContextWindow
	if contextWindow <= 0 {
		contextWindow = 163840
	}
	maxBudget := contextWindow - 8192
	if maxBudget < baseBudget {
		maxBudget = baseBudget
	}
	if scaledBudget > maxBudget {
		scaledBudget = maxBudget
	}

	if logger != nil && scaledBudget > baseBudget {
		logger.Debug("[Prompt Budget] Homepage token budget override applied",
			"base_budget", baseBudget,
			"effective_budget", scaledBudget,
			"base_calls", baseCalls,
			"homepage_calls", homepageCalls)
	}

	return scaledBudget
}

// toolCallScanText extracts the most security-relevant text fields from a ToolCall
// for regex-based injection scanning.
func toolCallScanText(tc ToolCall) string {
	parts := make([]string, 0, 4)
	if tc.Command != "" {
		parts = append(parts, tc.Command)
	}
	if tc.Code != "" {
		parts = append(parts, tc.Code)
	}
	if tc.Content != "" {
		parts = append(parts, tc.Content)
	}
	if tc.Body != "" {
		parts = append(parts, tc.Body)
	}
	if tc.URL != "" {
		parts = append(parts, tc.URL)
	}
	return strings.Join(parts, "\n")
}

// toolCallParams extracts key parameters from a ToolCall as a flat map for LLM Guardian evaluation.
func toolCallParams(tc ToolCall) map[string]string {
	m := make(map[string]string)
	if tc.Operation != "" {
		m["operation"] = tc.Operation
	}
	if tc.Command != "" {
		m["command"] = tc.Command
	}
	if tc.Code != "" {
		if len(tc.Code) > 300 {
			m["code"] = truncateUTF8Prefix(tc.Code, 300)
		} else {
			m["code"] = tc.Code
		}
	}
	if path := firstNonEmpty(tc.FilePath, tc.Path); path != "" {
		m["file_path"] = guardianDisplayPath(tc, path)
		if scope := guardianPathScope(tc, path); scope != "" {
			m["path_scope"] = scope
		}
	}
	if dest := firstNonEmpty(tc.Destination, tc.Dest); dest != "" {
		m["destination"] = guardianDisplayPath(tc, dest)
	}
	if len(tc.Items) > 0 {
		m["item_count"] = strconv.Itoa(len(tc.Items))
		if path := firstNonEmpty(batchItemValue(tc.Items[0], "file_path", "path")); path != "" {
			m["first_item_path"] = guardianDisplayPath(tc, path)
		}
		if dest := firstNonEmpty(batchItemValue(tc.Items[0], "destination", "dest")); dest != "" {
			m["first_item_destination"] = guardianDisplayPath(tc, dest)
		}
	}
	if tc.URL != "" {
		m["url"] = tc.URL
	}
	if tc.Hostname != "" {
		m["hostname"] = tc.Hostname
	}
	if tc.Name != "" {
		m["name"] = tc.Name
	}
	return m
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func batchItemValue(item map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

func guardianPathScope(tc ToolCall, path string) string {
	if path == "" {
		return ""
	}
	if tc.Action == "filesystem" || tc.Action == "filesystem_op" || tc.Action == "file_reader_advanced" || tc.Action == "smart_file_read" || tc.Action == "file_search" || tc.Action == "file_editor" {
		clean := filepath.ToSlash(filepath.Clean(path))
		if strings.HasPrefix(clean, "../../") {
			return "project_root_relative"
		}
	}
	return ""
}

func guardianDisplayPath(tc ToolCall, path string) string {
	if guardianPathScope(tc, path) != "project_root_relative" {
		return path
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	clean = strings.TrimPrefix(clean, "../../")
	return "project_root/" + clean
}
