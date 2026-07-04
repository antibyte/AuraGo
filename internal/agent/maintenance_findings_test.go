package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

type countingMaintenanceLLMClient struct {
	calls int
}

func (c *countingMaintenanceLLMClient) CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.calls++
	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{Content: "Maintenance summary."},
		}},
	}, nil
}

func (c *countingMaintenanceLLMClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, errors.New("streaming is not used in maintenance tests")
}

func TestParseTimeRejectsOutOfRangeMaintenanceTimes(t *testing.T) {
	for _, input := range []string{"24:00", "25:99", "-1:00", "04:60"} {
		t.Run(input, func(t *testing.T) {
			if _, _, err := parseTime(input); err == nil {
				t.Fatalf("parseTime(%q) error = nil, want invalid maintenance time", input)
			}
		})
	}
}

func TestGenerateDailySummarySkipsCanceledMaintenanceContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}

	if _, err := stm.InsertJournalEntry(memory.JournalEntry{
		EntryType:  "activity",
		Title:      "Maintenance context test",
		Content:    "Enough activity for a summary.",
		Importance: 2,
	}); err != nil {
		t.Fatalf("InsertJournalEntry: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	client := &countingMaintenanceLLMClient{}

	generateDailySummary(ctx, cfg, logger, client, stm)

	if client.calls != 0 {
		t.Fatalf("LLM calls = %d, want 0 after maintenance context cancellation", client.calls)
	}
}

func TestConsolidateSTMtoLTMSkipsCanceledMaintenanceContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	for i, role := range []string{"user", "assistant", "user"} {
		if _, err := stm.InsertMessage("s1", role, "remember the maintenance timeout context propagation detail "+role, false, false); err != nil {
			t.Fatalf("InsertMessage %d: %v", i, err)
		}
	}
	if err := stm.DeleteOldMessages("s1", 1); err != nil {
		t.Fatalf("DeleteOldMessages: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	cfg.Consolidation.MaxBatchMessages = 10
	cfg.Consolidation.ArchiveRetainDays = 30
	client := &countingMaintenanceLLMClient{}

	totalStored, messagesConsolidated := consolidateSTMtoLTMWithContext(ctx, cfg, logger, client, stm, &hierarchyVectorDB{}, nil)
	if totalStored != 0 || messagesConsolidated != 0 {
		t.Fatalf("stored=%d messages=%d, want 0/0 after cancellation", totalStored, messagesConsolidated)
	}
	if client.calls != 0 {
		t.Fatalf("LLM calls = %d, want 0 after maintenance context cancellation", client.calls)
	}

	candidates, err := stm.GetConsolidationCandidates(10, 3)
	if err != nil {
		t.Fatalf("GetConsolidationCandidates: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected archived messages to remain pending after canceled maintenance context")
	}
	for _, msg := range candidates {
		if msg.ConsolidationStatus != "pending" {
			t.Fatalf("status = %q, want pending", msg.ConsolidationStatus)
		}
	}
}

func TestMaintenanceTaskRecordsDeterministicPhaseErrorsInLedger(t *testing.T) {
	source, err := os.ReadFile("maintenance.go")
	if err != nil {
		t.Fatalf("read maintenance.go: %v", err)
	}
	text := string(source)
	for _, marker := range []string{
		"patterns_cleanup:",
		"archive_events_cleanup:",
		"mood_log_cleanup:",
		"error_patterns_cleanup:",
		"tool_transitions_cleanup:",
		"learned_rules_cleanup:",
		"profile_cleanup:",
		"activity_rollup:",
		"notes_cleanup:",
		"kg_cleanup:",
		"kg_semantic_reindex:",
		"kg_semantic_backlog:",
		"operational_issue_cleanup:",
		"daily_summary:",
	} {
		if !strings.Contains(text, "ledger.addError(\""+marker) {
			t.Fatalf("maintenance.go is missing ledger.addError for %s", marker)
		}
	}
}
