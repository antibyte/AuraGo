package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

const toolOutcomeParityTodo = "- [ ] verify tool outcome parity"

func newToolOutcomeTestState(t *testing.T, handle func(context.Context, ToolCall) (string, bool)) *agentLoopState {
	t.Helper()
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	history := memory.NewEphemeralHistoryManager()
	t.Cleanup(history.Close)
	return &agentLoopState{
		ctx:              context.Background(),
		broker:           NoopBroker{},
		currentLogger:    logger,
		recoveryState:    newToolRecoveryStateWithPolicy(buildRecoveryPolicy(cfg)),
		sessionUsedTools: make(map[string]bool),
		runCfg: RunConfig{
			Config:         cfg,
			SessionID:      "default",
			ShortTermMem:   stm,
			HistoryManager: history,
			ExecutionHooks: &ExecutionHooks{HandleTool: handle},
		},
	}
}

func flaggedCoreMemoryCall(callID string) ToolCall {
	return ToolCall{
		Action:             "core_memory",
		NativeCallID:       callID,
		Todo:               StringOrArray(toolOutcomeParityTodo),
		NotifyOnCompletion: true,
	}
}

func nativeAssistantForCalls(calls ...ToolCall) openai.ChatCompletionMessage {
	msg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant}
	for _, call := range calls {
		msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
			ID:       call.NativeCallID,
			Type:     openai.ToolTypeFunction,
			Function: openai.FunctionCall{Name: call.Action, Arguments: `{}`},
		})
	}
	return msg
}

func toolResultForCall(t *testing.T, messages []openai.ChatCompletionMessage, callID string) openai.ChatCompletionMessage {
	t.Helper()
	for _, msg := range messages {
		if msg.Role == openai.ChatMessageRoleTool && msg.ToolCallID == callID {
			return msg
		}
	}
	t.Fatalf("no tool result for %s in %#v", callID, messages)
	return openai.ChatCompletionMessage{}
}

func TestToolOutcomeParityAcrossDispatchPaths(t *testing.T) {
	succeed := func(context.Context, ToolCall) (string, bool) { return `{"status":"success","note":"done"}`, true }
	userTurn := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "check parity"}
	paths := []struct {
		name string
		run  func(t *testing.T, s *agentLoopState)
	}{
		{name: "queued", run: func(t *testing.T, s *agentLoopState) {
			call := flaggedCoreMemoryCall("call_flagged")
			s.req.Messages = []openai.ChatCompletionMessage{userTurn, nativeAssistantForCalls(call)}
			s.pendingTCs = []ToolCall{call}
			if !processPendingToolCalls(s, context.Background(), userTurn.Content) {
				t.Fatal("queued tool call was not processed")
			}
		}},
		{name: "primary", run: func(t *testing.T, s *agentLoopState) {
			call := flaggedCoreMemoryCall("call_flagged")
			s.req.Messages = []openai.ChatCompletionMessage{userTurn}
			if _, err, cont := executeAgentToolTurn(s, context.Background(), call, openai.ChatCompletionResponse{}, "", true, nativeAssistantForCalls(call), userTurn.Content, "", false); err != nil || !cont {
				t.Fatalf("primary tool turn: err=%v continue=%v", err, cont)
			}
		}},
		{name: "batched", run: func(t *testing.T, s *agentLoopState) {
			primary := ToolCall{Action: "list_tools", NativeCallID: "call_primary"}
			call := flaggedCoreMemoryCall("call_flagged")
			s.req.Messages = []openai.ChatCompletionMessage{userTurn}
			s.pendingTCs = []ToolCall{call}
			if _, err, cont := executeAgentToolTurn(s, context.Background(), primary, openai.ChatCompletionResponse{}, "", true, nativeAssistantForCalls(primary, call), userTurn.Content, "", false); err != nil || !cont {
				t.Fatalf("batched tool turn: err=%v continue=%v", err, cont)
			}
		}},
	}
	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			s := newToolOutcomeTestState(t, succeed)
			path.run(t, s)
			result := toolResultForCall(t, s.req.Messages, "call_flagged")
			if !strings.HasPrefix(result.Content, "[TOOL COMPLETION NOTIFICATION]") {
				t.Fatalf("notify_on_completion was ignored: %q", result.Content)
			}
			if !s.coreMemDirty {
				t.Fatal("core_memory did not invalidate the cached core memory")
			}
			if s.lastTool != "core_memory" {
				t.Fatalf("lastTool = %q, want core_memory (tool transition not recorded)", s.lastTool)
			}
			if s.sessionTodoList != toolOutcomeParityTodo {
				t.Fatalf("sessionTodoList = %q, want the piggybacked todo", s.sessionTodoList)
			}
		})
	}
}

func TestPrimaryNativeRecoveryGuidanceFollowsToolResults(t *testing.T) {
	const failure = `{"status":"error","message":"remote endpoint refused the request"}`
	s := newToolOutcomeTestState(t, func(context.Context, ToolCall) (string, bool) { return failure, true })
	s.req.Messages = []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "call the endpoint"}}

	var turnStart int
	for i, callID := range []string{"call_failing_1", "call_failing_2"} {
		turnStart = len(s.req.Messages)
		call := ToolCall{Action: "api_request", NativeCallID: callID}
		if _, err, cont := executeAgentToolTurn(s, context.Background(), call, openai.ChatCompletionResponse{}, "", true, nativeAssistantForCalls(call), "call the endpoint", "", false); err != nil || !cont {
			t.Fatalf("turn %d: err=%v continue=%v", i+1, err, cont)
		}
	}

	turn := s.req.Messages[turnStart:]
	if len(turn) < 3 {
		t.Fatalf("second turn messages = %#v, want assistant, tool result and recovery guidance", turn)
	}
	if turn[0].Role != openai.ChatMessageRoleAssistant || len(turn[0].ToolCalls) != 1 {
		t.Fatalf("second turn must start with the assistant tool call, got %#v", turn[0])
	}
	if turn[1].Role != openai.ChatMessageRoleTool || turn[1].ToolCallID != "call_failing_2" {
		t.Fatalf("tool result must directly follow its assistant tool call, got %#v", turn[1])
	}
	for _, msg := range turn[2:] {
		if msg.Role != openai.ChatMessageRoleSystem {
			t.Fatalf("only recovery guidance may follow the tool result, got %#v", msg)
		}
	}
}
