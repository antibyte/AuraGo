package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/tools/outputcompress"

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

	first := finalizeToolExecution(context.Background(), tc, `{"status":"error","message":"connect failed"}`, false, cfg, stm, "default", &state, &req, logger, scope, "optim-db", 100, RunConfig{})
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

	second := finalizeToolExecution(context.Background(), tc, `{"status":"success","message":"ok"}`, false, cfg, stm, "default", &state, &req, logger, scope, "optim-db", 100, RunConfig{})
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
	result := finalizeToolExecution(context.Background(), tc, guardianBlockedMsg, true, cfg, nil, "default", &state, &req, logger, scope, "v1", 100, RunConfig{})
	if !result.Failed {
		t.Fatal("expected guardian blocked to be marked as failed")
	}
	if result.Outcome != ExecutionOutcomeGuardianBlocked {
		t.Fatalf("result.Outcome = %v, want ExecutionOutcomeGuardianBlocked", result.Outcome)
	}
}

func TestFinalizeToolExecutionTracksInvokeToolAsUnderlyingTool(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	cfg := &config.Config{}
	cfg.Agent.ToolOutputLimit = 50000
	req := openai.ChatCompletionRequest{}
	state := newToolRecoveryState()

	result := finalizeToolExecution(context.Background(), ToolCall{
		Action: "invoke_tool",
		Params: map[string]interface{}{
			"tool_name": "yepapi_instagram",
			"arguments": map[string]interface{}{
				"operation": "user",
				"username":  "jopliness",
			},
		},
	}, `{"status":"success","data":{}}`, false, cfg, stm, "default", &state, &req, logger, AgentTelemetryScope{}, "v1", 100, RunConfig{})
	if result.Failed {
		t.Fatalf("expected success, got %+v", result)
	}

	result2 := finalizeToolExecution(context.Background(), ToolCall{
		Action: "invoke_tool",
		Params: map[string]interface{}{
			"tool": "yepapi_instagram",
			"arguments": map[string]interface{}{
				"operation": "user",
				"username":  "jopliness",
			},
		},
	}, `{"status":"success","data":{}}`, false, cfg, stm, "default", &state, &req, logger, AgentTelemetryScope{}, "v1", 100, RunConfig{})
	if result2.Failed {
		t.Fatalf("expected success, got %+v", result2)
	}

	invokeCount, err := stm.GetToolUsageCount("invoke_tool")
	if err != nil {
		t.Fatalf("GetToolUsageCount invoke_tool: %v", err)
	}
	if invokeCount != 0 {
		t.Fatalf("invoke_tool usage count = %d, want 0", invokeCount)
	}
	instagramCount, err := stm.GetToolUsageCount("yepapi_instagram")
	if err != nil {
		t.Fatalf("GetToolUsageCount yepapi_instagram: %v", err)
	}
	if instagramCount != 2 {
		t.Fatalf("yepapi_instagram usage count = %d, want 2", instagramCount)
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

	result := finalizeToolExecution(context.Background(), tc, `{"status":"error","message":"Unknown filesystem operation: 'read'"}`, false, cfg, nil, "default", &state, &req, logger, scope, "optim-db", 100, RunConfig{})
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

	result := finalizeToolExecution(context.Background(), tc, `{"status":"error","message":"connect failed"}`, false, cfg, stm, "default", &state, &req, logger, scope, "v1", 100, RunConfig{})
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

func TestFinalizeToolExecutionTruncatesBeforeCompression(t *testing.T) {
	resetAgentTelemetryForTest()
	outputcompress.ResetCompressionStats()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{}
	cfg.Agent.ToolOutputLimit = 500
	cfg.Agent.OutputCompression.Enabled = true
	cfg.Agent.OutputCompression.MinChars = 100
	cfg.Agent.OutputCompression.ShellCompression = true
	cfg.Agent.OutputCompression.PythonCompression = true
	cfg.Agent.OutputCompression.APICompression = true
	cfg.Agent.OutputCompression.RepetitiveSubstitution.Enabled = true
	cfg.Agent.OutputCompression.RepetitiveSubstitution.LZWEnabled = true
	cfg.Agent.OutputCompression.RepetitiveSubstitution.MinPhraseChars = 10
	cfg.Agent.OutputCompression.RepetitiveSubstitution.MinOccurrences = 2
	cfg.Agent.OutputCompression.RepetitiveSubstitution.MinSavingsPercent = 1
	cfg.Agent.OutputCompression.RepetitiveSubstitution.MaxInputChars = 2000
	cfg.Agent.OutputCompression.RepetitiveSubstitution.MaxDictionaryEntries = 8

	raw := strings.Repeat("INFO backup job completed for dataset alpha with stable repeated payload\n", 100)
	state := newToolRecoveryState()
	result := finalizeToolExecution(context.Background(), ToolCall{
		Action:       "execute_shell",
		Command:      "docker logs backup",
		NativeCallID: "call-compress",
	}, raw, false, cfg, nil, "default", &state, &openai.ChatCompletionRequest{}, logger, AgentTelemetryScope{}, "v1", 100, RunConfig{})
	if result.Content == "" {
		t.Fatal("expected finalized content")
	}

	snap := outputcompress.GetCompressionSnapshot()
	if len(snap.RecentCompressions) == 0 {
		t.Fatal("expected compression stats to be recorded")
	}
	got := snap.RecentCompressions[len(snap.RecentCompressions)-1]
	if got.RawChars > cfg.Agent.ToolOutputLimit {
		t.Fatalf("compression saw %d raw chars, want <= truncation limit %d", got.RawChars, cfg.Agent.ToolOutputLimit)
	}
}

func TestFinalizeToolExecutionUsesPrimaryOutputVaultForLargeNativeOutput(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	cfg := &config.Config{}
	cfg.Agent.ToolOutputLimit = 50000
	cfg.Agent.OutputCompression.Reversible.Enabled = true
	cfg.Agent.OutputCompression.Reversible.PrimaryOutputVault = true
	cfg.Agent.OutputCompression.Reversible.MaxInlineChars = 80

	var rawLines []string
	for i := 1; i <= 20; i++ {
		rawLines = append(rawLines, fmt.Sprintf("line-%02d /srv/app/main.go:%d INFO task chunk complete", i, 10+i))
	}
	raw := strings.Join(rawLines, "\n")
	tc := ToolCall{
		Action:       "execute_shell",
		Command:      "cat /srv/app/log.txt",
		NativeCallID: "call_primary_vault",
	}
	state := newToolRecoveryState()
	result := finalizeToolExecution(context.Background(), tc, raw, false, cfg, stm, "default",
		&state, &openai.ChatCompletionRequest{}, logger, AgentTelemetryScope{}, "v1", 100, RunConfig{})

	wantRef := memory.StableToolOutputRef("default", "call_primary_vault")
	if result.OutputRef != wantRef {
		t.Fatalf("OutputRef = %q, want %q", result.OutputRef, wantRef)
	}
	if result.EventContent != raw {
		t.Fatalf("EventContent should preserve raw output for SSE/media parsing")
	}
	if !strings.Contains(result.Content, `"output_ref":"`+wantRef+`"`) {
		t.Fatalf("compacted content should expose output_ref, got: %s", result.Content)
	}
	if strings.Contains(result.Content, "line-20") {
		t.Fatalf("compacted content should not inline the whole raw output, got: %s", result.Content)
	}
	if result.Outcome != ExecutionOutcomeSanitized {
		t.Fatalf("Outcome = %v, want sanitized for vaulted context output", result.Outcome)
	}

	archived, err := stm.RetrieveCompressedOutputByRef(context.Background(), "default", wantRef)
	if err != nil {
		t.Fatalf("RetrieveCompressedOutputByRef: %v", err)
	}
	if archived.OriginalContent != raw {
		t.Fatalf("archived original mismatch\n got: %q\nwant: %q", archived.OriginalContent, raw)
	}
	if archived.FilterUsed != "output-vault" {
		t.Fatalf("FilterUsed = %q, want output-vault", archived.FilterUsed)
	}
	if !strings.Contains(archived.SummaryContent, "/srv/app/main.go:11") {
		t.Fatalf("summary should retain useful path/line details, got: %s", archived.SummaryContent)
	}
}
