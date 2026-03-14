package agent

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/tools"
	"aurago/internal/webhooks"

	"github.com/sashabaranov/go-openai"
)

// DispatchToolCall executes the appropriate tool based on the parsed ToolCall.
// It automatically handles Redaction, Guardian sanitization, and ensures the output
// is correctly prefixed with "[Tool Output]\n" unless it's a known error marker.
func DispatchToolCall(ctx context.Context, tc ToolCall, cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, missionManager *tools.MissionManager, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, invasionDB *sql.DB, cheatsheetDB *sql.DB, imageGalleryDB *sql.DB, mediaRegistryDB *sql.DB, homepageRegistryDB *sql.DB, remoteHub *remote.RemoteHub, historyMgr *memory.HistoryManager, isMaintenance bool, surgeryPlan string, guardian *security.Guardian, sessionID string, coAgentRegistry *CoAgentRegistry, budgetTracker *budget.Tracker) string {

	rawResult := dispatchInner(ctx, tc, cfg, logger, llmClient, vault, registry, manifest, cronManager, missionManager, longTermMem, shortTermMem, kg, inventoryDB, invasionDB, cheatsheetDB, imageGalleryDB, mediaRegistryDB, homepageRegistryDB, remoteHub, historyMgr, isMaintenance, surgeryPlan, guardian, sessionID, coAgentRegistry, budgetTracker)

	// Apply redaction to tool output
	sanitized := security.RedactSensitiveInfo(rawResult)

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
		conn, err := net.Dial("udp", "8.8.8.8:80")
		if err == nil {
			defer conn.Close()
			return conn.LocalAddr().(*net.UDPAddr).IP.String()
		}
		return "127.0.0.1"
	}
	return host
}

