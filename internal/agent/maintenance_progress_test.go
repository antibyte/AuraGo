package agent

import (
	"context"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

type maintenanceCompletionTestClient struct {
	skillQualityTestClient
	complete func(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

func (c maintenanceCompletionTestClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return c.complete(ctx, req)
}

func TestConsolidationUsesRemainingPhaseBudget(t *testing.T) {
	for _, tc := range []struct {
		name      string
		timeout   time.Duration
		delay     time.Duration
		limit     int
		processed int
		claimed   int
	}{
		{"deadline", 2 * time.Minute, 50 * time.Second, 200, 80, 91},
		{"message cap", 2 * time.Minute, 50 * time.Second, 65, 65, 65},
		{"short remaining budget", 40 * time.Second, 10 * time.Second, 200, 91, 91},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				stm, logger := maintenanceRegressionStores(t)
				for range 92 {
					if _, err := stm.InsertMessage("direct", "user", "Remember the NAS backup target.", false, false); err != nil {
						t.Fatal(err)
					}
				}
				if err := stm.DeleteOldMessages("direct", 1); err != nil {
					t.Fatal(err)
				}
				cfg := &config.Config{}
				cfg.LLM.Model = "test-model"
				cfg.Consolidation.MaxBatchMessages = tc.limit
				client := maintenanceCompletionTestClient{complete: func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
					select {
					case <-ctx.Done():
						return openai.ChatCompletionResponse{}, ctx.Err()
					case <-time.After(tc.delay):
						return skillQualityTestClient{response: `{"facts":[]}`}.CreateChatCompletion(ctx, req)
					}
				}}
				ctx, cancel := context.WithTimeout(t.Context(), tc.timeout)
				defer cancel()
				result := consolidateSTMtoLTMWithContext(ctx, cfg, logger, client, stm, &hierarchyVectorDB{}, nil)
				if result.MessagesConsolidated != tc.processed || result.MessagesClaimed != tc.claimed {
					t.Fatalf("processed=%d claimed=%d, want %d/%d", result.MessagesConsolidated, result.MessagesClaimed, tc.processed, tc.claimed)
				}
				pending, err := stm.GetConsolidationCandidates(200, 3)
				if err != nil || len(pending) != 91-tc.processed {
					t.Fatalf("pending=%d, err=%v", len(pending), err)
				}
				for _, message := range pending {
					if message.ConsolidationStatus != "pending" || message.ConsolidationRetries != 0 {
						t.Fatalf("deadline must release claims without spending retries: %+v", message)
					}
				}
			})
		})
	}
}

