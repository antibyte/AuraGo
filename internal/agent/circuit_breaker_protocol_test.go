package agent

import (
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/memory"
)

func TestCircuitBreakerCompletesEveryRemainingNativeToolCallWithoutDispatch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(filepath.Join(t.TempDir(), "short-term.db"), logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	history := memory.NewHistoryManager(filepath.Join(t.TempDir(), "history.json"))
	s := &agentLoopState{
		currentLogger: logger,
		pendingTCs: []ToolCall{
			{Action: "telegram", NativeCallID: "call-telegram"},
			{Action: "execute_shell", NativeCallID: "call-shell"},
		},
	}

	appendCircuitBreakerSkippedNativeResults(s, stm, history, "test-session", NoopBroker{})
	if len(s.pendingTCs) != 0 {
		t.Fatalf("pending calls = %#v", s.pendingTCs)
	}
	if len(s.req.Messages) != 2 {
		t.Fatalf("protocol results = %#v", s.req.Messages)
	}
	for i, wantID := range []string{"call-telegram", "call-shell"} {
		msg := s.req.Messages[i]
		if msg.ToolCallID != wantID || !strings.Contains(msg.Content, "not_executed_due_to_circuit_breaker") {
			t.Fatalf("result %d = %#v", i, msg)
		}
	}
}
