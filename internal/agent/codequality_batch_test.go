package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

func TestNativeBatchStopsBetweenCallsAndClosesEveryResult(t *testing.T) {
	for _, reason := range []string{"cancel", "tokens", "interrupt"} {
		t.Run(reason, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			var s *agentLoopState
			s = newToolOutcomeTestState(t, func(context.Context, ToolCall) (string, bool) {
				calls++
				switch reason {
				case "cancel":
					cancel()
				case "tokens":
					s.sessionTokens = 10
				case "interrupt":
					InterruptSession(s.runCfg.SessionID)
				}
				return `{"status":"success"}`, true
			})
			s.runCfg.SessionID = "batch-" + reason
			s.runCfg.IsCoAgent = true
			s.runCfg.CoAgentTokenLimit = 10
			s.loopStartedAt = time.Now()
			primary := ToolCall{Action: "core_memory", NativeCallID: "first"}
			s.pendingTCs = []ToolCall{{Action: "core_memory", NativeCallID: "second"}, {Action: "core_memory", NativeCallID: "third"}}
			msg := nativeAssistantForCalls(primary, s.pendingTCs[0], s.pendingTCs[1])
			executeAgentToolTurn(s, ctx, primary, openai.ChatCompletionResponse{}, "", true, msg, "test", "", false)
			if calls != 1 {
				t.Fatalf("dispatches=%d, want 1", calls)
			}
			for _, id := range []string{"second", "third"} {
				if !strings.Contains(toolResultForCall(t, s.req.Messages, id).Content, "tool_batch_stopped") {
					t.Fatalf("missing skipped result for %s", id)
				}
			}
			checkAndClearInterrupt(s.runCfg.SessionID)
		})
	}
}

func TestNativeBudgetExhaustedBeforePrimaryDoesNotDispatch(t *testing.T) {
	calls := 0
	s := newToolOutcomeTestState(t, func(context.Context, ToolCall) (string, bool) { calls++; return "", true })
	s.runCfg.IsCoAgent = true
	s.runCfg.CoAgentTokenLimit = 10
	s.sessionTokens = 10
	primary := ToolCall{Action: "core_memory", NativeCallID: "first"}
	s.pendingTCs = []ToolCall{{Action: "core_memory", NativeCallID: "second"}}
	_, err, next := executeAgentToolTurn(s, context.Background(), primary, openai.ChatCompletionResponse{}, "", true, nativeAssistantForCalls(primary, s.pendingTCs[0]), "test", "", false)
	if calls != 0 || err == nil || next {
		t.Fatalf("calls=%d err=%v next=%v", calls, err, next)
	}
	for _, id := range []string{"first", "second"} {
		toolResultForCall(t, s.req.Messages, id)
	}
}
