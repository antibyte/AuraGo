package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/sashabaranov/go-openai"
)

var ErrToolOutcomeUnknown = errors.New("tool action outcome is unknown; verify its state before continuing")
var ErrActionRuleRequired = errors.New("action_rule_required: administrator policy must authorize this unattended action")

// noteToolRunBoundary distinguishes an explicit uncertain mutation from legacy
// unclassified prose. Neither approval nor uncertainty may trigger model replay.
func (s *agentLoopState) noteToolRunBoundary(tc ToolCall, dc *DispatchContext, output string) {
	payload, _ := vaultReadablePayload(output)
	envelope, ok := firstOutcomeEnvelope(payload)
	if !ok {
		return
	}
	if envelope.Code == "approval_required" {
		s.toolBoundaryNotice = "This action is waiting for administrator approval."
		return
	}
	if envelope.Code == "action_rule_required" {
		s.toolBoundaryErr = ErrActionRuleRequired
		return
	}
	if envelope.Code == "tool_panic" {
		s.toolBoundaryErr = ErrToolOutcomeUnknown
		return
	}
	if DescribeAction(tc, dc).Mutating {
		code := strings.ToLower(strings.TrimSpace(envelope.Code))
		if code == "outcome_unknown" || strings.HasSuffix(code, "_outcome_unknown") {
			s.toolBoundaryErr = ErrToolOutcomeUnknown
			return
		}
		switch strings.ToLower(strings.TrimSpace(envelope.Status)) {
		case "unknown", "outcome_unknown", "unclassified":
			s.toolBoundaryErr = ErrToolOutcomeUnknown
		}
	}
}

func (s *agentLoopState) toolRunBoundary(ctx context.Context) (openai.ChatCompletionResponse, error, bool) {
	if s.toolBoundaryErr == nil && s.toolBoundaryNotice == "" {
		return openai.ChatCompletionResponse{}, nil, false
	}
	appendSkippedNativeResults(s, s.runCfg.ShortTermMem, s.runCfg.HistoryManager, s.runCfg.SessionID, s.broker,
		`{"status":"skipped","code":"tool_batch_stopped","message":"Remaining calls were not executed while an action awaits approval or outcome verification."}`)
	s.pendingTCs = nil
	if err := ctx.Err(); err != nil {
		return openai.ChatCompletionResponse{}, err, true
	}
	if s.toolBoundaryErr != nil {
		return openai.ChatCompletionResponse{}, s.toolBoundaryErr, true
	}
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonStop,
		Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: s.toolBoundaryNotice}}}}, nil, true
}
