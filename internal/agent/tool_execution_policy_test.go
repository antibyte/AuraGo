package agent

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

func TestFinalizeToolExecutionRecordsErrorAndResolution(t *testing.T) {
	resetAgentTelemetryForTest()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()
	if err := stm.InitErrorLearningTable(); err != nil {
		t.Fatalf("InitErrorLearningTable: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.ToolOutputLimit = 50000
	scope := AgentTelemetryScope{ProviderType: "openrouter", Model: "gpt-4o-mini"}
	req := openai.ChatCompletionRequest{}
	state := newToolRecoveryState()
	tc := ToolCall{Action: "homepage"}

	first := finalizeToolExecution(tc, `{"status":"error","message":"connect failed"}`, false, cfg, stm, "default", &state, &req, logger, scope, "optim-db", 100)
	if !first.Failed {
		t.Fatal("expected failing tool output to be marked as failed")
	}
	if first.Outcome != ExecutionOutcomeFailed {
		t.Fatalf("first.Outcome = %v, want ExecutionOutcomeFailed", first.Outcome)
	}

	count, err := stm.GetErrorPatternsCount()
	if err != nil {
		t.Fatalf("GetErrorPatternsCount: %v", err)
	}
	if count != 1 {
		t.Fatalf("error pattern count = %d, want 1", count)
	}

	second := finalizeToolExecution(tc, `{"status":"success","message":"ok"}`, false, cfg, stm, "default", &state, &req, logger, scope, "optim-db", 100)
	if second.Failed {
		t.Fatal("expected success output to be marked as successful")
	}
	if second.Outcome != ExecutionOutcomeSuccess {
		t.Fatalf("second.Outcome = %v, want ExecutionOutcomeSuccess", second.Outcome)
	}

	patterns, err := stm.GetFrequentErrors("homepage", 1)
	if err != nil {
		t.Fatalf("GetFrequentErrors: %v", err)
	}
	if len(patterns) != 1 {
		t.Fatalf("frequent errors len = %d, want 1", len(patterns))
	}
	if patterns[0].Resolution != "Succeeded with adjusted parameters" {
		t.Fatalf("resolution = %q, want recorded resolution", patterns[0].Resolution)
	}
}

func TestFinalizeToolExecutionGuardianBlockedSetsOutcome(t *testing.T) {
	resetAgentTelemetryForTest()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{}
	scope := AgentTelemetryScope{}
	req := openai.ChatCompletionRequest{}
	state := newToolRecoveryState()
	tc := ToolCall{Action: "execute_shell"}

	guardianBlockedMsg := "[TOOL BLOCKED] Security check failed for execute_shell: remote code execution via curl pipe sh (risk: 85%)."
	result := finalizeToolExecution(tc, guardianBlockedMsg, true, cfg, nil, "default", &state, &req, logger, scope, "v1", 100)
	if !result.Failed {
		t.Fatal("expected guardian blocked to be marked as failed")
	}
	if result.Outcome != ExecutionOutcomeGuardianBlocked {
		t.Fatalf("result.Outcome = %v, want ExecutionOutcomeGuardianBlocked", result.Outcome)
	}
}

func TestExecutionOutcomeString(t *testing.T) {
	tests := []struct {
		outcome ExecutionOutcome
		want    string
	}{
		{ExecutionOutcomeSuccess, "success"},
		{ExecutionOutcomeFailed, "failed"},
		{ExecutionOutcomeGuardianBlocked, "guardian_blocked"},
		{ExecutionOutcomeSanitized, "sanitized"},
		{ExecutionOutcome(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.outcome.String(); got != tt.want {
			t.Errorf("ExecutionOutcome(%d).String() = %q, want %q", tt.outcome, got, tt.want)
		}
	}
}

func TestFinalizeToolExecutionAppendsSuggestedNextStep(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{}
	cfg.Agent.ToolOutputLimit = 50000
	scope := AgentTelemetryScope{}
	req := openai.ChatCompletionRequest{}
	state := newToolRecoveryState()
	tc := ToolCall{Action: "filesystem"}

	result := finalizeToolExecution(tc, `{"status":"error","message":"Unknown filesystem operation: 'read'"}`, false, cfg, nil, "default", &state, &req, logger, scope, "optim-db", 100)
	if !result.Failed {
		t.Fatal("expected tool failure")
	}
	if !strings.Contains(result.Content, "[Suggested next step]") {
		t.Fatalf("expected suggested next step in content, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "read_file") {
		t.Fatalf("expected filesystem-specific guidance, got: %s", result.Content)
	}
}

func TestFinalizeToolExecutionWarnsWhenMemoryPersistenceFails(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitErrorLearningTable(); err != nil {
		t.Fatalf("InitErrorLearningTable: %v", err)
	}
	if err := stm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.ToolOutputLimit = 50000
	cfg.Tools.Journal.Enabled = true
	cfg.Journal.AutoEntries = true
	scope := AgentTelemetryScope{}
	req := openai.ChatCompletionRequest{}
	state := newToolRecoveryState()
	tc := ToolCall{Action: "homepage"}

	result := finalizeToolExecution(tc, `{"status":"error","message":"connect failed"}`, false, cfg, stm, "default", &state, &req, logger, scope, "v1", 100)
	if !result.Failed {
		t.Fatal("expected tool failure")
	}

	logs := logBuf.String()
	for _, want := range []string{
		"Failed to persist tool usage stats",
		"Failed to persist tool error pattern",
		"Failed to persist error journal entry",
	} {
		if !strings.Contains(logs, want) {
			t.Fatalf("expected logs to contain %q, got %q", want, logs)
		}
	}
}
