package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/prompts"

	"github.com/sashabaranov/go-openai"
)

func TestMaintenanceBriefingWaitsForCatchupAndKeepsMainDeadlineIndependent(t *testing.T) {
	// Initialize shared tokenizer/network state outside the fake-clock bubble.
	prompts.CountTokensForModel("warmup", "test-model")
	synctest.Test(t, func(t *testing.T) {
		stm, logger := maintenanceRegressionStores(t)
		for range 5 {
			if _, err := stm.InsertMessage("direct", "user", "Remember the NAS backup target.", false, false); err != nil {
				t.Fatal(err)
			}
		}
		if err := stm.DeleteOldMessages("direct", 1); err != nil {
			t.Fatal(err)
		}
		cfg := &config.Config{}
		cfg.Tools.Memory.Enabled = true
		cfg.Tools.KnowledgeGraph.Enabled = true
		cfg.LLM.Model = "test-model"
		cfg.Server.UILanguage = "de"
		cfg.Consolidation.Enabled = true
		cfg.Consolidation.MaxBatchMessages = 2
		cfg.Consolidation.CatchupMinutes = 15
		cfg.CircuitBreaker.MaintenanceTimeoutMinutes = 3
		cfg.Directories.PromptsDir = t.TempDir()
		calls := 0
		client := maintenanceCompletionTestClient{complete: func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
			calls++
			notes, err := stm.GetUnreadSystemNotifications()
			if err != nil || len(notes) != 0 {
				t.Fatalf("briefing published before catch-up: %v, %v", notes, err)
			}
			if calls == 1 {
				<-ctx.Done()
				return openai.ChatCompletionResponse{}, ctx.Err()
			}
			if ctx.Err() != nil {
				t.Fatal("catch-up inherited the expired normal-phase deadline")
			}
			return skillQualityTestClient{response: `{"facts":[]}`}.CreateChatCompletion(ctx, req)
		}}
		runMaintenanceTask(t.Context(), cfg, logger, client, nil, nil, nil, nil, &hierarchyVectorDB{}, stm, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		run, err := stm.GetLatestMaintenanceRun()
		if err != nil || run == nil || calls != 2 {
			t.Fatalf("run=%+v calls=%d error=%v", run, calls, err)
		}
		results := run.PhaseResults
		if results.ConsolidationBacklog != 2 || results.Processed != 2 || results.Deferred != 2 {
			t.Fatalf("catch-up totals double-count or omit work: %+v", results)
		}
		if run.Status != "failed" {
			t.Fatal("catch-up hid the missing maintenance prompt")
		}
		notes, err := stm.GetUnreadSystemNotifications()
		if err != nil || len(notes) != 1 || !strings.Contains(notes[0].Message, "Nachhol-Lauf: partial; verarbeitete Archivnachrichten: 2; verbleibender Rückstand: 2") {
			t.Fatalf("final briefing=%+v error=%v", notes, err)
		}
	})
}

