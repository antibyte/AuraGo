package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/planner"
	openai "github.com/sashabaranov/go-openai"
)

type reflectionResponseClient struct {
	reflectionTestClient
	completions []openai.ChatCompletionResponse
}

func (c *reflectionResponseClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.requests = append(c.requests, req)
	if len(c.completions) == 0 {
		return openai.ChatCompletionResponse{}, nil
	}
	response := c.completions[0]
	c.completions = c.completions[1:]
	return response, nil
}

func reflectionCompletion(content string, reason openai.FinishReason) openai.ChatCompletionResponse {
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{
		Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content}, FinishReason: reason,
	}}}
}

const validReflectionJSON = `{"patterns":["Repeated workspace cleanup failures require checking whether the guest still exists."],"summary":"The sampled records show repeated workspace cleanup attempts. Confirm guest absence before retrying another close operation."}`

func TestMemoryReflectionRetriesUnusableCompletions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		first openai.ChatCompletionResponse
	}{
		{"no choices", openai.ChatCompletionResponse{}},
		{"empty", reflectionCompletion("", openai.FinishReasonStop)},
		{"reasoning only", reflectionCompletion("<think>private reasoning</think>", openai.FinishReasonStop)},
		{"truncated valid JSON", reflectionCompletion(validReflectionJSON, openai.FinishReasonLength)},
		{"invalid JSON", reflectionCompletion("{incomplete", openai.FinishReasonStop)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stm := newReflectionTestDB(t)
			for i := 0; i < 8; i++ {
				if _, err := stm.InsertJournalEntry(memory.JournalEntry{EntryType: "note", Title: "sample", Content: strings.Repeat("Workspace cleanup observation. ", 40)}); err != nil {
					t.Fatal(err)
				}
			}
			cfg := &config.Config{}
			cfg.LLM.Model = "test-model"
			client := &reflectionResponseClient{completions: []openai.ChatCompletionResponse{tc.first, reflectionCompletion(validReflectionJSON, openai.FinishReasonStop)}}
			result, err := runMemoryReflection(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), stm, nil, nil, client, nil, memoryReflectionRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if len(client.requests) != 2 {
				t.Fatalf("calls = %d, want one retry", len(client.requests))
			}
			if len(client.requests[1].Messages[0].Content) >= len(client.requests[0].Messages[0].Content) {
				t.Fatal("retry did not reduce the source sample")
			}
			if result.ScopeNote != "retry_with_reduced_source_sample" {
				t.Fatalf("retry scope not disclosed: %s", result.ScopeNote)
			}
			entries, err := stm.GetJournalEntries("", "", []string{"reflection"}, 10)
			if err != nil || len(entries) != 1 {
				t.Fatalf("journal count = %d, err=%v", len(entries), err)
			}
			if strings.Contains(entries[0].Content, "private reasoning") {
				t.Fatal("reasoning was persisted")
			}
		})
	}
}

func TestWeeklyReflectionFailureDoesNotPersistOrConsumeDailyClaim(t *testing.T) {
	releaseWeeklyReflectionClaim()
	t.Cleanup(releaseWeeklyReflectionClaim)
	stm := newReflectionTestDB(t)
	db, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reconcileMaintenancePhaseIssues(db, []memory.MaintenancePhaseResult{{Name: "weekly_reflection", Status: "partial", ErrorCodes: []string{"weekly_reflection"}}}, time.Now(), nil)
	cfg := &config.Config{}
	cfg.MemoryAnalysis.Enabled, cfg.MemoryAnalysis.WeeklyReflection = true, true
	cfg.MemoryAnalysis.ReflectionDay = strings.ToLower(time.Now().Weekday().String())
	cfg.LLM.Model = "test-model"
	client := &reflectionResponseClient{completions: []openai.ChatCompletionResponse{
		reflectionCompletion(validReflectionJSON, openai.FinishReasonLength),
		reflectionCompletion(validReflectionJSON, openai.FinishReasonLength),
	}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ran, err := runWeeklyReflectionJob(context.Background(), cfg, logger, client, stm, nil, nil, db)
	if ran || !errors.Is(err, llm.ErrJSONCompletionTruncated) {
		t.Fatalf("ran=%v err=%v", ran, err)
	}
	if len(client.requests) != 2 {
		t.Fatalf("unbounded retries: %d", len(client.requests))
	}
	entries, err := stm.GetJournalEntries("", "", []string{"reflection"}, 10)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid reflection persisted: %d %v", len(entries), err)
	}
	client.completions = []openai.ChatCompletionResponse{reflectionCompletion(validReflectionJSON, openai.FinishReasonStop)}
	ran, err = runWeeklyReflectionJob(context.Background(), cfg, logger, client, stm, nil, nil, db)
	if err != nil || !ran {
		t.Fatalf("retry claim blocked after failure: %v %v", ran, err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM operational_issues WHERE fingerprint = 'maintenance|phase|weekly_reflection'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "done" {
		t.Fatalf("successful background reflection did not resolve the issue: %s", status)
	}
}

func TestMemoryReflectionBudgetUsesReasoningAndProviderLimits(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Provider, cfg.LLM.ProviderType, cfg.LLM.Model = "stepfun", "openai", "step-3.7-flash"
	budget, _ := memoryReflectionBudget(cfg, memoryAnalysisLLMConfig{}, cfg.LLM.Model)
	if budget.CompletionReserve != llm.ReasoningOutputTokens {
		t.Fatalf("reasoning output budget = %d, want %d", budget.CompletionReserve, llm.ReasoningOutputTokens)
	}
	cfg.Providers = []config.ProviderEntry{{ID: "stepfun", Type: "openai", Model: cfg.LLM.Model, MaxOutputTokens: 3000, ContextWindow: 16000}}
	budget, _ = memoryReflectionBudget(cfg, memoryAnalysisLLMConfig{}, cfg.LLM.Model)
	if budget.CompletionReserve != 3000 || budget.Routes[0].Limits.ContextWindow != 16000 {
		t.Fatalf("provider limits not respected: %+v", budget)
	}
}

func TestWeeklyReflectionSkippedPhaseDoesNotResolveFailure(t *testing.T) {
	db, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	failed := []memory.MaintenancePhaseResult{{Name: "weekly_reflection", Status: "partial", ErrorCodes: []string{"weekly_reflection"}}}
	reconcileMaintenancePhaseIssues(db, failed, time.Now(), nil)
	ledger := newMaintenanceRunLedger()
	ledger.beginPhase("weekly_reflection")
	ledger.skipPhase("weekly_reflection")
	reconcileMaintenancePhaseIssues(db, ledger.results().Phases, time.Now(), nil)
	var status string
	if err := db.QueryRow(`SELECT status FROM operational_issues WHERE fingerprint = 'maintenance|phase|weekly_reflection'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "open" {
		t.Fatalf("skipped reflection incorrectly resolved the failure: %s", status)
	}
	ledger = newMaintenanceRunLedger()
	ledger.beginPhase("weekly_reflection")
	ledger.finishPhase("weekly_reflection", false)
	reconcileMaintenancePhaseIssues(db, ledger.results().Phases, time.Now(), nil)
	if err := db.QueryRow(`SELECT status FROM operational_issues WHERE fingerprint = 'maintenance|phase|weekly_reflection'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "done" {
		t.Fatalf("successful reflection did not resolve the failure: %s", status)
	}
}
