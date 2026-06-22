package agent

import (
	"aurago/internal/services/optimizer"
	"fmt"
	"hash"
	"hash/fnv"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/sashabaranov/go-openai"
)

type toolRecoveryState struct {
	mu                    *sync.RWMutex // pointer avoids copylock vet warnings on struct return by value
	Policy                RecoveryPolicy
	LastToolError         string
	ConsecutiveErrorCount int
	TotalErrorCount       int // cumulative errors this session; never resets (unlike ConsecutiveErrorCount)
	LastToolCallSig       string
	DuplicateToolCount    int
	ToolCallFrequency     map[string]int
	RecoveryHintFrequency map[string]int
}

const maxTrackedToolCallSignatures = 512

func newToolRecoveryState() toolRecoveryState {
	return newToolRecoveryStateWithPolicy(defaultRecoveryPolicy())
}

func newToolRecoveryStateWithPolicy(policy RecoveryPolicy) toolRecoveryState {
	return toolRecoveryState{
		mu:                    &sync.RWMutex{},
		Policy:                policy,
		ToolCallFrequency:     make(map[string]int),
		RecoveryHintFrequency: make(map[string]int),
	}
}

func buildToolSignature(tc ToolCall) string {
	path := firstNonEmptyToolString(toolArgString(tc.Params, "path", "file_path"), tc.Path, tc.FilePath)
	dest := firstNonEmptyToolString(toolArgString(tc.Params, "dest", "destination"), tc.Dest, tc.Destination)

	h := fnv.New64a()
	writeToolSignatureField(h, "action", tc.Action)
	writeToolSignatureField(h, "sub_operation", tc.SubOperation)
	writeToolSignatureField(h, "operation", tc.Operation)
	writeToolSignatureField(h, "command", tc.Command)
	writeToolSignatureField(h, "code", tc.Code)
	writeToolSignatureField(h, "path", path)
	writeToolSignatureField(h, "destination", dest)
	writeToolSignatureField(h, "pattern", toolArgString(tc.Params, "pattern"))
	writeToolSignatureField(h, "glob", toolArgString(tc.Params, "glob"))
	writeToolSignatureField(h, "query", tc.Query)
	writeToolSignatureField(h, "sampling_strategy", toolArgString(tc.Params, "sampling_strategy"))
	writeToolSignatureIntField(h, "max_tokens", toolArgInt(tc.Params, 0, "max_tokens"))
	writeToolSignatureIntField(h, "start_line", toolArgInt(tc.Params, 0, "start_line"))
	writeToolSignatureIntField(h, "end_line", toolArgInt(tc.Params, 0, "end_line"))
	writeToolSignatureIntField(h, "line_count", toolArgInt(tc.Params, 0, "line_count"))
	writeToolSignatureField(h, "old", toolArgString(tc.Params, "old"))
	writeToolSignatureField(h, "new", toolArgString(tc.Params, "new"))
	writeToolSignatureField(h, "marker", toolArgString(tc.Params, "marker"))
	writeToolSignatureField(h, "content", tc.Content)
	writeToolSignatureField(h, "url", tc.URL)
	writeToolSignatureField(h, "method", tc.Method)
	writeToolSignatureInterfaceMap(h, "params", tc.Params)
	writeToolSignatureStringMap(h, "headers", tc.Headers)
	writeToolSignatureField(h, "skill", tc.Skill)
	writeToolSignatureInterfaceMap(h, "skill_args", tc.SkillArgs)
	writeToolSignatureItems(h, "items", tc.Items)
	return strconv.FormatUint(h.Sum64(), 16)
}

func writeToolSignatureField(h hash.Hash64, name, value string) {
	_, _ = h.Write([]byte(name))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(value))
	_, _ = h.Write([]byte{0})
}

func writeToolSignatureIntField(h hash.Hash64, name string, value int) {
	writeToolSignatureField(h, name, strconv.Itoa(value))
}

