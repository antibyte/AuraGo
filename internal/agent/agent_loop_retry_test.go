package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestInvalidNativeToolRecoveryDropsQueuedCallsFromSameResponse(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(testDiscardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	s := &agentLoopState{
		ctx:             context.Background(),
		broker:          NoopBroker{},
		currentLogger:   logger,
		recoverySession: NewRecoverySessionState(logger, NoopBroker{}, cfg),
		runCfg: RunConfig{
			Config:       cfg,
			SessionID:    "retry-test",
			ShortTermMem: stm,
		},
		pendingTCs: []ToolCall{
			{Action: "tts", NativeCallID: "call_tts"},
		},
		pendingSummaryBatch: map[string]string{
			pendingSummaryBatchKey(ToolCall{Action: "tts", NativeCallID: "call_tts"}): "precomputed audio",
		},
	}

	tc := ToolCall{
		Action:              "docker",
		IsTool:              true,
		NativeArgsMalformed: true,
		NativeArgsError:     "json: cannot unmarshal string into Go struct field toolCallAlias.ports of type map[string]string",
	}

	_, _, shouldContinue, _ := handleAgentLoopRecoveries(
		s,
		"",
		tc,
		ParsedToolResponse{},
		true,
		emotionBehaviorPolicy{},
	)

	if !shouldContinue {
		t.Fatal("expected invalid native tool call to request a corrected function call")
	}
	if len(s.pendingTCs) != 0 {
		t.Fatalf("pendingTCs len = %d, want 0", len(s.pendingTCs))
	}
	if s.pendingSummaryBatch != nil {
		t.Fatalf("pendingSummaryBatch = %#v, want nil", s.pendingSummaryBatch)
	}
	if len(s.req.Messages) == 0 {
		t.Fatal("expected recovery feedback message to be appended")
	}
	last := s.req.Messages[len(s.req.Messages)-1].Content
	if !strings.Contains(last, `last native function call for "docker"`) {
		t.Fatalf("last recovery message = %q, want invalid docker feedback", last)
	}
}
