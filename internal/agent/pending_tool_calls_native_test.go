package agent

import (
	"context"
	"log/slog"
	"strings"
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
		ctx:              context.Background(),
		broker:           NoopBroker{},
		currentLogger:    logger,
		recoveryState:    newToolRecoveryStateWithPolicy(buildRecoveryPolicy(cfg)),
		sessionUsedTools: make(map[string]bool),
		runCfg: RunConfig{
			Config:         cfg,
			SessionID:      "default",
			ShortTermMem:   stm,
			HistoryManager: historyManager,
		},
		req:        openai.ChatCompletionRequest{Messages: append([]openai.ChatCompletionMessage(nil), initialMessages...)},
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
	history := historyManager.Get()
	if len(history) != 1 || history[0].Role != openai.ChatMessageRoleTool || history[0].ToolCallID != "call_second" {
		t.Fatalf("history = %#v, want only the queued native tool result", history)
	}
}

func TestDetachNewSystemMessagesKeepsNativeToolResultsAdjacent(t *testing.T) {
	req := openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{ID: "call_one"}, {ID: "call_two"}}},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call_one", Content: `{"status":"error"}`},
	}}
	start := len(req.Messages)
	req.Messages = append(req.Messages,
		openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: "RECOVERY HINT: retry correctly"},
		openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, ToolCallID: "call_two", Content: `{"status":"error"}`},
	)

	deferred := detachNewSystemMessages(&req, start)
	if len(deferred) != 1 || deferred[0].Role != openai.ChatMessageRoleSystem {
		t.Fatalf("deferred = %#v, want one system message", deferred)
	}
	if len(req.Messages) != 3 || req.Messages[2].Role != openai.ChatMessageRoleTool || req.Messages[2].ToolCallID != "call_two" {
		t.Fatalf("messages after detach = %#v, want adjacent second tool result", req.Messages)
	}
	req.Messages = append(req.Messages, deferred...)
	if _, dropped := SanitizeToolMessages(req.Messages); dropped != 0 {
		t.Fatalf("SanitizeToolMessages dropped %d messages from repaired native batch", dropped)
	}
}

func TestProcessPendingNativeToolCallDefersRecoveryHintUntilAfterResult(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(testDiscardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()
	historyManager := memory.NewEphemeralHistoryManager()
	defer historyManager.Close()

	second := ToolCall{Action: "virtual_desktop_app_install", NativeCallID: "call_second", Params: map[string]interface{}{
		"manifest": map[string]interface{}{"id": "demo", "name": "Demo", "entry": "index.html"},
	}}
	errorResult := `Tool Output: {"status":"error","message":"files are required"}`
	recoveryState := newToolRecoveryStateWithPolicy(buildRecoveryPolicy(cfg))
	firstFailure := second
	firstFailure.NativeCallID = "prior_failure"
	_ = recoveryState.updateToolErrorState(firstFailure, errorResult, &openai.ChatCompletionRequest{}, logger, AgentTelemetryScope{}, "v1", 1)

	nativeAssistant := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{
		{ID: "call_first", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "virtual_desktop_files", Arguments: `{}`}},
		{ID: "call_second", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "virtual_desktop_app_install", Arguments: `{}`}},
	}}
	s := &agentLoopState{
		ctx:              context.Background(),
		broker:           NoopBroker{},
		currentLogger:    logger,
		recoveryState:    recoveryState,
		sessionUsedTools: make(map[string]bool),
		runCfg:           RunConfig{Config: cfg, SessionID: "default", ShortTermMem: stm, HistoryManager: historyManager},
		req: openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "install app"},
			nativeAssistant,
			{Role: openai.ChatMessageRoleTool, Content: `{"status":"ok"}`, ToolCallID: "call_first"},
		}},
		pendingTCs: []ToolCall{second},
		pendingSummaryBatch: map[string]string{
			pendingSummaryBatchKey(second): errorResult,
		},
	}

	if !processPendingToolCalls(s, context.Background(), "install app") {
		t.Fatal("expected pending native tool call to be processed")
	}
	if len(s.req.Messages) != 5 {
		t.Fatalf("messages = %d, want user, assistant, two results, recovery hint", len(s.req.Messages))
	}
	if s.req.Messages[3].Role != openai.ChatMessageRoleTool || s.req.Messages[3].ToolCallID != "call_second" {
		t.Fatalf("fourth message = %#v, want second tool result", s.req.Messages[3])
	}
	if s.req.Messages[4].Role != openai.ChatMessageRoleSystem || !strings.Contains(s.req.Messages[4].Content, "RECOVERY HINT") {
		t.Fatalf("fifth message = %#v, want deferred recovery hint", s.req.Messages[4])
	}
	if _, dropped := SanitizeToolMessages(s.req.Messages); dropped != 0 {
		t.Fatalf("SanitizeToolMessages dropped %d messages from queued native recovery", dropped)
	}
}
