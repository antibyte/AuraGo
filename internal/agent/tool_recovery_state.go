package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sashabaranov/go-openai"
)

type toolRecoveryState struct {
	Policy                RecoveryPolicy
	LastToolError         string
	ConsecutiveErrorCount int
	LastToolCallSig       string
	DuplicateToolCount    int
	ToolCallFrequency     map[string]int
}

func newToolRecoveryState() toolRecoveryState {
	return newToolRecoveryStateWithPolicy(defaultRecoveryPolicy())
}

func newToolRecoveryStateWithPolicy(policy RecoveryPolicy) toolRecoveryState {
	return toolRecoveryState{
		Policy:            policy,
		ToolCallFrequency: make(map[string]int),
	}
}

func buildToolSignature(tc ToolCall) string {
	path := tc.Path
	if path == "" {
		path = tc.FilePath
	}
	dest := tc.Dest
	if dest == "" {
		dest = tc.Destination
	}

	signature := map[string]interface{}{
		"action":            tc.Action,
		"sub_operation":     tc.SubOperation,
		"operation":         tc.Operation,
		"command":           tc.Command,
		"code":              tc.Code,
		"path":              path,
		"destination":       dest,
		"pattern":           tc.Pattern,
		"glob":              tc.Glob,
		"query":             tc.Query,
		"sampling_strategy": tc.SamplingStrategy,
		"max_tokens":        tc.MaxTokens,
		"start_line":        tc.StartLine,
		"end_line":          tc.EndLine,
		"line_count":        tc.LineCount,
		"old":               tc.Old,
		"new":               tc.New,
		"marker":            tc.Marker,
		"content":           tc.Content,
		"url":               tc.URL,
		"method":            tc.Method,
		"params":            tc.Params,
		"headers":           tc.Headers,
		"skill":             tc.Skill,
		"skill_args":        tc.SkillArgs,
	}
	raw, err := json.Marshal(signature)
	if err != nil {
		return tc.Action + "|" + tc.Command + "|" + tc.Code + "|" + tc.Operation + "|" + path
	}
	return string(raw)
}

func recoveryHintForToolFailure(tc ToolCall, resultContent string) string {
	base := "Do not repeat the exact same tool call. Inspect the last error, read the tool manual if needed, verify the relevant files or inputs, and then choose a genuinely different approach."
	errText := extractErrorMessage(resultContent)
	if errText == "" {
		errText = resultContent
	}
	lower := strings.ToLower(errText)

	switch {
	case tc.Action == "homepage" && strings.Contains(lower, `missing script: "build"`):
		return "The project has no build script. Treat it as a static site or fix package.json before trying again. Check whether project_dir is correct and whether a dist/build/output directory is even needed."
	case tc.Action == "homepage" && strings.Contains(lower, "absolute paths not allowed"):
		return "Use a relative homepage workspace path such as 'ki-news', not '/workspace/ki-news'. Do not retry until project_dir/path arguments are relative."
	case tc.Action == "homepage" && strings.Contains(lower, "deploy path does not exist"):
		return "Check the homepage workspace first with homepage list_files/read_file. If files were created via the filesystem tool, recreate them with homepage write_file in the correct project directory."
	case tc.Action == "filesystem" && strings.Contains(lower, "unknown filesystem operation"):
		return "Use the exact filesystem operations read_file or write_file, not read or write. Correct the operation name before retrying."
	default:
		return base
	}
}

func isGenericToolSignature(tc ToolCall, toolSig string) bool {
	return tc.Action != "" &&
		tc.SubOperation == "" &&
		tc.Command == "" &&
		tc.Code == "" &&
		tc.Operation == "" &&
		tc.Path == "" &&
		tc.FilePath == "" &&
		tc.Destination == "" &&
		tc.Dest == "" &&
		tc.Pattern == "" &&
		tc.Glob == "" &&
		tc.Query == "" &&
		tc.SamplingStrategy == "" &&
		tc.MaxTokens == 0 &&
		tc.StartLine == 0 &&
		tc.EndLine == 0 &&
		tc.LineCount == 0 &&
		tc.Old == "" &&
		tc.New == "" &&
		tc.Marker == "" &&
		tc.Content == "" &&
		tc.URL == "" &&
		tc.Method == "" &&
		len(tc.Params) == 0 &&
		len(tc.Headers) == 0 &&
		tc.Skill == "" &&
		len(tc.SkillArgs) == 0
}

func (s *toolRecoveryState) handleDuplicateToolCall(tc ToolCall, req *openai.ChatCompletionRequest, logger *slog.Logger, scope AgentTelemetryScope) bool {
	toolSig := buildToolSignature(tc)
	if toolSig == s.LastToolCallSig && !isGenericToolSignature(tc, toolSig) {
		s.DuplicateToolCount++
	} else {
		s.DuplicateToolCount = 0
		s.LastToolCallSig = toolSig
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
		delete(s.ToolCallFrequency, toolSig)
		return true
	}

	return false
}

func (s *toolRecoveryState) shouldRecordFirstErrorInChain() bool {
	return s.ConsecutiveErrorCount == 0
}

func (s *toolRecoveryState) shouldRecordResolution() bool {
	return s.ConsecutiveErrorCount > 0 && s.LastToolError != ""
}

func (s *toolRecoveryState) updateToolErrorState(tc ToolCall, resultContent string, req *openai.ChatCompletionRequest, logger *slog.Logger, scope AgentTelemetryScope) bool {
	hasSandboxFailure := containsSandboxFailure(resultContent)
	isToolError := containsToolError(resultContent) || hasSandboxFailure
	if isToolError {
		if resultContent == s.LastToolError {
			s.ConsecutiveErrorCount++
			if s.ConsecutiveErrorCount == 2 && req != nil {
				req.Messages = append(req.Messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleSystem,
					Content: "RECOVERY HINT: " + recoveryHintForToolFailure(tc, resultContent),
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
