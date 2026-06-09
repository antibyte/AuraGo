package agent

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

func TestFinalizeToolExecutionSkipsJournalForHeartbeatErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	if err := stm.InitErrorLearningTable(); err != nil {
		t.Fatalf("InitErrorLearningTable: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.ToolOutputLimit = 50000
	cfg.Tools.Journal.Enabled = true
	cfg.Journal.AutoEntries = true

	state := newToolRecoveryState()
	tc := ToolCall{Action: "send_telegram"}
	runCfg := RunConfig{MessageSource: "heartbeat"}

	_ = finalizeToolExecution(
		context.Background(), tc,
		`{"status":"error","message":"message is required"}`,
		false, cfg, stm, "heartbeat", &state, &openai.ChatCompletionRequest{},
		logger, AgentTelemetryScope{}, "v1", 100, runCfg,
	)

	entries, err := stm.GetJournalEntries("", "", nil, 10)
	if err != nil {
		t.Fatalf("GetJournalEntries: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no journal entries for heartbeat tool errors, got %d", len(entries))
	}
}

func TestFinalizeToolExecutionDedupesRepeatedJournalErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	if err := stm.InitErrorLearningTable(); err != nil {
		t.Fatalf("InitErrorLearningTable: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.ToolOutputLimit = 50000
	cfg.Tools.Journal.Enabled = true
	cfg.Journal.AutoEntries = true

	runCfg := RunConfig{MessageSource: "web_chat"}
	tc := ToolCall{Action: "send_telegram"}
	payload := `{"status":"error","message":"message is required"}`

	firstState := newToolRecoveryState()
	_ = finalizeToolExecution(
		context.Background(), tc, payload,
		false, cfg, stm, "default", &firstState, &openai.ChatCompletionRequest{},
		logger, AgentTelemetryScope{}, "v1", 100, runCfg,
	)

	secondState := newToolRecoveryState()
	_ = finalizeToolExecution(
		context.Background(), tc, payload,
		false, cfg, stm, "default", &secondState, &openai.ChatCompletionRequest{},
		logger, AgentTelemetryScope{}, "v1", 100, runCfg,
	)

	entries, err := stm.GetJournalEntries("", "", nil, 10)
	if err != nil {
		t.Fatalf("GetJournalEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one deduplicated journal entry, got %d", len(entries))
	}
}