func TestConsolidationRequiresExplicitValidFacts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields string
		valid  bool
	}{
		{"empty", `"facts":[]`, true},
		{"fact", `"facts":[{"concept":"Backup","content":"Backups target the NAS."}]`, true},
		{"missing", `"other":[]`, false},
		{"null", `"facts":null`, false},
		{"malformed", `"facts":{}`, false},
		{"invalid fact", `"facts":[{"concept":"Backup","content":" "}]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, logger := maintenanceRegressionStores(t)
			cfg := &config.Config{}
			cfg.LLM.Model = "test-model"
			_, directErr := extractConsolidationFactsWithLLM(t.Context(), cfg, logger,
				skillQualityTestClient{response: "{" + tc.fields + "}"}, "test-model", "Conversation")
			_, helperErr := parseHelperConsolidationBatchResult(`{"batches":[{"batch_id":"one",` + tc.fields + `}]}`)
			if (directErr == nil) != tc.valid || (helperErr == nil) != tc.valid {
				t.Fatalf("valid=%t, direct error=%v, helper error=%v", tc.valid, directErr, helperErr)
			}
		})
	}
}

func TestMaintenanceConsolidationMakesProgressBeforeSlowPhases(t *testing.T) {
	for _, slow := range []string{"summary", "consolidation"} {
		t.Run(slow, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				stm, logger := maintenanceRegressionStores(t)
				for range 3 {
					if _, err := stm.InsertMessage("direct", "user", strings.Repeat("The backup target is the NAS. ", 100), false, false); err != nil {
						t.Fatal(err)
					}
				}
				if err := stm.DeleteOldMessages("direct", 1); err != nil {
					t.Fatal(err)
				}
				yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
				if _, err := stm.InsertJournalEntry(memory.JournalEntry{Date: yesterday, EntryType: "activity", Title: "Backup", Content: "Configured NAS backups."}); err != nil {
					t.Fatal(err)
				}
				cfg := &config.Config{}
				cfg.LLM.Model = "test-model"
				cfg.Consolidation.Enabled = true
				cfg.CircuitBreaker.MaintenanceTimeoutMinutes = 10
				cfg.Tools.Journal.Enabled = true
				cfg.Journal.DailySummary = true
				cfg.Directories.PromptsDir = t.TempDir()
				var calls []string
				client := maintenanceCompletionTestClient{complete: func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
					phase := "summary"
					content := "Configured NAS backups."
					if strings.Contains(req.Messages[0].Content, "knowledge extraction engine") {
						phase = "consolidation"
						content = `{"facts":[{"concept":"Backup target","content":"The backup target is the NAS."}]}`
					}
					calls = append(calls, phase)
					if phase == slow {
						<-ctx.Done()
						return openai.ChatCompletionResponse{}, ctx.Err()
					}
					return skillQualityTestClient{response: content}.CreateChatCompletion(ctx, req)
				}}
				runMaintenanceTask(t.Context(), cfg, logger, client, nil, nil, nil, nil, &hierarchyVectorDB{}, stm, nil, nil, nil, nil, nil, nil, nil, nil, nil)
				run, err := stm.GetLatestMaintenanceRun()
				if err != nil || run == nil {
					t.Fatalf("run=%v err=%v", run, err)
				}
				if len(calls) < 2 || calls[0] != "consolidation" || calls[len(calls)-1] != "summary" {
					t.Fatalf("consolidation must run early and leave time for summary: %v", calls)
				}
				for _, phase := range run.PhaseResults.Phases {
					if phase.Name != "consolidation" {
						continue
					}
					if slow == "summary" && (phase.Processed != 2 || phase.Status != "completed" || run.PhaseResults.ConsolidationBacklog != 0) {
						t.Fatalf("consolidation made no progress: %+v", phase)
					}
					if slow == "consolidation" && (phase.DurationMS > (2*time.Minute).Milliseconds() || phase.Status != "partial" || run.PhaseResults.ConsolidationBacklog != 2) {
						t.Fatalf("consolidation did not preserve its budget/backlog: %+v", phase)
					}
				}
				if slow == "consolidation" {
					candidates, err := stm.GetConsolidationCandidates(10, 3)
					// SQLite retry timestamps use the real clock; only the released
					// claim is immediately eligible inside the fake-clock test.
					if err != nil || len(candidates) != 1 {
						t.Fatalf("eligible candidates=%d err=%v", len(candidates), err)
					}
					pending := 0
					for _, candidate := range candidates {
						if candidate.ConsolidationStatus == "pending" && candidate.ConsolidationRetries == 0 {
							pending++
						}
					}
					if pending != 1 {
						t.Fatal("phase deadline consumed a retry")
					}
				}
			})
		})
	}
}

func TestMorningBriefingExplainsUnfinishedPhasesAndConsolidationProgress(t *testing.T) {
	for _, language := range []string{"de", "en"} {
		t.Run(language, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Server.UILanguage = language
			results := memory.MaintenancePhaseResults{
				Processed: 23, Deferred: 5517, ConsolidationBacklog: 5510,
				Phases: []memory.MaintenancePhaseResult{
					{Name: "consolidation", Status: "partial", Deferred: 5510, ErrorCodes: []string{"phase_budget_exhausted"}},
					{Name: "daily_summary", Status: "partial", Processed: 1, Deferred: 5, ErrorCodes: []string{"daily_summary"}},
					{Name: "profile_cleanup", Status: "completed"},
				},
			}
			got := formatMorningBriefing(cfg, time.Now(), "partial", results, 51)
			for _, want := range []string{"consolidation: partial", "phase_budget_exhausted", "daily_summary: partial", "5510", "51"} {
				if !strings.Contains(got, want) {
					t.Fatalf("briefing missing %q: %s", want, got)
				}
			}
			progress := "archive messages processed: 0"
			if language == "de" {
				progress = "verarbeitete Archivnachrichten: 0"
			}
			if !strings.Contains(got, progress) || strings.Contains(got, "profile_cleanup") {
				t.Fatalf("briefing confuses total work with consolidation progress: %s", got)
			}
		})
	}
}

func TestMaintenanceCompletionsReserveReasoningWithinProviderLimits(t *testing.T) {
	stm, logger := maintenanceRegressionStores(t)
	cfg := &config.Config{}
	cfg.LLM.Provider, cfg.LLM.ProviderType, cfg.LLM.Model = "main", "openai", "o3-mini"
	cfg.Providers = []config.ProviderEntry{{ID: "main", Type: "openai", Model: "o3-mini", ContextWindow: 32768, MaxOutputTokens: 4096}}
	date := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if _, err := stm.InsertJournalEntry(memory.JournalEntry{Date: date, EntryType: "activity", Title: "Backup", Content: "Configured NAS backups."}); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := stm.InsertMessage("direct", "user", "Remember the NAS backup target.", false, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := stm.DeleteOldMessages("direct", 1); err != nil {
		t.Fatal(err)
	}
	client := maintenanceCompletionTestClient{complete: func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		if req.MaxTokens != 4096 {
			t.Errorf("completion budget=%d, want reasoning reserve capped at provider limit 4096", req.MaxTokens)
		}
		content := "Configured NAS backups."
		if strings.Contains(req.Messages[0].Content, "knowledge extraction engine") {
			content = `{"facts":[{"concept":"Backup target","content":"The backup target is the NAS."}]}`
		}
		return skillQualityTestClient{response: content}.CreateChatCompletion(ctx, req)
	}}
	if err := generateDailySummary(t.Context(), cfg, logger, client, stm, date); err != nil {
		t.Fatal(err)
	}
	if _, processed := consolidateSTMtoLTM(cfg, logger, client, stm, &hierarchyVectorDB{}, nil); processed != 2 {
		t.Fatalf("consolidated=%d, want 2", processed)
	}
}

func TestMaintenanceCompletionBudgetUsesHelperLimitsAndRejectsOversizeInput(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Provider = "main"
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProvider = "helper"
	cfg.LLM.HelperProviderType = "openai"
	cfg.LLM.HelperResolvedModel = "o3-mini"
	cfg.Providers = []config.ProviderEntry{
		{ID: "main", Type: "openai", ContextWindow: 32768, MaxOutputTokens: 16384},
		{ID: "helper", Type: "openai", Model: "o3-mini", ContextWindow: 32768, MaxOutputTokens: 2048},
	}
	req := openai.ChatCompletionRequest{Model: "o3-mini", MaxTokens: 300,
		Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Summarize completed work."}},
	}
	got, err := maintenanceCompletionBudget(cfg, req)
	if err != nil || got != 2048 {
		t.Fatalf("helper output budget=%d err=%v, want 2048", got, err)
	}
	cfg.Agent.ContextWindow = 2100
	if _, err := maintenanceCompletionBudget(cfg, req); err == nil {
		t.Fatal("request exceeding the effective context limit was allowed")
	}
}