func TestMaintenanceCatchupCountsFactsAndOutstandingWorkOnce(t *testing.T) {
	stm, logger := maintenanceRegressionStores(t)
	for range 5 {
		if _, err := stm.InsertMessage("direct", "user", "Remember the NAS backup target.", false, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := stm.DeleteOldMessages("direct", 1); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Tools.Memory.Enabled = true
	cfg.Tools.KnowledgeGraph.Enabled = true
	cfg.LLM.Model = "test-model"
	cfg.Consolidation.Enabled = true
	cfg.Consolidation.CatchupMinutes = 15
	cfg.Consolidation.MaxBatchMessages = 2
	ledger := newMaintenanceRunLedger()
	ledger.beginPhase("consolidation")
	ledger.addProcessed("consolidation", 40)
	ledger.addDeferred("consolidation", 4)
	ledger.phaseResults.ConsolidationFacts = 11
	ledger.phaseResults.ConsolidationBacklog = 4
	ledger.finishPhase("consolidation", true)
	ledger.beginPhase("entity_extraction")
	ledger.addDeferred("entity_extraction", 1)
	ledger.finishPhase("entity_extraction", true)
	client := skillQualityTestClient{response: `{"facts":[{"concept":"Backup target","content":"The backup target is the NAS."}]}`}
	runConsolidationCatchup(t.Context(), cfg, logger, client, stm, &hierarchyVectorDB{}, nil, ledger)
	results := ledger.results()
	if results.Processed != 42 || results.ConsolidationFacts != 12 || results.ConsolidationBacklog != 2 || results.Deferred != 3 {
		t.Fatalf("combined results=%+v", results)
	}
	if ledger.phaseDeferred("consolidation") != 0 || ledger.phaseDeferred("consolidation_catchup") != 2 || ledger.phaseDeferred("entity_extraction") != 1 {
		t.Fatalf("outstanding work assigned to the wrong phase: %+v", results.Phases)
	}
	for _, language := range []string{"de", "en"} {
		cfg.Server.UILanguage = language
		briefing := formatMorningBriefing(cfg, time.Now(), ledger.status(), results, 0)
		for _, want := range map[string][]string{
			"de": {"verarbeitete Archivnachrichten: 42", "neue Fakten: 12", "Nachhol-Lauf: partial"},
			"en": {"archive messages processed: 42", "new facts: 12", "Catch-up: partial"},
		}[language] {
			if !strings.Contains(briefing, want) {
				t.Fatalf("missing %q in %s", want, briefing)
			}
		}
	}
}

func TestMaintenanceSummaryAndKGRecoverIndependently(t *testing.T) {
	for _, failure := range []string{"none", "summary", "kg"} {
		t.Run(failure, func(t *testing.T) {
			ResetGlobalHelperLLMManager()
			t.Cleanup(ResetGlobalHelperLLMManager)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request openai.ChatCompletionRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Messages) == 0 {
					http.Error(w, "invalid fixture request", http.StatusBadRequest)
					return
				}
				kind, content := "batch", "not valid JSON"
				system := request.Messages[0].Content
				if strings.Contains(system, "concise activity summarizer") {
					kind, content = "summary", "Configured NAS backups."
				} else if strings.Contains(system, "entity extraction engine") {
					kind, content = "kg", `{"nodes":[],"edges":[]}`
				}
				w.Header().Set("Content-Type", "application/json")
				if kind == failure {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"error":{"message":"synthetic fixture failure","type":"invalid_request_error"}}`))
					return
				}
				_ = json.NewEncoder(w).Encode(openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: "assistant", Content: content}, FinishReason: openai.FinishReasonStop}}})
			}))
			defer server.Close()
			stm, logger := maintenanceRegressionStores(t)
			kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
			if err != nil {
				t.Fatal(err)
			}
			defer kg.Close()
			date := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
			if _, err := stm.InsertJournalEntry(memory.JournalEntry{Date: date, EntryType: "activity", Title: "Backups", Content: "Configured NAS backups."}); err != nil {
				t.Fatal(err)
			}
			if _, err := stm.InsertMessage("direct", "user", strings.Repeat("The backup target is the NAS. ", 10), false, false); err != nil {
				t.Fatal(err)
			}
			cfg := &config.Config{}
			cfg.Tools.Memory.Enabled = true
			cfg.Tools.KnowledgeGraph.Enabled = true
			cfg.LLM.Model = "test-model"
			cfg.LLM.HelperEnabled = true
			cfg.LLM.HelperProviderType = "openai"
			cfg.LLM.HelperBaseURL = server.URL + "/v1"
			cfg.LLM.HelperResolvedModel = "test-model"
			cfg.Tools.Journal.Enabled, cfg.Journal.DailySummary = true, true
			cfg.Tools.KnowledgeGraph.Enabled, cfg.Tools.KnowledgeGraph.AutoExtraction = true, true
			cfg.Directories.PromptsDir = t.TempDir()
			runMaintenanceTask(t.Context(), cfg, logger, nil, nil, nil, nil, nil, nil, stm, nil, kg, nil, nil, nil, nil, nil, nil, nil, nil)
			run, err := stm.GetLatestMaintenanceRun()
			if err != nil || run == nil {
				t.Fatalf("run=%+v error=%v", run, err)
			}
			for _, phase := range run.PhaseResults.Phases {
				kind := map[string]string{"daily_summary": "summary", "entity_extraction": "kg"}[phase.Name]
				if kind == "" {
					continue
				}
				if kind == failure {
					if phase.Status != "partial" || len(phase.ErrorCodes) == 0 || len(phase.WarningCodes) != 0 {
						t.Fatalf("unrecovered %s hidden: %+v", kind, phase)
					}
				} else if phase.Status != "completed" || len(phase.ErrorCodes) != 0 || len(phase.WarningCodes) != 1 {
					t.Fatalf("recovered %s still failed: %+v", kind, phase)
				}
			}
			summary, err := stm.GetDailySummary(date)
			if err != nil || (summary != nil) != (failure != "summary") {
				t.Fatalf("summary persistence=%+v error=%v", summary, err)
			}
		})
	}
}

func TestMaintenanceReservesEntityTimeBeforeGraphWork(t *testing.T) {
	prompts.CountTokensForModel("warmup", "test-model")
	synctest.Test(t, func(t *testing.T) {
		stm, logger := maintenanceRegressionStores(t)
		kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
		if err != nil {
			t.Fatal(err)
		}
		defer kg.Close()
		date := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		if _, err := stm.InsertJournalEntry(memory.JournalEntry{Date: date, EntryType: "activity", Title: "Backup", Content: "Configured NAS backups."}); err != nil {
			t.Fatal(err)
		}
		if _, err := stm.InsertMessage("direct", "user", strings.Repeat("The backup target is the NAS. ", 10), false, false); err != nil {
			t.Fatal(err)
		}
		cfg := &config.Config{}
		cfg.Tools.Memory.Enabled = true
		cfg.Tools.KnowledgeGraph.Enabled = true
		cfg.LLM.Model = "test-model"
		cfg.Tools.Journal.Enabled, cfg.Journal.DailySummary = true, true
		cfg.Tools.KnowledgeGraph.Enabled, cfg.Tools.KnowledgeGraph.AutoExtraction = true, true
		cfg.CircuitBreaker.MaintenanceTimeoutMinutes = 10
		cfg.Directories.PromptsDir = t.TempDir()
		started := time.Now()
		entityRan := false
		client := maintenanceCompletionTestClient{complete: func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
			if strings.Contains(req.Messages[0].Content, "activity summarizer") {
				<-ctx.Done()
				return openai.ChatCompletionResponse{}, ctx.Err()
			}
			entityRan = true
			deadline, ok := ctx.Deadline()
			if !ok || !deadline.After(time.Now()) || deadline.After(started.Add(10*time.Minute-maintenanceProtectedTailReserve)) {
				t.Fatalf("entity request lost usable time or protected reserve: %v", deadline)
			}
			return skillQualityTestClient{response: `{"nodes":[],"edges":[]}`}.CreateChatCompletion(ctx, req)
		}}
		runMaintenanceTask(t.Context(), cfg, logger, client, nil, nil, nil, nil, nil, stm, nil, kg, nil, nil, nil, nil, nil, nil, nil, nil)
		run, err := stm.GetLatestMaintenanceRun()
		if err != nil || run == nil || !entityRan {
			t.Fatalf("entity extraction starved: run=%+v error=%v", run, err)
		}
		entityIndex, graphIndex := -1, -1
		for i, phase := range run.PhaseResults.Phases {
			if phase.Name == "entity_extraction" {
				entityIndex = i
				if phase.Status != "completed" || phase.DurationMS > maintenanceEntityBudget.Milliseconds() {
					t.Fatalf("entity phase=%+v", phase)
				}
			}
			if phase.Name == "knowledge_graph" {
				graphIndex = i
			}
		}
		if entityIndex < 0 || graphIndex <= entityIndex {
			t.Fatalf("expensive graph work precedes extraction: %+v", run.PhaseResults.Phases)
		}
	})
}