func writeToolSignatureStringMap(h hash.Hash64, name string, values map[string]string) {
	if len(values) == 0 {
		writeToolSignatureField(h, name, "")
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	writeToolSignatureField(h, name+"#len", strconv.Itoa(len(keys)))
	for _, key := range keys {
		writeToolSignatureField(h, name+"."+key, values[key])
	}
}

func writeToolSignatureInterfaceMap(h hash.Hash64, name string, values map[string]interface{}) {
	if len(values) == 0 {
		writeToolSignatureField(h, name, "")
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	writeToolSignatureField(h, name+"#len", strconv.Itoa(len(keys)))
	for _, key := range keys {
		writeToolSignatureValue(h, name+"."+key, values[key])
	}
}

func writeToolSignatureItems(h hash.Hash64, name string, items []map[string]interface{}) {
	writeToolSignatureField(h, name+"#len", strconv.Itoa(len(items)))
	for index, item := range items {
		writeToolSignatureInterfaceMap(h, name+"["+strconv.Itoa(index)+"]", item)
	}
}

func writeToolSignatureValue(h hash.Hash64, name string, value interface{}) {
	switch typed := value.(type) {
	case nil:
		writeToolSignatureField(h, name, "<nil>")
	case string:
		writeToolSignatureField(h, name, typed)
	case bool:
		writeToolSignatureField(h, name, strconv.FormatBool(typed))
	case int:
		writeToolSignatureField(h, name, strconv.Itoa(typed))
	case int64:
		writeToolSignatureField(h, name, strconv.FormatInt(typed, 10))
	case float64:
		writeToolSignatureField(h, name, strconv.FormatFloat(typed, 'g', -1, 64))
	case map[string]interface{}:
		writeToolSignatureInterfaceMap(h, name, typed)
	case map[string]string:
		writeToolSignatureStringMap(h, name, typed)
	case []map[string]interface{}:
		writeToolSignatureItems(h, name, typed)
	case []string:
		writeToolSignatureField(h, name+"#len", strconv.Itoa(len(typed)))
		for index, item := range typed {
			writeToolSignatureField(h, name+"["+strconv.Itoa(index)+"]", item)
		}
	case []interface{}:
		writeToolSignatureField(h, name+"#len", strconv.Itoa(len(typed)))
		for index, item := range typed {
			writeToolSignatureValue(h, name+"["+strconv.Itoa(index)+"]", item)
		}
	default:
		writeToolSignatureField(h, name, fmt.Sprintf("%T:%v", value, value))
	}
}

func recoveryHintForToolFailure(tc ToolCall, resultContent string) string {
	base := "Do not repeat the exact same tool call. Inspect the last error, read the tool manual if needed, verify the relevant files or inputs, and then choose a genuinely different approach."
	errText := extractErrorMessage(resultContent)
	if errText == "" {
		errText = resultContent
	}
	lower := strings.ToLower(errText)

	switch {
	case tc.Action == "read_tool_output" && strings.Contains(lower, "ref is required"):
		return "read_tool_output requires the output_ref/ref from a previous compact tool result. Do not call it without a ref; inspect the prior tool output for an output_ref first, or continue with the compact output if no ref was provided."
	case tc.Action == "homepage" && strings.Contains(lower, `missing script: "build"`):
		return "The project has no build script. Treat it as a static site or fix package.json before trying again. Check whether project_dir is correct and whether a dist/build/output directory is even needed."
	case tc.Action == "homepage" && strings.Contains(lower, "path is required"):
		return `Homepage file operations require a relative path inside the homepage workspace. Retry with path/file_path including the project directory, for example {"operation":"write_file","path":"my-site/index.html","content":"..."} or {"operation":"read_file","path":"my-site/src/main.ts"}. Do not use filesystem for homepage project files.`
	case tc.Action == "homepage" && strings.Contains(lower, "absolute paths not allowed"):
		return "Use a relative homepage workspace path such as 'ki-news', not '/workspace/ki-news'. Do not retry until project_dir/path arguments are relative."
	case tc.Action == "homepage" && strings.Contains(lower, "deploy path does not exist"):
		return "Check the homepage workspace first with homepage list_files/read_file. If files were created via the filesystem tool, recreate them with homepage write_file in the correct project directory."
	case tc.Action == "filesystem" && strings.Contains(lower, "unknown filesystem operation"):
		return "Use the exact filesystem operations read_file or write_file, not read or write. Correct the operation name before retrying."
	case tc.Action == "virtual_desktop" && strings.Contains(lower, "desktop app") && strings.Contains(lower, "not found"):
		return "The requested app is not installed or built-in. Use virtual_desktop list_apps to see available apps, then use a valid app_id. If you want to run a generated app, first install it with install_app or write_file to Apps/<app_id>.html."
	case tc.Action == "virtual_desktop" && strings.Contains(lower, "desktop widget") && strings.Contains(lower, "not found"):
		return "The requested widget is not registered. Use virtual_desktop list_widgets to see available widgets, or create a standalone widget by writing non-empty HTML to Widgets/<widget_id>.html or Widgets/<widget_id>/index.html."
	case tc.Action == "virtual_desktop" && strings.Contains(lower, "entry file is unavailable"):
		return "The app is registered but its entry file is missing or empty. Use virtual_desktop diagnose_app to inspect the issue, then reinstall or rewrite the entry file."
	case tc.Action == "virtual_desktop" && strings.Contains(lower, "path is required"):
		return "Virtual desktop file operations require a workspace-relative path such as 'Apps/my-app/index.html' or 'Widgets/weather.html'. Do not use absolute paths like '/workspace/...' or host filesystem paths."
	case tc.Action == "virtual_desktop" && strings.Contains(lower, "content is required"):
		return "Apps/ and Widgets/ paths require non-empty content. Provide complete HTML or script content, or use patch_file for targeted edits."
	case tc.Action == "virtual_desktop" && strings.Contains(lower, "desktop widget html file must not be empty"):
		return "Standalone widget HTML files must contain non-empty HTML. Rewrite the widget with valid HTML content."
	case tc.Action == "virtual_desktop" && (strings.Contains(lower, "desktop path escapes workspace") || strings.Contains(lower, "absolute") || strings.Contains(lower, "/workspace") || strings.Contains(lower, "host filesystem")):
		return "Virtual desktop file operations require a workspace-relative path such as 'Apps/my-app/index.html' or 'Widgets/weather.html'. Do not use absolute paths like '/workspace/...' or host filesystem paths. Use only the relative path inside the virtual desktop workspace."
	case tc.Action == "virtual_desktop" && (strings.Contains(lower, "icon must use") || strings.Contains(lower, "icon_catalog")):
		return "The specified desktop icon is not supported. Use virtual_desktop status to retrieve the icon_catalog, then choose a preferred or alias semantic name from the catalog. Do not use emoji icons or arbitrary icon strings."
	default:
		return base
	}
}

func recoveryHintKey(hint string) string {
	h := fnv.New64a()
	writeToolSignatureField(h, "hint", strings.TrimSpace(hint))
	return strconv.FormatUint(h.Sum64(), 16)
}

func (s *toolRecoveryState) shouldSendRecoveryHintLocked(hint string, maxHits int) bool {
	if s == nil || strings.TrimSpace(hint) == "" {
		return true
	}
	if maxHits <= 0 {
		maxHits = 2
	}
	key := recoveryHintKey(hint)
	if s.RecoveryHintFrequency == nil {
		s.RecoveryHintFrequency = make(map[string]int)
	}
	s.RecoveryHintFrequency[key]++
	return s.RecoveryHintFrequency[key] <= maxHits
}

func isGenericToolSignature(tc ToolCall, toolSig string) bool {
	pattern := toolArgString(tc.Params, "pattern")
	glob := toolArgString(tc.Params, "glob")
	sampling := toolArgString(tc.Params, "sampling_strategy")
	maxTokens := toolArgInt(tc.Params, 0, "max_tokens")
	startLine := toolArgInt(tc.Params, 0, "start_line")
	endLine := toolArgInt(tc.Params, 0, "end_line")
	lineCount := toolArgInt(tc.Params, 0, "line_count")
	old := toolArgString(tc.Params, "old")
	newValue := toolArgString(tc.Params, "new")
	marker := toolArgString(tc.Params, "marker")

	return tc.Action != "" &&
		tc.SubOperation == "" &&
		tc.Command == "" &&
		tc.Code == "" &&
		tc.Operation == "" &&
		tc.Path == "" &&
		tc.FilePath == "" &&
		tc.Destination == "" &&
		tc.Dest == "" &&
		pattern == "" &&
		glob == "" &&
		tc.Query == "" &&
		sampling == "" &&
		maxTokens == 0 &&
		startLine == 0 &&
		endLine == 0 &&
		lineCount == 0 &&
		old == "" &&
		newValue == "" &&
		marker == "" &&
		tc.Content == "" &&
		tc.URL == "" &&
		tc.Method == "" &&
		len(tc.Params) == 0 &&
		len(tc.Headers) == 0 &&
		tc.Skill == "" &&
		len(tc.SkillArgs) == 0 &&
		len(tc.Items) == 0
}

func isCoAgentMonitoringToolCall(tc ToolCall) bool {
	action := strings.ToLower(strings.TrimSpace(tc.Action))
	if action != "co_agent" && action != "co_agents" {
		return false
	}
	operation := strings.ToLower(strings.TrimSpace(firstNonEmptyToolString(
		tc.Operation,
		tc.SubOperation,
		toolArgString(tc.Params, "operation", "op"),
	)))
	switch operation {
	case "list", "status", "get_status", "get_result", "result":
		return true
	default:
		return false
	}
}

func (s *toolRecoveryState) handleDuplicateToolCall(tc ToolCall, req *openai.ChatCompletionRequest, logger *slog.Logger, scope AgentTelemetryScope) bool {
	if isCoAgentMonitoringToolCall(tc) {
		s.mu.Lock()
		s.DuplicateToolCount = 0
		s.LastToolCallSig = ""
		s.mu.Unlock()
		return false
	}

	toolSig := buildToolSignature(tc)
	s.mu.Lock()
	defer s.mu.Unlock()
	if toolSig == s.LastToolCallSig && !isGenericToolSignature(tc, toolSig) {
		s.DuplicateToolCount++
	} else {
		s.DuplicateToolCount = 0
		s.LastToolCallSig = toolSig
	}
	if _, exists := s.ToolCallFrequency[toolSig]; !exists && len(s.ToolCallFrequency) >= maxTrackedToolCallSignatures {
		s.ToolCallFrequency = make(map[string]int, maxTrackedToolCallSignatures)
	}
	s.ToolCallFrequency[toolSig]++
	freqCount := s.ToolCallFrequency[toolSig]

	if (s.DuplicateToolCount >= s.Policy.duplicateConsecutiveHits() || freqCount >= s.Policy.duplicateFrequencyHits()) && !isGenericToolSignature(tc, toolSig) {
		RecordToolRecoveryEventForScope(scope, "duplicate_tool_call_blocked")
		if logger != nil {
			logger.Warn("[Sync] Duplicate tool call detected — circuit breaker triggered",
				"action", tc.Action, "consecutive", s.DuplicateToolCount, "total", freqCount)
		}
		abortMsg := fmt.Sprintf(
			"CIRCUIT BREAKER: You are calling '%s' with the exact same parameters for the %d. time. "+
				"Repeating it will produce the same result. Do NOT call it again. "+
				"Either try a completely DIFFERENT approach (different command, different tool) or "+
				"inform the user about the situation and ask what they want to do next. "+
				"Before any alternative retry, inspect the previous error, read the relevant tool manual, and verify the target files/paths.",
			tc.Action, freqCount)
		req.Messages = append(req.Messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: abortMsg,
		})
		s.DuplicateToolCount = 0
		s.LastToolCallSig = ""
		// Do NOT delete ToolCallFrequency[toolSig] here.
		// Keeping the frequency count ensures every subsequent call to the same
		// signature immediately re-triggers the circuit breaker (freqCount stays
		// above the threshold), preventing the LLM from looping indefinitely by
		// cycling through groups of N calls between resets.
		return true
	}

	return false
}

func (s *toolRecoveryState) shouldRecordFirstErrorInChain() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ConsecutiveErrorCount == 0
}

