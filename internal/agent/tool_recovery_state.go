package agent

import (
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
	return tc.Action + "|" + tc.Command + "|" + tc.Code + "|" + tc.Operation + "|" + tc.Path
}

func isGenericToolSignature(tc ToolCall, toolSig string) bool {
	return toolSig == tc.Action+"|||||"
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
				"inform the user about the situation and ask what they want to do next.",
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
						"(e.g. wrong URL, missing credentials, service unavailable), and wait for their input.",
					tc.Action, s.ConsecutiveErrorCount, tc.Action)
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