// runMemoryOrchestrator handles the Priority-Based Forgetting System across both RAG and Knowledge Graph.
func runMemoryOrchestrator(tc ToolCall, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph) string {
	thresholdLow := tc.ThresholdLow
	if thresholdLow == 0 {
		thresholdLow = 1
	}
	thresholdMedium := tc.ThresholdMedium
	if thresholdMedium == 0 {
		thresholdMedium = 3
	}

	metas, err := shortTermMem.GetAllMemoryMeta()
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
		daysSince := 0
		if err == nil {
			daysSince = int(time.Since(lastA).Hours() / 24)
		}

		priority := meta.AccessCount - daysSince

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
	if !tc.Preview {
		// 1. Process VectorDB Low Priority
		for _, docID := range lowDocs {
			_ = longTermMem.DeleteDocument(docID)
			_ = shortTermMem.DeleteMemoryMeta(docID)
		}

		// 2. Process VectorDB Medium Priority (Compression)
		for _, docID := range mediumDocs {
			content, err := longTermMem.GetByID(docID)
			if err != nil || len(content) < 300 {
				continue
			}

			// Compress via LLM
			resp, err := llm.ExecuteWithRetry(
				context.Background(),
				client,
				openai.ChatCompletionRequest{
					Model: cfg.LLM.Model,
					Messages: []openai.ChatCompletionMessage{
						{Role: openai.ChatMessageRoleSystem, Content: "You are an AI compressing old memories. Summarize the following RAG memory into a dense, concise bullet-point list containing only core facts. Lose the verbose narrative immediately."},
						{Role: openai.ChatMessageRoleUser, Content: content},
					},
					MaxTokens: 500,
				},
				logger,
				nil,
			)
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
		tc.Preview, highCount, mediumCount, lowCount, graphRemoved,
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
			var tmp ToolCall
			if json.Unmarshal([]byte(candidate), &tmp) == nil && tmp.Action != "" {
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

func ParseToolCall(content string) ToolCall {
	var tc ToolCall
	lowerContent := strings.ToLower(content)

	// Stepfun / OpenRouter <tool_call> fallback
	// Format 1: <function=name> ... </function>
	// Format 2: <tool_calls><invoke name="..."> ... </invoke></tool_calls>
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
	if (strings.Contains(lowerContent, "\"action\"") || strings.Contains(lowerContent, "'action'") || strings.Contains(lowerContent, "\"tool\"") || strings.Contains(lowerContent, "\"command\"") || strings.Contains(lowerContent, "\"operation\"") || (strings.Contains(lowerContent, "\"name\"") && strings.Contains(lowerContent, "\"arguments\""))) && (strings.Contains(lowerContent, "{") || strings.Contains(lowerContent, "```")) {
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
						var tmp ToolCall
						if json.Unmarshal([]byte(candidate), &tmp) == nil && (tmp.Action != "" || tmp.Operation != "" || tmp.Name != "" || tmp.Tool != "" || tmp.Command != "") {
							tc = tmp
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
						var tmp ToolCall
						if json.Unmarshal([]byte(candidate), &tmp) == nil && (tmp.Action != "" || tmp.Operation != "" || tmp.Name != "" || tmp.Tool != "" || tmp.Command != "") {
							tc = tmp
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
		if tc.Action != "" || tc.Operation != "" || tc.Name != "" || tc.Tool != "" || tc.Command != "" {
			tc.IsTool = true

			// AGGRESSIVE RECOVERY: Handle wrappers like {"action": "execute_tool", "tool": "name", "args": {...}}
			if (tc.Action == "execute_tool" || tc.Action == "run_tool" || tc.Action == "execute_tool_call") && tc.Tool != "" {
				tc.Action = tc.Tool
			}

			// Fallback: LLM used "tool" key instead of "action"
			if tc.Action == "" && tc.Tool != "" {
				tc.Action = tc.Tool
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
			return tc
		}
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

	if strings.HasPrefix(lowerContent, "import ") ||
		strings.HasPrefix(lowerContent, "def ") ||
		strings.HasPrefix(lowerContent, "print(") ||
		strings.HasPrefix(lowerContent, "# ") ||
		strings.Contains(lowerContent, "```python") {
		return ToolCall{RawCodeDetected: true}
	}

	return ToolCall{}
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
	return s[:n] + "..."
}

// truncateToolOutput trims a tool result that exceeds limit characters.
// It keeps the first portion of the output and appends a clear notice so the
// LLM knows the result was cut. limit=0 means no truncation.
func truncateToolOutput(result string, limit int) string {
	if limit <= 0 || len(result) <= limit {
		return result
	}
	notice := fmt.Sprintf("\n\n[Tool output truncated: %d of %d characters shown. Use a more specific command to get less output.]", limit, len(result))
	return result[:limit] + notice
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
func handleWebhookToolCall(tc ToolCall, mgr *webhooks.Manager, logger *slog.Logger) string {
	switch tc.Operation {
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
		if tc.ID == "" {
			return `Tool Output: {"status":"error","message":"id is required"}`
		}
		w, err := mgr.Get(tc.ID)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err)
		}
		data, _ := json.Marshal(map[string]any{"status": "ok", "webhook": w})
		return "Tool Output: " + string(data)

	case "create":
		w := webhooks.Webhook{
			Name:    tc.Name,
			Slug:    tc.Slug,
			Enabled: true,
		}
		if tc.TokenID != "" {
			w.TokenID = tc.TokenID
		}
		created, err := mgr.Create(w)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err)
		}
		logger.Info("Webhook created via tool", "id", created.ID, "slug", created.Slug)
		data, _ := json.Marshal(map[string]any{"status": "ok", "webhook_id": created.ID, "slug": created.Slug, "url": "/webhook/" + created.Slug})
		return "Tool Output: " + string(data)

	case "update":
		if tc.ID == "" {
			return `Tool Output: {"status":"error","message":"id is required"}`
		}
		patch := webhooks.Webhook{Name: tc.Name, Slug: tc.Slug, Enabled: tc.Enabled}
		if tc.TokenID != "" {
			patch.TokenID = tc.TokenID
		}
		updated, err := mgr.Update(tc.ID, patch)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err)
		}
		logger.Info("Webhook updated via tool", "id", updated.ID)
		data, _ := json.Marshal(map[string]any{"status": "ok", "webhook_id": updated.ID})
		return "Tool Output: " + string(data)

	case "delete":
		if tc.ID == "" {
			return `Tool Output: {"status":"error","message":"id is required"}`
		}
		err := mgr.Delete(tc.ID)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err)
		}
		logger.Info("Webhook deleted via tool", "id", tc.ID)
		return `Tool Output: {"status":"ok","message":"webhook deleted"}`

	case "logs":
		whLog := mgr.GetLog()
		n := 20
		var entries []webhooks.LogEntry
		if tc.ID != "" {
			entries = whLog.ForWebhook(tc.ID, n)
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
// basierend auf Personality Traits, Homepage-Multiplier und explizitem Override.
// Wenn tc leer ist (ToolCall{}), werden nur die Basis-Anpassungen berechnet (Personality).
// Tool-spezifische Anpassungen erfolgen später wenn tc bekannt ist.
func calculateEffectiveMaxCalls(cfg *config.Config, tc ToolCall, personalityEnabled bool, shortTermMem *memory.SQLiteMemory, logger *slog.Logger) int {
	effectiveMaxCalls := cfg.CircuitBreaker.MaxToolCalls

	// 1. Personality Engine V2: Thoroughness Trait
	if personalityEnabled && cfg.Agent.PersonalityEngineV2 && shortTermMem != nil {
		if traits, err := shortTermMem.GetTraits(); err == nil {
			if thoroughness, ok := traits[memory.TraitThoroughness]; ok && thoroughness > 0.8 {
				effectiveMaxCalls = int(float64(effectiveMaxCalls) * 1.5)
				logger.Debug("[Behavioral Tool Calling] Increased MaxToolCalls due to high Thoroughness", "new_max", effectiveMaxCalls)
			}
		}
	}

	// 2. Homepage Tool: Multiplier für komplexe Web-Workflows
	// Nur anwenden wenn tc bekannt ist (nicht leer)
	if tc.Tool != "" && tc.Tool == "homepage" && cfg.Homepage.Enabled {
		multiplier := cfg.Homepage.CircuitBreakerMultiplier
		if multiplier > 0 {
			// Cap bei 5x
			if multiplier > 5.0 {
				multiplier = 5.0
			}
			newLimit := int(float64(effectiveMaxCalls) * multiplier)
			logger.Debug("[Circuit Breaker] Homepage multiplier applied", "base_limit", effectiveMaxCalls, "multiplier", multiplier, "new_limit", newLimit)
			effectiveMaxCalls = newLimit
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
