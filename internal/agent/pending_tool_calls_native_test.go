package agent

import (
	"context"
	"log/slog"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

func TestProcessPendingToolCallsNativeAppendsOnlyToolResultToRequest(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(testDiscardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	historyManager := memory.NewEphemeralHistoryManager()
	defer historyManager.Close()

	nativeAssistant := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleAssistant,
		ToolCalls: []openai.ToolCall{
			{ID: "call_first", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "docker", Arguments: `{}`}},
			{ID: "call_second", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "shell", Arguments: `{}`}},
		},
	}
	initialMessages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "run both tools"},
		nativeAssistant,
		{Role: openai.ChatMessageRoleTool, Content: `{"status":"ok"}`, ToolCallID: "call_first"},
	}

	ptc := ToolCall{Action: "shell", NativeCallID: "call_second"}
	s := &agentLoopState{
		ctx:               context.Background(),
		broker:            NoopBroker{},
		currentLogger:     logger,
		recoveryState:     newToolRecoveryStateWithPolicy(buildRecoveryPolicy(cfg)),
		sessionUsedTools:  make(map[string]bool),
		runCfg: RunConfig{
			Config:         cfg,
			SessionID:      "default",
			ShortTermMem:   stm,
			HistoryManager: historyManager,
		},
		req: openai.ChatCompletionRequest{Messages: append([]openai.ChatCompletionMessage(nil), initialMessages...)},
		pendingTCs: []ToolCall{ptc},
		pendingSummaryBatch: map[string]string{
			pendingSummaryBatchKey(ptc): `{"status":"shell ok"}`,
		},
	}

	beforeLen := len(s.req.Messages)
	if !processPendingToolCalls(s, context.Background(), "run both tools") {
		t.Fatal("expected pending native tool call to be processed")
	}
	if len(s.req.Messages) != beforeLen+1 {
		t.Fatalf("req.Messages len = %d, want %d (only tool result appended)", len(s.req.Messages), beforeLen+1)
	}
	last := s.req.Messages[len(s.req.Messages)-1]
	if last.Role != openai.ChatMessageRoleTool {
		t.Fatalf("last role = %q, want tool", last.Role)
	}
	if last.ToolCallID != "call_second" {
		t.Fatalf("last ToolCallID = %q, want call_second", last.ToolCallID)
	}
	if len(last.ToolCalls) != 0 {
		t.Fatalf("expected no extra assistant tool_calls in appended message, got %d", len(last.ToolCalls))
	}
}