func (s *toolRecoveryState) shouldRecordResolution() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ConsecutiveErrorCount > 0 && s.LastToolError != ""
}

func (s *toolRecoveryState) updateToolErrorState(tc ToolCall, resultContent string, req *openai.ChatCompletionRequest, logger *slog.Logger, scope AgentTelemetryScope, promptVersion string, execTimeMs int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	hasSandboxFailure := containsSandboxFailure(resultContent)
	isToolError := containsToolError(resultContent) || hasSandboxFailure

	// Async trace logging for optimization
	consecutiveCount := s.ConsecutiveErrorCount
	go func() {
		errMsg := ""
		if isToolError {
			errMsg = extractErrorMessage(resultContent)
			if len(errMsg) > 200 {
				errMsg = errMsg[:200]
			}
		}

		if promptVersion == "" {
			promptVersion = "v1"
		}

		// In the context of the recovery state, we might not always have exec time, passing 0 for now.
		optimizer.LogToolTrace(tc.Action, !isToolError, consecutiveCount, promptVersion, errMsg, execTimeMs)
	}()

	if isToolError {
		s.TotalErrorCount++
		if resultContent == s.LastToolError {
			s.ConsecutiveErrorCount++
			hint := recoveryHintForToolFailure(tc, resultContent)
			if s.ConsecutiveErrorCount == 2 && req != nil && s.shouldSendRecoveryHintLocked(hint, 2) {
				req.Messages = append(req.Messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleSystem,
					Content: "RECOVERY HINT: " + hint,
				})
			}
			if s.ConsecutiveErrorCount >= s.Policy.identicalToolErrorHits() {
				RecordToolRecoveryEventForScope(scope, "identical_tool_error_blocked")
				if logger != nil {
					logger.Warn("[Sync] Consecutive identical error — circuit breaker triggered",
						"action", tc.Action, "count", s.ConsecutiveErrorCount)
				}
				abortMsg := fmt.Sprintf(
					"CIRCUIT BREAKER: The tool '%s' returned the same error %d times in a row. "+
						"You MUST stop retrying it — calling it again will produce the exact same result. "+
						"Do NOT call '%s' again this session. "+
						"Instead: inform the user about the error, explain what likely needs to be fixed "+
						"(e.g. wrong URL, missing credentials, wrong relative path, missing build script, service unavailable), and wait for their input. "+
						"Recovery guidance: %s",
					tc.Action, s.ConsecutiveErrorCount, tc.Action, recoveryHintForToolFailure(tc, resultContent))
				req.Messages = append(req.Messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleSystem,
					Content: abortMsg,
				})
				s.ConsecutiveErrorCount = 0
				s.LastToolError = ""
				return true
			}
		} else {
			s.ConsecutiveErrorCount = 1
		}
		s.LastToolError = resultContent
		return false
	}

	s.ConsecutiveErrorCount = 0
	s.LastToolError = ""
	return false
}

func containsToolError(resultContent string) bool {
	return containsAny(resultContent,
		`"status": "error"`,
		`"status":"error"`,
		`[EXECUTION ERROR]`,
	)
}

func containsSandboxFailure(resultContent string) bool {
	hasNonZeroExitCode := strings.Contains(resultContent, `"exit_code":`) &&
		!strings.Contains(resultContent, `"exit_code": 0`) &&
		!strings.Contains(resultContent, `"exit_code":0`)
	return hasNonZeroExitCode || containsAny(resultContent, `"sandbox_error":true`, `"sandbox_failure":true`)
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
